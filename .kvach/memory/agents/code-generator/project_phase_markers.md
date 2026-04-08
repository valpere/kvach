---
name: project_phase_markers
description: TODO phase convention and current counts
type: project
---

# Phase Markers

## Convention

Every TODO must have a phase marker: `TODO(phase1)`, `TODO(phase2)`, `TODO(phase3)`.

## Current counts (50 total)

### Phase 1 — Minimal Working Agent (15 TODOs)
- `agent/agent.go` — implement streaming agentic loop
- `provider/anthropic` — hardcoded model list, SSE streaming
- `provider/openai` — model fetching, SSE streaming
- `tool/bash` — command execution with timeout
- `tool/read` — file reading with output cap
- `tool/write` — atomic write
- `tool/edit` — unique-match replacement
- `tool/glob` — filepath.Glob
- `tool/grep` — ripgrep exec
- `tool/ls` — directory listing
- `config` — ResolvePaths
- `cli` — wire up agent, model listing
- `git` — parse status, parse log

### Phase 2 — Core Features (22 TODOs)
- `memory` — 9 TODOs: MEMORY.md reading, topic CRUD, transcript logging
- `cli` — 4 TODOs: TUI, session list/resume/show
- `tool/multipatch` — 2 TODOs: MultiEdit, ApplyPatch
- `tool/websearch` — 2 TODOs: API key check, search impl
- `provider/google` — 2 TODOs: model list, Gemini streaming
- Various tools — 1 TODO each: todo, question, webfetch, task, skill

### Phase 3 — Advanced Features (11 TODOs)
- `snapshot` — 9 TODOs: shadow git operations, worktree management
- `server` — 1 TODO: wire up chi router
- `cli/serve` — 1 TODO: start server
