// Package glob implements the Glob tool.
//
// The Glob tool finds files in the project whose paths match a glob pattern.
// Results are sorted by modification time (most recently modified first) so
// the most relevant files appear at the top of the list.
//
// This package self-registers via init().
package glob

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/valpere/kvach/internal/tool"
)

// MaxResults is the maximum number of file paths returned.
const MaxResults = 1000

// Input is the schema for a Glob tool call.
type Input struct {
	// Pattern is the glob pattern to match (e.g. "**/*.go", "src/**/*.ts").
	Pattern string `json:"pattern"`
	// Path restricts the search to a sub-directory. Defaults to WorkDir.
	Path string `json:"path,omitempty"`
}

type globTool struct{}

func init() {
	tool.DefaultRegistry.Register(&globTool{})
}

func (g *globTool) Name() string      { return "Glob" }
func (g *globTool) Aliases() []string { return nil }

func (g *globTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"pattern": map[string]any{
				"type":        "string",
				"description": "Glob pattern to match file paths (e.g. \"**/*.go\").",
			},
			"path": map[string]any{
				"type":        "string",
				"description": "Directory to search in. Defaults to the working directory.",
			},
		},
		"required": []string{"pattern"},
	}
}

func (g *globTool) ValidateInput(raw json.RawMessage) error {
	var in Input
	return json.Unmarshal(raw, &in)
}

func (g *globTool) CheckPermissions(_ json.RawMessage, _ *tool.Context) tool.PermissionOutcome {
	return tool.PermissionOutcome{Decision: "allow"}
}

func (g *globTool) IsEnabled(_ *tool.Context) bool           { return true }
func (g *globTool) IsConcurrencySafe(_ json.RawMessage) bool { return true }
func (g *globTool) IsReadOnly(_ json.RawMessage) bool        { return true }
func (g *globTool) IsDestructive(_ json.RawMessage) bool     { return false }

func (g *globTool) Prompt(_ tool.PromptOptions) string {
	return `## Glob tool

Use the Glob tool to find files matching a pattern. Results are sorted by modification time.
Examples: "**/*.go", "src/**/*.ts", "*.md"`
}

func (g *globTool) Call(_ context.Context, raw json.RawMessage, tctx *tool.Context) (*tool.Result, error) {
	var in Input
	if err := json.Unmarshal(raw, &in); err != nil {
		return nil, fmt.Errorf("parse input: %w", err)
	}

	searchDir := in.Path
	if searchDir == "" && tctx != nil {
		searchDir = tctx.WorkDir
	}
	if searchDir == "" {
		searchDir = "."
	}

	pattern := in.Pattern

	// Collect matches via filepath.WalkDir for ** support.
	type fileEntry struct {
		path    string
		modTime int64
	}
	var matches []fileEntry

	err := filepath.WalkDir(searchDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // skip unreadable dirs
		}
		// Skip hidden directories (like .git).
		if d.IsDir() && strings.HasPrefix(d.Name(), ".") && path != searchDir {
			return filepath.SkipDir
		}
		if d.IsDir() {
			return nil
		}

		// Get path relative to searchDir for matching.
		rel, err := filepath.Rel(searchDir, path)
		if err != nil {
			return nil
		}

		// Match against the pattern. Try both the basename and the full rel path.
		matched, _ := filepath.Match(pattern, rel)
		if !matched {
			matched, _ = filepath.Match(pattern, filepath.Base(rel))
		}
		// Support **/ prefix by matching the pattern against just the filename
		// when the pattern contains **.
		if !matched && strings.Contains(pattern, "**") {
			// Strip leading **/ and match against the remaining pattern.
			simple := strings.TrimPrefix(pattern, "**/")
			matched, _ = filepath.Match(simple, filepath.Base(rel))
			if !matched {
				matched, _ = filepath.Match(simple, rel)
			}
		}

		if matched {
			var mtime int64
			if info, err := d.Info(); err == nil {
				mtime = info.ModTime().UnixNano()
			}
			matches = append(matches, fileEntry{path: rel, modTime: mtime})
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walk %s: %w", searchDir, err)
	}

	// Sort by modification time, newest first.
	sort.Slice(matches, func(i, j int) bool {
		return matches[i].modTime > matches[j].modTime
	})

	// Cap results.
	truncated := false
	if len(matches) > MaxResults {
		matches = matches[:MaxResults]
		truncated = true
	}

	var b strings.Builder
	for _, m := range matches {
		b.WriteString(m.path + "\n")
	}

	if len(matches) == 0 {
		return &tool.Result{Content: "No files found matching pattern: " + in.Pattern}, nil
	}

	return &tool.Result{
		Content:   b.String(),
		Truncated: truncated,
	}, nil
}
