// Package task implements the Task tool.
//
// Task is the supervisor-to-subagent delegation entry point.
//
// The supervisor provides a short description plus full prompt instructions,
// chooses a specialist profile, and optionally resumes a prior delegated task.
// The subagent executes in an isolated context and returns a structured result
// envelope (summary, findings, changed files, next actions).
//
// This package self-registers via init().
package task

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/valpere/kvach/internal/multiagent"
	"github.com/valpere/kvach/internal/tool"
)

// Input is the schema for a Task tool call.
type Input struct {
	// Description is a short label shown in the TUI while the task runs.
	Description string `json:"description"`
	// Prompt is the full task instructions given to the subagent.
	Prompt string `json:"prompt"`
	// SubagentType selects the specialist profile (for example: "explore",
	// "general"). Defaults to "general".
	SubagentType string `json:"subagent_type,omitempty"`
	// Subagent is a legacy alias for SubagentType.
	Subagent string `json:"subagent,omitempty"`
	// TaskID resumes an existing delegated task when non-empty.
	TaskID string `json:"task_id,omitempty"`
	// Command records which supervisor command triggered this task.
	Command string `json:"command,omitempty"`
}

func (in Input) profile() string {
	if strings.TrimSpace(in.SubagentType) != "" {
		return strings.TrimSpace(in.SubagentType)
	}
	if strings.TrimSpace(in.Subagent) != "" {
		return strings.TrimSpace(in.Subagent)
	}
	return multiagent.DefaultProfileGeneral
}

func (in Input) validate() error {
	if strings.TrimSpace(in.Description) == "" {
		return errors.New("description is required")
	}
	if strings.TrimSpace(in.Prompt) == "" {
		return errors.New("prompt is required")
	}
	if len(in.Description) > 140 {
		return errors.New("description must be 140 characters or less")
	}
	return nil
}

type taskTool struct{}

func init() { tool.DefaultRegistry.Register(&taskTool{}) }

func (t *taskTool) Name() string      { return "Task" }
func (t *taskTool) Aliases() []string { return []string{"dispatch_agent"} }

func (t *taskTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"description": map[string]any{"type": "string", "description": "Short label shown while the delegated task runs."},
			"prompt":      map[string]any{"type": "string", "description": "Full instructions for the subagent."},
			"subagent_type": map[string]any{
				"type":        "string",
				"description": "Specialist profile to run (for example: \"general\", \"explore\").",
			},
			"subagent": map[string]any{
				"type":        "string",
				"description": "Legacy alias for subagent_type.",
			},
			"task_id": map[string]any{
				"type":        "string",
				"description": "Optional existing task ID to resume instead of starting a fresh subagent.",
			},
			"command": map[string]any{
				"type":        "string",
				"description": "Optional supervisor command that triggered this delegation (for audit trail).",
			},
		},
		"required": []string{"description", "prompt"},
	}
}

func (t *taskTool) ValidateInput(raw json.RawMessage) error {
	var in Input
	if err := json.Unmarshal(raw, &in); err != nil {
		return err
	}
	return in.validate()
}

func (t *taskTool) CheckPermissions(_ json.RawMessage, _ *tool.Context) tool.PermissionOutcome {
	return tool.PermissionOutcome{Decision: "allow"}
}

func (t *taskTool) IsEnabled(_ *tool.Context) bool           { return true }
func (t *taskTool) IsConcurrencySafe(_ json.RawMessage) bool { return false }
func (t *taskTool) IsReadOnly(_ json.RawMessage) bool        { return false }
func (t *taskTool) IsDestructive(_ json.RawMessage) bool     { return false }
func (t *taskTool) Prompt(_ tool.PromptOptions) string       { return "" }

func (t *taskTool) Call(ctx context.Context, raw json.RawMessage, tctx *tool.Context) (*tool.Result, error) {
	var in Input
	if err := json.Unmarshal(raw, &in); err != nil {
		return nil, err
	}
	if err := in.validate(); err != nil {
		return nil, err
	}

	profile := in.profile()
	resume := "new"
	if strings.TrimSpace(in.TaskID) != "" {
		resume = "resume"
	}

	runner, ok := any(nil), false
	if tctx != nil && tctx.TaskRunner != nil {
		runner, ok = tctx.TaskRunner, true
	}
	if ok {
		if r, ok := runner.(multiagent.Runner); ok {
			opts := multiagent.Options{
				TaskID:      strings.TrimSpace(in.TaskID),
				Profile:     profile,
				Description: in.Description,
				Prompt:      in.Prompt,
				Command:     in.Command,
			}
			opts.Normalize()
			if err := opts.Validate(); err != nil {
				return nil, err
			}

			res, err := r.Run(ctx, opts)
			if err != nil {
				return nil, err
			}

			content := res.Output
			if strings.TrimSpace(content) == "" {
				content = res.Contract.Raw
			}
			if strings.TrimSpace(content) == "" {
				content = fmt.Sprintf(
					"Task %s (%s) completed with state=%s\nSummary: %s\nFindings: %d\nChanged files: %d\nNext actions: %d",
					res.TaskID,
					res.Profile,
					res.State,
					res.Contract.Summary,
					len(res.Contract.Findings),
					len(res.Contract.ChangedFiles),
					len(res.Contract.NextActions),
				)
			}
			return &tool.Result{Content: content}, nil
		}
	}

	// TODO(phase2): wire up multiagent.Runner and return the real subagent
	// result envelope.
	content := fmt.Sprintf(
		"TODO: Task tool not yet implemented. mode=%s profile=%q description=%q command=%q\nExpected subagent output contract:\n- summary\n- findings[]\n- changed_files[]\n- next_actions[]",
		resume,
		profile,
		in.Description,
		in.Command,
	)
	return &tool.Result{Content: content}, nil
}
