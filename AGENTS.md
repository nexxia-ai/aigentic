# Repository Guidelines

## Package API Notes
- Create models with `ai.New(<model identifier>, apiKey)` after importing provider modules (e.g., `_ "github.com/nexxia-ai/aigentic-openai"`); identifiers are listed via `ai.Models()`.
- Agent tools use the `run.AgentTool` type and `run.NewTool` helper; built-in tools in `tools/` return `run.AgentTool`.
- Documents come from the `document` package and are passed as `[]*document.Document` via `Agent.Documents` or `Agent.DocumentReferences`.

## Project Structure & Module Organization
- Root Go module: `aigentic` (see `go.mod`).
- Core package files at repo root: `agent.go`, `interface.go`, etc.
- Packages/directories:
  - `ai/` model/tool interfaces and helpers
  - `run/` agent runtime execution engine - contains `AgentRun`, events, tools, context management, conversation history, interceptors, tracing, and execution orchestration
  - `tools/` built-in tool implementations
  - `utils/` utility helpers
  - `document/` document manipulation, processing pipelines, and storage - contains `Document`, `Pipeline`, `Store`, and related types
- Tests live alongside code as `*_test.go` (e.g., `agent_test.go`, `run/run_test.go`).

### The `run` Package

The `run` package (`github.com/nexxia-ai/aigentic/run`) provides the agent runtime execution engine. It contains:

- **AgentRun** (`run.go`) - Main execution runtime type that orchestrates agent execution, handles LLM calls, tool execution, and event streaming
- **Events** (`event.go`) - Event types for execution lifecycle: `ContentEvent`, `ToolEvent`, `ThinkingEvent`, `ErrorEvent`, `ApprovalEvent`, `LLMCallEvent`, `EvalEvent`, etc.
- **AgentTool** (`agent_tool.go`) - Tool definition type and `NewTool()` helper for creating type-safe tools
- **AgentContext** (`context.go`) - Context management for agent state, messages, memories, and documents
- **ContextManager** (`context_manager.go`) - Interface for custom context management implementations
- **ContextFunction** (`context_function.go`) - Function type for dynamic context injection
- **ConversationHistory** (`conversation_history.go`) - Conversation tracking across multiple agent runs
- **ConversationTurn** (`conversation_turn.go`) - Individual conversation turn representation
- **Interceptor** (`interceptor.go`) - Interface for intercepting and modifying LLM calls
- **Tracer** (`trace_run.go`) - Tracing support for debugging agent execution
- **Retriever** (`retriever.go`) - Interface for document retrieval systems
- **MemoryEntry** (`memory_entry.go`) - Memory entry representation

The root `aigentic` package (`agent.go`) provides the declarative `Agent` type that users configure, which internally creates and manages `run.AgentRun` instances for execution.

### The `ctxt` Package

The `ctxt` package (`github.com/nexxia-ai/aigentic/ctxt`) provides context management and execution environment for agents:

- **AgentContext** (`context.go`) - Manages agent state including memories, documents, conversation history, and execution environment
- **ExecutionEnvironment** (`environment.go`) - Provides structured directory layout for agent execution with `memory/`, `files/`, and `output/` directories. Memory files are automatically loaded into prompts.
- **ConversationHistory** (`conversation_history.go`) - Tracks conversation turns across multiple agent runs
- **ConversationTurn** (`conversation_turn.go`) - Represents individual conversation turns
- **PromptBuilder** (`prompt_builder.go`) - Builds LLM prompts from context, memories, documents, and memory files

### The `document` Package

The `document` package (`github.com/nexxia-ai/aigentic/document`) provides document manipulation, processing, and storage capabilities:

- **Document** (`document.go`) - Core document type representing files with metadata (filename, MIME type, file size, chunking info). Supports lazy loading via loader functions. Documents can be passed to agents and are automatically included in the agent's context.
- **DocumentProcessor** (`document.go`) - Interface for processing documents. Processors take a document and return zero or more processed documents (e.g., chunking, transformation).
- **Pipeline** (`pipeline.go`) - Chains multiple document processors together. Create with `NewPipeline()`, add processors with `Add()`, then execute with `Run()` or `Process()`.
- **Store** (`store.go`) - Interface for document storage with `Save()`, `Load()`, `List()`, and `Delete()` operations.
- **LocalStore** (`local_store.go`) - File system-based implementation of Store that persists documents and metadata to disk. Supports lazy loading of document content.

Documents can be attached to agents via the `Agent.Documents` and `Agent.DocumentReferences` fields, and are automatically included in the agent's context when building prompts.

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
- Prefer small, focused files; keep package boundaries clear. The `run` package depends on `ai` and `document` packages. Lower-level packages (`ai`, `document`, `tools`, `utils`) should not depend on the root `aigentic` package.

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
