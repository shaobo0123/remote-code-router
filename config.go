package main

import (
	"fmt"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	bypassHeaderName = "X-CPA-Remote-Code-Router-Bypass"

	activeCandidateAuto = "auto"
)

var defaultFallbackOnStatus = []int{401, 403, 408, 409, 429, 500, 502, 503, 504}
var defaultNoFallbackOnStatus = []int{400, 404, 422}

type PluginConfig struct {
	ClientModels    []string       `yaml:"client_models"`
	ModelAliases    []ModelAlias   `yaml:"model_aliases"`
	ActiveCandidate string         `yaml:"active_candidate"`
	Candidates      []Candidate    `yaml:"candidates"`
	Fallback        FallbackConfig `yaml:"fallback"`
}

type ModelAlias struct {
	ID                  string   `yaml:"id"`
	DisplayName         string   `yaml:"display_name"`
	Name                string   `yaml:"name"`
	Description         string   `yaml:"description"`
	OwnedBy             string   `yaml:"owned_by"`
	Type                string   `yaml:"type"`
	ContextLength       int64    `yaml:"context_length"`
	InputTokenLimit     int64    `yaml:"input_token_limit"`
	OutputTokenLimit    int64    `yaml:"output_token_limit"`
	MaxCompletionTokens int64    `yaml:"max_completion_tokens"`
	Thinking            bool     `yaml:"thinking"`
	SupportedParameters []string `yaml:"supported_parameters"`
}

type Candidate struct {
	Name        string `yaml:"name"`
	Provider    string `yaml:"provider"`
	Model       string `yaml:"model"`
	Priority    int    `yaml:"priority"`
	Description string `yaml:"description"`
	Disabled    bool   `yaml:"disabled"`
	Order       int    `yaml:"-"`
}

type FallbackConfig struct {
	Enabled                            bool  `yaml:"enabled"`
	FallbackOnStatus                   []int `yaml:"fallback_on_status"`
	NoFallbackOnStatus                 []int `yaml:"no_fallback_on_status"`
	StreamFallbackBeforeFirstChunkOnly bool  `yaml:"stream_fallback_before_first_chunk_only"`
}

func defaultPluginConfig() PluginConfig {
	return PluginConfig{
		ClientModels:    []string{"code"},
		ActiveCandidate: activeCandidateAuto,
		ModelAliases: []ModelAlias{
			{
				ID:                  "code",
				DisplayName:         "Remote Code Router",
				Name:                "code",
				Description:         "Server-side switchable code model alias.",
				OwnedBy:             pluginProvider,
				Type:                "chat",
				ContextLength:       128000,
				MaxCompletionTokens: 8192,
				Thinking:            true,
				SupportedParameters: []string{"temperature", "top_p", "max_tokens", "stream"},
			},
		},
		Fallback: FallbackConfig{
			Enabled:                            true,
			FallbackOnStatus:                   append([]int(nil), defaultFallbackOnStatus...),
			NoFallbackOnStatus:                 append([]int(nil), defaultNoFallbackOnStatus...),
			StreamFallbackBeforeFirstChunkOnly: true,
		},
	}
}

func parseRemoteCodeRouterConfig(raw []byte) (PluginConfig, error) {
	return parseRemoteCodeRouterConfigWithOptions(raw, false)
}

func parseRemoteCodeRouterRegistrationConfig(raw []byte) (PluginConfig, error) {
	return parseRemoteCodeRouterConfigWithOptions(raw, true)
}

func parseRemoteCodeRouterConfigWithOptions(raw []byte, allowMissingCandidates bool) (PluginConfig, error) {
	cfg := defaultPluginConfig()
	if strings.TrimSpace(string(raw)) != "" {
		if err := yaml.Unmarshal(raw, &cfg); err != nil {
			return PluginConfig{}, fmt.Errorf("invalid %s config: %w", pluginName, err)
		}
	}
	normalizeConfig(&cfg)
	if err := validateConfig(cfg, allowMissingCandidates); err != nil {
		return PluginConfig{}, err
	}
	return cfg, nil
}

func normalizeConfig(cfg *PluginConfig) {
	cfg.ClientModels = trimStringList(cfg.ClientModels, false)
	cfg.ActiveCandidate = strings.ToLower(strings.TrimSpace(cfg.ActiveCandidate))
	if cfg.ActiveCandidate == "" {
		cfg.ActiveCandidate = activeCandidateAuto
	}
	for i := range cfg.ModelAliases {
		alias := &cfg.ModelAliases[i]
		alias.ID = strings.TrimSpace(alias.ID)
		alias.Name = strings.TrimSpace(alias.Name)
		alias.DisplayName = strings.TrimSpace(alias.DisplayName)
		alias.Description = strings.TrimSpace(alias.Description)
		alias.OwnedBy = strings.TrimSpace(alias.OwnedBy)
		alias.Type = strings.TrimSpace(alias.Type)
		alias.SupportedParameters = trimStringList(alias.SupportedParameters, false)
		if alias.Name == "" {
			alias.Name = alias.ID
		}
		if alias.DisplayName == "" {
			alias.DisplayName = alias.ID
		}
		if alias.OwnedBy == "" {
			alias.OwnedBy = pluginProvider
		}
		if alias.Type == "" {
			alias.Type = "chat"
		}
	}
	if len(cfg.ClientModels) == 0 {
		for _, alias := range cfg.ModelAliases {
			if alias.ID != "" {
				cfg.ClientModels = append(cfg.ClientModels, alias.ID)
			}
		}
	}
	for i := range cfg.Candidates {
		candidate := &cfg.Candidates[i]
		candidate.Name = strings.ToLower(strings.TrimSpace(candidate.Name))
		candidate.Provider = strings.ToLower(strings.TrimSpace(candidate.Provider))
		candidate.Model = strings.TrimSpace(candidate.Model)
		candidate.Description = strings.TrimSpace(candidate.Description)
		candidate.Order = i
	}
}

func validateConfig(cfg PluginConfig, allowMissingCandidates bool) error {
	if len(cfg.ClientModels) == 0 {
		return fmt.Errorf("%s config requires at least one client_models or model_aliases entry", pluginName)
	}
	if len(cfg.ModelAliases) == 0 {
		return fmt.Errorf("%s config requires at least one model_aliases entry", pluginName)
	}
	seenAliases := make(map[string]struct{}, len(cfg.ModelAliases))
	for i, alias := range cfg.ModelAliases {
		if alias.ID == "" {
			return fmt.Errorf("%s model_aliases[%d] requires id", pluginName, i)
		}
		key := strings.ToLower(alias.ID)
		if _, ok := seenAliases[key]; ok {
			return fmt.Errorf("%s has duplicate model alias %q", pluginName, alias.ID)
		}
		seenAliases[key] = struct{}{}
	}
	if len(cfg.Candidates) == 0 {
		if allowMissingCandidates {
			return validateFallbackConfig(cfg.Fallback)
		}
		return fmt.Errorf("%s config requires at least one candidates entry", pluginName)
	}
	seenCandidates := make(map[string]struct{}, len(cfg.Candidates))
	for i, candidate := range cfg.Candidates {
		prefix := fmt.Sprintf("%s candidates[%d]", pluginName, i)
		if candidate.Name == "" {
			return fmt.Errorf("%s requires name", prefix)
		}
		if _, exists := seenCandidates[candidate.Name]; exists {
			return fmt.Errorf("%s has duplicate candidate name %q", pluginName, candidate.Name)
		}
		seenCandidates[candidate.Name] = struct{}{}
		if candidate.Provider == "" {
			return fmt.Errorf("%s requires provider", prefix)
		}
		if candidate.Model == "" {
			return fmt.Errorf("%s requires model", prefix)
		}
	}
	if cfg.ActiveCandidate != activeCandidateAuto {
		if _, ok := seenCandidates[cfg.ActiveCandidate]; !ok && !allowMissingCandidates {
			return fmt.Errorf("%s active_candidate %q does not match any candidate", pluginName, cfg.ActiveCandidate)
		}
	}
	return validateFallbackConfig(cfg.Fallback)
}

func validateFallbackConfig(fallback FallbackConfig) error {
	if err := validateStatusCodes("fallback_on_status", fallback.FallbackOnStatus); err != nil {
		return err
	}
	if err := validateStatusCodes("no_fallback_on_status", fallback.NoFallbackOnStatus); err != nil {
		return err
	}
	return nil
}

func validateStatusCodes(field string, codes []int) error {
	for _, code := range codes {
		if code < 100 || code > 599 {
			return fmt.Errorf("%s config %s contains invalid HTTP status %d", pluginName, field, code)
		}
	}
	return nil
}

func trimStringList(input []string, lower bool) []string {
	out := make([]string, 0, len(input))
	for _, item := range input {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		if lower {
			item = strings.ToLower(item)
		}
		out = append(out, item)
	}
	return out
}

func sortCandidates(input []Candidate, active string) []Candidate {
	active = strings.ToLower(strings.TrimSpace(active))
	candidates := make([]Candidate, 0, len(input))
	for i, candidate := range input {
		if candidate.Disabled {
			continue
		}
		candidate.Order = i
		candidates = append(candidates, candidate)
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		a := candidates[i]
		b := candidates[j]
		if active != "" && active != activeCandidateAuto {
			if a.Name == active && b.Name != active {
				return true
			}
			if b.Name == active && a.Name != active {
				return false
			}
		}
		if a.Priority != b.Priority {
			return a.Priority > b.Priority
		}
		return a.Order < b.Order
	})
	return candidates
}

func normalizeProtocol(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "anthropic":
		return "claude"
	case "responses", "openai-responses", "openai_responses":
		return "openai-response"
	case "chat-completions", "chat_completions", "openai-chat-completions", "openai_chat_completions":
		return "openai"
	default:
		return strings.ToLower(strings.TrimSpace(raw))
	}
}
