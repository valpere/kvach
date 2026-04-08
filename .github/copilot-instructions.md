# kvach — Copilot Instructions

kvach is an AI coding agent runtime written in Go 1.24+. It gives an LLM tools
(file I/O, shell, search, web) and runs a loop: call LLM, execute tools, feed
results back, repeat. The binary is `kvach` with subcommands (`run`, `serve`,
`session`, `models`).

## Build, Test, Validate

Always run all three before considering a change complete:

```bash
go build ./...    # must pass — zero compilation errors
go vet ./...      # must pass — zero static analysis warnings
go test ./...     # must pass — zero test failures
gofmt -w .        # format all Go files before committing
```

There is no Makefile. These four commands are the entire validation pipeline.
The project has no CI workflow yet — rely on local validation.

External dependencies are minimal: `gopkg.in/yaml.v3`, `github.com/spf13/cobra`.
Do not add new external deps without explicit justification.

## Project Layout

```
cmd/kvach/main.go              CLI entrypoint — blank-imports all tool packages for init()
internal/
  agent/                       Core agentic loop, profiles, pipeline, events
    agent.go                   Agent.Run(), loop(), processStream(), executeToolCalls()
    profile.go                 Profile model, ProfileRegistry, built-in profiles
    loader.go                  Parse .kvach/agents/*.md -> Profile structs
    pipeline.go                Generic Handler[I,O] chain-of-responsibility
    types.go                   Event, EventType, TerminationReason
  provider/                    LLM abstraction
    provider.go                Provider interface, StreamRequest/StreamEvent, Message, Part
    failover.go                FailoverProvider: primary -> failover -> failback
    anthropic/anthropic.go     Full SSE streaming implementation
    openai/ google/ ollama/    Stubs
  tool/                        Tool interface + registry + validation
    tool.go                    Tool interface (12 methods), Registry, Context, Result
    validate.go                FieldValidator for input validation
    bash/ read/ write/ edit/   Core tools (implemented)
    glob/ grep/ ls/            Search tools (implemented)
    task/ skill/               Delegation tools (schemas done, Call partially stubbed)
    question/ todo/ webfetch/ websearch/ multipatch/  Other tools (stubs)
  multiagent/multiagent.go     Subagent contracts: Options, Result, Runner interface
  session/session.go           Session/Message/Part model + Store interface (stubbed)
  permission/                  Permission pipeline + MatchRule() pattern syntax
  memory/memory.go             Three-layer memory with per-agent scoping
  skill/skill.go               Agent Skills spec: Skill, Loader, CatalogEntry
  prompt/prompt.go             {{key}} template interpolation engine
  config/config.go             Config loading, CLAUDE.md/AGENTS.md discovery, XDG paths
  hook/ mcp/ snapshot/ bus/ git/ lsp/ server/ tui/ cli/  Supporting packages
```

## Coding Conventions

- Interfaces live in the consumer package, not the provider package
- `tool/` packages must never import `provider/` types — tool results are plain strings
- Every blocking or I/O function takes `context.Context` as first parameter
- Errors: `fmt.Errorf("context: %w", err)` — always wrap with context
- Self-registering tools: each `internal/tool/<name>/` has `init()` calling
  `tool.DefaultRegistry.Register(...)`
- Tests: table-driven with `t.Run()`, `t.Fatal` not `t.Error`, `t.TempDir()` for filesystem,
  no external test frameworks — stdlib `testing` only
- TODO markers must include phase: `TODO(phase1)`, `TODO(phase2)`, `TODO(phase3)`

## Naming

- Package names: lowercase, singular (`session` not `sessions`)
- Files: lowercase, underscores (`profile_test.go` not `profileTest.go`)
- Types: PascalCase. Methods: PascalCase (exported) or camelCase (unexported)
- Constants: PascalCase for exported, camelCase for unexported

## Commit Messages

Conventional Commits enforced by commitlint. Scope must match a known module
path from `commitlint.config.js`:

```
feat(agent): add profile registry
fix(tool/bash): handle timeout cancellation
refactor(permission): extract matcher
test(provider): add failover test
docs: update architecture plan
```

## Agent System

Agent profiles are named specialist configurations in `.kvach/agents/*.md` with
YAML frontmatter (name, description, tools, model, color, memory) + Markdown
system prompt body. Built-in profiles: `general`, `explore`, `build`, `review`.
Loaded by `internal/agent/loader.go`.

## Skill System

Skills follow Agent Skills spec (agentskills.io). Located in `.kvach/skills/*/SKILL.md`
with YAML frontmatter. Optional companion `config.yaml` and `lib/` directory.
5 project skills: implement, fix-review, housekeeping, improve, test-coverage.

## Key Interfaces

- `tool.Tool` (12 methods) — `internal/tool/tool.go`
- `provider.Provider` (4 methods) — `internal/provider/provider.go`
- `session.Store` (9 methods) — `internal/session/session.go`
- `permission.Checker` — `internal/permission/permission.go`
- `skill.Loader` — `internal/skill/skill.go`
- `multiagent.Runner` — `internal/multiagent/multiagent.go`

## What NOT To Do

- Do not import provider types from tool packages
- Do not add external deps without justification (stdlib-first policy)
- Do not skip `go vet` or `go test` before committing
- Do not implement features speculatively — YAGNI
- Do not use `interface{}` when a concrete type or generic is clearer
- Do not create TODO without a phase marker
