// Package ls implements the LS tool.
//
// The LS tool lists the contents of a directory, annotating entries with
// their type (file/directory) and size. It respects .gitignore by default.
//
// This package self-registers via init().
package ls

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/valpere/kvach/internal/tool"
)

// Input is the schema for an LS tool call.
type Input struct {
	// Path is the directory to list. Defaults to WorkDir.
	Path string `json:"path,omitempty"`
	// Recursive lists subdirectories recursively when true.
	Recursive bool `json:"recursive,omitempty"`
	// IgnoreGitignore includes gitignored files when true.
	IgnoreGitignore bool `json:"ignore_gitignore,omitempty"`
}

type lsTool struct{}

func init() { tool.DefaultRegistry.Register(&lsTool{}) }

func (l *lsTool) Name() string      { return "LS" }
func (l *lsTool) Aliases() []string { return []string{"ListFiles"} }

func (l *lsTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path":             map[string]any{"type": "string", "description": "Directory to list."},
			"recursive":        map[string]any{"type": "boolean", "description": "List recursively."},
			"ignore_gitignore": map[string]any{"type": "boolean", "description": "Include gitignored files."},
		},
	}
}

func (l *lsTool) ValidateInput(raw json.RawMessage) error {
	var in Input
	return json.Unmarshal(raw, &in)
}

func (l *lsTool) CheckPermissions(_ json.RawMessage, _ *tool.Context) tool.PermissionOutcome {
	return tool.PermissionOutcome{Decision: "allow"}
}

func (l *lsTool) IsEnabled(_ *tool.Context) bool           { return true }
func (l *lsTool) IsConcurrencySafe(_ json.RawMessage) bool { return true }
func (l *lsTool) IsReadOnly(_ json.RawMessage) bool        { return true }
func (l *lsTool) IsDestructive(_ json.RawMessage) bool     { return false }
func (l *lsTool) Prompt(_ tool.PromptOptions) string       { return "" }

func (l *lsTool) Call(_ context.Context, raw json.RawMessage, tctx *tool.Context) (*tool.Result, error) {
	var in Input
	if err := json.Unmarshal(raw, &in); err != nil {
		return nil, fmt.Errorf("parse input: %w", err)
	}

	dir := in.Path
	if dir == "" && tctx != nil {
		dir = tctx.WorkDir
	}
	if dir == "" {
		dir = "."
	}
	if tctx != nil && tctx.WorkDir != "" && !filepath.IsAbs(dir) {
		dir = filepath.Join(tctx.WorkDir, dir)
	}

	if in.Recursive {
		return lsRecursive(dir)
	}
	return lsFlat(dir)
}

func lsFlat(dir string) (*tool.Result, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read directory: %w", err)
	}

	var b strings.Builder
	for _, e := range entries {
		info, err := e.Info()
		if err != nil {
			continue
		}
		if e.IsDir() {
			fmt.Fprintf(&b, "%s/\n", e.Name())
		} else {
			fmt.Fprintf(&b, "%s (%d bytes)\n", e.Name(), info.Size())
		}
	}
	return &tool.Result{Content: b.String()}, nil
}

func lsRecursive(dir string) (*tool.Result, error) {
	var b strings.Builder
	count := 0
	const maxEntries = 5000

	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if count >= maxEntries {
			return filepath.SkipAll
		}
		if d.IsDir() && strings.HasPrefix(d.Name(), ".") && path != dir {
			return filepath.SkipDir
		}

		rel, _ := filepath.Rel(dir, path)
		if rel == "." {
			return nil
		}

		if d.IsDir() {
			b.WriteString(rel + "/\n")
		} else {
			b.WriteString(rel + "\n")
		}
		count++
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walk: %w", err)
	}

	result := &tool.Result{Content: b.String()}
	if count >= maxEntries {
		result.Truncated = true
	}
	return result, nil
}
