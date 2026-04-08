# Implementation Plan: Patterns Extracted from ClubTasker

Findings from investigating `daily-clean-spark` and their application to kvach.

## What ClubTasker teaches us

ClubTasker is not an AI coding agent itself. It is a production app that uses an AI coding agent (Claude Code) with heavily customized agents, skills, memory, and pipelines. The useful information is in its `.claude/` directory: 12 specialized agents, 17 skills (including orchestration pipelines), per-agent memory, granular permissions, and multi-model review loops.

The patterns below are the ones worth implementing in kvach.

---

## 1. Agent Profiles (high priority)

### Problem
kvach has a single `agent.Config` with `AgentName` string. ClubTasker defines 12 agents, each with its own model, tool allowlist, system prompt body, memory scope, and behavioral instructions.

### What to build

**`internal/agent/profile.go`** — Agent profile model and registry.

Each profile has:
- `Name` — identifier (`general`, `explore`, `build`, `review`, etc.)
- `Description` — with example invocations (used in Task tool's subagent selector)
- `Model` — model override (empty = inherit session default)
- `Tools` — allowlist of tool names; empty = all enabled tools
- `DeniedTools` — denylist applied after allowlist
- `SystemPrompt` — full system prompt body (Markdown)
- `MaxTurns` — per-profile turn limit
- `MemoryScope` — `project` or `global`
- `Color` — UI hint for TUI display

Built-in profiles should be defined in Go code. User overrides come from:
- config file (`config.agents` map)
- markdown files (`.kvach/agents/*.md`, `.agents/agents/*.md`)

### File format for user-defined agents

YAML frontmatter + Markdown body, matching the pattern from ClubTasker:

```yaml
---
name: my-reviewer
description: "Security expert"
tools: Glob, Grep, Read, WebFetch
model: anthropic/claude-sonnet-4-5
color: blue
memory: project
---

# Security Review Instructions

When reviewing code...
```

### Registry

```go
type ProfileRegistry struct {
    profiles map[string]Profile
}

func (r *ProfileRegistry) Get(name string) (Profile, bool)
func (r *ProfileRegistry) Register(p Profile)
func (r *ProfileRegistry) All() []Profile
func (r *ProfileRegistry) LoadFromDir(dir string) error  // scan *.md files
```

The Task tool and multiagent.Runner resolve profile name to Profile at delegation time.

---

## 2. Agent Profile Loader (high priority)

### Problem
Need to parse markdown files with YAML frontmatter into Profile structs. Also need to merge built-in profiles with user/project overrides.

### What to build

**`internal/agent/loader.go`** — Parse `*.md` agent definition files.

Logic:
1. Split on `---` delimiters to extract YAML frontmatter.
2. Unmarshal frontmatter into profile metadata.
3. Take everything after second `---` as system prompt body.
4. Validate: name is required, tools must be recognized names.

Discovery paths (like skills):
- User scope: `~/.kvach/agents/`, `~/.agents/agents/`
- Project scope: `<project>/.kvach/agents/`, `<project>/.agents/agents/`
- Project scope overrides user scope on name collision.

---

## 3. Tool Output Validation (high priority)

### Problem
LLMs return tool call arguments that are structurally valid JSON but semantically wrong — enum values that don't exist, numbers outside bounds, missing conditional fields. ClubTasker calls this "LLM Theater" and validates every tool output with Zod schemas.

In kvach the tool `Call` receives `json.RawMessage` and trusts it after `ValidateInput`. But the real problem is validating what the LLM returns as tool_use arguments *before* we call the tool. We already validate input, but we should make this more robust.

### What to build

**`internal/tool/validate.go`** — Generic validation framework.

```go
// Validator validates a decoded tool call argument against constraints
// beyond what JSON Schema covers.
type Validator interface {
    Validate(decoded any) error
}

// SchemaValidator validates against the tool's InputSchema.
type SchemaValidator struct {
    Schema map[string]any
}

// EnumValidator checks that a string field is one of allowed values.
// RangeValidator checks numeric bounds.
// RequiredIfValidator checks conditional required fields.
// CompositeValidator chains multiple validators.
```

Each tool can optionally provide a `Validator` via a new interface method. The dispatcher runs the validator before `Call`.

Additionally: add `ValidateOutput` method to the Tool interface for post-call validation (the agent can feed errors back to the LLM as retry hints).

---

## 4. Prompt Template System (high priority)

### Problem
ClubTasker uses `{{variable}}` interpolation in DB-stored prompts with a cache layer. kvach has no prompt templating — system prompts are hardcoded strings in tool Prompt() methods.

### What to build

**`internal/prompt/prompt.go`** — Template loading and rendering.

```go
type Engine struct {
    templates map[string]string  // name -> template string
    defaults  map[string]string  // hardcoded fallbacks
}

func (e *Engine) Render(name string, vars map[string]string) string
func (e *Engine) Register(name, template string)
func (e *Engine) LoadDir(dir string) error  // load *.md or *.txt files as templates
```

Interpolation: `{{key}}` replaced with value from vars map. Unknown keys left as-is (or empty string — configurable).

Use cases:
- system prompts per agent profile
- skill instruction templates with project-specific variables
- tool Prompt() bodies that adapt to context

This is intentionally simple — no logic, no loops, just variable substitution. Complex templating is a code smell in agent prompts.

---

## 5. Per-Agent Memory (medium priority)

### Problem
kvach's memory system is project-scoped. ClubTasker puts each agent's memories in separate directories (`agent-memory/<agent-name>/MEMORY.md`), so the code-generator's knowledge about hooks doesn't pollute the security-reviewer's context.

### What to extend

**`internal/memory/memory.go`** — Add agent-scoped memory support.

Changes:
- `System.BaseDir` stays as the project-level memory root.
- Add `AgentDir(agentName string) string` — returns `BaseDir/agents/<agent>/`.
- Each agent-scoped dir has its own `MEMORY.md` index + topic files.
- `LoadIndexPrompt` gains an `agentName` parameter; when non-empty, loads the agent-specific index; otherwise loads the project-level index.
- Memory types stay the same: `user`, `feedback`, `project`, `reference`.

Memory file format (matches ClubTasker):
```yaml
---
name: hook-patterns
description: Known hook naming and query key conventions
type: project
---

Content here...
```

---

## 6. Provider Failover (medium priority)

### Problem
ClubTasker's `callLLM()` implements automatic failover: primary provider fails -> switch to failover -> after cooldown -> attempt failback. kvach's Provider interface has no failover.

### What to build

**`internal/provider/failover.go`** — Failover wrapper around Provider.

```go
type FailoverProvider struct {
    primary   Provider
    failover  Provider
    failback  time.Duration  // how long before retrying primary
    lastFail  time.Time
    mu        sync.Mutex
}

func (f *FailoverProvider) Stream(ctx context.Context, req *StreamRequest) (<-chan StreamEvent, error)
```

Logic:
1. If primary hasn't failed recently, try primary.
2. On error, record failure time, try failover.
3. After `failback` duration, next request tries primary again.
4. If both fail, return error (not wrap in retry — let the agentic loop handle retries).

This composes transparently — the agent loop sees a single Provider.

---

## 7. Handler Pipeline (medium priority)

### Problem
ClubTasker routes incoming messages through an ordered pipeline of handlers (photo, location, ready, reschedule, question, blocker, late). First handler to return non-nil wins. This is chain-of-responsibility — clean, testable, and order-explicit.

kvach's agentic loop doesn't need this for LLM responses (those always go through the same tool dispatch), but it's useful for:
- pre-processing user input (slash commands, @-mentions, file drops),
- post-processing LLM output (safety filters, output formatting),
- event routing (different UI handlers for different event types).

### What to build

**`internal/agent/pipeline.go`** — Generic handler pipeline.

```go
type Handler[I, O any] func(ctx context.Context, input I) (O, bool)

type Pipeline[I, O any] struct {
    handlers []Handler[I, O]
}

func (p *Pipeline[I, O]) Run(ctx context.Context, input I) (O, bool)
```

Each handler returns `(result, handled)`. If `handled` is true, pipeline stops. If all handlers return false, pipeline returns zero value + false.

---

## 8. Granular Permission Patterns (medium priority)

### Problem
ClubTasker's `.claude/settings.local.json` shows fine-grained patterns:
- `Bash(git:*)` — allow any git command
- `WebFetch(domain:docs.github.com)` — allow fetching from specific domain
- `Read(//home/val/wrk/oblabz/**)` — allow reading specific paths
- `mcp__supabase__execute_sql` — allow specific MCP tool

kvach's `permission.Rule` already has `Tool` and `Pattern` fields. The parsing and matching logic needs implementation.

### What to implement

**`internal/permission/matcher.go`** — Pattern matching for permission rules.

Pattern syntax:
- `Bash(command-prefix:*)` — matches Bash tool calls where command starts with prefix
- `Read(path-glob)` — matches file tool calls where path matches the glob
- `WebFetch(domain:hostname)` — matches WebFetch calls to specific domains
- `mcp__server__tool` — matches MCP tool calls by qualified name
- Bare tool name (no parens) — matches any call to that tool
- `*` as pattern value — match everything

```go
func MatchRule(rule Rule, toolName string, input map[string]any) bool
```

---

## 9. Skill System Enhancements (medium priority)

### Problem
kvach's skill system handles SKILL.md with frontmatter, discovery, and activation. Missing:
- companion config files (like `/fix-review/config.yaml`)
- helper script libraries (like `skills/lib/*.sh`)
- orchestration skills (skills that invoke agents and other skills)

### What to extend

**`internal/skill/skill.go`** — Extend Skill struct.

Add:
- `Config map[string]any` — parsed from `config.yaml` or `config.json` alongside SKILL.md
- `Libraries []string` — helper scripts found in `lib/` subdirectory
- `HasConfig bool`, `ConfigPath string` — metadata about companion config

The Loader's Discover/ParseFile needs to:
1. Check for `config.yaml` / `config.json` next to SKILL.md.
2. Parse it into the Config map.
3. Scan for `lib/` directory.
4. Include config and libs in the ActivationXML output.

---

## Implementation order

| Phase | Item | Files | Priority | Status |
|-------|------|-------|----------|--------|
| A | Agent profiles + registry | `internal/agent/profile.go` | high | DONE |
| A | Agent profile loader (markdown) | `internal/agent/loader.go` | high | DONE |
| A | Tool output validation | `internal/tool/validate.go` | high | DONE |
| A | Prompt template engine | `internal/prompt/prompt.go` | high | DONE |
| B | Per-agent memory | `internal/memory/memory.go` (extend) | medium | DONE |
| B | Provider failover | `internal/provider/failover.go` | medium | DONE |
| B | Handler pipeline | `internal/agent/pipeline.go` | medium | DONE |
| B | Permission matching | `internal/permission/matcher.go` | medium | DONE |
| B | Skill config + libs | `internal/skill/skill.go` (extend) | medium | DONE |

All Phase A and Phase B items are complete. See corresponding `*_test.go` files
for each module.

Additionally completed (not in the original plan):
- 6 agent definitions in `.kvach/agents/` (code-generator, explorer, reviewer, test-generator, docs-maintainer, refactorer)
- 5 skills in `.kvach/skills/` (implement, fix-review, housekeeping, improve, test-coverage)
- Per-agent memory indexes in `.kvach/memory/`
- `CLAUDE.md` and `AGENTS.md` project-level config files
- Integration test verifying all agent definitions parse correctly

---

## What NOT to build

- Multi-model review loops (like `/fix-review`): These are skill-level workflows, not agent infrastructure. Users write them as skills.
- ClickUp/Jira integration: MCP server responsibility, not agent core.
- Playwright MCP: also an MCP server, not agent core.
- Domain-specific handlers (photo validation, late detection): application-level, not framework.
- Database-stored prompts with cache: overkill for a local agent. File-based templates are sufficient.
