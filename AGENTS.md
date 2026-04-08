# Repository Guidelines

## Project Structure

- `cmd/kvach/` is the CLI entrypoint (Cobra).
- `internal/` contains all packages. No public API — everything is internal.
- Each tool lives in `internal/tool/<name>/` and self-registers via `init()`.
- Each provider lives in `internal/provider/<name>/`.
- Tests live next to source files as `*_test.go`.
- Documentation lives in `docs/`.
- Agent definitions in `.kvach/agents/*.md`, skills in `.kvach/skills/*/SKILL.md`.

## Build, Test, and Vet

```bash
go build ./...          # must pass — no compilation errors
go test ./...           # must pass — no test failures
go vet ./...            # must pass — no static analysis warnings
gofmt -w .              # format before committing
```

Run all three before every commit. CI will reject anything that fails.

## Coding Conventions

- Go 1.24+. Use stdlib when possible. Justify external dependencies.
- Interfaces live in the package that uses them, not the package that
  implements them (dependency inversion).
- Error handling: wrap with `fmt.Errorf("context: %w", err)`. Use sentinel
  errors for expected conditions.
- Context: every blocking or I/O function takes `context.Context` as first
  parameter.
- Concurrency: channels for streaming, `sync.Mutex` for shared state,
  `context.WithCancel` for cancellation cascades.
- Self-registering tools: each `internal/tool/<name>/` package has an
  `init()` that calls `tool.DefaultRegistry.Register(...)`.

## Naming Conventions

- Package names: lowercase, singular (`session`, not `sessions`).
- Files: lowercase, underscores (`profile_test.go`, not `profileTest.go`).
- Types: PascalCase. Methods: PascalCase (exported) or camelCase (unexported).
- Constants: PascalCase for exported, camelCase for unexported.

## Commit Messages

Conventional Commits format. Scope must match a known module path from
`commitlint.config.js`. Examples:

```
feat(agent): add profile registry with built-in profiles
fix(tool/bash): handle timeout cancellation correctly
refactor(permission): extract matcher into separate file
test(provider): add failover cooldown test
docs: update architecture plan
```

## Architecture Constraints

- The agentic loop (`internal/agent/`) must remain provider-agnostic.
- Tools must never import provider types. Tool results are plain strings.
- The permission system sits between the agent and tool execution — tools
  must not bypass it.
- Memory is file-based (no database). Three layers: index, topics, transcripts.
- Session persistence will use SQLite. Interfaces are defined; implementations
  are stubbed with TODO(phase) markers.
- Skills follow the Agent Skills spec (https://agentskills.io).

## Agent Definitions

Agent profiles in `.kvach/agents/*.md` use YAML frontmatter:

```yaml
---
name: my-agent
description: "What this agent does"
tools: Read, Glob, Grep, Bash
model: anthropic/claude-sonnet-4-5
color: blue
memory: project
---

System prompt body here...
```

## Skill Definitions

Skills in `.kvach/skills/<name>/SKILL.md` use YAML frontmatter:

```yaml
---
name: my-skill
description: "When to use this skill"
---

Step-by-step instructions...
```

Optional companion files: `config.yaml`, `lib/*.sh`, `scripts/`, `references/`.
