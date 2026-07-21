# SDK tests

Language suites call `cli.mjs` to prepare stages and validate JSON output with
`protocol.ts`. Put portable cases and model profiles in `tests.json`; keep only
SDK execution and unavoidable input conversion in each language adapter.
Use focused groups such as `conversation`, `tool_use`, or `structured_output`
when several shared cases exercise the same behavior area.

Node.js must be on `PATH` when running the Go or Rust SDK tests. Run each SDK's
normal test command; no separate shared-test build step is required.

CI generates Cobertura coverage during each SDK test run and uploads it to
GitHub Code Quality; no separate coverage test command is required.

## Transport tests

Provider wire fixtures live in `transports.json`. Each fixture defines the SDK
input, expected request subset, raw JSON or SSE response, and normalized output
expectations. `transport-server.mjs` replays the response and validates the
captured request. Language suites should only register a transport group and
provide a model factory configured with the replay server URL.

Keep provider HTTP shapes in these central fixtures instead of duplicating them
across JS, Go, and Rust transport tests. Use `tests.json` for live portable
behavior and `transports.json` for deterministic provider protocol coverage.
