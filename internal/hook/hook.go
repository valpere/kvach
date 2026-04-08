// Package hook implements the lifecycle hook system.
//
// Hooks allow users and tools to intercept well-defined agent events and run
// custom logic (shell scripts, HTTP webhooks, sidecar LLM prompts) at each
// point.
//
// Hook configuration lives in the project config under the "hooks" key. Each
// entry maps a [Event] name to a list of [Matcher] objects that each carry one
// or more [Config] entries.
//
// The execution protocol:
//
//   - A hook command writes JSON to stdout (the [Response] struct).
//   - Exit 0  → read stdout for JSON; absent or empty JSON means "continue".
//   - Exit 1  → non-blocking error: log and continue.
//   - Exit 2  → blocking: include stderr in the LLM context as additional info.
package hook

import "context"

// Event names the lifecycle point at which a hook fires.
type Event string

const (
	EventPreToolUse         Event = "PreToolUse"
	EventPostToolUse        Event = "PostToolUse"
	EventPostToolUseFailure Event = "PostToolUseFailure"
	EventUserPromptSubmit   Event = "UserPromptSubmit"
	EventNotification       Event = "Notification"
	EventStop               Event = "Stop"
	EventSessionStart       Event = "SessionStart"
	EventPermissionDenied   Event = "PermissionDenied"
	EventFileChanged        Event = "FileChanged"
	EventCwdChanged         Event = "CwdChanged"
	EventSubagentStart      Event = "SubagentStart"
)

// Type identifies the hook's execution mechanism.
type Type string

const (
	// TypeCommand runs a shell script and reads its JSON stdout.
	TypeCommand Type = "command"
	// TypeHTTP POSTs the payload to a URL and reads the JSON response body.
	TypeHTTP Type = "http"
	// TypePrompt asks a sidecar LLM model to evaluate the event.
	TypePrompt Type = "prompt"
	// TypeAgent runs a read-only subagent as a verifier.
	TypeAgent Type = "agent"
)

// Config is a single hook entry from the project configuration.
type Config struct {
	Type Type

	// Command is the shell command for TypeCommand hooks.
	Command string

	// URL is the endpoint for TypeHTTP hooks.
	URL string

	// Prompt is the system prompt for TypePrompt/TypeAgent hooks.
	Prompt string

	// If is an optional permission-rule pattern that further restricts when
	// this hook fires (e.g. "Bash(rm *)").
	If string

	// Timeout is the maximum seconds to wait for the hook to respond.
	// Zero means use the default (30 s).
	Timeout int

	// StatusMessage is shown in the TUI while the hook is running.
	StatusMessage string

	// Once removes the hook after it fires for the first time.
	Once bool
}

// Matcher maps an event (and optional tool matcher) to a list of hooks.
type Matcher struct {
	// Matcher is a tool-name or glob pattern (e.g. "Bash", "Write").
	// Empty means "match all tools".
	Matcher string
	Hooks   []Config
}

// Payload is the data sent to a hook when it fires.
type Payload struct {
	Event     Event
	SessionID string
	ToolName  string
	ToolInput map[string]any
	ToolError string
	// Extra carries event-specific supplementary data.
	Extra map[string]any
}

// Response is the JSON structure a hook command writes to stdout.
type Response struct {
	// Continue defaults to true. Set false to stop the agent after this event.
	Continue *bool `json:"continue,omitempty"`

	// Decision is "approve" or "block" for PreToolUse hooks.
	Decision string `json:"decision,omitempty"`

	// Reason is shown to the user when Decision is "block".
	Reason string `json:"reason,omitempty"`

	// StopReason is the message passed to the agent when Continue is false.
	StopReason string `json:"stopReason,omitempty"`

	// HookSpecificOutput carries event-type-specific structured data.
	HookSpecificOutput *HookSpecificOutput `json:"hookSpecificOutput,omitempty"`
}

// HookSpecificOutput carries PreToolUse-specific output fields.
type HookSpecificOutput struct {
	HookEventName      string         `json:"hookEventName"`
	PermissionDecision string         `json:"permissionDecision,omitempty"` // "allow" | "deny" | "ask"
	UpdatedInput       map[string]any `json:"updatedInput,omitempty"`
	AdditionalContext  string         `json:"additionalContext,omitempty"`
}

// Result is the aggregated outcome of running all matching hooks for one event.
type Result struct {
	// PreventContinuation is true when any hook set continue=false.
	PreventContinuation bool
	// StopReason is the message from the hook that stopped the agent.
	StopReason string
	// PermissionOverride is "allow", "deny", or "ask" if a PreToolUse hook
	// overrode the permission decision.
	PermissionOverride string
	// UpdatedInput, when non-nil, is the hook's suggested replacement input.
	UpdatedInput map[string]any
	// AdditionalContext is injected into the LLM context (exit-2 stderr).
	AdditionalContext string
}

// Executor runs hooks for a given event and aggregates their results.
type Executor interface {
	Run(ctx context.Context, event Event, payload Payload) (Result, error)
}
