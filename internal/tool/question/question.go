// Package question implements the Question tool.
//
// The Question tool pauses the agent loop and presents a question to the
// user. The agent's next turn begins only after the user has answered.
// This is useful when the agent needs clarification before taking a
// potentially irreversible action.
//
// This package self-registers via init().
package question

import (
	"context"
	"encoding/json"

	"github.com/valpere/kvach/internal/tool"
)

// Input is the schema for a Question tool call.
type Input struct {
	// Question is the text shown to the user.
	Question string `json:"question"`
	// Options, when non-empty, renders a multiple-choice prompt instead of a
	// free-form text input.
	Options []string `json:"options,omitempty"`
}

type questionTool struct{}

func init() { tool.DefaultRegistry.Register(&questionTool{}) }

func (q *questionTool) Name() string      { return "Question" }
func (q *questionTool) Aliases() []string { return nil }

func (q *questionTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"question": map[string]any{"type": "string", "description": "Question to ask the user."},
			"options":  map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "Optional list of answer choices."},
		},
		"required": []string{"question"},
	}
}

func (q *questionTool) ValidateInput(raw json.RawMessage) error {
	var in Input
	return json.Unmarshal(raw, &in)
}

func (q *questionTool) CheckPermissions(_ json.RawMessage, _ *tool.Context) tool.PermissionOutcome {
	return tool.PermissionOutcome{Decision: "allow"}
}

func (q *questionTool) IsEnabled(_ *tool.Context) bool {
	// Disabled in headless (non-interactive) mode.
	return true
}

func (q *questionTool) IsConcurrencySafe(_ json.RawMessage) bool { return false }
func (q *questionTool) IsReadOnly(_ json.RawMessage) bool        { return true }
func (q *questionTool) IsDestructive(_ json.RawMessage) bool     { return false }
func (q *questionTool) Prompt(_ tool.PromptOptions) string       { return "" }

func (q *questionTool) Call(_ context.Context, _ json.RawMessage, _ *tool.Context) (*tool.Result, error) {
	// TODO(phase2): block via Asker until the user responds.
	return &tool.Result{Content: "TODO: Question tool not yet implemented"}, nil
}
