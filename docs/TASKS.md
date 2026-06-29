# Remote Code Router Tasks

## Done

- Forked the CPA dynamic library plugin shape from `cpa-plugin-priority-auto-router`.
- Renamed plugin ID, dynamic library exports, registry metadata, and Makefile output to `remote-code-router`.
- Replaced client-profile routing with virtual model alias routing.
- Added server-side active candidate state.
- Added `ModelRegistrar` metadata for virtual models.
- Added Management API routes and a static switching page.
- Added focused unit tests for routing, sorting, management selection, and model registration.

## Next

- Verify `make build` on a machine with a C compiler in `PATH`.
- Install the built library into CPA with config key `remote-code-router`.
- Confirm the provider IDs in `config.example.yaml` match local CPA provider IDs.
- Extend candidate metadata if the management UI should show cost, latency, or health.
