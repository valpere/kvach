# CLAUDE.md — kvach

kvach is an AI coding agent runtime written in Go. It gives an LLM a set of
tools (file I/O, shell, search, web) and runs a loop: call the LLM, execute
chosen tools, feed results back, repeat until done.

## Commands

```bash
go build ./...          # build all packages — must pass
go test ./...           # run all tests — must pass
go vet ./...            # static analysis — must pass
gofmt -w .              # format before committing
```

## Architecture

### Package layout (38 packages, 48 source files, 10 test files)

```
cmd/kvach/              CLI entrypoint (Cobra)
internal/
  agent/                Agentic loop, profiles, pipeline, events
    agent.go            Core loop (TODO phase1)
    types.go            Event types, termination reasons
    profile.go          Profile model + registry (general, explore, build, review)
    loader.go           Parse .kvach/agents/*.md -> Profile structs
    pipeline.go         Generic Handler[I,O] chain-of-responsibility
  provider/             LLM provider abstraction
    provider.go         Provider interface, StreamRequest/StreamEvent, Message, ToolSchema
    failover.go         FailoverProvider: primary -> failover -> failback after cooldown
    anthropic/          Anthropic Claude (stub)
    openai/             OpenAI-compatible (stub)
    google/             Google Gemini (stub)
    ollama/             Ollama native (stub)
  tool/                 Tool interface, registry, validation
    tool.go             Tool interface, Registry, Context, Result, PermissionOutcome
    validate.go         FieldValidator: required, enum, range, maxlen, nested fields
    bash/ read/ write/ edit/ glob/ grep/ ls/   Core tools (stubs)
    task/               Subagent delegation (schema + validation done, Call is stub)
    skill/              activate_skill tool (catalog + dedup done, Loader not wired)
    question/ todo/ webfetch/ websearch/ multipatch/   Other tools (stubs)
  multiagent/           Supervisor/subagent contracts
    multiagent.go       Options, Result, OutputContract, Usage, TaskState, Runner interface
  session/              Session + message + part persistence (stubbed, will be SQLite)
  permission/           Permission pipeline + pattern matching
    permission.go       Mode, Rule, Context, Outcome, Checker, Asker interfaces
    matcher.go          MatchRule(): Bash(git:*), WebFetch(domain:x), Read(//path/**)
  memory/               Three-layer memory with per-agent scoping
    memory.go           System with AgentDir(), LoadIndexPrompt(agent), topic CRUD
  skill/                Agent Skills spec (agentskills.io)
    skill.go            Skill, Frontmatter, CatalogEntry, Loader, ActivationXML
  prompt/               Template engine
    prompt.go           Engine with {{key}} interpolation, Register/Render, LoadDir
  config/               Multi-source config loading (stub)
  hook/                 Pre/post tool-use hooks (types defined)
  mcp/                  MCP client (types defined)
  snapshot/             Shadow-git snapshots (stub)
  bus/                  Event bus
  git/ lsp/ server/ tui/ cli/   Supporting packages (stubs)
```

### Agent profiles

6 project-specific agents in `.kvach/agents/`:

| Agent | Tools | Color | Role |
|-------|-------|-------|------|
| code-generator | Full + Task, Skill | yellow | Primary builder, orchestrates review pipeline |
| explorer | Read, Glob, Grep, Bash (restricted) | cyan | Read-only codebase research |
| reviewer | Read-only + Write | blue | Code review with structured reports |
| test-generator | Read, Glob, Grep, Bash, Write | orange | Generate Go tests |
| docs-maintainer | Read, Glob, Grep, Write, Edit | gray | Keep docs in sync |
| refactorer | Read, Glob, Grep, Bash, Edit | purple | Simplify without behavior change |

### Skills

5 project skills in `.kvach/skills/`:

| Skill | Purpose |
|-------|---------|
| /implement | Full pipeline: design -> code-generator -> parallel review -> docs |
| /fix-review | Fix review findings by severity, grouped by file, test after each |
| /housekeeping | 8 read-only repo health checks, pass/fail table |
| /improve | Architecture critique, 6-dimension scoring, SHIP/IMPROVE/RETHINK/KILL |
| /test-coverage | Measure gaps, delegate to test-generator, measure improvement |

### Key interfaces

- `tool.Tool` — every capability exposed to the LLM (`internal/tool/tool.go`)
- `provider.Provider` — LLM backend abstraction (`internal/provider/provider.go`)
- `session.Store` — persistence (`internal/session/session.go`)
- `permission.Checker` — permission evaluation (`internal/permission/permission.go`)
- `skill.Loader` — skill discovery + activation (`internal/skill/skill.go`)
- `multiagent.Runner` — subagent lifecycle (`internal/multiagent/multiagent.go`)

### Implementation status

- Phase 1 (minimal agent): interfaces + types defined, loop is TODO
- Phase 2 (core features): profiles, validation, templates, memory, failover, permissions, skills — contracts done
- Phase 3 (advanced): snapshots, MCP, server — types defined
- 50 TODO markers total (15 phase1, 22 phase2, 11 phase3, 2 untagged)

### External dependencies

- `gopkg.in/yaml.v3` — YAML parsing for agent/skill frontmatter
- `github.com/spf13/cobra` — CLI command tree
- No other external deps. Stdlib for everything else.

### Commit conventions

Conventional Commits enforced by commitlint. Scope must match a known module
path (see `commitlint.config.js`). Examples:

```
feat(agent): add profile registry
fix(tool/bash): handle timeout cancellation
refactor(permission): extract matcher
```

### Rules

- Do not import provider types from tool packages
- Do not add external deps without justification
- Do not skip `go vet` or `go test` before committing
- Do not implement speculatively — YAGNI
- Errors: `fmt.Errorf("context: %w", err)`
- Context as first parameter for blocking/IO functions
- TODO markers must have phase: `TODO(phase1)`, `TODO(phase2)`, etc.
