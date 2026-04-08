// Package edit implements the Edit tool.
//
// The Edit tool performs a targeted exact-string replacement within an
// existing file. It is safer than Write for modifications because:
//   - It only changes the specified substring, leaving the rest intact.
//   - The match must be unique — ambiguous replacements are rejected.
//   - The caller must read the file first (the agent is instructed to do so).
//
// This package self-registers via init().
package edit

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/valpere/kvach/internal/tool"
)

// Input is the schema for an Edit tool call.
type Input struct {
	// Path is the file to edit, relative to the session WorkDir.
	Path string `json:"path"`
	// OldString is the exact substring to find and replace. Must match exactly
	// once in the file.
	OldString string `json:"old_string"`
	// NewString is the replacement text.
	NewString string `json:"new_string"`
}

type editTool struct{}

func init() {
	tool.DefaultRegistry.Register(&editTool{})
}

func (e *editTool) Name() string      { return "Edit" }
func (e *editTool) Aliases() []string { return []string{"FileEdit", "str_replace_editor"} }

func (e *editTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path": map[string]any{
				"type":        "string",
				"description": "Path to the file to edit, relative to the working directory.",
			},
			"old_string": map[string]any{
				"type":        "string",
				"description": "The exact string to replace. Must appear exactly once in the file.",
			},
			"new_string": map[string]any{
				"type":        "string",
				"description": "The replacement string.",
			},
		},
		"required": []string{"path", "old_string", "new_string"},
	}
}

func (e *editTool) ValidateInput(raw json.RawMessage) error {
	var in Input
	return json.Unmarshal(raw, &in)
}

func (e *editTool) CheckPermissions(_ json.RawMessage, _ *tool.Context) tool.PermissionOutcome {
	return tool.PermissionOutcome{Decision: "ask"}
}

func (e *editTool) IsEnabled(_ *tool.Context) bool           { return true }
func (e *editTool) IsConcurrencySafe(_ json.RawMessage) bool { return false }
func (e *editTool) IsReadOnly(_ json.RawMessage) bool        { return false }
func (e *editTool) IsDestructive(_ json.RawMessage) bool     { return false }

func (e *editTool) Prompt(_ tool.PromptOptions) string {
	return `## Edit tool

Use the Edit tool to replace an exact string within a file.
The old_string must appear exactly once — include enough surrounding context to make it unique.
Always read the file first to get the exact current content before editing.`
}

func (e *editTool) Call(_ context.Context, raw json.RawMessage, tctx *tool.Context) (*tool.Result, error) {
	var in Input
	if err := json.Unmarshal(raw, &in); err != nil {
		return nil, fmt.Errorf("parse input: %w", err)
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
	count := strings.Count(content, in.OldString)

	switch count {
	case 0:
		return nil, fmt.Errorf("old_string not found in %s", in.Path)
	case 1:
		// Exactly one match — proceed.
	default:
		return nil, fmt.Errorf("found %d matches for old_string in %s; provide more context to make it unique", count, in.Path)
	}

	newContent := strings.Replace(content, in.OldString, in.NewString, 1)

	if err := os.WriteFile(path, []byte(newContent), 0o644); err != nil {
		return nil, fmt.Errorf("write %s: %w", in.Path, err)
	}

	return &tool.Result{Content: fmt.Sprintf("Edited %s: replaced 1 occurrence", in.Path)}, nil
}
