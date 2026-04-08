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
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

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
	if err := json.Unmarshal(raw, &in); err != nil {
		return err
	}
	if strings.TrimSpace(in.Question) == "" {
		return errors.New("question is required")
	}
	return nil
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

func (q *questionTool) Call(ctx context.Context, raw json.RawMessage, tctx *tool.Context) (*tool.Result, error) {
	var in Input
	if err := json.Unmarshal(raw, &in); err != nil {
		return nil, fmt.Errorf("parse input: %w", err)
	}

	if asker, ok := tctxAsker(tctx); ok {
		answer, err := asker.AskQuestion(ctx, in.Question, in.Options)
		if err != nil {
			return nil, err
		}
		return &tool.Result{Content: answer}, nil
	}

	// Fallback for CLI/headless when no custom asker is injected.
	answer, err := askViaStdin(ctx, in.Question, in.Options)
	if err != nil {
		return nil, err
	}
	return &tool.Result{Content: answer}, nil
}

type questionAsker interface {
	AskQuestion(ctx context.Context, question string, options []string) (string, error)
}

func tctxAsker(tctx *tool.Context) (questionAsker, bool) {
	if tctx == nil || tctx.Asker == nil {
		return nil, false
	}
	qa, ok := tctx.Asker.(questionAsker)
	return qa, ok
}

func askViaStdin(ctx context.Context, question string, options []string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}

	fmt.Fprintln(os.Stdout)
	fmt.Fprintln(os.Stdout, question)
	if len(options) > 0 {
		for i, opt := range options {
			fmt.Fprintf(os.Stdout, "%d) %s\n", i+1, opt)
		}
		fmt.Fprint(os.Stdout, "Select option number (or type exact value): ")
	} else {
		fmt.Fprint(os.Stdout, "Your answer: ")
	}

	scanner := bufio.NewScanner(os.Stdin)
	if !scanner.Scan() {
		if err := scanner.Err(); err != nil {
			return "", err
		}
		return "", errors.New("no answer provided")
	}
	ans := strings.TrimSpace(scanner.Text())
	if ans == "" {
		return "", errors.New("empty answer")
	}

	if len(options) > 0 {
		if n, err := strconv.Atoi(ans); err == nil {
			if n >= 1 && n <= len(options) {
				return options[n-1], nil
			}
		}
		for _, opt := range options {
			if ans == opt {
				return ans, nil
			}
		}
		return "", fmt.Errorf("invalid option %q", ans)
	}

	return ans, nil
}
