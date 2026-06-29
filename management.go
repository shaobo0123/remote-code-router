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
	managementStatusPath = "plugins/remote-code-router/status"
	managementSelectPath = "plugins/remote-code-router/select"
	resourceStatusPath   = "status.json"
	resourceIndexPath    = "index.html"
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

type selectCandidateRequest struct {
	ActiveCandidate string `json:"active_candidate"`
	Candidate       string `json:"candidate"`
}

func (p *remoteCodeRouterPlugin) RegisterManagement(context.Context, pluginapi.ManagementRegistrationRequest) (pluginapi.ManagementRegistrationResponse, error) {
	return pluginapi.ManagementRegistrationResponse{
		Routes: []pluginapi.ManagementRoute{
			{
				Method:      http.MethodGet,
				Path:        managementStatusPath,
				Description: "Read remote-code-router status and candidate list.",
				Handler:     p,
			},
			{
				Method:      http.MethodPost,
				Path:        managementSelectPath,
				Description: "Set the active server-side model candidate.",
				Handler:     p,
			},
		},
		Resources: []pluginapi.ResourceRoute{
			{
				Path:        resourceStatusPath,
				Description: "Read remote-code-router status without a management key.",
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
	case method == http.MethodGet && (path == managementStatusPath || strings.HasSuffix(path, "/"+managementStatusPath)):
		return jsonManagementResponse(http.StatusOK, p.managementStatus()), nil
	case method == http.MethodPost && (path == managementSelectPath || strings.HasSuffix(path, "/"+managementSelectPath)):
		return p.handleSelectCandidate(req.Body)
	case method == http.MethodGet && (path == resourceStatusPath || strings.HasSuffix(path, "/"+resourceStatusPath)):
		return jsonManagementResponse(http.StatusOK, p.managementStatus()), nil
	case method == http.MethodGet && (path == resourceIndexPath || path == "" || strings.HasSuffix(path, "/"+resourceIndexPath)):
		return htmlManagementResponse(remoteCodeRouterHTML()), nil
	default:
		return jsonManagementResponse(http.StatusNotFound, map[string]any{"error": "route not found"}), nil
	}
}

func (p *remoteCodeRouterPlugin) handleSelectCandidate(body []byte) (pluginapi.ManagementResponse, error) {
	var req selectCandidateRequest
	if len(body) > 0 {
		if err := json.Unmarshal(body, &req); err != nil {
			return jsonManagementResponse(http.StatusBadRequest, map[string]any{"error": "invalid json body"}), nil
		}
	}
	candidate := firstNonEmpty(req.ActiveCandidate, req.Candidate)
	if err := p.validateCandidateSelection(candidate); err != nil {
		return jsonManagementResponse(http.StatusBadRequest, map[string]any{"error": err.Error()}), nil
	}
	if err := p.state.setActiveCandidate(candidate); err != nil {
		return pluginapi.ManagementResponse{}, err
	}
	return jsonManagementResponse(http.StatusOK, p.managementStatus()), nil
}

func (p *remoteCodeRouterPlugin) validateCandidateSelection(candidate string) error {
	candidate = normalizeActiveCandidate(candidate)
	if candidate == activeCandidateAuto {
		return nil
	}
	for _, item := range p.cfg.Candidates {
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
		Description: "Use candidate priority and fallback rules.",
		Active:      active == activeCandidateAuto,
	})
	for _, candidate := range p.cfg.Candidates {
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
    header { display: flex; align-items: flex-end; justify-content: space-between; gap: 20px; margin-bottom: 22px; }
    h1 { margin: 0; font-size: 26px; font-weight: 720; letter-spacing: 0; }
    .subtle { color: #657085; font-size: 13px; }
    .toolbar { display: flex; align-items: center; gap: 10px; }
    input { border: 1px solid #cad2df; background: #fff; color: #172033; border-radius: 7px; padding: 9px 10px; min-width: 190px; }
    button { border: 1px solid #cad2df; background: #fff; color: #172033; border-radius: 7px; padding: 9px 12px; cursor: pointer; font-weight: 650; }
    button:hover { border-color: #6b7a99; }
    .grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(240px, 1fr)); gap: 12px; }
    .card { background: #fff; border: 1px solid #dde3ed; border-radius: 8px; padding: 16px; min-height: 150px; display: flex; flex-direction: column; gap: 10px; }
    .active { border-color: #177245; box-shadow: inset 0 0 0 1px #177245; }
    .disabled { opacity: .48; }
    .name { font-size: 17px; font-weight: 720; word-break: break-word; }
    .model { color: #3b465a; font-family: ui-monospace, "SFMono-Regular", Consolas, monospace; font-size: 12px; word-break: break-all; }
    .meta { display: flex; flex-wrap: wrap; gap: 6px; margin-top: auto; }
    .pill { border: 1px solid #d8deea; border-radius: 999px; padding: 4px 8px; font-size: 12px; color: #3b465a; background: #f8fafc; }
    .pick { margin-top: 4px; align-self: flex-start; }
    .pick[disabled] { cursor: default; }
    .notice { margin-bottom: 16px; padding: 10px 12px; border-radius: 8px; background: #edf7f0; color: #195233; border: 1px solid #bfe3cb; display: none; }
    @media (prefers-color-scheme: dark) {
      body { background: #10141c; color: #f0f3f8; }
      .card, button, input { background: #171d28; color: #f0f3f8; border-color: #30394a; }
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
        <input id="managementKey" type="password" placeholder="Management key">
        <button id="refresh" type="button">Refresh</button>
      </div>
    </header>
    <div class="notice" id="notice"></div>
    <section class="grid" id="grid"></section>
  </main>
  <script>
    const statusURLs = [
      "/v0/resource/plugins/remote-code-router/status.json",
      "/v0/management/plugins/remote-code-router/status",
      "/v0/management/remote-code-router/status"
    ];
    const selectURLs = [
      "/v0/management/plugins/remote-code-router/select",
      "/v0/management/remote-code-router/select"
    ];
    const grid = document.getElementById("grid");
    const summary = document.getElementById("summary");
    const notice = document.getElementById("notice");
    const managementKey = document.getElementById("managementKey");
    managementKey.value = localStorage.getItem("remoteCodeRouterManagementKey") || "";
    managementKey.addEventListener("change", () => {
      localStorage.setItem("remoteCodeRouterManagementKey", managementKey.value);
    });
    document.getElementById("refresh").addEventListener("click", load);
    async function load() {
      const data = await fetchJSON(statusURLs);
      render(data);
    }
    async function selectCandidate(name) {
      const key = managementKey.value.trim();
      if (!key) {
        alert("Enter the management key before switching candidates.");
        managementKey.focus();
        return;
      }
      localStorage.setItem("remoteCodeRouterManagementKey", key);
      const data = await fetchJSON(selectURLs, {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          "Accept": "application/json",
          "X-Management-Key": key
        },
        body: JSON.stringify({ active_candidate: name })
      });
      notice.textContent = "Active candidate: " + data.active_candidate;
      notice.style.display = "block";
      render(data);
    }
    async function fetchJSON(urls, options) {
      let lastError = "";
      for (const url of urls) {
        const res = await fetch(url, options || { headers: { "Accept": "application/json" } });
        let data = null;
        try {
          data = await res.json();
        } catch (err) {
          lastError = url + ": invalid JSON response";
          continue;
        }
        if (res.ok && Array.isArray(data.models) && Array.isArray(data.candidates)) return data;
        lastError = url + ": " + (data.error || data.message || ("HTTP " + res.status));
      }
      throw new Error(lastError || "Remote Code Router status API is unavailable");
    }
    function render(data) {
      const models = Array.isArray(data.models) ? data.models : [];
      const candidates = Array.isArray(data.candidates) ? data.candidates : [];
      summary.textContent = models.map(m => m.id).join(", ") + " -> " + data.active_candidate;
      grid.replaceChildren(...candidates.map(candidateCard));
    }
    function candidateCard(item) {
      const card = document.createElement("article");
      card.className = "card" + (item.active ? " active" : "") + (item.disabled ? " disabled" : "");
      card.innerHTML =
        '<div class="name"></div>' +
        '<div class="model"></div>' +
        '<div class="subtle"></div>' +
        '<div class="meta">' +
        '<span class="pill"></span>' +
        '<span class="pill"></span>' +
        '</div>' +
        '<button class="pick" type="button"></button>';
      card.querySelector(".name").textContent = item.name;
      card.querySelector(".model").textContent = item.model;
      card.querySelector(".subtle").textContent = item.description || "";
      const pills = card.querySelectorAll(".pill");
      pills[0].textContent = item.provider;
      pills[1].textContent = "priority " + item.priority;
      const button = card.querySelector("button");
      button.textContent = item.active ? "Active" : "Use";
      button.disabled = item.active || item.disabled;
      button.addEventListener("click", () => selectCandidate(item.name).catch(err => alert(err.message)));
      return card;
    }
    load().catch(err => { summary.textContent = err.message; });
  </script>
</body>
</html>`
}
