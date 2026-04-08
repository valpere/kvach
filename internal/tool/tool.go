// Package tool defines the Tool interface and the Registry that manages it.
//
// Every capability the agent exposes to the LLM is implemented as a Tool.
// Built-in tools (Bash, Read, Write, …) live in sub-packages such as
// tool/bash; each sub-package registers itself via its init() function.
//
// The dispatcher runs concurrent-safe tools in parallel and serial tools one
// at a time, aborting siblings via context cancellation on hard failures.
package tool

import (
	"context"
	"encoding/json"
	"sync"
)

// Tool is the interface every built-in and MCP-backed tool must satisfy.
type Tool interface {
	// Name returns the canonical tool name used in LLM API calls.
	Name() string

	// Aliases returns alternative names the tool also responds to.
	// May be nil.
	Aliases() []string

	// InputSchema returns the JSON Schema object that describes the tool's
	// parameters. It is sent to the LLM verbatim.
	InputSchema() map[string]any

	// Call executes the tool with the given JSON-encoded input and returns a
	// result. Implementations must respect ctx cancellation.
	Call(ctx context.Context, input json.RawMessage, tctx *Context) (*Result, error)

	// ValidateInput performs structural validation of the decoded input before
	// Call is invoked. Returns a non-nil error with a human-readable message
	// on failure; the agent feeds this back to the LLM as a tool error.
	ValidateInput(input json.RawMessage) error

	// CheckPermissions returns the permission outcome for the given input
	// before the permission system's general rules are applied.
	// Implementations use this to enforce tool-specific constraints
	// (e.g. path containment checks in file tools).
	CheckPermissions(input json.RawMessage, tctx *Context) PermissionOutcome

	// IsEnabled reports whether the tool should be included in the active tool
	// pool for the given context.
	IsEnabled(tctx *Context) bool

	// IsConcurrencySafe reports whether this tool can safely run in parallel
	// with other tools. Defaults to false (fail-closed).
	IsConcurrencySafe(input json.RawMessage) bool

	// IsReadOnly reports whether this tool has no persistent side-effects.
	// Read-only tools may be auto-approved in plan mode.
	IsReadOnly(input json.RawMessage) bool

	// IsDestructive reports whether this tool performs irreversible operations.
	// Destructive tools always require explicit user approval.
	IsDestructive(input json.RawMessage) bool

	// Prompt returns the section of the system prompt that describes this tool
	// to the LLM. May return an empty string to omit a dedicated section.
	Prompt(opts PromptOptions) string
}

// PromptOptions controls how [Tool.Prompt] renders its system-prompt section.
type PromptOptions struct {
	// Verbose includes extended documentation in the prompt.
	Verbose bool
}

// Context carries the per-call execution environment passed to every Tool.
type Context struct {
	SessionID string
	WorkDir   string

	// Permissions is the resolved permission context for this session.
	Permissions any // will be *permission.Context once that package is ready

	// Asker is used by tools that need to pause and ask the user a question.
	Asker any // will be permission.Asker once that package is ready

	// SkillLoader is used by activate_skill to resolve skills from disk.
	SkillLoader any // will be skill.Loader once import cycles are settled

	// TaskRunner executes delegated subagent tasks for the Task tool.
	TaskRunner any // will be multiagent.Runner once import cycles are settled

	// Abort is cancelled when the user interrupts the current run.
	Abort context.Context
}

// Result is what a tool returns to the agent after a successful Call.
type Result struct {
	// Content is the text shown to the LLM as the tool's output.
	Content string

	// ExtraMessages are additional messages appended to the conversation after
	// the tool result. Rarely needed; used by the Task tool to embed subtask
	// context.
	ExtraMessages []any // []provider.Message once import cycle is resolved

	// Truncated is true when Content was shortened to fit within the size cap.
	Truncated bool

	// FullBytes is the byte length of the full output before truncation.
	FullBytes int
}

// PermissionOutcome is the tool-level permission verdict.
type PermissionOutcome struct {
	// Decision is "allow", "deny", or "ask".
	Decision string

	// Reason is a human-readable explanation shown to the user when Decision
	// is "deny" or "ask".
	Reason string

	// UpdatedInput, when non-nil, is a sanitised version of the tool input
	// that the tool suggests be used instead (e.g. a Bash command with
	// dangerous flags stripped).
	UpdatedInput map[string]any
}

// Registry holds all registered tools and is the single source of truth for
// tool lookup during a session.
type Registry struct {
	mu    sync.RWMutex
	tools map[string]Tool
	order []string // preserves registration order for deterministic tool lists
}

// NewRegistry returns an empty Registry.
func NewRegistry() *Registry {
	return &Registry{tools: make(map[string]Tool)}
}

// Register adds t to the registry. If a tool with the same name already
// exists it is silently replaced.
func (r *Registry) Register(t Tool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	name := t.Name()
	if _, exists := r.tools[name]; !exists {
		r.order = append(r.order, name)
	}
	r.tools[name] = t
	for _, alias := range t.Aliases() {
		r.tools[alias] = t
	}
}

// Get returns the tool registered under name, or false if not found.
func (r *Registry) Get(name string) (Tool, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	t, ok := r.tools[name]
	return t, ok
}

// All returns all tools in registration order, de-duplicated (aliases are
// excluded from the result).
func (r *Registry) All() []Tool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]Tool, 0, len(r.order))
	for _, name := range r.order {
		out = append(out, r.tools[name])
	}
	return out
}

// FilterForSession returns the subset of tools for which IsEnabled returns
// true given the current context.
func (r *Registry) FilterForSession(tctx *Context) []Tool {
	all := r.All()
	out := make([]Tool, 0, len(all))
	for _, t := range all {
		if t.IsEnabled(tctx) {
			out = append(out, t)
		}
	}
	return out
}

// DefaultRegistry is the process-wide registry that init() functions use to
// self-register built-in tools.
var DefaultRegistry = NewRegistry()
