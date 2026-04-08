# Subagents Research and Kvach Implementation Plan

This document consolidates practical subagent patterns observed across major agent runtimes and translates them into concrete design decisions for `kvach`.

## Scope and intent

The goal is not to copy any specific implementation. The goal is to:

1. identify stable patterns that appear across multiple systems,
2. keep the design simple and implementable in Go,
3. define interfaces and tool contracts that support safe delegation.

## Sources reviewed

Primary sources included:

- Anthropic Claude Code subagents docs
- Gemini CLI subagents docs
- OpenCode agents/subagent docs and community writeups
- LangChain supervisor/subagent architecture guidance
- Qwen Code subagents docs
- Goose subagents article
- Augment subagent docs
- OpenAI Codex subagent concept/docs pages

The same core ideas recur across these systems despite differences in implementation details.

## Cross-source patterns that matter

### 1) Supervisor + specialist decomposition

Successful systems split responsibilities:

- a supervisor handles planning, decomposition, and integration,
- specialists execute focused tasks with narrow objectives.

This yields better reliability than one agent with one huge prompt.

### 2) Context isolation is the main value

Subagents are useful primarily because they isolate context:

- each delegated task has its own conversation window,
- irrelevant reasoning from one task does not pollute others,
- token budgets stay bounded per task.

### 3) Delegation must support explicit and resumable workflows

Common invocation modes:

- explicit spawn: supervisor starts a fresh task,
- resume: supervisor continues a prior task by task ID,
- profile selection: supervisor picks specialist behavior (for example `explore` vs `general`).

### 4) Tool and permission scoping are mandatory

Subagent power should be bounded by policy:

- profile defaults define allowed tools,
- supervisor may further tighten tools per task,
- destructive actions still pass through permission checks.

### 5) Structured output contracts beat free-form text

Delegated task results should include machine-friendly fields:

- summary,
- findings,
- changed files,
- next actions.

This makes orchestration deterministic and easier to test.

### 6) Lifecycle tracking is essential for orchestration

At minimum, track:

- `queued`, `running`, `completed`, `failed`, `cancelled`, `timeout`.

Without explicit states, supervisor logic becomes fragile.

## Design decisions for kvach

## Terminology

- **Subagent type** = execution isolation strategy (`in_process`, `teammate`, `worktree`).
- **Profile** = specialist behavior (`general`, `explore`, future profiles).
- **Task** = one delegated unit of work with its own ID and lifecycle.

## Task tool contract

`Task` input should support:

- `description` (short UI label),
- `prompt` (full instructions),
- `subagent_type` (profile selector),
- `task_id` (resume existing task),
- `command` (audit lineage from supervisor command).

Backward compatibility:

- keep legacy `subagent` as an alias for `subagent_type`.

## Multiagent runner contract

Runner interface should support:

- starting/running tasks,
- status lookups,
- cancellation.

This keeps supervisor control flow explicit and enables future background execution.

## Result envelope

Subagent output should be represented as:

- raw text output for display,
- structured contract fields for orchestration,
- usage accounting (tokens/cost),
- timing and terminal state.

## Safety defaults

- default profile is `general`,
- default execution type is `in_process`,
- `Task` is not concurrency-safe at tool-call level until scheduler is implemented,
- input validation rejects empty description/prompt and overly long descriptions.

## Code changes applied in this iteration

### `internal/multiagent/multiagent.go`

Expanded from a minimal placeholder into an orchestration contract layer:

- added lifecycle model (`TaskState`),
- added richer `Options` (profile, resume ID, denylist, duration, command, metadata),
- added `Options.Normalize()` and `Options.Validate()`,
- added structured `Usage` and `OutputContract`,
- expanded `Result` with timing, state, contract, and error fields,
- expanded `Runner` interface with `Status` and `Cancel`.

### `internal/tool/task/task.go`

Updated tool contract toward realistic delegation:

- expanded input schema with `subagent_type`, `task_id`, `command`,
- retained legacy `subagent` alias,
- added strict validation (`description`, `prompt`, length cap),
- normalized profile selection with `general` fallback,
- marked tool as non-concurrency-safe for now,
- updated placeholder `Call` response to expose expected output contract shape.

## Progress since initial document

### Completed

1. Profile definitions with explicit tool allowlists:
   - Built-in profiles: `general`, `explore`, `build`, `review` (`internal/agent/profile.go`)
   - 6 project-specific agent definitions in `.kvach/agents/` with full system prompts
   - Profile loader from YAML-frontmatter markdown files (`internal/agent/loader.go`)
   - Integration test verifying all `.kvach/agents/*.md` files parse correctly

2. Orchestration skills that use profiles:
   - `/implement` — full pipeline: code-generator -> parallel (test-generator + reviewer) -> docs-maintainer
   - `/fix-review` — prioritize and fix review findings with companion `config.yaml`
   - `/test-coverage` — measure gaps, delegate to test-generator, measure improvement

3. Supporting infrastructure:
   - Prompt template engine (`internal/prompt/prompt.go`) — `{{key}}` interpolation
   - Tool output validation (`internal/tool/validate.go`) — catch "LLM Theater"
   - Provider failover (`internal/provider/failover.go`) — automatic switch + failback
   - Permission pattern matching (`internal/permission/matcher.go`) — `Bash(git:*)` syntax
   - Handler pipeline (`internal/agent/pipeline.go`) — chain-of-responsibility generic
   - Per-agent memory scoping (`internal/memory/memory.go`) — `AgentDir()` method

### Remaining next steps

1. Implement a concrete `multiagent.Runner` with in-process execution first.
2. Wire `Task.Call` to the runner and persist task metadata in session storage.
3. Implement the actual agentic loop in `internal/agent/agent.go`.
4. Add supervisor-side parsing of the structured output contract.
5. Add tests for:
   - status transitions,
   - cancellation and timeout behavior,
   - profile/tool restriction enforcement at runtime.

## Non-goals for current phase

- background daemon orchestration,
- distributed worker pools,
- speculative execution trees,
- automatic conflict resolution across multiple worktree writers.

These can be layered later without changing the contracts introduced here.
