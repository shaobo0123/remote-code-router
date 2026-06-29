# Remote Code Router

`remote-code-router` is a CLIProxyAPI dynamic-library plugin that exposes a stable virtual model, then lets the server choose the real upstream model.

## Features

- Registers virtual models through CPA `ModelRegistrar`.
- Routes configured virtual model names to this plugin executor.
- Executes the selected real model through CPA `host.model.*` callbacks.
- Auto-imports CPA `/v1/models` from the management page.
- Persists imported candidates and the current selection in `remote-code-router.state.yaml`.

## Configuration

Minimal plugin config:

```yaml
client_models: [code]
active_candidate: auto
model_aliases:
  - id: code
    display_name: Remote Code Router
    context_length: 128000
    thinking: true
candidates: []
```

Open `Remote Code Router` in CPA. The page reads `/v1/models` once, saves the imported candidates once, then shows a compact list for switching.

## Management Resources

- `GET /v0/resource/plugins/remote-code-router/index.html`
- `GET /v0/resource/plugins/remote-code-router/import.json`
- `GET /v0/resource/plugins/remote-code-router/select.json`
- `GET /v0/resource/plugins/remote-code-router/status.json`

## Build

```bash
go test ./...
make build
```
