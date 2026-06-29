package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/router-for-me/CLIProxyAPI/v7/sdk/pluginapi"
)

const (
	resourceStatusPath = "status.json"
	resourceSelectPath = "select.json"
	resourceImportPath = "import.json"
	resourceIndexPath  = "index.html"
)

type managementStatus struct {
	Plugin          string             `json:"plugin"`
	Version         string             `json:"version"`
	ActiveCandidate string             `json:"active_candidate"`
	Mode            string             `json:"mode"`
	Models          []ModelAlias       `json:"models"`
	Candidates      []managementTarget `json:"candidates"`
}

type managementTarget struct {
	Name        string `json:"name"`
	Provider    string `json:"provider"`
	Model       string `json:"model"`
	Priority    int    `json:"priority"`
	Description string `json:"description,omitempty"`
	Disabled    bool   `json:"disabled,omitempty"`
	Active      bool   `json:"active"`
}

func (p *remoteCodeRouterPlugin) RegisterManagement(context.Context, pluginapi.ManagementRegistrationRequest) (pluginapi.ManagementRegistrationResponse, error) {
	return pluginapi.ManagementRegistrationResponse{
		Resources: []pluginapi.ResourceRoute{
			{
				Path:        resourceStatusPath,
				Description: "Read remote-code-router status without a management key.",
				Handler:     p,
			},
			{
				Path:        resourceSelectPath,
				Description: "Set the active server-side model candidate from the plugin page.",
				Handler:     p,
			},
			{
				Path:        resourceImportPath,
				Description: "Import CPA model entries from the plugin page.",
				Handler:     p,
			},
			{
				Path:        resourceIndexPath,
				Menu:        "Remote Code Router",
				Description: "Switch the server-side model used by remote-code-router aliases.",
				Handler:     p,
			},
		},
	}, nil
}

func (p *remoteCodeRouterPlugin) HandleManagement(_ context.Context, req pluginapi.ManagementRequest) (pluginapi.ManagementResponse, error) {
	path := strings.Trim(strings.TrimSpace(req.Path), "/")
	method := strings.ToUpper(strings.TrimSpace(req.Method))
	switch {
	case method == http.MethodGet && (path == resourceStatusPath || strings.HasSuffix(path, "/"+resourceStatusPath)):
		return jsonManagementResponse(http.StatusOK, p.managementStatus()), nil
	case method == http.MethodGet && (path == resourceSelectPath || strings.HasSuffix(path, "/"+resourceSelectPath)):
		return p.handleResourceSelectCandidate(req)
	case method == http.MethodGet && (path == resourceImportPath || strings.HasSuffix(path, "/"+resourceImportPath)):
		return p.handleResourceImportCandidate(req)
	case method == http.MethodGet && (path == resourceIndexPath || path == "" || strings.HasSuffix(path, "/"+resourceIndexPath)):
		return htmlManagementResponse(remoteCodeRouterHTML()), nil
	default:
		return jsonManagementResponse(http.StatusNotFound, map[string]any{"error": "route not found"}), nil
	}
}

func (p *remoteCodeRouterPlugin) handleResourceSelectCandidate(req pluginapi.ManagementRequest) (pluginapi.ManagementResponse, error) {
	candidate := firstNonEmpty(req.Query.Get("active_candidate"), req.Query.Get("candidate"))
	if err := p.validateCandidateSelection(candidate); err != nil {
		return jsonManagementResponse(http.StatusBadRequest, map[string]any{"error": err.Error()}), nil
	}
	if err := p.state.setActiveCandidate(candidate); err != nil {
		return pluginapi.ManagementResponse{}, err
	}
	return jsonManagementResponse(http.StatusOK, p.managementStatus()), nil
}

func (p *remoteCodeRouterPlugin) handleResourceImportCandidate(req pluginapi.ManagementRequest) (pluginapi.ManagementResponse, error) {
	raw := strings.TrimSpace(req.Query.Get("candidates"))
	if raw == "" {
		return jsonManagementResponse(http.StatusBadRequest, map[string]any{"error": "candidates are required"}), nil
	}
	var candidates []Candidate
	if err := json.Unmarshal([]byte(raw), &candidates); err != nil {
		return jsonManagementResponse(http.StatusBadRequest, map[string]any{"error": "invalid candidates json"}), nil
	}
	if err := p.state.setImportedCandidates(candidates); err != nil {
		return pluginapi.ManagementResponse{}, err
	}
	return jsonManagementResponse(http.StatusOK, p.managementStatus()), nil
}

func (p *remoteCodeRouterPlugin) validateCandidateSelection(candidate string) error {
	candidate = normalizeActiveCandidate(candidate)
	if candidate == activeCandidateAuto {
		return nil
	}
	for _, item := range p.effectiveCandidates() {
		if item.Disabled {
			continue
		}
		if strings.EqualFold(item.Name, candidate) {
			return nil
		}
	}
	return fmt.Errorf("unknown or disabled candidate %q", candidate)
}

func (p *remoteCodeRouterPlugin) managementStatus() managementStatus {
	active := p.state.activeCandidate(p.cfg.ActiveCandidate)
	targets := make([]managementTarget, 0, len(p.cfg.Candidates)+1)
	targets = append(targets, managementTarget{
		Name:        activeCandidateAuto,
		Provider:    pluginProvider,
		Model:       activeCandidateAuto,
		Description: "Use candidate priority order.",
		Active:      active == activeCandidateAuto,
	})
	for _, candidate := range p.effectiveCandidates() {
		targets = append(targets, managementTarget{
			Name:        candidate.Name,
			Provider:    candidate.Provider,
			Model:       candidate.Model,
			Priority:    candidate.Priority,
			Description: candidate.Description,
			Disabled:    candidate.Disabled,
			Active:      strings.EqualFold(candidate.Name, active),
		})
	}
	mode := "manual"
	if active == activeCandidateAuto {
		mode = activeCandidateAuto
	}
	return managementStatus{
		Plugin:          pluginName,
		Version:         pluginVersion,
		ActiveCandidate: active,
		Mode:            mode,
		Models:          append([]ModelAlias(nil), p.cfg.ModelAliases...),
		Candidates:      targets,
	}
}

func jsonManagementResponse(status int, body any) pluginapi.ManagementResponse {
	raw, _ := json.Marshal(body)
	return pluginapi.ManagementResponse{
		StatusCode: status,
		Headers:    http.Header{"Content-Type": []string{"application/json; charset=utf-8"}},
		Body:       raw,
	}
}

func htmlManagementResponse(body string) pluginapi.ManagementResponse {
	return pluginapi.ManagementResponse{
		StatusCode: http.StatusOK,
		Headers:    http.Header{"Content-Type": []string{"text/html; charset=utf-8"}},
		Body:       []byte(body),
	}
}

func remoteCodeRouterHTML() string {
	return `<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>Remote Code Router</title>
  <style>
    :root { color-scheme: light dark; font-family: Inter, ui-sans-serif, system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif; }
    body { margin: 0; background: #f5f7fa; color: #172033; }
    main { max-width: 1040px; margin: 0 auto; padding: 28px 20px 40px; }
    header { display: flex; align-items: flex-end; justify-content: space-between; gap: 20px; margin-bottom: 18px; }
    h1 { margin: 0; font-size: 26px; font-weight: 720; letter-spacing: 0; }
    .subtle { color: #657085; font-size: 13px; }
    .toolbar { display: flex; align-items: center; gap: 10px; }
    button { border: 1px solid #cad2df; background: #fff; color: #172033; border-radius: 7px; padding: 9px 12px; cursor: pointer; font-weight: 650; }
    button:hover { border-color: #6b7a99; }
    .list { background: #fff; border: 1px solid #dde3ed; border-radius: 8px; overflow: hidden; }
    .row { min-height: 58px; display: grid; grid-template-columns: minmax(160px, 220px) minmax(220px, 1fr) 120px 96px 84px; align-items: center; gap: 12px; padding: 10px 14px; border-top: 1px solid #edf1f7; }
    .row:first-child { border-top: 0; }
    .active { border-color: #177245; box-shadow: inset 0 0 0 1px #177245; }
    .disabled { opacity: .48; }
    .name { font-size: 14px; font-weight: 720; word-break: break-word; }
    .model { color: #3b465a; font-family: ui-monospace, "SFMono-Regular", Consolas, monospace; font-size: 12px; word-break: break-all; }
    .pill { border: 1px solid #d8deea; border-radius: 999px; padding: 4px 8px; font-size: 12px; color: #3b465a; background: #f8fafc; }
    .pick[disabled] { cursor: default; }
    .notice { margin-bottom: 16px; padding: 10px 12px; border-radius: 8px; background: #edf7f0; color: #195233; border: 1px solid #bfe3cb; display: none; }
    @media (max-width: 760px) {
      header { display: block; }
      .toolbar { margin-top: 12px; }
      .row { grid-template-columns: 1fr; align-items: start; }
      .pick { justify-self: start; }
    }
    @media (prefers-color-scheme: dark) {
      body { background: #10141c; color: #f0f3f8; }
      .list, button { background: #171d28; color: #f0f3f8; border-color: #30394a; }
      .row { border-color: #263044; }
      .subtle, .model, .pill { color: #aeb8c9; }
      .pill { background: #111722; border-color: #30394a; }
      .notice { background: #13261c; color: #bde5ca; border-color: #285c3c; }
    }
  </style>
</head>
<body>
  <main>
    <header>
      <div>
        <h1>Remote Code Router</h1>
        <div class="subtle" id="summary">Loading</div>
      </div>
      <div class="toolbar">
        <button id="refresh" type="button">Refresh</button>
      </div>
    </header>
    <div class="notice" id="notice"></div>
    <section class="list" id="list"></section>
  </main>
  <script>
    const selectURL = "/v0/resource/plugins/remote-code-router/select.json";
    const importURL = "/v0/resource/plugins/remote-code-router/import.json";
    const list = document.getElementById("list");
    const summary = document.getElementById("summary");
    const notice = document.getElementById("notice");
    document.getElementById("refresh").addEventListener("click", load);
    async function load() {
      render(await importCPAModels());
    }
    async function importCPAModels() {
      const modelPayload = await fetchJSON("/v1/models");
      const rawModels = Array.isArray(modelPayload.data) ? modelPayload.data : (Array.isArray(modelPayload.models) ? modelPayload.models : []);
      const candidates = [];
      const seen = new Set();
      for (const item of rawModels) {
        const id = String(item.id || item.name || item.model || "").trim();
        if (!id) continue;
        const lower = id.toLowerCase();
        const ownedBy = String(item.owned_by || item.ownedBy || item.provider || item.type || "cpa").trim();
        if (lower === "remote-code-router" || ownedBy.toLowerCase() === "remote-code-router") continue;
        if (seen.has(lower)) continue;
        seen.add(lower);
        candidates.push({
          name: candidateName(id, ownedBy),
          provider: ownedBy || "cpa",
          model: id,
          priority: Math.max(1, 1000 - candidates.length),
          description: item.description || item.display_name || item.displayName || ""
        });
      }
      if (!candidates.length) throw new Error("No CPA models were found to import.");
      const params = new URLSearchParams();
      params.set("candidates", JSON.stringify(candidates));
      const data = await fetchJSON(importURL + "?" + params.toString());
      notice.textContent = "Imported candidates: " + candidates.length + " from /v1/models";
      notice.style.display = "block";
      return data;
    }
    function candidateName(id, provider) {
      const base = String((provider || "cpa") + "-" + id)
        .toLowerCase()
        .replace(/[^a-z0-9._-]+/g, "-")
        .replace(/^-+|-+$/g, "")
        .slice(0, 96);
      return base || "cpa-model";
    }
    async function selectCandidate(name) {
      const data = await fetchJSON(selectURL + "?candidate=" + encodeURIComponent(name));
      notice.textContent = "Active candidate: " + data.active_candidate;
      notice.style.display = "block";
      render(data);
    }
    async function fetchJSON(url) {
      const res = await fetch(url, { headers: { "Accept": "application/json" } });
      let data = null;
      try {
        data = await res.json();
      } catch (err) {
        throw new Error(url + ": invalid JSON response");
      }
      if (res.ok) return data;
      throw new Error(url + ": " + (data.error || data.message || ("HTTP " + res.status)));
    }
    function render(data) {
      const models = Array.isArray(data.models) ? data.models : [];
      const candidates = Array.isArray(data.candidates) ? data.candidates : [];
      summary.textContent = models.map(m => m.id).join(", ") + " -> " + data.active_candidate;
      list.replaceChildren(...candidates.map(candidateRow));
    }
    function candidateRow(item) {
      const row = document.createElement("article");
      row.className = "row" + (item.active ? " active" : "") + (item.disabled ? " disabled" : "");
      row.innerHTML =
        '<div class="name"></div>' +
        '<div class="model"></div>' +
        '<span class="pill"></span>' +
        '<span class="pill"></span>' +
        '<button class="pick" type="button"></button>';
      row.querySelector(".name").textContent = item.name;
      row.querySelector(".model").textContent = item.model;
      const pills = row.querySelectorAll(".pill");
      pills[0].textContent = item.provider;
      pills[1].textContent = "priority " + item.priority;
      const button = row.querySelector("button");
      button.textContent = item.active ? "Active" : "Use";
      button.disabled = item.active || item.disabled;
      button.addEventListener("click", () => selectCandidate(item.name).catch(err => alert(err.message)));
      return row;
    }
    load().catch(err => { summary.textContent = err.message; });
  </script>
</body>
</html>`
}
