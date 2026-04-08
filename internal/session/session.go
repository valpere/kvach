// Package session manages the lifecycle of agent sessions and their message
// history.
//
// A Session represents a single conversation thread. Each session contains an
// ordered list of Messages; each Message contains an ordered list of Parts
// (text chunks, tool calls, tool results, reasoning blocks, …).
//
// Persistence is handled by implementations of [Store]. The default
// implementation uses SQLite.
package session

import (
	"context"
	"time"
)

// Session is a single conversation thread between the user and the agent.
type Session struct {
	ID        string
	ProjectID string
	Directory string
	Title     string
	// ParentID is non-empty for subagent sessions spawned by a parent session.
	ParentID    string
	CreatedAt   time.Time
	UpdatedAt   time.Time
	CompactedAt *time.Time
	ArchivedAt  *time.Time
}

// Message is a single turn in a Session's conversation.
type Message struct {
	ID        string
	SessionID string
	// Role is "user" or "assistant".
	Role string
	// Metadata set on user messages.
	AgentName string
	ModelID   string
	// Metadata set on assistant messages.
	InputTokens  int
	OutputTokens int
	CostUSD      float64
	FinishReason string
	CreatedAt    time.Time
}

// PartType identifies the content type of a [Part].
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
	ID        string
	MessageID string
	Type      PartType
	// Data is the JSON-encoded part-type-specific payload.
	// The concrete struct depends on Type:
	//   PartTypeText        -> TextData
	//   PartTypeReasoning   -> ReasoningData
	//   PartTypeToolUse     -> ToolUseData
	//   PartTypeToolResult  -> ToolResultData
	//   PartTypeFile        -> FileData
	//   PartTypeCompaction  -> CompactionData
	Data      []byte
	CreatedAt time.Time
}

// TextData is the payload for PartTypeText parts.
type TextData struct {
	Text      string `json:"text"`
	Synthetic bool   `json:"synthetic,omitempty"`
}

// ReasoningData is the payload for PartTypeReasoning parts.
type ReasoningData struct {
	Reasoning string `json:"reasoning"`
}

// ToolUseData is the payload for PartTypeToolUse parts.
type ToolUseData struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Input []byte `json:"input"` // raw JSON
	State string `json:"state"` // "pending" | "running" | "completed" | "error"
}

// ToolResultData is the payload for PartTypeToolResult parts.
type ToolResultData struct {
	ToolUseID string `json:"tool_use_id"`
	Content   string `json:"content"`
	IsError   bool   `json:"is_error,omitempty"`
}

// FileData is the payload for PartTypeFile parts.
type FileData struct {
	Path     string `json:"path"`
	MimeType string `json:"mime_type"`
	URL      string `json:"url,omitempty"`
}

// CompactionData is the payload for PartTypeCompaction parts.
type CompactionData struct {
	Summary string `json:"summary"`
}

// Store is the persistence interface for sessions and their messages.
type Store interface {
	// Session CRUD
	CreateSession(ctx context.Context, s Session) error
	GetSession(ctx context.Context, id string) (Session, error)
	ListSessions(ctx context.Context, projectID string) ([]Session, error)
	UpdateSession(ctx context.Context, s Session) error
	ArchiveSession(ctx context.Context, id string) error

	// Message and Part writes
	AppendMessage(ctx context.Context, m Message) error
	AppendPart(ctx context.Context, p Part) error
	UpdatePart(ctx context.Context, p Part) error

	// Reads
	GetMessages(ctx context.Context, sessionID string) ([]Message, error)
	GetParts(ctx context.Context, messageID string) ([]Part, error)
}
