---
name: project_import_rules
description: Package import constraints and dependency direction
type: project
---

# Import Rules

## Hard constraints (build will fail or review will reject)

1. `internal/tool/*` must NOT import `internal/provider/` — tool results are plain strings
2. `internal/provider/*` sub-packages import only their parent `internal/provider/`
3. No import cycles — Go compiler enforces this
4. `internal/agent/` imports provider, session, tool — it's the orchestrator
5. `internal/tool/task/` imports multiagent (for profile constant) and tool — nothing else
6. `internal/tool/skill/` imports only tool — skill domain types are in `internal/skill/`
7. `internal/config/` imports hook, mcp, permission — for config struct composition

## Interface placement

Interfaces live in the CONSUMER package, not the PROVIDER package:
- `tool.Tool` in `internal/tool/` — consumed by agent, not defined by tools
- `provider.Provider` in `internal/provider/` — consumed by agent
- `session.Store` in `internal/session/` — consumed by agent
- `skill.Loader` in `internal/skill/` — consumed by tool/skill
- `multiagent.Runner` in `internal/multiagent/` — consumed by tool/task
- `permission.Checker` in `internal/permission/` — consumed by agent
