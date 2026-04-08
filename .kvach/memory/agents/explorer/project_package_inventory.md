---
name: project_package_inventory
description: Complete package inventory with test coverage status
type: project
---

# Package Inventory

## Packages WITH tests (8 of 38)

| Package | Source | Test | Key exports |
|---------|--------|------|-------------|
| internal/agent | 5 | 4 | Agent, Profile, ProfileRegistry, Pipeline, Handler |
| internal/provider | 2 | 1 | Provider, FailoverProvider, StreamEvent |
| internal/tool | 2 | 1 | Tool, Registry, FieldValidator |
| internal/tool/task | 1 | 1 | Input (Task delegation) |
| internal/permission | 2 | 1 | MatchRule, Mode, Rule, Checker |
| internal/prompt | 1 | 1 | Engine, Interpolate |
| internal/multiagent | 1 | 1 | Options, Result, Runner |

## Packages WITHOUT tests (30 of 38)

Stub packages (types defined, methods return TODO):
cmd/kvach, internal/cli (6 files), internal/tui, internal/server,
internal/bus, internal/git, internal/lsp, internal/snapshot,
internal/session, internal/memory, internal/config, internal/mcp,
internal/hook, internal/skill, internal/tool/skill

Tool stubs (registered, schema defined, Call returns TODO):
internal/tool/bash, internal/tool/read, internal/tool/write,
internal/tool/edit, internal/tool/glob, internal/tool/grep,
internal/tool/ls, internal/tool/multipatch, internal/tool/todo,
internal/tool/question, internal/tool/webfetch, internal/tool/websearch

Provider stubs:
internal/provider/anthropic, internal/provider/openai,
internal/provider/google, internal/provider/ollama
