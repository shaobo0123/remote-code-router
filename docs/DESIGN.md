# Remote Code Router Design

## Architecture

`remote-code-router` uses these CPA extension points:

```text
ModelRegistrar + ModelRouter + Executor + ManagementAPI + host.model.*
```

Flow:

```text
Client requests model=code
  -> CPA calls model.route
  -> plugin checks configured virtual aliases
  -> plugin returns TargetKind=self
  -> CPA calls plugin executor
  -> executor calls host.model.execute / host.model.execute_stream
  -> CPA provider credentials and upstream transport stay owned by CPA
```

## Configuration

```yaml
client_models: [code, ark-code-latest]
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
fallback:
  enabled: true
```

`active_candidate` can be:

- `auto`: candidates are sorted by `priority` descending.
- a candidate name: that candidate is tried first, then remaining candidates by priority.

Disabled candidates are skipped with:

```yaml
disabled: true
```

## Runtime State

Management selection is persisted in:

```text
<plugin_dir>/remote-code-router.state.yaml
```

This state overrides `active_candidate` from the static config. Removing the state file returns the plugin to the configured default.

## Fallback

Non-stream execution can fallback on configured status codes and network-like errors.

Streaming execution can fallback only before the first downstream chunk is emitted. After output begins, switching upstream models would corrupt the stream contract.

## Management API

Registered routes:

```text
GET  remote-code-router/status
POST remote-code-router/select
GET  index.html
```

The resource route is exposed by CPA under:

```text
/v0/resource/plugins/remote-code-router/index.html
```
