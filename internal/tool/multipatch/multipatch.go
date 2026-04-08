// Package multipatch implements the MultiEdit and ApplyPatch tools.
//
// MultiEdit applies multiple Edit operations to a single file atomically —
// useful when the agent needs to make several non-overlapping changes to one
// file in a single tool call.
//
// ApplyPatch applies a unified diff patch to one or more files, which is
// useful for large refactors where the model generates a patch rather than
// enumerating individual edits.
//
// Both tools self-register via init().
package multipatch

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/valpere/kvach/internal/tool"
)

// EditOperation is one replacement within a MultiEdit call.
type EditOperation struct {
	OldString string `json:"old_string"`
	NewString string `json:"new_string"`
}

// MultiEditInput is the schema for the MultiEdit tool call.
type MultiEditInput struct {
	// Path is the file to edit, relative to the session WorkDir.
	Path string `json:"path"`
	// Edits is the ordered list of replacements. Each OldString must be
	// unique within the file at the time it is applied.
	Edits []EditOperation `json:"edits"`
}

// ApplyPatchInput is the schema for the ApplyPatch tool call.
type ApplyPatchInput struct {
	// Patch is the unified diff string to apply.
	Patch string `json:"patch"`
}

// --- MultiEdit ---

type multiEditTool struct{}

func init() {
	tool.DefaultRegistry.Register(&multiEditTool{})
	tool.DefaultRegistry.Register(&applyPatchTool{})
}

func (m *multiEditTool) Name() string      { return "MultiEdit" }
func (m *multiEditTool) Aliases() []string { return nil }

func (m *multiEditTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path": map[string]any{"type": "string", "description": "File to edit."},
			"edits": map[string]any{
				"type": "array",
				"items": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"old_string": map[string]any{"type": "string"},
						"new_string": map[string]any{"type": "string"},
					},
					"required": []string{"old_string", "new_string"},
				},
				"description": "List of replacements to apply in order.",
			},
		},
		"required": []string{"path", "edits"},
	}
}

func (m *multiEditTool) ValidateInput(raw json.RawMessage) error {
	var in MultiEditInput
	return json.Unmarshal(raw, &in)
}

func (m *multiEditTool) CheckPermissions(_ json.RawMessage, _ *tool.Context) tool.PermissionOutcome {
	return tool.PermissionOutcome{Decision: "ask"}
}

func (m *multiEditTool) IsEnabled(_ *tool.Context) bool           { return true }
func (m *multiEditTool) IsConcurrencySafe(_ json.RawMessage) bool { return false }
func (m *multiEditTool) IsReadOnly(_ json.RawMessage) bool        { return false }
func (m *multiEditTool) IsDestructive(_ json.RawMessage) bool     { return false }
func (m *multiEditTool) Prompt(_ tool.PromptOptions) string       { return "" }

func (m *multiEditTool) Call(_ context.Context, raw json.RawMessage, tctx *tool.Context) (*tool.Result, error) {
	var in MultiEditInput
	if err := json.Unmarshal(raw, &in); err != nil {
		return nil, fmt.Errorf("parse input: %w", err)
	}
	if len(in.Edits) == 0 {
		return nil, fmt.Errorf("at least one edit is required")
	}

	path := in.Path
	if tctx != nil && tctx.WorkDir != "" && !filepath.IsAbs(path) {
		path = filepath.Join(tctx.WorkDir, path)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", in.Path, err)
	}

	content := string(data)

	// Apply edits sequentially. Each edit must match exactly once.
	for i, edit := range in.Edits {
		count := strings.Count(content, edit.OldString)
		switch count {
		case 0:
			return nil, fmt.Errorf("edit %d: old_string not found in %s", i, in.Path)
		case 1:
			content = strings.Replace(content, edit.OldString, edit.NewString, 1)
		default:
			return nil, fmt.Errorf("edit %d: found %d matches for old_string in %s; provide more context", i, count, in.Path)
		}
	}

	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return nil, fmt.Errorf("write %s: %w", in.Path, err)
	}

	return &tool.Result{Content: fmt.Sprintf("Applied %d edits to %s", len(in.Edits), in.Path)}, nil
}

// --- ApplyPatch ---

type applyPatchTool struct{}

func (a *applyPatchTool) Name() string      { return "ApplyPatch" }
func (a *applyPatchTool) Aliases() []string { return nil }

func (a *applyPatchTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"patch": map[string]any{"type": "string", "description": "Unified diff patch to apply."},
		},
		"required": []string{"patch"},
	}
}

func (a *applyPatchTool) ValidateInput(raw json.RawMessage) error {
	var in ApplyPatchInput
	return json.Unmarshal(raw, &in)
}

func (a *applyPatchTool) CheckPermissions(_ json.RawMessage, _ *tool.Context) tool.PermissionOutcome {
	return tool.PermissionOutcome{Decision: "ask"}
}

func (a *applyPatchTool) IsEnabled(_ *tool.Context) bool           { return true }
func (a *applyPatchTool) IsConcurrencySafe(_ json.RawMessage) bool { return false }
func (a *applyPatchTool) IsReadOnly(_ json.RawMessage) bool        { return false }
func (a *applyPatchTool) IsDestructive(_ json.RawMessage) bool     { return false }
func (a *applyPatchTool) Prompt(_ tool.PromptOptions) string       { return "" }

func (a *applyPatchTool) Call(_ context.Context, _ json.RawMessage, _ *tool.Context) (*tool.Result, error) {
	// TODO(phase2): implement using a Go unified-diff library.
	return &tool.Result{Content: "TODO: ApplyPatch tool not yet implemented"}, nil
}
