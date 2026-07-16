# SDK tests

Language suites call `cli.mjs` to prepare stages and validate JSON output with
`protocol.mjs`. Put portable cases and model profiles in `tests.json`; keep only
SDK execution and unavoidable input conversion in each language adapter.
Use focused groups such as `conversation`, `tool_use`, or `structured_output`
when several shared cases exercise the same behavior area.

Node.js must be on `PATH` when running the Go or Rust SDK tests. Run each SDK's
normal test command; no separate shared-test build step is required.

CI generates Cobertura coverage during each SDK test run and uploads it to
GitHub Code Quality; no separate coverage test command is required.
