package main

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/router-for-me/CLIProxyAPI/v7/sdk/pluginapi"
)

func testConfigYAML() []byte {
	return []byte(`
client_models: [code, ark-code-latest]
active_candidate: doubao-code
model_aliases:
  - id: code
    display_name: Remote Code
    name: code
    description: Server-side model alias
    context_length: 128000
    max_completion_tokens: 8192
    thinking: true
candidates:
  - name: doubao-code
    provider: ark
    model: doubao-seed-2.0-code
    priority: 100
  - name: deepseek
    provider: deepseek
    model: deepseek-v4-pro
    priority: 90
  - name: glm
    provider: zhipu
    model: glm-5.2
    priority: 80
`)
}

func newTestPlugin(t *testing.T) *remoteCodeRouterPlugin {
	t.Helper()
	plugin, err := buildPlugin(testConfigYAML(), t.TempDir())
	if err != nil {
		t.Fatalf("buildPlugin() error = %v", err)
	}
	p, ok := plugin.Capabilities.ModelRouter.(*remoteCodeRouterPlugin)
	if !ok || p == nil {
		t.Fatalf("plugin did not expose router")
	}
	return p
}

func TestRouteModelHandlesConfiguredAlias(t *testing.T) {
	p := newTestPlugin(t)
	resp := p.routeModel(pluginapi.ModelRouteRequest{
		RequestedModel:     "code",
		AvailableProviders: []string{"ark", "deepseek"},
	})
	if !resp.Handled {
		t.Fatalf("expected route to be handled")
	}
	plan, ok := p.plans.consume(routePlanKeyFromRouteRequest(pluginapi.ModelRouteRequest{RequestedModel: "code"}))
	if !ok {
		t.Fatalf("expected stored route plan")
	}
	if got := plan.Candidates[0].Name; got != "doubao-code" {
		t.Fatalf("first candidate = %q, want doubao-code", got)
	}
	if len(plan.Candidates) != 2 {
		t.Fatalf("candidate count = %d, want 2 after provider filtering", len(plan.Candidates))
	}
}

func TestRouteModelIgnoresUnknownAliasAndBypass(t *testing.T) {
	p := newTestPlugin(t)
	if resp := p.routeModel(pluginapi.ModelRouteRequest{RequestedModel: "other"}); resp.Handled {
		t.Fatalf("unexpected handling for unknown model")
	}
	if resp := p.routeModel(pluginapi.ModelRouteRequest{
		RequestedModel: "code",
		Headers:        http.Header{bypassHeaderName: []string{"1"}},
	}); resp.Handled {
		t.Fatalf("unexpected handling for bypass request")
	}
}

func TestAutoSortsByPriority(t *testing.T) {
	cfg, err := parseRemoteCodeRouterConfig([]byte(`
model_aliases:
  - id: code
client_models: [code]
active_candidate: auto
candidates:
  - name: slow
    provider: a
    model: slow
    priority: 10
  - name: fast
    provider: b
    model: fast
    priority: 30
`))
	if err != nil {
		t.Fatalf("parse config: %v", err)
	}
	sorted := sortCandidates(cfg.Candidates, cfg.ActiveCandidate)
	if got := sorted[0].Name; got != "fast" {
		t.Fatalf("first candidate = %q, want fast", got)
	}
}

func TestManualSelectionTakesPrecedence(t *testing.T) {
	cfg, err := parseRemoteCodeRouterConfig([]byte(`
model_aliases:
  - id: code
client_models: [code]
active_candidate: slow
candidates:
  - name: fast
    provider: b
    model: fast
    priority: 30
  - name: slow
    provider: a
    model: slow
    priority: 10
`))
	if err != nil {
		t.Fatalf("parse config: %v", err)
	}
	sorted := sortCandidates(cfg.Candidates, cfg.ActiveCandidate)
	if got := sorted[0].Name; got != "slow" {
		t.Fatalf("first candidate = %q, want slow", got)
	}
}

func TestResourceImportAndSelectWithoutManagementKey(t *testing.T) {
	plugin, err := buildPlugin([]byte(`
client_models: [code]
active_candidate: auto
model_aliases:
  - id: code
`), t.TempDir())
	if err != nil {
		t.Fatalf("buildPlugin() error = %v", err)
	}
	p := plugin.Capabilities.ModelRouter.(*remoteCodeRouterPlugin)
	resp, err := p.HandleManagement(context.Background(), pluginapi.ManagementRequest{
		Method: http.MethodGet,
		Path:   "/v0/resource/plugins/remote-code-router/import.json",
		Query: map[string][]string{
			"candidates": {`[{"name":"cpa-gpt-5","provider":"cpa","model":"gpt-5","priority":100}]`},
		},
	})
	if err != nil {
		t.Fatalf("resource import error = %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("import status = %d, body=%s", resp.StatusCode, string(resp.Body))
	}
	resp, err = p.HandleManagement(context.Background(), pluginapi.ManagementRequest{
		Method: http.MethodGet,
		Path:   "/v0/resource/plugins/remote-code-router/select.json",
		Query: map[string][]string{
			"candidate": {"cpa-gpt-5"},
		},
	})
	if err != nil {
		t.Fatalf("resource select error = %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("select status = %d, body=%s", resp.StatusCode, string(resp.Body))
	}
	plan, ok := p.buildRoutePlan(pluginapi.ModelRouteRequest{RequestedModel: "code"}, false)
	if !ok {
		t.Fatalf("expected route plan")
	}
	if got := plan.Candidates[0].Model; got != "gpt-5" {
		t.Fatalf("first candidate model = %q, want gpt-5", got)
	}
}

func TestResourceStatusReturnsAliases(t *testing.T) {
	p := newTestPlugin(t)
	resp, err := p.HandleManagement(context.Background(), pluginapi.ManagementRequest{
		Method: http.MethodGet,
		Path:   "/v0/resource/plugins/remote-code-router/status.json",
	})
	if err != nil {
		t.Fatalf("HandleManagement() error = %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, body=%s", resp.StatusCode, string(resp.Body))
	}
	var status managementStatus
	if err := json.Unmarshal(resp.Body, &status); err != nil {
		t.Fatalf("decode status: %v", err)
	}
	if len(status.Models) != 1 || len(status.Candidates) == 0 {
		t.Fatalf("unexpected status: %+v", status)
	}
}

func TestRegisterModelsReturnsAliases(t *testing.T) {
	p := newTestPlugin(t)
	resp, err := p.RegisterModels(context.Background(), pluginapi.ModelRegistrationRequest{})
	if err != nil {
		t.Fatalf("RegisterModels() error = %v", err)
	}
	if resp.Provider != pluginProvider {
		t.Fatalf("provider = %q, want %q", resp.Provider, pluginProvider)
	}
	if len(resp.Models) != 1 {
		t.Fatalf("models len = %d, want 1", len(resp.Models))
	}
	if resp.Models[0].ID != "code" || resp.Models[0].Thinking == nil {
		t.Fatalf("unexpected model metadata: %+v", resp.Models[0])
	}
}
