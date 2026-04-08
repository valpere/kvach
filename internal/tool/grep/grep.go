// Package grep implements the Grep tool.
//
// The Grep tool searches file contents for a regular expression pattern.
// It uses ripgrep (rg) when available on PATH, falling back to a pure-Go
// regexp walk. Results include file paths and line numbers.
//
// Insight from the Claude Code leak: "Search, don't index." Grep over
// ripgrep is faster, cheaper, and more reliable for code search than
// vector embeddings for the vast majority of queries.
//
// This package self-registers via init().
package grep

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/valpere/kvach/internal/tool"
)

// MaxResults is the maximum number of matching lines returned.
const MaxResults = 2000

// Input is the schema for a Grep tool call.
type Input struct {
	// Pattern is the regular expression to search for.
	Pattern string `json:"pattern"`
	// Path restricts the search to a sub-directory or file. Defaults to WorkDir.
	Path string `json:"path,omitempty"`
	// Include filters results to files matching this glob (e.g. "*.go").
	Include string `json:"include,omitempty"`
	// CaseInsensitive enables case-insensitive matching.
	CaseInsensitive bool `json:"case_insensitive,omitempty"`
}

type grepTool struct{}

func init() {
	tool.DefaultRegistry.Register(&grepTool{})
}

func (g *grepTool) Name() string      { return "Grep" }
func (g *grepTool) Aliases() []string { return []string{"GrepTool", "rg"} }

func (g *grepTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"pattern": map[string]any{
				"type":        "string",
				"description": "Regular expression to search for in file contents.",
			},
			"path": map[string]any{
				"type":        "string",
				"description": "Directory or file to search. Defaults to the working directory.",
			},
			"include": map[string]any{
				"type":        "string",
				"description": "Glob pattern to restrict which files are searched (e.g. \"*.go\").",
			},
			"case_insensitive": map[string]any{
				"type":        "boolean",
				"description": "Enable case-insensitive matching.",
			},
		},
		"required": []string{"pattern"},
	}
}

func (g *grepTool) ValidateInput(raw json.RawMessage) error {
	var in Input
	return json.Unmarshal(raw, &in)
}

func (g *grepTool) CheckPermissions(_ json.RawMessage, _ *tool.Context) tool.PermissionOutcome {
	return tool.PermissionOutcome{Decision: "allow"}
}

func (g *grepTool) IsEnabled(_ *tool.Context) bool           { return true }
func (g *grepTool) IsConcurrencySafe(_ json.RawMessage) bool { return true }
func (g *grepTool) IsReadOnly(_ json.RawMessage) bool        { return true }
func (g *grepTool) IsDestructive(_ json.RawMessage) bool     { return false }

func (g *grepTool) Prompt(_ tool.PromptOptions) string {
	return `## Grep tool

Use the Grep tool to search file contents using a regular expression.
Returns matching lines with file path and line number.
Use the include parameter to restrict to specific file types (e.g. "*.go").`
}

func (g *grepTool) Call(ctx context.Context, raw json.RawMessage, tctx *tool.Context) (*tool.Result, error) {
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

	// Try ripgrep first — it's faster and respects .gitignore.
	if result, err := grepWithRg(ctx, in, searchDir); err == nil {
		return result, nil
	}

	// Fallback: pure-Go walk + regexp.
	return grepFallback(in, searchDir)
}

func grepWithRg(ctx context.Context, in Input, dir string) (*tool.Result, error) {
	rgPath, err := exec.LookPath("rg")
	if err != nil {
		return nil, err
	}

	args := []string{
		"--line-number", "--no-heading", "--color=never",
		"--max-count=50", // per file
	}
	if in.CaseInsensitive {
		args = append(args, "--ignore-case")
	}
	if in.Include != "" {
		args = append(args, "--glob", in.Include)
	}
	args = append(args, in.Pattern, dir)

	cmd := exec.CommandContext(ctx, rgPath, args...)
	out, err := cmd.Output()
	// rg exits 1 when no matches — that's not an error for us.
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return &tool.Result{Content: "No matches found."}, nil
		}
		return nil, err
	}

	content := string(out)
	truncated := false
	lines := strings.Count(content, "\n")
	if lines > MaxResults {
		// Truncate to MaxResults lines.
		idx := 0
		for i := 0; i < MaxResults; i++ {
			next := strings.IndexByte(content[idx:], '\n')
			if next < 0 {
				break
			}
			idx += next + 1
		}
		content = content[:idx]
		truncated = true
	}

	return &tool.Result{Content: content, Truncated: truncated}, nil
}

func grepFallback(in Input, dir string) (*tool.Result, error) {
	flags := ""
	if in.CaseInsensitive {
		flags = "(?i)"
	}
	re, err := regexp.Compile(flags + in.Pattern)
	if err != nil {
		return nil, fmt.Errorf("invalid regex %q: %w", in.Pattern, err)
	}

	var b strings.Builder
	count := 0

	err = filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			if d != nil && d.IsDir() && strings.HasPrefix(d.Name(), ".") && path != dir {
				return filepath.SkipDir
			}
			return nil
		}
		if count >= MaxResults {
			return filepath.SkipAll
		}

		// Include filter.
		if in.Include != "" {
			matched, _ := filepath.Match(in.Include, d.Name())
			if !matched {
				return nil
			}
		}

		f, err := os.Open(path)
		if err != nil {
			return nil
		}
		defer f.Close()

		rel, _ := filepath.Rel(dir, path)
		scanner := bufio.NewScanner(f)
		lineNum := 0
		for scanner.Scan() {
			lineNum++
			if count >= MaxResults {
				break
			}
			line := scanner.Text()
			if re.MatchString(line) {
				b.WriteString(fmt.Sprintf("%s:%d:%s\n", rel, lineNum, line))
				count++
			}
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("grep walk: %w", err)
	}

	if count == 0 {
		return &tool.Result{Content: "No matches found."}, nil
	}
	return &tool.Result{Content: b.String(), Truncated: count >= MaxResults}, nil
}
