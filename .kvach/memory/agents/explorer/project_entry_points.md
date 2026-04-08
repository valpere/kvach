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
- `kvach run "prompt"` is fully wired: loads config, creates Anthropic provider, builds system prompt, creates agent, streams events to stdout
- Flags: `--resume`, `--continue`, `--model`, `--dir`, `--debug`

## Core loop (implemented)
- `agent.New(provider, registry, sessions, config)` -> `*Agent`
- `agent.Run(ctx, RunOptions)` -> `<-chan Event` (streaming, goroutine)
- `agent.loop()` — while-loop: stream LLM -> processStream() -> executeToolCalls() -> repeat
- `processStream()` — assembles assistant message + tool calls from StreamEvent channel
- `executeToolCalls()` — validates input, dispatches via registry, collects results
- Terminates on: no tool calls (completed), max turns, context cancelled

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

## Config loading (implemented)
- `config.Load(projectDir)` -> `*Config` — discovers CLAUDE.md/AGENTS.md walking upward, reads KVACH_MODEL env
- `config.ResolvePaths()` -> `Paths` — XDG-compliant: ~/.config/kvach, ~/.local/share/kvach, ~/.cache/kvach
