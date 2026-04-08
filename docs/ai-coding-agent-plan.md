# Building an AI Coding Agent in Go

A comprehensive plan for implementing an AI coding agent (like Claude Code or OpenCode) from scratch in Go.

This document synthesizes findings from:
- Multi-model analysis of Claude Code internals (`../context/investigations/`)
- The Claude Code v2.1.88 leaked source (`../repos/leak/claw-cli-claude-code-source-code-v2.1.88/`)
- The OpenCode open-source agent (`../repos/opencode/`)
- Python reimplementations (`../repos/leak/python/`)
- Vietnamese security research reports (`../repos/leak/for-learn-claude-cli/`)

---

## Table of Contents

1. [What an AI Coding Agent Actually Is](#1-what-an-ai-coding-agent-actually-is)
2. [Reference Implementations Analyzed](#2-reference-implementations-analyzed)
3. [Core Architecture](#3-core-architecture)
4. [System Components](#4-system-components)
   - 4.1 [The Agentic Loop](#41-the-agentic-loop)
   - 4.2 [Tool System](#42-tool-system)
   - 4.3 [Provider System](#43-provider-system)
   - 4.4 [Permission System](#44-permission-system)
   - 4.5 [Memory System](#45-memory-system)
   - 4.6 [Session & Persistence](#46-session--persistence)
   - 4.7 [Context Compaction](#47-context-compaction)
   - 4.8 [Hook System](#48-hook-system)
   - 4.9 [MCP Integration](#49-mcp-integration)
   - 4.10 [Multi-Agent / Subagents](#410-multi-agent--subagents)
   - 4.11 [Skill System](#411-skill-system)
   - 4.12 [Snapshot & Worktree](#412-snapshot--worktree)
   - 4.13 [HTTP API Server](#413-http-api-server)
   - 4.14 [TUI / CLI](#414-tui--cli)
   - 4.15 [Configuration System](#415-configuration-system)
5. [Go Package Layout](#5-go-package-layout)
6. [Key Go Interfaces](#6-key-go-interfaces)
7. [Data Models](#7-data-models)
8. [The Agentic Loop in Detail](#8-the-agentic-loop-in-detail)
9. [Implementation Roadmap](#9-implementation-roadmap)
   - Phase 1: Minimal Working Agent
   - Phase 2: Core Features
   - Phase 3: Advanced Features
   - Phase 4: Production Quality
10. [Go-Specific Design Decisions](#10-go-specific-design-decisions)
11. [What NOT to Do](#11-what-not-to-do)
12. [Key Insights from the Leak](#12-key-insights-from-the-leak)

---

## 1. What an AI Coding Agent Actually Is

An AI coding agent is **not** a chatbot wrapper. It is a local execution runtime that:

1. Maintains a conversation with an LLM
2. Gives the LLM a set of **tools** (read file, write file, run bash, search code, etc.)
3. Runs a **loop**: call the LLM → execute whichever tools it chose → feed results back → repeat
4. Stops when the LLM stops requesting tools (task complete) or an abort condition is hit

The "intelligence" lives in the LLM. The agent's job is to:
- Manage context (what the LLM sees)
- Mediate tool access (permissions, sandboxing)
- Persist state (sessions, memory)
- Expose interfaces (CLI, TUI, HTTP API)
- Extend the LLM's reach (MCP servers, subagents, skills)

The analogy from the Claude Code source is apt: **brain-in-body**. The LLM is the brain; the agent is the body that gives it hands (tools) and memory.

---

## 2. Reference Implementations Analyzed

### Claude Code v2.1.88 (TypeScript, leaked March 31 2026)
- ~1,884 TypeScript/TSX files, ~512,664 lines of code
- Runtime: Bun, React 19 + Ink (terminal UI), Commander.js (CLI)
- Key file: `QueryEngine.ts` (~46K lines) — the LLM tool-call loop
- Key file: `Tool.ts` (~29K lines) — base tool definitions
- 40+ built-in tools, 80+ slash commands, 140+ UI components
- Hidden features: KAIROS (always-on daemon), BUDDY (Tamagotchi pet), ULTRAPLAN, AutoDream

### OpenCode v1.3.17 (TypeScript + Rust, open source, MIT)
- 19-package monorepo (Turborepo + Bun workspaces)
- Effect framework for DI, SolidJS for TUI, Hono for HTTP, Drizzle+SQLite for DB
- Vercel AI SDK for unified LLM access across 20+ providers
- 45 tool implementations, 2 built-in agents (build, plan)
- Clean client/server architecture (HTTP API + SSE events)

### claw-code-agent (Python reimplementation)
- 47 source files, enterprise-grade architecture
- Zero external dependencies (pure stdlib)
- Explicit budget system, hook/policy manifests, session serialization
- Single provider (OpenAI-compatible)

### nano-claude-code (Python reimplementation)
- ~15 core files, pragmatic architecture
- Generator-based event streaming
- 10 providers, self-registering tool modules
- File-based memory with MEMORY.md index

---

## 3. Core Architecture

```
┌──────────────────────────────────────────────────────────────────────┐
│                         User Interfaces                              │
│   ┌──────────┐  ┌──────────┐  ┌──────────────┐  ┌───────────────┐  │
│   │  CLI     │  │  TUI     │  │  HTTP API    │  │  IDE Bridge   │  │
│   │ (cobra)  │  │(bubbletea│  │  (net/http)  │  │  (LSP/stdio)  │  │
│   └────┬─────┘  └────┬─────┘  └──────┬───────┘  └───────┬───────┘  │
└────────┼─────────────┼───────────────┼───────────────────┼──────────┘
         └─────────────┴───────────────┴───────────────────┘
                                 │
                    ┌────────────▼────────────┐
                    │      Session Manager    │
                    │  (create/resume/list)   │
                    └────────────┬────────────┘
                                 │
                    ┌────────────▼────────────┐
                    │       Agent Runner      │  ← The Agentic Loop
                    │   (QueryEngine equiv.)  │
                    └──┬──────────────────┬───┘
                       │                  │
         ┌─────────────▼───┐    ┌─────────▼────────────┐
         │  Provider Layer │    │    Tool Dispatcher   │
         │ (LLM API calls) │    │ (execute tools,      │
         │  Anthropic      │    │  permissions,        │
         │  OpenAI         │    │  concurrency)        │
         │  Google         │    └──────────┬───────────┘
         │  Ollama ...     │               │
         └─────────────────┘    ┌──────────▼────────────┐
                                │    Built-in Tools     │
                    ┌───────────┤  Bash, Read, Write,   │
                    │           │  Edit, Glob, Grep,    │
                    │           │  WebFetch, Task, ...  │
                    │           └───────────────────────┘
         ┌──────────▼──────────┐
         │   Cross-cutting     │
         │  ┌───────────────┐  │
         │  │ Permission Sys│  │
         │  ├───────────────┤  │
         │  │ Memory System │  │
         │  ├───────────────┤  │
         │  │ Hook System   │  │
         │  ├───────────────┤  │
         │  │ MCP Clients   │  │
         │  ├───────────────┤  │
         │  │ Skill Loader  │  │
         │  ├───────────────┤  │
         │  │ Snapshot/Git  │  │
         │  ├───────────────┤  │
         │  │ Config System │  │
         │  └───────────────┘  │
         └─────────────────────┘
                    │
         ┌──────────▼──────────┐
         │   Persistence       │
         │  SQLite (sessions,  │
         │  messages, parts,   │
         │  permissions, MCP   │
         │  state)             │
         │  + File system      │
         │  (memory, transcripts│
         │   snapshots)        │
         └─────────────────────┘
```

---

## 4. System Components

### 4.1 The Agentic Loop

The heart of the system. Everything else serves this loop.

**What it does:**
```
while true:
    1. Build system prompt (agent config + memory + skills + env context)
    2. Build message history (with compaction applied as needed)
    3. Convert to provider wire format
    4. Stream LLM response
    5. Process stream events in real time:
       - Text chunks → emit to UI
       - Reasoning chunks → emit to UI
       - Tool use blocks → collect
    6. If no tool calls → DONE (return to user)
    7. For each tool call (in parallel where safe):
       a. Check permissions (rules → mode → hooks → prompt user)
       b. Execute tool
       c. Emit progress to UI
    8. Append tool results to message history
    9. Check termination conditions (budget, max turns, abort)
    10. Loop
```

**Critical design decisions from the leak:**
- **Tool calls can be parallel**: Tools marked `IsConcurrencySafe()` run concurrently; others run serially
- **Abort cascades**: If one tool fails hard (e.g., bash error in non-safe mode), sibling tool goroutines are cancelled
- **Multiple termination reasons** must be tracked explicitly:
  - `completed` — model stopped voluntarily
  - `aborted` — user cancelled
  - `max_turns` — turn budget exhausted
  - `context_overflow` — context too long (triggers compaction then retry)
  - `model_error` — unrecoverable API error
  - `hook_stopped` — a stop hook prevented continuation
- **Context overflow is not fatal**: on `ContextTooLong` from the API, compact and retry the same turn
- **Max-output recovery**: if the model stopped due to `max_tokens`, inject a continuation message and keep looping

**State machine transitions (from `query()` in Claude Code):**
```
continue_reasons:
  stop_hook_blocking        — stop hook said retry
  reactive_compact_retry    — compacted due to context overflow
  collapse_drain_retry      — context collapse drained staged collapses
  max_output_tokens_recovery — output truncated, inject resume
  max_output_tokens_escalate — retry with higher max_tokens
  token_budget_continuation  — auto-continue for long tasks

terminal_reasons:
  completed
  aborted_streaming
  aborted_tools
  blocking_limit
  model_error
  prompt_too_long
  hook_stopped
  stop_hook_prevented
```

### 4.2 Tool System

Tools are the agent's hands. Every interaction the LLM has with the world is through a tool.

**Core tool interface:**
```go
type Tool interface {
    // Identity
    Name() string
    Aliases() []string

    // Schema for the LLM (JSON Schema format)
    InputSchema() map[string]any

    // Execution
    Call(ctx context.Context, input json.RawMessage, tctx *ToolContext) (*ToolResult, error)

    // Pre-call validation (structural, not permission-based)
    ValidateInput(input json.RawMessage, tctx *ToolContext) error

    // Permission check (tool-specific, e.g., path validation)
    CheckPermissions(input json.RawMessage, tctx *ToolContext) PermissionOutcome

    // Metadata for the dispatcher
    IsEnabled(tctx *ToolContext) bool
    IsConcurrencySafe(input json.RawMessage) bool
    IsReadOnly(input json.RawMessage) bool
    IsDestructive(input json.RawMessage) bool

    // System prompt section (what the LLM sees)
    Prompt(opts PromptOptions) string

    // For permission rule matching (e.g., Bash(git *))
    PreparePermissionMatcher(input json.RawMessage) func(pattern string) bool
}
```

**Built-in tools to implement (priority order):**

| Tool | Priority | Description |
|------|----------|-------------|
| `Bash` | P0 | Execute shell commands. Most complex tool (23 security checks in Claude Code's 2,593-line implementation) |
| `Read` | P0 | Read file contents with line range support |
| `Write` | P0 | Write/overwrite a file |
| `Edit` | P0 | Targeted string replacement within a file |
| `Glob` | P0 | Find files by pattern |
| `Grep` | P0 | Search file contents by regex |
| `LS` | P0 | List directory contents |
| `Task` | P1 | Spawn subagents for parallel work |
| `WebFetch` | P1 | Fetch and parse a URL |
| `WebSearch` | P1 | Search the web |
| `MultiEdit` | P1 | Multiple edits to a file atomically |
| `ApplyPatch` | P1 | Apply a unified diff patch |
| `TodoWrite` | P1 | Manage a structured task list |
| `Question` | P1 | Ask the user a question |
| `Plan` | P2 | Exit planning mode and switch to execution |
| `Skill` | P2 | Load and execute a skill from markdown |
| `Batch` | P2 | Execute multiple read-only tools in parallel |

**Tool result type:**
```go
type ToolResult struct {
    // Text shown to the LLM
    Content string
    // Optional: additional messages to append to history
    ExtraMessages []Message
    // Optional: modifications to tool context (e.g., permission grants)
    ContextMutator func(*ToolContext) *ToolContext
    // Whether result was truncated
    Truncated bool
    // Bytes of the full result before truncation
    FullBytes int
}
```

**Tool registration:**
```go
type Registry struct {
    mu    sync.RWMutex
    tools map[string]Tool
}

func (r *Registry) Register(t Tool)
func (r *Registry) Get(name string) (Tool, bool)
func (r *Registry) All() []Tool
func (r *Registry) FilterForSession(ctx *ToolContext) []Tool
```

Tools self-register in `init()` functions — each tool package registers itself on import. The session builder imports all tool packages to trigger their `init()`.

**Concurrency dispatch (StreamingToolExecutor pattern):**
```go
type ToolDispatcher struct {
    registry *Registry
    maxConcurrency int  // default 10
}

// Dispatch concurrent-safe tools in goroutines, serial tools one by one.
// Abort sibling via context cancel on hard failure.
func (d *ToolDispatcher) Dispatch(
    ctx context.Context,
    calls []ToolCall,
    tctx *ToolContext,
    events chan<- Event,
) []ToolResult
```

**The BashTool deserves special attention.** From the leaked source, it has:
- 23 numbered security checks
- Blocks 18 Zsh builtins by default
- Timeouts (default 120s, max 600s)
- Working directory scoping
- Output size limits
- Pattern-based allow/deny for auto-mode

### 4.3 Provider System

Abstracts away the differences between LLM APIs.

**What differs between providers:**
- Authentication (API keys, OAuth, credential chains)
- Wire format (Anthropic Messages API vs OpenAI Chat Completions vs Google Gemini)
- Tool calling format (how tools are declared and results returned)
- Streaming format (SSE format, event types)
- Capabilities (vision, extended thinking, reasoning tokens, PDFs, audio)
- Context limits, pricing, model families

**Provider interface:**
```go
type Provider interface {
    ID() string
    Name() string

    // List available models
    Models(ctx context.Context) ([]Model, error)

    // Create a streaming LLM call
    Stream(ctx context.Context, req *StreamRequest) (<-chan StreamEvent, error)
}

type StreamRequest struct {
    Model       string
    Messages    []Message
    Tools       []ToolSchema
    System      string
    MaxTokens   int
    Temperature *float64
    Options     map[string]any
}

type StreamEvent struct {
    Type string // "text_delta", "tool_use_start", "tool_use_delta",
                // "tool_use_end", "reasoning_delta", "step_start",
                // "step_end", "message_start", "message_end", "error"
    // ... union fields
}
```

**Providers to implement (priority order):**

| Provider | Priority | Notes |
|----------|----------|-------|
| Anthropic | P0 | Messages API, streaming SSE, extended thinking |
| OpenAI-compatible | P0 | Covers OpenAI, Groq, Together, Perplexity, DeepInfra, OpenRouter, local models |
| Google Gemini | P1 | Different streaming format, multimodal |
| Ollama | P1 | Local models, uses OpenAI-compat format |
| Amazon Bedrock | P2 | AWS credential chain |
| Azure OpenAI | P2 | OpenAI-compat with different auth |
| Google Vertex | P2 | OAuth-based Google Cloud auth |

**Model metadata** (from `models.dev` integration in OpenCode):
```go
type Model struct {
    ID         string
    ProviderID string
    Name       string
    Capabilities ModelCapabilities
    Limits     ModelLimits
    Cost       ModelCost
    Status     string // "active", "deprecated", "beta", "alpha"
}

type ModelCapabilities struct {
    ToolCalling  bool
    Vision       bool
    Reasoning    bool
    PDFInput     bool
    AudioInput   bool
}

type ModelLimits struct {
    Context int // total context window tokens
    Output  int // max output tokens
}

type ModelCost struct {
    InputPerMToken  float64
    OutputPerMToken float64
    CacheRead        float64
    CacheWrite       float64
}
```

**Provider registry:**
```go
type ProviderRegistry struct {
    providers map[string]Provider
    models    map[string]Model  // providerID/modelID -> Model
}

func (r *ProviderRegistry) Register(p Provider)
func (r *ProviderRegistry) Resolve(modelID string) (Provider, Model, error)
func (r *ProviderRegistry) DetectFromEnv() []Provider  // check API key env vars
```

### 4.4 Permission System

Guards tool execution. Prevents the LLM from doing things the user hasn't authorized.

**Modes (from Claude Code):**
```go
type PermissionMode string
const (
    ModeDefault          PermissionMode = "default"       // ask for writes
    ModeAcceptEdits      PermissionMode = "acceptEdits"   // auto-allow file edits, ask for bash
    ModePlan             PermissionMode = "plan"          // read-only
    ModeBypass           PermissionMode = "bypassPermissions" // allow all (requires sandbox)
    ModeDontAsk          PermissionMode = "dontAsk"       // never prompt, deny if not pre-allowed
    ModeAuto             PermissionMode = "auto"          // ML classifier (not in MVP)
)
```

**Rule structure:**
```go
type PermissionRule struct {
    Source   string // "user", "project", "local", "session", "cli"
    Behavior string // "allow", "deny", "ask"
    Tool     string // Tool name (e.g., "Bash")
    Pattern  string // Optional pattern (e.g., "git *")
}
```

**Rule syntax** (from Claude Code's format):
```
Bash(git *)        — match Bash tool with git commands
Read(*.ts)         — match Read tool on TypeScript files
Write               — match any Write invocation
FileEdit(src/*)    — match FileEdit in src/ directory
```

**Resolution pipeline:**
```
1. Tool.ValidateInput()              — structural validation (always runs)
2. Tool.CheckPermissions()           — tool-specific rules (e.g., path containment)
3. Check deny rules (all sources)    — hard blocks
4. Check allow rules (all sources)   — auto-approve
5. Check ask rules (all sources)     — force prompt regardless of mode
6. Apply mode logic:
   - default:       ask for non-read writes
   - acceptEdits:   auto-allow file edits, ask for bash/destructive
   - plan:          deny all writes
   - bypass:        allow everything
   - dontAsk:       deny if not explicitly allowed
7. Run PreToolUse hooks              — can block or override
8. Interactive prompt if needed      — block until user responds
```

**Permission context** (immutable, computed at session start):
```go
type PermissionContext struct {
    Mode                PermissionMode
    AllowRules          []PermissionRule
    DenyRules           []PermissionRule
    AskRules            []PermissionRule
    WorkingDirectories  []string
    BypassAvailable     bool
    HeadlessMode        bool  // never prompt, deny if not pre-approved
}
```

**Permission outcome:**
```go
type PermissionOutcome struct {
    Decision      string // "allow", "deny", "ask"
    Reason        string
    UpdatedInput  map[string]any // tool can suggest modified safe input
    Suggestions   []PermissionUpdate
}
```

**Ask flow:**
```go
// PermissionAsker blocks the tool call until the user responds
type PermissionAsker interface {
    Ask(ctx context.Context, req PermissionRequest) (PermissionReply, error)
}

type PermissionRequest struct {
    ToolName    string
    Description string
    Input       map[string]any
    Pattern     string
    Risk        string // "low", "medium", "high", "destructive"
}

type PermissionReply struct {
    Decision   string // "allow_once", "allow_always", "deny"
    ToolName   string
    Pattern    string
}
```

### 4.5 Memory System

Three-layer file-based memory system. Persists across sessions.

**Layer 1 — Index (always loaded)**
- File: `~/.config/<agent>/projects/<git-root-slug>/MEMORY.md`
- Hard limits: 200 lines, 25KB
- Format: one fact per line, `[type] description | ref:filename.md`
- Always injected into every LLM call (in user context, not system prompt, for cache efficiency)

**Layer 2 — Topic files (on-demand)**
- Files: `memory/<topic>.md` in the same directory
- Format: Markdown with YAML frontmatter:
  ```yaml
  ---
  name: user_role
  description: User is a backend Go developer
  type: user
  created: 2026-04-07
  ---
  Content here
  ```
- Loaded only when the LLM reads them (via a ReadMemory tool or at session start if referenced in index)
- Memory types: `user`, `feedback`, `project`, `reference`

**Layer 3 — Transcripts (grep only, never fully loaded)**
- Daily append-only logs in `logs/YYYY/MM/DD.jsonl`
- Never loaded into context; only searched via grep
- Used by autoDream (KAIROS memory consolidation) to extract facts

**Write discipline:**
The LLM writes memories through explicit tool calls (`update_memory` / `TodoWrite`-style). Key rules extracted from Claude Code's system prompt:
- Only save information that will be useful in a future session
- Do NOT save ephemeral task state (todos, in-progress work)
- DO save: user preferences, project structure decisions, team conventions, feedback/corrections
- The index cap forces prioritization — older/less-relevant entries get evicted

**Memory system interface:**
```go
type MemorySystem struct {
    BaseDir   string
    IndexFile string
    MaxLines  int  // 200
    MaxBytes  int  // 25_000
}

func (m *MemorySystem) IsEnabled() bool
func (m *MemorySystem) LoadIndexPrompt() (string, error)
func (m *MemorySystem) ReadTopic(name string) (MemoryFile, error)
func (m *MemorySystem) WriteTopic(f MemoryFile) error
func (m *MemorySystem) DeleteTopic(name string) error
func (m *MemorySystem) RebuildIndex() error
func (m *MemorySystem) SearchTranscripts(query string) ([]string, error)
```

**Path resolution:**
```go
func MemoryDir(configHome, gitRoot string) string {
    // e.g. ~/.config/myagent/projects/my-project-abc123/memory/
    slug := sanitizeGitRoot(gitRoot)
    return filepath.Join(configHome, "projects", slug, "memory")
}
```

### 4.6 Session & Persistence

Sessions persist the conversation history across invocations.

**Session schema:**
```go
type Session struct {
    ID          string    // ULID, descending order
    ProjectID   string
    Directory   string
    Title       string
    ParentID    string    // for subagent sessions
    CreatedAt   time.Time
    UpdatedAt   time.Time
    CompactedAt *time.Time
    ArchivedAt  *time.Time
}
```

**Message schema:**
```go
type Message struct {
    ID        string
    SessionID string
    Role      string // "user", "assistant"
    // For user messages:
    AgentName  string
    ModelID    string
    // For assistant messages:
    Cost       float64
    InputTokens  int
    OutputTokens int
    FinishReason string
    CreatedAt  time.Time
}
```

**Part schema (content of a message — discriminated union):**
```go
type Part struct {
    ID        string
    MessageID string
    Type      string // "text", "tool_use", "tool_result", "reasoning", "file", "compaction"
    // Text part
    Text      string
    // Tool use part
    ToolName  string
    ToolInput json.RawMessage
    ToolState string // "pending", "running", "completed", "error"
    // Tool result part
    ToolUseID  string
    ToolOutput string
    IsError    bool
    // Reasoning part
    Reasoning  string
    // File attachment part
    FilePath   string
    MimeType   string
    // Compaction marker
    Summary    string
}
```

**Storage:**
- SQLite via `database/sql` with `modernc.org/sqlite` (pure Go, no CGO) or `mattn/go-sqlite3` (CGO)
- Tables: `sessions`, `messages`, `parts`, `permissions`, `mcp_servers`
- JSONL transcript files for raw conversation replay (append-only)

**Session transcript (JSONL):**
```
{"type":"message_start","message":{"id":"...","role":"user",...}}
{"type":"content_block_start","index":0,"content_block":{"type":"text"}}
{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"..."}}
...
```

**Resume behavior:**
```go
func ResumeSession(id string) (*Session, []Message, error)
func LastSession(projectID string) (*Session, error)
func ContinueSession(sess *Session, newMessage string) error  // --continue flag
```

### 4.7 Context Compaction

Prevents context overflow. When the context grows too large, summarize the old parts.

**Two-layer strategy (from both Python reimplementations):**

**Layer 1 — Snip** (fast, no LLM call):
- Walk backwards through tool result messages
- Keep the N most recent turns fully intact (default: last 6)
- Truncate older tool results to `maxChars` (default: 2000)
- Insert `[... N chars truncated ...]` markers

**Layer 2 — Compact** (requires LLM call):
- Find a split point where ~30% of tokens are in the "recent" portion
- Use a cheap/fast model to summarize the "old" portion into a compaction message
- Replace old messages with: `[compaction_summary, ack_message, *recent_messages]`

**Trigger conditions:**
- Proactive: before each turn, if token usage > 70% of context window
- Reactive: if the LLM API returns a "context too long" error

**Token estimation:**
```go
func EstimateTokens(text string) int {
    // Rough heuristic: 1 token ≈ 3.5 chars
    return len(text) / 4
}
```

**Compaction interface:**
```go
type Compactor interface {
    // Returns compacted message list with compaction marker injected
    Compact(ctx context.Context, messages []Message, opts CompactOptions) ([]Message, error)
    // Just truncates tool results, no LLM call
    Snip(messages []Message, opts SnipOptions) []Message
}

type CompactOptions struct {
    Model           string   // cheap/fast model for summarization
    PreserveRecent  int      // min number of recent turns to keep intact
    TargetPercent   float64  // target context used after compaction (e.g., 0.4)
}
```

### 4.8 Hook System

Hooks allow extensibility at well-defined lifecycle points. Users can run scripts, HTTP webhooks, or LLM-based validators at each hook point.

**Hook events:**
```go
type HookEvent string
const (
    HookPreToolUse         HookEvent = "PreToolUse"
    HookPostToolUse        HookEvent = "PostToolUse"
    HookPostToolUseFailure HookEvent = "PostToolUseFailure"
    HookUserPromptSubmit   HookEvent = "UserPromptSubmit"
    HookNotification       HookEvent = "Notification"
    HookStop               HookEvent = "Stop"
    HookSessionStart       HookEvent = "SessionStart"
    HookPermissionDenied   HookEvent = "PermissionDenied"
    HookFileChanged        HookEvent = "FileChanged"
    HookCwdChanged         HookEvent = "CwdChanged"
    HookSubagentStart      HookEvent = "SubagentStart"
)
```

**Hook types:**
```go
type HookType string
const (
    HookTypeCommand HookType = "command"  // run a shell script
    HookTypeHTTP    HookType = "http"     // POST to a webhook URL
    HookTypePrompt  HookType = "prompt"   // ask a sidecar LLM
    HookTypeAgent   HookType = "agent"    // run a subagent verifier
)
```

**Hook configuration** (in `opencode.json`/`CLAUDE.md`):
```json
{
  "hooks": {
    "PreToolUse": [
      {
        "matcher": "Bash",
        "if": "Bash(rm *)",
        "type": "command",
        "command": "safety-check.sh",
        "timeout": 30
      }
    ],
    "Stop": [
      {
        "type": "command",
        "command": "notify-done.sh"
      }
    ]
  }
}
```

**Hook response protocol:**
Hook commands write JSON to stdout:
```json
{
    "continue": true,
    "decision": "approve",
    "reason": "...",
    "hookSpecificOutput": {
        "hookEventName": "PreToolUse",
        "permissionDecision": "allow",
        "updatedInput": {"command": "git status"},
        "additionalContext": "..."
    }
}
```

Exit code semantics:
- `0` → success, read JSON from stdout
- `1` → non-blocking error (log but continue)
- `2` → blocking error (include stderr in LLM context)

**Hook executor interface:**
```go
type HookExecutor interface {
    Run(ctx context.Context, event HookEvent, payload HookPayload) (HookResult, error)
}

type HookResult struct {
    Continue            bool
    Decision            string // "approve", "block"
    Reason              string
    PermissionOverride  string // "allow", "deny", "ask"
    UpdatedInput        map[string]any
    AdditionalContext   string
    StopReason          string
}
```

### 4.9 MCP Integration

Model Context Protocol allows external tools, resources, and prompts to be plugged in from any language.

**Transport types:**
```go
type MCPTransport string
const (
    MCPStdio    MCPTransport = "stdio"   // local process, stdin/stdout JSON-RPC
    MCPSSE      MCPTransport = "sse"     // HTTP SSE (deprecated but widely used)
    MCPStreamable MCPTransport = "http"  // Streamable HTTP (new standard)
)
```

**MCP server config:**
```json
{
  "mcp": {
    "servers": {
      "filesystem": {
        "type": "stdio",
        "command": "npx",
        "args": ["-y", "@modelcontextprotocol/server-filesystem", "/tmp"]
      },
      "github": {
        "type": "http",
        "url": "https://mcp.example.com",
        "headers": {"Authorization": "Bearer token"}
      }
    }
  }
}
```

**MCP client interface:**
```go
type MCPClient interface {
    Connect(ctx context.Context) error
    Disconnect() error
    ListTools(ctx context.Context) ([]MCPToolDef, error)
    CallTool(ctx context.Context, name string, args map[string]any) (*MCPToolResult, error)
    ListResources(ctx context.Context) ([]MCPResource, error)
    ReadResource(ctx context.Context, uri string) (*MCPResourceContent, error)
    ListPrompts(ctx context.Context) ([]MCPPromptDef, error)
    GetPrompt(ctx context.Context, name string, args map[string]any) (*MCPPromptContent, error)
}
```

**Go MCP libraries:**
- `github.com/mark3labs/mcp-go` — well maintained Go MCP implementation
- The MCP JSON-RPC protocol can also be implemented directly (it is straightforward)

MCP tools are wrapped as `Tool` implementations using the standard tool interface. This means from the agent's perspective, MCP tools and built-in tools are identical.

### 4.10 Multi-Agent / Subagents

Subagents allow the primary agent to delegate subtasks to specialized instances.

**Subagent types (from Claude Code):**
1. **In-process subagent** (Fork): Cheapest. Shares memory. Created via the `Task` tool. Good for simple delegation.
2. **Teammate** (out-of-process): Separate goroutine with its own agent loop. Same binary. Good for parallel work.
3. **Worktree subagent**: Teammate running in an isolated git worktree. Good for parallel file changes without conflicts.

**Task tool (spawns subagents):**
```go
// TaskTool spawns a subagent and waits for results
type TaskInput struct {
    Description string `json:"description"`
    Prompt      string `json:"prompt"`
    Subagent    string `json:"subagent"` // "general", "explore", "plan", etc.
}
```

**Subagent interface (implemented in `internal/multiagent/multiagent.go`):**
```go
type Runner interface {
    Run(ctx context.Context, opts Options) (Result, error)
    Status(ctx context.Context, taskID string) (TaskState, error)
    Cancel(ctx context.Context, taskID string) error
}

type Options struct {
    TaskID          string        // resume existing task
    Type            SubagentType  // in_process, teammate, worktree
    Profile         string        // "general", "explore", etc.
    Description     string        // short UI label
    Prompt          string        // full instructions
    AllowedTools    []string      // restrict from profile
    DeniedTools     []string      // further restrict
    WorkDir         string
    ParentSessionID string
    MaxTurns        int
    MaxDuration     time.Duration
    Metadata        map[string]string
}

type Result struct {
    TaskID     string
    State      TaskState  // queued, running, completed, failed, cancelled, timeout
    Output     string
    Contract   OutputContract  // summary, findings, changed_files, next_actions
    SessionID  string
    Duration   time.Duration
    Usage      Usage  // input/output tokens, cost
    Error      string
}
```

**Agent profiles (implemented in `internal/agent/profile.go`):**

Agent profiles define named specialist configurations. Built-in profiles
(`general`, `explore`, `build`, `review`) are registered at init. User profiles
are loaded from `.kvach/agents/*.md` with YAML frontmatter:

```yaml
---
name: code-generator
tools: Read, Write, Edit, Glob, Grep, Bash, WebFetch, Task, Skill, Question
model: anthropic/claude-sonnet-4-5
color: yellow
memory: agent
---
System prompt body...
```

The profile registry supports allow/deny tool lists, model override, memory
scope (project vs per-agent), and validation.

**Coordinator mode** (multi-agent orchestration):
A special operating mode where the primary agent acts as an orchestrator only — it spawns and coordinates worker subagents but does not do implementation itself. Activated by config/env var. The coordinator gets a specialized system prompt replacing the default.

**Communication:**
- Workers return results as structured `OutputContract` (summary, findings, changed files, next actions)
- Coordinator uses `Task` tool to spawn and resume workers
- Task lifecycle states: `queued`, `running`, `completed`, `failed`, `cancelled`, `timeout`

### 4.11 Skill System

Skills are markdown files providing specialized instructions for specific task types.

**Skill file format** (`SKILL.md` or `my-skill.md`):
```markdown
---
name: golang-pro
description: Expert Go developer skill with idiomatic patterns
when_to_use: Use when writing Go code, goroutines, interfaces
---

# Go Expert Instructions

When writing Go code:
- Always use proper error handling
- Prefer interfaces over concrete types
...
```

**Discovery locations:**
```
~/.config/myagent/skills/
~/.agents/skills/
.opencode/skills/
.claude/skills/
./skills/
```

**Skill loading (implemented in `internal/skill/skill.go`):**
```go
type Loader interface {
    Discover(projectDir string, extraDirs []string) ([]CatalogEntry, error)
    Activate(name string) (*Skill, error)
    ParseFile(path string) (*Skill, error)
}

type Skill struct {
    Frontmatter              // name, description, license, compatibility, metadata, allowed-tools
    Location    string       // absolute path to SKILL.md
    BaseDir     string       // skill root directory
    Body        string       // Markdown content (Tier 2)
    Resources   []string     // scripts/, references/, assets/ (Tier 3)
    Config      map[string]any // parsed config.yaml companion
    ConfigPath  string       // path to companion config
    Libraries   []string     // lib/ helper scripts
    Source      string       // user-client, project-agents, etc.
}
```

Discovery scans user (`~/.kvach/skills/`, `~/.agents/skills/`) and project
(`.kvach/skills/`, `.agents/skills/`) scopes. Project overrides user on name
collision. The `activate_skill` tool returns structured `<skill_content>` XML
with config summary, library listing, and resource listing.

The `Skill` tool allows the LLM to request and load a skill into its context on demand rather than pre-loading all skills (which would waste tokens).

### 4.12 Snapshot & Worktree

**Snapshot system** — tracks file state at each agent step for undo/revert:
```go
type SnapshotManager struct {
    ShadowGitDir string // separate GIT_DIR, not the project's .git
    ProjectID    string
}

func (s *SnapshotManager) Track(ctx context.Context) (string, error)  // returns hash
func (s *SnapshotManager) Patch(hash string) ([]FilePatch, error)
func (s *SnapshotManager) Restore(hash string) error
func (s *SnapshotManager) Diff(from, to string) (string, error)
func (s *SnapshotManager) Prune(olderThan time.Duration) error  // run hourly
```

Uses a shadow git repository (separate `GIT_DIR`) so agent file changes are tracked independently from the project's own git history. This enables revert without touching the project's commits.

**Git worktree management:**
```go
type WorktreeManager struct {
    BaseDir   string // ~/.local/share/myagent/worktrees/
    ProjectID string
}

func (w *WorktreeManager) Create(ctx context.Context, branch string) (*Worktree, error)
func (w *WorktreeManager) Remove(ctx context.Context, path string) error
func (w *WorktreeManager) Reset(ctx context.Context, path string) error
func (w *WorktreeManager) List() ([]Worktree, error)
```

Both operations shell out to `git` subprocesses. Use a per-directory `sync.Mutex` to serialize git operations.

### 4.13 HTTP API Server

Exposes the agent as an HTTP service, enabling IDE integration, web UIs, and remote clients.

**Architecture:**
- `net/http` with `chi` or `gorilla/mux` router
- SSE (Server-Sent Events) for streaming session events to clients
- WebSocket for PTY (terminal session) streams
- Basic auth (optional, via env var/config)

**Key API endpoints:**

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/session` | List sessions |
| `POST` | `/session` | Create session |
| `GET` | `/session/:id` | Get session |
| `DELETE` | `/session/:id` | Archive session |
| `POST` | `/session/:id/prompt` | Send message to session |
| `GET` | `/session/:id/messages` | Get message history |
| `GET` | `/session/:id/events` | SSE stream of session events |
| `GET` | `/provider` | List providers |
| `GET` | `/provider/:id/models` | List models for provider |
| `GET` | `/config` | Get config |
| `PUT` | `/config` | Update config |
| `GET` | `/mcp` | List MCP servers and status |
| `POST` | `/mcp/:name/connect` | Connect MCP server |
| `GET` | `/permission` | List pending permissions |
| `POST` | `/permission/:id/reply` | Reply to permission request |
| `GET` | `/file` | Read file |
| `GET` | `/project` | Get project info |
| `WS` | `/pty` | WebSocket PTY session |

**SSE event stream format:**
```
event: session.updated
data: {"id":"01H...","title":"Fix the login bug","updatedAt":"2026-04-07T..."}

event: message.part.updated
data: {"sessionID":"...","messageID":"...","partID":"...","type":"text","text":"..."}

event: permission.asked
data: {"id":"...","toolName":"Bash","description":"rm -rf /tmp/old","risk":"high"}
```

**Directory scoping:**
Each request is scoped to a working directory (passed via query param or `X-Opencode-Directory` header). This allows a single server process to handle multiple projects.

### 4.14 TUI / CLI

**CLI commands (cobra):**
```
myagent                    Interactive TUI (default)
myagent run "prompt"       Non-interactive single prompt
myagent serve              Start HTTP API server
myagent session list       List sessions
myagent session resume ID  Resume a session
myagent config             Show/edit config
myagent mcp list           List MCP servers
myagent mcp connect NAME   Connect an MCP server
myagent models             List available models
myagent upgrade            Upgrade to latest version
```

**TUI with bubbletea:**
The TUI is a multi-pane terminal application:
- Input pane: text input with multiline support and history
- Conversation pane: scrollable message history with tool use visualization
- Status bar: current model, token usage, tool in progress
- Permission prompt overlay: blocks on unanswered permission requests

Key bubbletea libraries:
- `github.com/charmbracelet/bubbletea` — TUI framework
- `github.com/charmbracelet/bubbles` — pre-built components (textarea, viewport, spinner)
- `github.com/charmbracelet/lipgloss` — styling
- `github.com/charmbracelet/glamour` — markdown rendering

### 4.15 Configuration System

Multi-source configuration with last-source-wins merge.

**Sources (in merge order):**
```
1. Built-in defaults (hardcoded)
2. System config (/etc/myagent/config.json)
3. Global user config (~/.config/myagent/config.json, config.jsonc)
4. Project config (.myagent/config.json, .myagent/config.jsonc)
5. Environment variables (MYAGENT_* prefix)
6. CLI flags
```

**Config schema (key sections):**
```go
type Config struct {
    // Active provider/model
    Model    string         `json:"model"`    // e.g., "anthropic/claude-sonnet-4-5"
    Provider ProviderConfig `json:"provider"` // provider overrides

    // Feature settings
    AutoMemory       bool   `json:"autoMemory"`
    PermissionMode   string `json:"permissionMode"`
    MaxTurns         int    `json:"maxTurns"`         // default 50
    MaxOutputTokens  int    `json:"maxOutputTokens"`  // 0 = use model default

    // MCP servers
    MCP MCPConfig `json:"mcp"`

    // Hooks
    Hooks map[string][]HookMatcher `json:"hooks"`

    // Custom agents
    Agents map[string]AgentConfig `json:"agents"`

    // HTTP server
    Server ServerConfig `json:"server"`

    // Skill directories
    SkillDirs []string `json:"skillDirs"`

    // Instructions (like CLAUDE.md content)
    Instructions string `json:"instructions"`
}
```

**CLAUDE.md / AGENTS.md**: Project instruction files that inject context into every LLM call. Discovered by walking from the project root upward. Format: plain markdown, no special syntax required.

**JSONC support:** JSON with comments. Use `github.com/tailscale/hujson` or a simple comment-stripping preprocessor.

---

## 5. Go Package Layout

```
cmd/
    myagent/
        main.go               # CLI entry point (cobra root command)
internal/
    agent/
        agent.go              # Agent struct, Run(), agentic loop
        loop.go               # The core while(true) loop
        state.go              # AgentState: messages, usage, etc.
        system_prompt.go      # System prompt assembly
        compaction.go         # Context compaction (snip + compact)
        types.go              # Message, Part, Event types
    tool/
        tool.go               # Tool interface
        registry.go           # Tool registry
        dispatcher.go         # Concurrent tool dispatch
        bash/
            bash.go           # BashTool
            security.go       # 23 security checks
        read/
            read.go           # ReadTool
        write/
            write.go          # WriteTool
        edit/
            edit.go           # EditTool (exact string replacement)
        glob/
            glob.go           # GlobTool
        grep/
            grep.go           # GrepTool (uses ripgrep or stdlib)
        ls/
            ls.go             # LSTool
        task/
            task.go           # TaskTool (spawn subagents)
        webfetch/
            webfetch.go       # WebFetchTool
        websearch/
            websearch.go      # WebSearchTool
        question/
            question.go       # QuestionTool (ask user)
        todo/
            todo.go           # TodoWriteTool
        skill/
            skill.go          # SkillTool
        multipatch/
            multipatch.go     # MultiEdit, ApplyPatch
    provider/
        provider.go           # Provider interface
        registry.go           # Provider registry with auto-detection
        models.go             # Model metadata, models.dev integration
        anthropic/
            anthropic.go      # Anthropic API (Messages API, SSE)
            stream.go         # Streaming implementation
        openai/
            openai.go         # OpenAI-compatible provider
            stream.go
        google/
            google.go         # Google Gemini
            stream.go
        ollama/
            ollama.go         # Ollama (local models)
        types.go              # Shared types: StreamRequest, StreamEvent, Model
    permission/
        permission.go         # PermissionChecker
        rules.go              # Rule evaluation (glob matching)
        asker.go              # Interactive permission prompter
        types.go
    memory/
        memory.go             # MemorySystem
        index.go              # MEMORY.md index management
        topic.go              # Individual topic files
        transcript.go         # Append-only daily logs
        paths.go              # Path resolution (git-root-aware)
        frontmatter.go        # YAML frontmatter parsing
    session/
        session.go            # Session CRUD
        message.go            # Message and Part CRUD
        transcript.go         # JSONL transcript I/O
        store.go              # SQLite storage
        types.go
    hook/
        hook.go               # HookExecutor
        command.go            # Shell command hooks
        http.go               # HTTP webhook hooks
        matcher.go            # Event + tool matcher
        types.go
    mcp/
        client.go             # MCPClient interface
        stdio.go              # Stdio transport
        sse.go                # SSE transport
        http.go               # Streamable HTTP transport
        registry.go           # MCP server registry
        tool_adapter.go       # Wrap MCP tools as Tool interface
    multiagent/
        runner.go             # SubagentRunner
        coordinator.go        # Coordinator mode
        worktree.go           # Worktree-isolated subagents
    skill/
        loader.go             # SkillLoader
        types.go
    snapshot/
        snapshot.go           # Shadow git repository
        worktree.go           # Git worktree management
    config/
        config.go             # Config struct and loading
        merge.go              # Multi-source merge
        jsonc.go              # JSONC parser
        claudemd.go           # CLAUDE.md / AGENTS.md discovery
        paths.go              # Platform-aware paths
    server/
        server.go             # HTTP server (net/http)
        routes/
            session.go        # /session routes
            provider.go       # /provider routes
            config.go         # /config routes
            mcp.go            # /mcp routes
            permission.go     # /permission routes
            event.go          # SSE event stream
            file.go           # /file routes
            pty.go            # WebSocket PTY
        middleware.go         # Auth, logging, CORS
        sse.go                # SSE utilities
    tui/
        tui.go                # bubbletea root model
        conversation.go       # Conversation pane (viewport)
        input.go              # Input pane (textarea)
        statusbar.go          # Status bar
        permission.go         # Permission prompt overlay
        renderer.go           # Markdown/code rendering
        styles.go             # lipgloss styles
    cli/
        root.go               # Root cobra command + global flags
        run.go                # `run` command (non-interactive)
        serve.go              # `serve` command
        session.go            # `session` subcommands
        config.go             # `config` subcommand
        mcp.go                # `mcp` subcommands
        models.go             # `models` command
    bus/
        bus.go                # In-process pub/sub event bus
        events.go             # Event type definitions
    git/
        git.go                # Git utilities (status, log, diff)
        snapshot.go           # Shadow git abstraction
    lsp/
        lsp.go                # LSP client (for code intelligence in tools)
```

---

## 6. Key Go Interfaces

```go
// ===== AGENT =====

type Agent interface {
    Run(ctx context.Context, opts RunOptions) (<-chan Event, error)
}

type RunOptions struct {
    SessionID   string // empty = new session
    Prompt      string
    Attachments []Attachment
    AgentName   string
    WorkDir     string
}

// ===== TOOL =====

type Tool interface {
    Name() string
    Aliases() []string
    InputSchema() map[string]any
    Call(ctx context.Context, input json.RawMessage, tctx *ToolContext) (*ToolResult, error)
    ValidateInput(input json.RawMessage) error
    CheckPermissions(input json.RawMessage, tctx *ToolContext) PermissionOutcome
    IsEnabled(tctx *ToolContext) bool
    IsConcurrencySafe(input json.RawMessage) bool
    IsReadOnly(input json.RawMessage) bool
    IsDestructive(input json.RawMessage) bool
    Prompt(opts PromptOptions) string
}

type ToolContext struct {
    SessionID   string
    WorkDir     string
    Permissions *PermissionContext
    Memory      *MemorySystem
    Hooks       HookExecutor
    Asker       PermissionAsker
    Abort       context.Context
    MCPClients  map[string]MCPClient
    Skills      SkillLoader
}

// ===== PROVIDER =====

type Provider interface {
    ID() string
    Name() string
    Models(ctx context.Context) ([]Model, error)
    Stream(ctx context.Context, req *StreamRequest) (<-chan StreamEvent, error)
}

// ===== PERMISSION =====

type PermissionChecker interface {
    Check(tool Tool, input json.RawMessage, tctx *ToolContext) PermissionOutcome
}

type PermissionAsker interface {
    Ask(ctx context.Context, req PermissionRequest) (PermissionReply, error)
}

// ===== MEMORY =====

type Memory interface {
    LoadPrompt() (string, error)
    Write(ctx context.Context, fact MemoryFact) error
    Search(ctx context.Context, query string) ([]MemoryFact, error)
    IsEnabled() bool
}

// ===== COMPACTION =====

type Compactor interface {
    Compact(ctx context.Context, messages []Message, opts CompactOptions) ([]Message, error)
    Snip(messages []Message, opts SnipOptions) []Message
    ShouldCompact(messages []Message, modelContextWindow int) bool
}

// ===== HOOKS =====

type HookExecutor interface {
    Run(ctx context.Context, event HookEvent, payload HookPayload) (HookResult, error)
}

// ===== MCP =====

type MCPClient interface {
    Connect(ctx context.Context) error
    Disconnect() error
    ListTools(ctx context.Context) ([]MCPToolDef, error)
    CallTool(ctx context.Context, name string, args map[string]any) (*MCPToolResult, error)
}

// ===== SESSION =====

type SessionStore interface {
    Create(ctx context.Context, sess Session) error
    Get(ctx context.Context, id string) (Session, error)
    List(ctx context.Context, projectID string) ([]Session, error)
    Update(ctx context.Context, sess Session) error
    Archive(ctx context.Context, id string) error
    AppendMessage(ctx context.Context, msg Message) error
    AppendPart(ctx context.Context, part Part) error
    GetMessages(ctx context.Context, sessionID string) ([]Message, error)
    GetParts(ctx context.Context, messageID string) ([]Part, error)
}

// ===== BUS =====

type EventBus interface {
    Publish(event Event)
    Subscribe(filter func(Event) bool) (<-chan Event, func())
}
```

---

## 7. Data Models

### Message (provider-agnostic format)

```go
type Message struct {
    ID      string
    Role    string // "user", "assistant"
    Parts   []Part
    // Metadata
    ModelID     string
    InputTokens  int
    OutputTokens int
    Cost         float64
    FinishReason string
    CreatedAt    time.Time
}

type Part interface {
    PartType() string
}

type TextPart struct {
    Text      string
    Synthetic bool // injected by compaction, not from LLM
}

type ReasoningPart struct {
    Reasoning string
}

type ToolUsePart struct {
    ID    string
    Name  string
    Input json.RawMessage
    State string // "pending", "running", "completed", "error"
}

type ToolResultPart struct {
    ToolUseID string
    Content   string
    IsError   bool
}

type CompactionPart struct {
    Summary string
}

type FilePart struct {
    Path     string
    MimeType string
    Data     []byte // nil if URL-referenced
    URL      string
}
```

### Event (pub/sub between agent and UI)

```go
type Event struct {
    Type      string
    SessionID string
    MessageID string
    PartID    string
    Payload   any
}

// Event types:
//   "session.created", "session.updated"
//   "message.started", "message.completed"
//   "part.added", "part.updated"
//   "tool.started", "tool.completed", "tool.error"
//   "permission.asked", "permission.resolved"
//   "text.delta"       — streaming text chunk
//   "reasoning.delta"  — streaming reasoning chunk
//   "usage.updated"    — token counts updated
//   "error"            — agent error
//   "done"             — agent loop complete
```

---

## 8. The Agentic Loop in Detail

```go
func (a *Agent) runLoop(ctx context.Context, state *AgentState, events chan<- Event) error {
    for turn := 0; turn < a.config.MaxTurns; turn++ {
        // 1. Build context
        messages, err := a.buildMessages(state)
        if err != nil {
            return err
        }

        // 2. Check if compaction needed
        if a.compactor.ShouldCompact(messages, a.model.Limits.Context) {
            messages, err = a.compactor.Compact(ctx, messages, CompactOptions{
                Model:          a.config.CompactionModel,
                PreserveRecent: 6,
                TargetPercent:  0.4,
            })
            if err != nil {
                return fmt.Errorf("compaction failed: %w", err)
            }
            state.SetMessages(messages)
        }

        // 3. Stream LLM response
        streamReq := a.buildStreamRequest(state)
        streamCh, err := a.provider.Stream(ctx, streamReq)
        if err != nil {
            if isContextTooLong(err) {
                // Reactive compaction
                messages, _ = a.compactor.Compact(ctx, messages, CompactOptions{TargetPercent: 0.3})
                state.SetMessages(messages)
                turn-- // retry this turn
                continue
            }
            return err
        }

        // 4. Process stream
        assistantMsg, toolCalls, err := a.processStream(ctx, streamCh, state, events)
        if err != nil {
            return err
        }
        state.AppendMessage(assistantMsg)

        // 5. No tool calls → done
        if len(toolCalls) == 0 {
            events <- Event{Type: "done", SessionID: state.SessionID}
            return nil
        }

        // 6. Run PreToolUse hooks
        for i, call := range toolCalls {
            result := a.hooks.Run(ctx, HookPreToolUse, HookPayload{ToolCall: call})
            if result.Decision == "block" {
                // Append blocked result to messages and continue the LLM turn
                toolCalls[i].Blocked = true
                toolCalls[i].BlockReason = result.Reason
            }
            if result.UpdatedInput != nil {
                toolCalls[i].Input = mustMarshal(result.UpdatedInput)
            }
        }

        // 7. Dispatch tools (concurrent + serial)
        toolResults := a.dispatcher.Dispatch(ctx, toolCalls, state.ToolContext(), events)

        // 8. Run PostToolUse hooks
        for _, result := range toolResults {
            a.hooks.Run(ctx, HookPostToolUse, HookPayload{ToolResult: result})
        }

        // 9. Append tool results
        for _, result := range toolResults {
            state.AppendToolResult(result)
        }

        // 10. Check budget
        if state.Usage.TotalCost > a.config.MaxCost {
            return ErrBudgetExceeded
        }
    }

    return ErrMaxTurnsExceeded
}

func (a *Agent) processStream(
    ctx context.Context,
    ch <-chan StreamEvent,
    state *AgentState,
    events chan<- Event,
) (Message, []ToolCall, error) {
    var assistantMsg Message
    var toolCalls []ToolCall
    var currentText strings.Builder
    var currentTool *ToolCall

    for event := range ch {
        switch event.Type {
        case "text_delta":
            currentText.WriteString(event.Text)
            events <- Event{Type: "text.delta", Payload: event.Text}

        case "reasoning_delta":
            events <- Event{Type: "reasoning.delta", Payload: event.Reasoning}

        case "tool_use_start":
            currentTool = &ToolCall{ID: event.ToolUseID, Name: event.ToolName}

        case "tool_use_delta":
            currentTool.InputJSON += event.PartialJSON

        case "tool_use_end":
            currentTool.Input = json.RawMessage(currentTool.InputJSON)
            toolCalls = append(toolCalls, *currentTool)
            currentTool = nil

        case "message_end":
            assistantMsg.FinishReason = event.FinishReason
            assistantMsg.InputTokens = event.Usage.InputTokens
            assistantMsg.OutputTokens = event.Usage.OutputTokens
            state.UpdateUsage(event.Usage)
            events <- Event{Type: "usage.updated", Payload: event.Usage}

        case "error":
            return Message{}, nil, fmt.Errorf("stream error: %s", event.Error)
        }
    }

    if currentText.Len() > 0 {
        assistantMsg.Parts = append(assistantMsg.Parts, TextPart{Text: currentText.String()})
    }
    for _, call := range toolCalls {
        assistantMsg.Parts = append(assistantMsg.Parts, ToolUsePart{
            ID: call.ID, Name: call.Name, Input: call.Input,
        })
    }

    return assistantMsg, toolCalls, nil
}
```

---

## 9. Implementation Roadmap

### Phase 1: Minimal Working Agent (Week 1-2)

Goal: A CLI that can have a multi-turn conversation with tool use.

**Deliverables:**
- [ ] Config loading (single JSON file, no JSONC yet)
- [ ] Anthropic provider (streaming, tool calling)
- [ ] Core tools: `Read`, `Write`, `Bash`, `Glob`, `Grep`
- [ ] Basic agentic loop (no compaction, no permissions)
- [ ] Session persistence (SQLite, messages + parts)
- [ ] Non-interactive CLI: `myagent run "prompt"`
- [ ] Basic streaming output to stdout

**Success criteria:** Can run `myagent run "list all Go files and count them"` and get a correct answer.

---

### Phase 2: Core Features (Week 3-5)

Goal: A usable interactive agent with safety guarantees.

**Deliverables:**
- [ ] TUI (bubbletea): input pane + conversation pane + status bar
- [ ] Full tool set: `Edit`, `MultiEdit`, `LS`, `WebFetch`, `TodoWrite`, `Question`
- [ ] Permission system (rules + interactive prompt)
- [ ] Context compaction (snip + LLM-compact)
- [ ] Memory system (MEMORY.md index + topic files)
- [ ] CLAUDE.md / AGENTS.md discovery and injection
- [ ] Hook system (command type only)
- [ ] Session resume (`--continue`, `--resume ID`)
- [ ] OpenAI-compatible provider (covers Groq, Ollama, OpenRouter, etc.)
- [ ] JSONC config support
- [ ] Multi-source config merge

**Success criteria:** Can have a productive coding session across multiple invocations with memory and safety.

---

### Phase 3: Advanced Features (Week 6-9)

Goal: Feature parity with core OpenCode functionality.

**Deliverables:**
- [ ] MCP client (stdio transport + Streamable HTTP)
- [ ] Subagents via `Task` tool
- [ ] Snapshot system (shadow git for revert)
- [ ] HTTP API server (REST + SSE events)
- [ ] Google Gemini provider
- [ ] Skill system (discovery + `Skill` tool)
- [ ] WebSearch tool (Brave/Tavily API)
- [ ] LSP integration (code intelligence in tools)
- [ ] Worktree management (isolated parallel agents)
- [ ] Hook types: http, prompt
- [ ] Built-in agents config (build, plan, explore)
- [ ] Auto-title generation (background, cheap model)

**Success criteria:** Can be driven from an IDE extension; can run parallel tasks in worktrees.

---

### Phase 4: Production Quality (Week 10+)

Goal: Robust, extensible, deployable.

**Deliverables:**
- [ ] Plugin system (WASM or gRPC-based for language-agnostic plugins)
- [ ] Coordinator mode (multi-agent orchestration)
- [ ] Budget system (token + cost limits)
- [ ] Amazon Bedrock + Azure OpenAI providers
- [ ] Desktop wrapper (Wails or Tauri equivalent in Go)
- [ ] CI/CD integration (non-interactive headless mode)
- [ ] Metrics / observability (OpenTelemetry)
- [ ] Structured logging (slog)
- [ ] Comprehensive test suite
- [ ] MCP OAuth support
- [ ] models.dev integration (live model catalog)
- [ ] Share session (generate share URL)
- [ ] Diff/patch viewer in TUI

---

## 10. Go-Specific Design Decisions

### Concurrency model: goroutines + channels (not async generators)

TypeScript uses `AsyncGenerator<Event>` to stream events. The Go equivalent is `chan Event`:

```go
// WRONG: Return value-based
func (a *Agent) Run(ctx context.Context, opts RunOptions) ([]Event, error) { ... }

// RIGHT: Channel-based streaming
func (a *Agent) Run(ctx context.Context, opts RunOptions) (<-chan Event, error) {
    ch := make(chan Event, 64)
    go func() {
        defer close(ch)
        // ... agent loop, send events to ch
    }()
    return ch, nil
}
```

### Context cancellation for abort

```go
// User pressing Ctrl+C cancels the context, which cascades
ctx, cancel := context.WithCancel(context.Background())
defer cancel()

// Tool execution respects context
func (t *BashTool) Call(ctx context.Context, ...) (*ToolResult, error) {
    cmd := exec.CommandContext(ctx, "bash", "-c", input.Command)
    // If ctx is cancelled, bash is killed
}

// Sibling tool cancellation
toolCtx, cancelAll := context.WithCancel(parentCtx)
for _, call := range concurrentCalls {
    go func(call ToolCall) {
        result, err := dispatch(toolCtx, call)
        if err != nil && isHardError(err) {
            cancelAll() // cancel siblings
        }
    }(call)
}
```

### Generic Store for pub/sub

```go
type Store[T any] struct {
    mu        sync.RWMutex
    state     T
    listeners []chan struct{}
    onChange  func(old, new T)
}

func (s *Store[T]) Get() T {
    s.mu.RLock()
    defer s.mu.RUnlock()
    return s.state
}

func (s *Store[T]) Set(updater func(T) T) {
    s.mu.Lock()
    old := s.state
    s.state = updater(s.state)
    new := s.state
    listeners := s.listeners
    s.mu.Unlock()
    if s.onChange != nil {
        s.onChange(old, new)
    }
    for _, ch := range listeners {
        select {
        case ch <- struct{}{}:
        default:
        }
    }
}
```

### Functional options for configuration

```go
type AgentOptions struct {
    maxTurns        int
    compactionModel string
    permMode        PermissionMode
}

type AgentOption func(*AgentOptions)

func WithMaxTurns(n int) AgentOption {
    return func(o *AgentOptions) { o.maxTurns = n }
}

func NewAgent(provider Provider, registry *Registry, opts ...AgentOption) *Agent {
    options := &AgentOptions{
        maxTurns: 50,
        permMode: ModeDefault,
    }
    for _, opt := range opts {
        opt(options)
    }
    return &Agent{provider: provider, registry: registry, options: options}
}
```

### Error types

Define domain-specific errors, not strings:

```go
var (
    ErrContextTooLong  = errors.New("context exceeded model limit")
    ErrMaxTurns        = errors.New("maximum turns exceeded")
    ErrBudgetExceeded  = errors.New("cost budget exceeded")
    ErrPermissionDenied = errors.New("tool use denied by permission system")
    ErrToolNotFound    = errors.New("tool not found in registry")
    ErrAborted         = errors.New("agent aborted by user")
)

// Wrap with context
return fmt.Errorf("bash tool failed: %w", ErrPermissionDenied)

// Check in loop
if errors.Is(err, ErrContextTooLong) {
    // reactive compaction
}
```

### SQL schema migration

Use `golang-migrate/migrate` for versioned migrations:

```sql
-- 000001_init.up.sql
CREATE TABLE sessions (
    id TEXT PRIMARY KEY,
    project_id TEXT NOT NULL,
    directory TEXT NOT NULL,
    title TEXT NOT NULL DEFAULT '',
    parent_id TEXT,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL,
    compacted_at INTEGER,
    archived_at INTEGER
);

CREATE TABLE messages (
    id TEXT PRIMARY KEY,
    session_id TEXT NOT NULL REFERENCES sessions(id),
    role TEXT NOT NULL,
    model_id TEXT,
    input_tokens INTEGER DEFAULT 0,
    output_tokens INTEGER DEFAULT 0,
    cost_usd REAL DEFAULT 0,
    finish_reason TEXT,
    created_at INTEGER NOT NULL
);

CREATE TABLE parts (
    id TEXT PRIMARY KEY,
    message_id TEXT NOT NULL REFERENCES messages(id),
    type TEXT NOT NULL,
    data TEXT NOT NULL,  -- JSON-encoded part-type-specific payload
    created_at INTEGER NOT NULL
);
```

### Build tags for optional features

```go
//go:build !nolsp
// +build !nolsp

package lsp

// LSP support, can be excluded with -tags nolsp for smaller binary
```

---

## 11. What NOT to Do

These are anti-patterns found in the reference implementations that should be avoided:

### Do NOT: Replicate KAIROS before getting the basics right

KAIROS (always-on background daemon) is an advanced, unreleased feature that requires a solid foundation:
- File watchers
- Scheduled ticks
- PushNotification delivery
- AutoDream memory consolidation
- Multi-session communication

Build the reactive agent first. KAIROS is Phase 5+.

### Do NOT: Use a monolithic state object

Claude Code's `AppState` has ~450 fields including UI-specific state mixed in with agent state. Keep state in focused, domain-scoped structs.

### Do NOT: Implement tool timeout as a goroutine sleep

Tool timeouts must use `context.WithTimeout`. A goroutine that's "asleep" and orphaned leaks resources and can't respond to cancellation.

### Do NOT: Skip input validation

Every tool call from the LLM must be validated against the tool's JSON schema *before* execution. The LLM sometimes generates malformed tool inputs.

### Do NOT: Share the project's git history for snapshots

Use a separate `GIT_DIR` (shadow git) for agent snapshots. Polluting the project's git history with agent work is confusing.

### Do NOT: Copy the anti-distillation mechanisms

Fake tool injection, response obfuscation, and Zig-based DRM attestation are commercial protections. An open-source tool doesn't need them and they add significant complexity.

### Do NOT: Hard-code Anthropic

Even if launching with Anthropic-only, design the provider interface from day one. Adding a second provider later is much harder if the abstraction isn't there.

### Do NOT: Assume the LLM follows instructions perfectly

Handle these failure cases explicitly:
- LLM returns `max_tokens` stop reason (inject continuation, retry)
- LLM generates tool input that fails schema validation (return error as tool result)
- LLM calls a tool that doesn't exist (return `tool_not_found` as tool result)
- LLM calls too many tools at once (respect `maxConcurrency`)
- LLM gets stuck in a tool call loop (doom loop detection: 3 identical consecutive calls)

---

## 12. Key Insights from the Leak

These are the most important learnings from the Claude Code source analysis:

### 1. The loop is intentionally "stupid"

From the codebase: the orchestrator is a **deliberately dumb** while-loop (TAOR: Think-Act-Observe-Repeat). The intelligence is entirely in the model. The agent's job is faithfully executing tool calls and feeding results back, not being clever about sequencing.

### 2. Four minimal tool primitives

At its core, an agent only needs four tool categories:
- **Read** (read files, search, fetch URLs)
- **Write** (write files, execute commands)
- **Execute** (bash, run tests)
- **Connect** (spawn subagents, call MCP servers)

Everything else is a quality-of-life specialization of these four.

### 3. "Search, don't index"

Claude Code abandoned vector embeddings (Voyage) in favor of ripgrep for code search. Grep is faster, cheaper (no embedding costs), and more reliable for code. The leaked code uses `ripgrep` (or equivalent) as the search primitive. Only index-based search if grep proves insufficient.

### 4. Three-layer memory with strict write discipline

The memory system is asymmetric by design:
- What goes *in* (index + topic files): carefully curated, permanent knowledge
- What gets *searched* (transcripts): everything, cheap, never fully loaded
- Write discipline is enforced by the system prompt, not code

Don't try to automatically extract memories with ML. Let the LLM decide what to remember.

### 5. Context pressure management is critical

The leaked `QueryEngine.ts` has 5 distinct compaction strategies and a circuit breaker after 3 consecutive failures. One bug caused 250K wasted API calls per day. Context management is not an afterthought — it's a core system. Start with snip + compact; add more strategies as you find failure modes.

### 6. The permission system is a security boundary, not just UX

The `BashTool` in Claude Code has 2,593 lines of code for security checks. The permission system is not just "ask the user" — it's also:
- Pattern-based command blocking
- Working directory containment
- Blocking of dangerous shell builtins
- Auto-mode ML classifier for headless operation

For a production agent, the BashTool must be treated as a security surface.

### 7. Hooks are the correct extensibility mechanism

Rather than plugin APIs with their own CLIs and protocols, hooks are simple: shell script + JSON response. This makes them debuggable, language-agnostic, and composable with `jq` and Unix tools. Claude Code's hackathon discovery (hooks can inject content into LLM context via exit-2 + stderr) shows both the power and the risk.

### 8. MCP is the right answer for external tool integration

Rather than reimplementing integrations for every service (GitHub, Jira, Slack, etc.), MCP lets those services define their own tools and the agent consumes them uniformly. The protocol is simple JSON-RPC over stdio or HTTP. Implement a solid MCP client and leave the rest to the ecosystem.

### 9. The system prompt is modular and assembled dynamically

Claude Code's system prompt is NOT a static string. It is assembled per-session from:
- Agent-specific instructions
- Tool prompt sections (each tool contributes a section)
- Memory index (from MEMORY.md)
- CLAUDE.md / AGENTS.md content (project-specific)
- Skill content (if loaded)
- Environment context (date, OS, git status)
- Dynamic customizations (coordinator mode, etc.)

Build a `SystemPromptBuilder` that assembles these components, and keep the components independently testable.

### 10. The TUI is a React-equivalent architecture — use bubbletea the same way

Claude Code uses React + Ink for its TUI. OpenCode uses custom SolidJS-based terminal UI. Both follow the same pattern: immutable state, reactive re-renders on state change, component composition. Bubbletea's `Model`/`Update`/`View` pattern is functionally identical. Structure the TUI the same way: root model that delegates to child models (conversation, input, statusbar, etc.).

---

*Document synthesized April 2026. Based on analysis of Claude Code v2.1.88 leaked source, OpenCode v1.3.17, and multiple research investigations.*
