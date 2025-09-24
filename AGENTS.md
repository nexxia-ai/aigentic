# Repository Guidelines

## Project Structure & Module Organization
- Root Go module: `aigentic` (see `go.mod`).
- Core package files at repo root: `agent.go`, `run.go`, `context_manager.go`, etc.
- Packages/directories:
  - `ai/` model/tool interfaces and helpers
  - `memory/` persistent and run/session memory logic
  - `tools/` built-in tool implementations
  - `utils/` utility helpers
  - `etc/` specs and docs - not code
  - `etc/specs/*` contains design notes
  - `run/` agent runtime 
  - `document/` document manipulation 
- Tests live alongside code as `*_test.go` (e.g., `agent_test.go`, `run_test.go`).

## Build, Test, and Development Commands
- Build library: `go build ./...` — compile all packages.
- Run tests: `go test ./...` — execute unit/integration tests.
- Race + coverage: `go test -race -cover ./...` — safety and coverage.
- Lint (if installed): `golangci-lint run` — run linters across the repo.
- Example run (if you add a main): `go run ./cmd/<your-app>`.

## Coding Style & Naming Conventions
- Language: Go 1.21+; format with `gofmt`/`go fmt` before committing.
- Indentation: tabs (default Go style). Line length: prefer ≤ 100 chars.
- Naming: exported identifiers use PascalCase; unexported use camelCase.
- Files: tests end with `_test.go`; examples may use `example_*.go`.
- Imports: standard → external → internal; keep groups separated.
- Prefer small, focused files; keep package boundaries clear (`memory` must not depend on `aigentic`).

## Testing Guidelines
- Framework: standard `testing` package; use `assert` only where already present.
- Place tests near sources (same package). Name tests `TestXxx` and table‑driven where useful.
- Run full suite locally: `go test ./...`. Aim for meaningful coverage of core paths (agent run, memory flows, tracing).
- Avoid network calls in tests; use fakes/stubs.

## Commit & Pull Request Guidelines
- Commits: present‑tense, imperative subject (e.g., "Add memory tools"), small scoped changes.
- Include context in body when behavior changes or APIs shift.
- PRs: describe the change, link issues, note breaking changes, and include test evidence (commands/output). Screenshots for docs/UI where applicable.
- CI must be green (build + tests + lint, if enabled).

## Security & Configuration Tips
- Do not commit secrets; use env vars and `.gitignore`d files.
- Keep test data small and anonymized. Review `memory.json` artifacts before committing.
