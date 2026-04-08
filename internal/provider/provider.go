// Package provider abstracts LLM API differences behind a common interface.
//
// Each provider (Anthropic, OpenAI-compatible, Google, Ollama …) implements
// [Provider]. The agent only speaks to this interface; wire-format details
// live inside sub-packages such as provider/anthropic.
//
// The canonical operation is [Provider.Stream], which sends a request to the
// LLM and returns a channel of [StreamEvent] values that the agent loop
// consumes in real time.
package provider

import "context"

// Provider is the single interface every LLM backend must implement.
type Provider interface {
	// ID returns a stable identifier for this provider (e.g. "anthropic").
	ID() string

	// Name returns a human-readable name (e.g. "Anthropic").
	Name() string

	// Models returns the list of models available from this provider.
	// Implementations may cache the result.
	Models(ctx context.Context) ([]Model, error)

	// Stream sends req to the LLM and streams the response as events.
	// The returned channel is closed when the response is complete or ctx is
	// cancelled. The caller must drain the channel.
	Stream(ctx context.Context, req *StreamRequest) (<-chan StreamEvent, error)
}

// StreamRequest is the provider-agnostic request sent to [Provider.Stream].
type StreamRequest struct {
	Model    string
	Messages []Message
	Tools    []ToolSchema
	System   string
	// MaxTokens of 0 means use the model default.
	MaxTokens   int
	Temperature *float64
	// Options carries provider-specific parameters (e.g. top_p, stop sequences).
	Options map[string]any
}

// StreamEventType identifies the kind of event in a [StreamEvent].
type StreamEventType string

const (
	StreamEventTextDelta      StreamEventType = "text_delta"
	StreamEventReasoningDelta StreamEventType = "reasoning_delta"
	StreamEventToolUseStart   StreamEventType = "tool_use_start"
	StreamEventToolUseDelta   StreamEventType = "tool_use_delta"
	StreamEventToolUseEnd     StreamEventType = "tool_use_end"
	StreamEventStepStart      StreamEventType = "step_start"
	StreamEventStepEnd        StreamEventType = "step_end"
	StreamEventMessageStart   StreamEventType = "message_start"
	StreamEventMessageEnd     StreamEventType = "message_end"
	StreamEventError          StreamEventType = "error"
)

// StreamEvent is one element of the streaming response from the LLM.
type StreamEvent struct {
	Type StreamEventType

	// Text is populated for StreamEventTextDelta.
	Text string

	// Reasoning is populated for StreamEventReasoningDelta.
	Reasoning string

	// ToolUseID, ToolName are populated for StreamEventToolUseStart.
	ToolUseID string
	ToolName  string

	// PartialJSON is populated for StreamEventToolUseDelta.
	PartialJSON string

	// FinishReason is populated for StreamEventMessageEnd.
	FinishReason string

	// Usage is populated for StreamEventMessageEnd.
	Usage *UsageStats

	// Error is populated for StreamEventError.
	Error string
}

// UsageStats holds token consumption figures from a single LLM response.
type UsageStats struct {
	InputTokens  int
	OutputTokens int
	CacheRead    int
	CacheWrite   int
}

// Message is a provider-agnostic conversation turn.
type Message struct {
	ID   string
	Role string // "user" | "assistant"
	// Parts holds the content blocks of this message.
	Parts []Part
	// Metadata is populated only on assistant messages.
	Metadata *MessageMetadata
}

// MessageMetadata carries model-returned statistics attached to an assistant
// message.
type MessageMetadata struct {
	ModelID      string
	FinishReason string
	Usage        UsageStats
	CostUSD      float64
}

// PartType identifies the kind of content a [Part] carries.
type PartType string

const (
	PartTypeText       PartType = "text"
	PartTypeReasoning  PartType = "reasoning"
	PartTypeToolUse    PartType = "tool_use"
	PartTypeToolResult PartType = "tool_result"
	PartTypeFile       PartType = "file"
	PartTypeCompaction PartType = "compaction"
)

// Part is a single content block within a [Message].
type Part struct {
	Type PartType

	// Text is set for PartTypeText and PartTypeReasoning.
	Text string

	// Synthetic marks text injected by the agent (e.g. compaction notice),
	// not produced by the model.
	Synthetic bool

	// ToolUse is set for PartTypeToolUse.
	ToolUse *ToolUsePart

	// ToolResult is set for PartTypeToolResult.
	ToolResult *ToolResultPart

	// File is set for PartTypeFile.
	File *FilePart

	// CompactionSummary is set for PartTypeCompaction.
	CompactionSummary string
}

// ToolUsePart carries a single tool call issued by the model.
type ToolUsePart struct {
	ID    string
	Name  string
	Input []byte // raw JSON matching the tool's input schema
	State string // "pending" | "running" | "completed" | "error"
}

// ToolResultPart carries the outcome of executing a tool call.
type ToolResultPart struct {
	ToolUseID string
	Content   string
	IsError   bool
}

// FilePart carries a file attachment (image, PDF, …).
type FilePart struct {
	Path     string
	MimeType string
	Data     []byte // nil when referenced by URL
	URL      string
}

// ToolSchema is the JSON-Schema description of a single tool, sent to the LLM
// so it knows how to invoke the tool.
type ToolSchema struct {
	Name        string
	Description string
	// InputSchema is the JSON Schema object describing the tool's parameters.
	InputSchema map[string]any
}

// Model holds metadata about a single LLM model.
type Model struct {
	ID           string
	ProviderID   string
	Name         string
	Capabilities ModelCapabilities
	Limits       ModelLimits
	Cost         ModelCost
	Status       string // "active" | "deprecated" | "beta" | "alpha"
}

// ModelCapabilities describes what a model can do.
type ModelCapabilities struct {
	ToolCalling bool
	Vision      bool
	Reasoning   bool
	PDFInput    bool
	AudioInput  bool
}

// ModelLimits describes a model's token constraints.
type ModelLimits struct {
	// Context is the total context window size in tokens.
	Context int
	// Output is the maximum output tokens. 0 means use the API default.
	Output int
}

// ModelCost holds per-token pricing in USD per million tokens.
type ModelCost struct {
	InputPerMToken  float64
	OutputPerMToken float64
	CacheRead       float64
	CacheWrite      float64
}
