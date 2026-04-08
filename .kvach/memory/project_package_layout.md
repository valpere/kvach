---
name: project_package_layout
description: Package layout and structure conventions
type: project
---

# Package Layout

38 packages in two trees:

- `cmd/kvach/` — single main package, delegates to `internal/cli/`
- `internal/` — all logic, no public API

Key subsystems:
- `agent/` — core loop + profiles + pipeline + events (5 source, 4 test files)
- `provider/` — LLM abstraction + failover + 4 sub-packages (anthropic, openai, google, ollama)
- `tool/` — interface + registry + validation + 14 tool sub-packages
- `multiagent/` — subagent contracts (Options, Result, Runner)
- `skill/` — Agent Skills spec (Skill, Loader, CatalogEntry)
- `permission/` — pipeline + pattern matching
- `memory/` — three-layer system with per-agent scoping
- `prompt/` — template engine

Supporting (types defined, stubs):
- `session/`, `config/`, `hook/`, `mcp/`, `snapshot/`, `bus/`, `git/`, `lsp/`, `server/`, `tui/`, `cli/`

Stats: 48 source files, 10 test files, 50 TODOs.
