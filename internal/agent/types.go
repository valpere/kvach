package agent

// EventType identifies what kind of event an Event carries.
type EventType string

const (
	// EventTextDelta is emitted for each streaming text chunk from the model.
	EventTextDelta EventType = "text.delta"
	// EventReasoningDelta is emitted for each streaming reasoning/thinking chunk.
	EventReasoningDelta EventType = "reasoning.delta"
	// EventToolStarted is emitted when a tool call begins execution.
	EventToolStarted EventType = "tool.started"
	// EventToolCompleted is emitted when a tool call finishes successfully.
	EventToolCompleted EventType = "tool.completed"
	// EventToolError is emitted when a tool call returns an error.
	EventToolError EventType = "tool.error"
	// EventUsageUpdated is emitted after each model response with token counts.
	EventUsageUpdated EventType = "usage.updated"
	// EventPermissionAsked is emitted when a tool needs the user's approval.
	EventPermissionAsked EventType = "permission.asked"
	// EventPermissionResolved is emitted when a pending permission is answered.
	EventPermissionResolved EventType = "permission.resolved"
	// EventDone is emitted when the agentic loop completes without error.
	EventDone EventType = "done"
	// EventError is emitted when the agentic loop terminates with an error.
	EventError EventType = "error"
)

// Event is the unit of communication between the agent loop and its callers.
// Callers receive events from the channel returned by [Agent.Run].
type Event struct {
	Type      EventType
	SessionID string
	MessageID string
	PartID    string
	// Payload carries type-specific data. Its concrete type depends on Type:
	//   EventTextDelta        -> string
	//   EventReasoningDelta   -> string
	//   EventToolStarted      -> ToolCallInfo
	//   EventToolCompleted    -> ToolResultInfo
	//   EventToolError        -> ToolErrorInfo
	//   EventUsageUpdated     -> UsageInfo
	//   EventPermissionAsked  -> PermissionInfo
	//   EventPermissionResolved -> PermissionResolutionInfo
	//   EventError            -> string (error message)
	Payload any
}

// ToolCallInfo is the payload for EventToolStarted.
type ToolCallInfo struct {
	ID    string
	Name  string
	Input []byte // raw JSON
}

// ToolResultInfo is the payload for EventToolCompleted.
type ToolResultInfo struct {
	ID      string
	Name    string
	Content string
}

// ToolErrorInfo is the payload for EventToolError.
type ToolErrorInfo struct {
	ID      string
	Name    string
	Message string
}

// UsageInfo is the payload for EventUsageUpdated.
type UsageInfo struct {
	InputTokens  int
	OutputTokens int
	CacheRead    int
	CacheWrite   int
	TotalCostUSD float64
}

// PermissionInfo is the payload for EventPermissionAsked.
type PermissionInfo struct {
	ID          string
	ToolName    string
	Description string
	Risk        string // "low", "medium", "high", "destructive"
}

// PermissionResolutionInfo is the payload for EventPermissionResolved.
type PermissionResolutionInfo struct {
	ID       string
	Decision string // "allow", "allow_always", "deny"
	Reason   string
}

// TerminationReason describes why the agentic loop stopped.
type TerminationReason string

const (
	// ReasonCompleted means the model stopped requesting tools voluntarily.
	ReasonCompleted TerminationReason = "completed"
	// ReasonAborted means the context was cancelled (user pressed Ctrl+C).
	ReasonAborted TerminationReason = "aborted"
	// ReasonMaxTurns means the turn budget was exhausted.
	ReasonMaxTurns TerminationReason = "max_turns"
	// ReasonContextOverflow means context was too long and compaction failed.
	ReasonContextOverflow TerminationReason = "context_overflow"
	// ReasonModelError means an unrecoverable API error occurred.
	ReasonModelError TerminationReason = "model_error"
	// ReasonHookStopped means a stop hook blocked continuation.
	ReasonHookStopped TerminationReason = "hook_stopped"
	// ReasonBudgetExceeded means the cost or token budget was exceeded.
	ReasonBudgetExceeded TerminationReason = "budget_exceeded"
)
