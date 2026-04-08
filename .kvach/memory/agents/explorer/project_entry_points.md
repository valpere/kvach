---
name: project_entry_points
description: Key runtime entry points and global registries
type: project
---

# Entry Points

## CLI
- `cmd/kvach/main.go` calls `cli.Execute()`
- `internal/cli/root.go` defines Cobra root command with subcommands:
  run, serve, session (list/resume/show), models

## Core loop
- `agent.New(provider, registry, sessions, config)` -> `*Agent`
- `agent.Run(ctx, RunOptions)` -> `<-chan Event` (streaming)
- `agent.loop()` is the internal while-loop (TODO phase1)

## Global registries
- `tool.DefaultRegistry` — process-wide tool registry, populated by init() in each tool package
- `agent.DefaultProfileRegistry` — process-wide profile registry, populated at init with 4 built-in profiles

## Tool registration (init pattern)
Every tool package has:
```go
func init() { tool.DefaultRegistry.Register(&myTool{}) }
```
Currently 16 tools registered: Bash, Read, Write, Edit, Glob, Grep, LS,
Task, activate_skill, Question, TodoWrite, WebFetch, WebSearch, MultiEdit,
ApplyPatch (two from multipatch).

## Config loading
- `config.Load(projectDir)` -> `*Config` (stub, returns defaults)
- `config.ResolvePaths()` -> `Paths` (stub)
