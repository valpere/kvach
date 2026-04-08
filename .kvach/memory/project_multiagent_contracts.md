---
name: project_multiagent_contracts
description: Subagent orchestration contracts and task tool
type: project
---

# Multiagent Contracts

## `internal/multiagent/multiagent.go`

### SubagentType
- `in_process` (default): goroutine in same process
- `teammate`: separate goroutine with own message history
- `worktree`: teammate in isolated git worktree

### TaskState lifecycle
queued -> running -> completed | failed | cancelled | timeout

### Options
Key fields: TaskID (resume), Type, Profile (default "general"), Description,
Prompt, AllowedTools, DeniedTools, WorkDir, ParentSessionID, ParentTaskID,
Command (audit), MaxTurns, MaxDuration, Metadata.
- Normalize() applies defaults (in_process, general)
- Validate() checks required fields, type enum, non-negative limits

### Result
TaskID, State, Profile, Type, Output (raw text), Contract (OutputContract),
SessionID, StartedAt/FinishedAt, Duration, Usage (tokens + cost), Error.

### OutputContract
Structured extraction: Summary, Findings[], ChangedFiles[], NextActions[], Raw.

### Runner interface
- Run(ctx, opts) -> (Result, error)
- Status(ctx, taskID) -> (TaskState, error)
- Cancel(ctx, taskID) -> error

## Task tool (`internal/tool/task/task.go`)

Input: description, prompt, subagent_type, subagent (legacy alias), task_id, command.
Profile resolution: subagent_type > subagent > "general".
Validation: description required (max 140 chars), prompt required.
Not concurrency-safe. Call is TODO(phase2).
