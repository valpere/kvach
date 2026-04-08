// Package todo implements the TodoWrite tool.
//
// The TodoWrite tool lets the agent maintain a structured task list that is
// visible in the TUI. This helps the user track what the agent is doing and
// gives the agent a mechanism to show progress on multi-step tasks.
//
// This package self-registers via init().
package todo

import (
	"context"
	"encoding/json"

	"github.com/valpere/kvach/internal/tool"
)

// Status represents the lifecycle state of a todo item.
type Status string

const (
	StatusPending    Status = "pending"
	StatusInProgress Status = "in_progress"
	StatusCompleted  Status = "completed"
	StatusCancelled  Status = "cancelled"
)

// Priority represents the importance of a todo item.
type Priority string

const (
	PriorityHigh   Priority = "high"
	PriorityMedium Priority = "medium"
	PriorityLow    Priority = "low"
)

// Item is a single entry in the todo list.
type Item struct {
	Content  string   `json:"content"`
	Status   Status   `json:"status"`
	Priority Priority `json:"priority"`
}

// Input is the schema for a TodoWrite tool call.
type Input struct {
	// Todos is the complete replacement todo list. The entire list is replaced
	// on each call — there is no patch/append operation.
	Todos []Item `json:"todos"`
}

type todoTool struct{}

func init() { tool.DefaultRegistry.Register(&todoTool{}) }

func (t *todoTool) Name() string      { return "TodoWrite" }
func (t *todoTool) Aliases() []string { return nil }

func (t *todoTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"todos": map[string]any{
				"type": "array",
				"items": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"content":  map[string]any{"type": "string"},
						"status":   map[string]any{"type": "string", "enum": []string{"pending", "in_progress", "completed", "cancelled"}},
						"priority": map[string]any{"type": "string", "enum": []string{"high", "medium", "low"}},
					},
					"required": []string{"content", "status", "priority"},
				},
				"description": "The complete updated todo list.",
			},
		},
		"required": []string{"todos"},
	}
}

func (t *todoTool) ValidateInput(raw json.RawMessage) error {
	var in Input
	return json.Unmarshal(raw, &in)
}

func (t *todoTool) CheckPermissions(_ json.RawMessage, _ *tool.Context) tool.PermissionOutcome {
	return tool.PermissionOutcome{Decision: "allow"}
}

func (t *todoTool) IsEnabled(_ *tool.Context) bool           { return true }
func (t *todoTool) IsConcurrencySafe(_ json.RawMessage) bool { return false }
func (t *todoTool) IsReadOnly(_ json.RawMessage) bool        { return false }
func (t *todoTool) IsDestructive(_ json.RawMessage) bool     { return false }
func (t *todoTool) Prompt(_ tool.PromptOptions) string       { return "" }

func (t *todoTool) Call(_ context.Context, _ json.RawMessage, _ *tool.Context) (*tool.Result, error) {
	// TODO(phase2): store todos in the session state and emit a bus event
	// so the TUI can render them.
	return &tool.Result{Content: "TODO: TodoWrite tool not yet implemented"}, nil
}
