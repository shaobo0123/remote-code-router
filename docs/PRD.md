# Remote Code Router PRD

## Goal

Build a CPA plugin that lets clients request a stable model name while the server operator switches the real upstream model without changing client configuration.

Primary user story:

```text
As a CPA server operator,
I want clients to request model=code,
so that I can switch between Ark, DeepSeek, GLM, Kimi, or other providers on the server.
```

## Requirements

- Register virtual models such as `code` and `ark-code-latest`.
- Route only configured virtual model names.
- Use CPA host model callbacks for execution.
- Never read or persist provider secrets.
- Support manual candidate selection.
- Support `auto` priority routing.
- Provide a simple Management UI for switching candidates.
- Persist runtime selection under the plugin directory.
- Support fallback before streaming output starts.

## Non-Goals

- Implementing an Ark provider from scratch.
- Reading API keys or OAuth token files.
- Switching models after stream output has already begun.
- Rewriting client request formats.

## Success Criteria

- `go test ./...` passes.
- The plugin registers model metadata through `ModelRegistrar`.
- `POST /v0/management/remote-code-router/select` changes subsequent route plans.
- The executor uses `host.model.execute` and `host.model.execute_stream` instead of direct upstream HTTP calls.
