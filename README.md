# Remote Code Router

`remote-code-router` is a CLIProxyAPI (CPA) dynamic-library plugin that exposes stable virtual code models to clients while the CPA server chooses the real upstream provider/model.

Clients can keep requesting `code` or `ark-code-latest`; the server can switch the active target to `doubao-seed-2.0-code`, `deepseek-v4-pro`, `glm-5.2`, `kimi-k2.7-code`, or any configured CPA provider model.

## Features

- Registers user-facing virtual models through CPA `ModelRegistrar`.
- Routes matching model aliases to the plugin executor with `ModelRouter`.
- Executes real upstream models through CPA `host.model.*` callbacks, so credentials stay owned by CPA providers.
- Supports `active_candidate: auto` priority routing and manual server-side selection.
- Persists management-page selection to `remote-code-router.state.yaml` under the plugin directory.
- Supports non-stream fallback and stream fallback before the first emitted chunk.
- Provides a Management API page for switching the active candidate.

## Configuration

Install the compiled dynamic library as the CPA plugin ID/config key `remote-code-router`, then configure it with YAML like `config.example.yaml`.

Minimal shape:

```yaml
client_models: [code]
active_candidate: auto
model_aliases:
  - id: code
    display_name: Remote Code Router
    context_length: 128000
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
fallback:
  enabled: true
```

Set `active_candidate` to `auto` to use priority order, or to a candidate name such as `doubao-code` to pin the first target. If the pinned target fails with a fallback status, the plugin tries the remaining enabled candidates by priority.

## Management

The plugin registers:

- `GET /v0/management/remote-code-router/status`
- `POST /v0/management/remote-code-router/select`
- `GET /v0/resource/plugins/remote-code-router/index.html`

POST body:

```json
{"active_candidate":"deepseek"}
```

Use `auto` to return to priority routing.

## Building

Requirements:

- Go matching the module version in `go.mod`.
- A C compiler available in `PATH`, because CPA dynamic libraries use `go build -buildmode=c-shared`.

Build for the host platform:

```bash
make build
```

Expected outputs:

- Linux/FreeBSD: `remote-code-router.so`
- macOS: `remote-code-router.dylib`
- Windows: `remote-code-router.dll`

Run local checks:

```bash
go test ./...
go vet ./...
```

## Notes

- The plugin does not read API keys, OAuth files, or provider secrets.
- Streaming fallback is only safe before the first downstream chunk is emitted.
- The `provider` values in `candidates` must match provider IDs known to CPA, for example `ark`, `deepseek`, `zhipu`, or `moonshot`.
