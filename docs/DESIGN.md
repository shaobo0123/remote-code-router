# Remote Code Router Design

## Flow

```text
Client requests model=code
  -> CPA calls model.route
  -> plugin routes configured virtual aliases to itself
  -> executor calls host.model.execute or host.model.execute_stream
```

The management page imports `/v1/models`, saves candidates to plugin state, and lets the operator choose one active candidate.

## State

```text
<plugin_dir>/remote-code-router.state.yaml
```
