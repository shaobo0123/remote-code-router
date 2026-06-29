package main

import (
	"os"
	"path/filepath"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"
)

const stateFileName = "remote-code-router.state.yaml"

type routerState struct {
	ActiveCandidate    string      `yaml:"active_candidate" json:"active_candidate"`
	ImportedCandidates []Candidate `yaml:"imported_candidates,omitempty" json:"imported_candidates,omitempty"`
}

type stateStore struct {
	mu   sync.RWMutex
	path string
	data routerState
}

func newStateStore(pluginDir string, cfg PluginConfig) *stateStore {
	path := statePath(pluginDir)
	store := &stateStore{
		path: path,
		data: routerState{ActiveCandidate: cfg.ActiveCandidate},
	}
	if raw, err := os.ReadFile(path); err == nil && len(raw) > 0 {
		var loaded routerState
		if yaml.Unmarshal(raw, &loaded) == nil {
			if strings.TrimSpace(loaded.ActiveCandidate) != "" {
				store.data.ActiveCandidate = strings.ToLower(strings.TrimSpace(loaded.ActiveCandidate))
			}
			store.data.ImportedCandidates = normalizeStateCandidates(loaded.ImportedCandidates)
		}
	}
	return store
}

func statePath(pluginDir string) string {
	pluginDir = strings.TrimSpace(pluginDir)
	if pluginDir == "" {
		if cwd, err := os.Getwd(); err == nil {
			pluginDir = cwd
		}
	}
	if pluginDir == "" {
		return stateFileName
	}
	return filepath.Join(pluginDir, stateFileName)
}

func (s *stateStore) activeCandidate(defaultCandidate string) string {
	if s == nil {
		return normalizeActiveCandidate(defaultCandidate)
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	if strings.TrimSpace(s.data.ActiveCandidate) == "" {
		return normalizeActiveCandidate(defaultCandidate)
	}
	return normalizeActiveCandidate(s.data.ActiveCandidate)
}

func (s *stateStore) setActiveCandidate(candidate string) error {
	if s == nil {
		return nil
	}
	candidate = normalizeActiveCandidate(candidate)
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data.ActiveCandidate = candidate
	if dir := filepath.Dir(s.path); dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	return s.saveLocked()
}

func (s *stateStore) importedCandidates() []Candidate {
	if s == nil {
		return nil
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return cloneCandidates(s.data.ImportedCandidates)
}

func (s *stateStore) setImportedCandidates(candidates []Candidate) error {
	if s == nil {
		return nil
	}
	normalized := normalizeStateCandidates(candidates)
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data.ImportedCandidates = normalized
	return s.saveLocked()
}

func (s *stateStore) saveLocked() error {
	if dir := filepath.Dir(s.path); dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	raw, err := yaml.Marshal(s.data)
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, raw, 0o600)
}

func normalizeActiveCandidate(candidate string) string {
	candidate = strings.ToLower(strings.TrimSpace(candidate))
	if candidate == "" {
		return activeCandidateAuto
	}
	return candidate
}

func normalizeStateCandidates(candidates []Candidate) []Candidate {
	out := make([]Candidate, 0, len(candidates))
	seen := make(map[string]struct{}, len(candidates))
	for i, candidate := range candidates {
		candidate.Name = strings.ToLower(strings.TrimSpace(candidate.Name))
		candidate.Provider = strings.ToLower(strings.TrimSpace(candidate.Provider))
		candidate.Model = strings.TrimSpace(candidate.Model)
		candidate.Description = strings.TrimSpace(candidate.Description)
		candidate.Order = i
		if candidate.Name == "" || candidate.Model == "" {
			continue
		}
		if candidate.Provider == "" {
			candidate.Provider = "cpa"
		}
		if _, ok := seen[candidate.Name]; ok {
			continue
		}
		seen[candidate.Name] = struct{}{}
		out = append(out, candidate)
	}
	return out
}
