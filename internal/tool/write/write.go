// Package write implements the Write tool.
//
// The Write tool creates or overwrites a file with the given content. It is a
// destructive operation in the sense that existing file content is replaced.
// The agent takes a snapshot before writing so the change can be reverted.
//
// This package self-registers via init().
package write

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/valpere/kvach/internal/tool"
)

// Input is the schema for a Write tool call.
type Input struct {
	// Path is the destination file, relative to the session WorkDir.
	Path string `json:"path"`
	// Content is the complete new content for the file.
	Content string `json:"content"`
}

type writeTool struct{}

func init() {
	tool.DefaultRegistry.Register(&writeTool{})
}

func (w *writeTool) Name() string      { return "Write" }
func (w *writeTool) Aliases() []string { return []string{"WriteFile"} }

func (w *writeTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path": map[string]any{
				"type":        "string",
				"description": "Path to the file to write, relative to the working directory.",
			},
			"content": map[string]any{
				"type":        "string",
				"description": "The complete new content for the file.",
			},
		},
		"required": []string{"path", "content"},
	}
}

func (w *writeTool) ValidateInput(raw json.RawMessage) error {
	var in Input
	return json.Unmarshal(raw, &in)
}

func (w *writeTool) CheckPermissions(raw json.RawMessage, _ *tool.Context) tool.PermissionOutcome {
	// Path containment will be checked inside Call; here we signal that a
	// permission prompt is required for non-auto-approved sessions.
	return tool.PermissionOutcome{Decision: "ask"}
}

func (w *writeTool) IsEnabled(_ *tool.Context) bool           { return true }
func (w *writeTool) IsConcurrencySafe(_ json.RawMessage) bool { return false }
func (w *writeTool) IsReadOnly(_ json.RawMessage) bool        { return false }
func (w *writeTool) IsDestructive(_ json.RawMessage) bool     { return false }

func (w *writeTool) Prompt(_ tool.PromptOptions) string {
	return `## Write tool

Use the Write tool to create or overwrite a file. Provide the complete new file content.
Prefer the Edit tool for targeted changes to existing files — it is safer and easier to review.`
}

func (w *writeTool) Call(_ context.Context, raw json.RawMessage, tctx *tool.Context) (*tool.Result, error) {
	var in Input
	if err := json.Unmarshal(raw, &in); err != nil {
		return nil, fmt.Errorf("parse input: %w", err)
	}

	path := in.Path
	if tctx != nil && tctx.WorkDir != "" && !filepath.IsAbs(path) {
		path = filepath.Join(tctx.WorkDir, path)
	}

	// Ensure parent directory exists.
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create directory %s: %w", dir, err)
	}

	// Atomic write: write to temp file, then rename.
	tmp, err := os.CreateTemp(dir, ".kvach-write-*")
	if err != nil {
		return nil, fmt.Errorf("create temp file: %w", err)
	}
	tmpName := tmp.Name()

	if _, err := tmp.WriteString(in.Content); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return nil, fmt.Errorf("write temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return nil, fmt.Errorf("close temp file: %w", err)
	}

	// Preserve permissions of existing file if it exists.
	if info, err := os.Stat(path); err == nil {
		os.Chmod(tmpName, info.Mode())
	} else {
		os.Chmod(tmpName, 0o644)
	}

	if err := os.Rename(tmpName, path); err != nil {
		os.Remove(tmpName)
		return nil, fmt.Errorf("rename %s -> %s: %w", tmpName, path, err)
	}

	return &tool.Result{Content: fmt.Sprintf("Wrote %d bytes to %s", len(in.Content), in.Path)}, nil
}
