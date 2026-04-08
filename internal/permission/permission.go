// Package permission implements the permission system that guards tool
// execution.
//
// Before any tool call reaches its Call method the agent runs it through the
// permission pipeline:
//
//  1. Tool.ValidateInput  — structural schema check
//  2. Tool.CheckPermissions — tool-specific rules (e.g. path containment)
//  3. Deny rules          — hard blocks from any rule source
//  4. Allow rules         — auto-approvals from any rule source
//  5. Ask rules           — forced prompts regardless of mode
//  6. Mode logic          — default / acceptEdits / plan / bypass / dontAsk
//  7. PreToolUse hooks    — can block or override
//  8. Interactive prompt  — blocks until the user responds (if needed)
package permission

import "context"

// Mode is the active permission policy for a session.
type Mode string

const (
	// ModeDefault asks the user for any non-read-only operation.
	ModeDefault Mode = "default"
	// ModeAcceptEdits auto-approves file edits; asks for shell commands.
	ModeAcceptEdits Mode = "acceptEdits"
	// ModePlan is read-only: all write and execute operations are denied.
	ModePlan Mode = "plan"
	// ModeBypass approves everything. Should only be used inside a sandbox.
	ModeBypass Mode = "bypassPermissions"
	// ModeDontAsk never prompts; denies anything not explicitly allowed.
	ModeDontAsk Mode = "dontAsk"
)

// RuleSource identifies where a permission rule came from.
type RuleSource string

const (
	RuleSourceUser    RuleSource = "user"    // ~/.config/myagent/config.json
	RuleSourceProject RuleSource = "project" // .myagent/config.json
	RuleSourceLocal   RuleSource = "local"   // .myagent/config.local.json
	RuleSourceSession RuleSource = "session" // granted interactively this session
	RuleSourceCLI     RuleSource = "cli"     // --allow / --deny flags
)

// Rule is a single allow/deny/ask entry in the permission system.
type Rule struct {
	Source   RuleSource
	Behavior string // "allow" | "deny" | "ask"
	// Tool is the tool name this rule applies to (e.g. "Bash").
	Tool string
	// Pattern further restricts the rule within the tool's input space
	// (e.g. "git *" for Bash, "src/**" for file tools).
	// Empty means "match any input".
	Pattern string
}

// Context is the immutable permission configuration for a session.
type Context struct {
	Mode       Mode
	AllowRules []Rule
	DenyRules  []Rule
	AskRules   []Rule
	// WorkingDirectories is the set of directories the agent may access.
	WorkingDirectories []string
	// BypassAvailable indicates the runtime is running inside a sandbox and
	// ModeBypass is therefore permitted.
	BypassAvailable bool
	// HeadlessMode disables interactive prompts; unanswered asks are denied.
	HeadlessMode bool
}

// Outcome is the verdict returned by the permission pipeline.
type Outcome struct {
	// Decision is "allow", "deny", or "ask".
	Decision string
	// Reason is a human-readable explanation (shown when Decision is not
	// "allow").
	Reason string
	// UpdatedInput, when non-nil, is a sanitised alternative input the tool
	// suggests using instead of the original.
	UpdatedInput map[string]any
}

// Request is the information presented to the user when a tool asks for
// approval.
type Request struct {
	// ID is a unique identifier for this pending permission request.
	ID       string
	ToolName string
	// Description is a plain-English summary of what the tool wants to do.
	Description string
	Input       map[string]any
	Pattern     string
	// Risk is "low", "medium", "high", or "destructive".
	Risk string
}

// Reply is the user's response to a [Request].
type Reply struct {
	// Decision is "allow_once", "allow_always", or "deny".
	Decision string
	// ToolName and Pattern are echoed from the originating Request so the
	// permission system can record an "allow_always" rule.
	ToolName string
	Pattern  string
}

// Asker is the interface used by tools to block and ask the user for
// permission. The CLI implements this via a TUI prompt; the HTTP server
// implements it via SSE + REST.
type Asker interface {
	Ask(ctx context.Context, req Request) (Reply, error)
}

// Checker evaluates the full permission pipeline for a tool call.
type Checker interface {
	Check(toolName string, input map[string]any, pctx *Context) Outcome
}
