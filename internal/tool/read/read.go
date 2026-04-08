// Package read implements the Read tool.
//
// The Read tool returns the contents of a file from the filesystem. It
// supports optional line-range selection (startLine / endLine) to fetch only
// the relevant portion of large files, reducing context usage.
//
// This package self-registers via init().
package read

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/valpere/kvach/internal/tool"
)

// MaxOutputBytes is the maximum byte length returned for a single read.
const MaxOutputBytes = 500_000

// Input is the schema for a Read tool call.
type Input struct {
	// Path is the file to read, relative to the session WorkDir.
	Path string `json:"path"`
	// StartLine, if > 0, returns output starting from this 1-based line.
	StartLine int `json:"start_line,omitempty"`
	// EndLine, if > 0, returns output up to and including this 1-based line.
	EndLine int `json:"end_line,omitempty"`
}

type readTool struct{}

func init() {
	tool.DefaultRegistry.Register(&readTool{})
}

func (r *readTool) Name() string      { return "Read" }
func (r *readTool) Aliases() []string { return []string{"ReadFile"} }

func (r *readTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path": map[string]any{
				"type":        "string",
				"description": "Path to the file to read, relative to the working directory.",
			},
			"start_line": map[string]any{
				"type":        "integer",
				"description": "1-based line number to start reading from.",
			},
			"end_line": map[string]any{
				"type":        "integer",
				"description": "1-based line number to stop reading at (inclusive).",
			},
		},
		"required": []string{"path"},
	}
}

func (r *readTool) ValidateInput(raw json.RawMessage) error {
	var in Input
	return json.Unmarshal(raw, &in)
}

func (r *readTool) CheckPermissions(_ json.RawMessage, _ *tool.Context) tool.PermissionOutcome {
	return tool.PermissionOutcome{Decision: "allow"}
}

func (r *readTool) IsEnabled(_ *tool.Context) bool           { return true }
func (r *readTool) IsConcurrencySafe(_ json.RawMessage) bool { return true }
func (r *readTool) IsReadOnly(_ json.RawMessage) bool        { return true }
func (r *readTool) IsDestructive(_ json.RawMessage) bool     { return false }

func (r *readTool) Prompt(_ tool.PromptOptions) string {
	return `## Read tool

Use the Read tool to view the contents of files. Provide a relative path.
Use start_line and end_line to fetch a specific range when you only need part of a large file.`
}

func (r *readTool) Call(_ context.Context, raw json.RawMessage, tctx *tool.Context) (*tool.Result, error) {
	var in Input
	if err := json.Unmarshal(raw, &in); err != nil {
		return nil, fmt.Errorf("parse input: %w", err)
	}

	path := in.Path
	if tctx != nil && tctx.WorkDir != "" && !filepath.IsAbs(path) {
		path = filepath.Join(tctx.WorkDir, path)
	}

	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("stat %s: %w", in.Path, err)
	}

	// Directory: return listing instead.
	if info.IsDir() {
		entries, err := os.ReadDir(path)
		if err != nil {
			return nil, fmt.Errorf("read directory %s: %w", in.Path, err)
		}
		var b strings.Builder
		for _, e := range entries {
			name := e.Name()
			if e.IsDir() {
				name += "/"
			}
			b.WriteString(name + "\n")
		}
		return &tool.Result{Content: b.String()}, nil
	}

	// File: read with optional line range.
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", in.Path, err)
	}
	defer f.Close()

	var b strings.Builder
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024) // up to 1MB per line
	lineNum := 0
	totalBytes := 0
	truncated := false

	for scanner.Scan() {
		lineNum++
		if in.StartLine > 0 && lineNum < in.StartLine {
			continue
		}
		if in.EndLine > 0 && lineNum > in.EndLine {
			break
		}
		line := fmt.Sprintf("%d: %s\n", lineNum, scanner.Text())
		totalBytes += len(line)
		if totalBytes > MaxOutputBytes {
			truncated = true
			break
		}
		b.WriteString(line)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read %s: %w", in.Path, err)
	}

	result := &tool.Result{Content: b.String(), Truncated: truncated}
	if truncated {
		result.FullBytes = totalBytes
	}
	return result, nil
}
