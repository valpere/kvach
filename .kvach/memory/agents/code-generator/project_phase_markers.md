---
name: project_phase_markers
description: TODO phase convention and current counts
type: project
---

# Phase Markers

## Convention

Every TODO must have a phase marker: `TODO(phase1)`, `TODO(phase2)`, `TODO(phase3)`.

## Phase 1 — Minimal Working Agent (MOSTLY DONE)

Implemented:
- agent/agent.go — agentic loop (loop, processStream, executeToolCalls)
- provider/anthropic — full SSE streaming, tool-use, all event types
- tool/bash — command execution with timeout, output cap
- tool/read — file reading with line-range selection, directory listing
- tool/write — atomic write (temp+rename), auto-creates parent dirs
- tool/edit — unique-match replacement with count check
- tool/glob — recursive walk with **/ support, mtime-sorted
- tool/grep — ripgrep first, pure-Go fallback
- tool/ls — flat and recursive listing
- config — Load (CLAUDE.md/AGENTS.md discovery), ResolvePaths (XDG)
- git — GetStatus (porcelain), RecentCommits (log format)
- cli/run — fully wired end-to-end

Remaining phase1 TODOs:
- provider/openai — SSE streaming (used for OpenRouter, Groq, etc.)
- cli/models — model listing command

## Phase 2 — Core Features (22 TODOs)
- memory — 9 TODOs: MEMORY.md reading, topic CRUD, transcript logging
- cli — 4 TODOs: TUI, session list/resume/show
- tool/multipatch — 2 TODOs: MultiEdit, ApplyPatch
- tool/websearch — 2 TODOs: API key check, search impl
- provider/google — 2 TODOs: model list, Gemini streaming
- config — 1 TODO: multi-source merge, JSONC parsing
- Various tools — 1 TODO each: todo, question, webfetch, task, skill

## Phase 3 — Advanced Features (11 TODOs)
- snapshot — 9 TODOs: shadow git operations, worktree management
- server — 1 TODO: wire up chi router
- cli/serve — 1 TODO: start server
