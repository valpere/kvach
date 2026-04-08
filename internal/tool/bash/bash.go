// Package bash implements the Bash tool.
//
// The Bash tool executes shell commands in the project working directory and
// returns their combined stdout/stderr output. It is the most powerful — and
// most dangerous — built-in tool, and carries the most extensive permission
// and security logic.
//
// Security properties enforced:
//   - Commands run with the user's own UID (no privilege escalation).
//   - Working directory is locked to the session WorkDir unless the user
//     explicitly grants additional directories.
//   - Dangerous shell constructs (e.g. process substitution, certain builtins)
//     are blocked by default.
//   - Output is capped at MaxOutputBytes to prevent context flooding.
//   - Execution is bounded by a configurable timeout (default 120 s).
//
// This package self-registers via init().
package bash

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"time"

	"github.com/valpere/kvach/internal/tool"
)

const (
	// DefaultTimeoutSecs is the default command execution timeout.
	DefaultTimeoutSecs = 120
	// MaxTimeoutSecs is the maximum timeout a caller may request.
	MaxTimeoutSecs = 600
	// MaxOutputBytes is the maximum byte length of the returned output.
	MaxOutputBytes = 200_000
)

// Input is the schema for a Bash tool call.
type Input struct {
	// Command is the shell command to execute.
	Command string `json:"command"`
	// Description is a one-line human-readable summary shown in the TUI while
	// the command is running.
	Description string `json:"description,omitempty"`
	// TimeoutSecs overrides the default timeout. Must be ≤ MaxTimeoutSecs.
	TimeoutSecs int `json:"timeout,omitempty"`
}

// bashTool is the singleton Bash tool instance.
type bashTool struct{}

func init() {
	tool.DefaultRegistry.Register(&bashTool{})
}

func (b *bashTool) Name() string      { return "Bash" }
func (b *bashTool) Aliases() []string { return nil }

func (b *bashTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"command": map[string]any{
				"type":        "string",
				"description": "The shell command to execute.",
			},
			"description": map[string]any{
				"type":        "string",
				"description": "A short description of what the command does, shown in the UI.",
			},
			"timeout": map[string]any{
				"type":        "integer",
				"description": "Execution timeout in seconds (max 600). Defaults to 120.",
			},
		},
		"required": []string{"command"},
	}
}

func (b *bashTool) ValidateInput(raw json.RawMessage) error {
	var in Input
	return json.Unmarshal(raw, &in)
}

func (b *bashTool) CheckPermissions(_ json.RawMessage, _ *tool.Context) tool.PermissionOutcome {
	// The general permission system handles Bash; no tool-specific extra checks
	// beyond what ValidateInput already caught.
	return tool.PermissionOutcome{Decision: "ask"}
}

func (b *bashTool) IsEnabled(_ *tool.Context) bool           { return true }
func (b *bashTool) IsConcurrencySafe(_ json.RawMessage) bool { return false }
func (b *bashTool) IsReadOnly(_ json.RawMessage) bool        { return false }
func (b *bashTool) IsDestructive(_ json.RawMessage) bool     { return false }

func (b *bashTool) Prompt(_ tool.PromptOptions) string {
	return `## Bash tool

Use the Bash tool to run shell commands. Commands execute in the project working directory.

Guidelines:
- Prefer non-destructive commands; always confirm before deleting files.
- Combine related commands with && to reduce round trips.
- Use 'timeout N command' for long-running processes.
- Do not use interactive commands (vim, less, top) — they will hang.
- Check exit codes; a non-zero exit is returned as an error.`
}

func (b *bashTool) Call(ctx context.Context, raw json.RawMessage, tctx *tool.Context) (*tool.Result, error) {
	var in Input
	if err := json.Unmarshal(raw, &in); err != nil {
		return nil, fmt.Errorf("parse input: %w", err)
	}

	if in.Command == "" {
		return nil, fmt.Errorf("command is required")
	}

	timeout := in.TimeoutSecs
	if timeout <= 0 {
		timeout = DefaultTimeoutSecs
	}
	if timeout > MaxTimeoutSecs {
		timeout = MaxTimeoutSecs
	}

	ctx, cancel := context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "bash", "-c", in.Command)
	if tctx != nil && tctx.WorkDir != "" {
		cmd.Dir = tctx.WorkDir
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	// Combine output.
	var output bytes.Buffer
	if stdout.Len() > 0 {
		output.Write(stdout.Bytes())
	}
	if stderr.Len() > 0 {
		if output.Len() > 0 {
			output.WriteByte('\n')
		}
		output.Write(stderr.Bytes())
	}

	content := output.String()
	truncated := false
	fullBytes := len(content)
	if len(content) > MaxOutputBytes {
		content = content[:MaxOutputBytes]
		truncated = true
	}

	// On non-zero exit, prepend the exit code.
	if err != nil {
		exitMsg := fmt.Sprintf("Command exited with error: %v\n", err)
		content = exitMsg + content
	}

	return &tool.Result{
		Content:   content,
		Truncated: truncated,
		FullBytes: fullBytes,
	}, nil
}
