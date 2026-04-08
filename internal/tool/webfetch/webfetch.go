// Package webfetch implements the WebFetch tool.
//
// The WebFetch tool fetches a URL and returns its content converted to
// Markdown (HTML → Markdown) or as plain text. Images and binary content
// are omitted unless the model supports vision and the caller opts in.
//
// This package self-registers via init().
package webfetch

import (
	"context"
	"encoding/json"

	"github.com/valpere/kvach/internal/tool"
)

// MaxOutputBytes is the maximum byte length returned for a single fetch.
const MaxOutputBytes = 300_000

// Input is the schema for a WebFetch tool call.
type Input struct {
	// URL is the URL to fetch.
	URL string `json:"url"`
	// Format controls the output format: "markdown" (default) or "text".
	Format string `json:"format,omitempty"`
	// Timeout is the request timeout in seconds. Default 30.
	Timeout int `json:"timeout,omitempty"`
}

type webFetchTool struct{}

func init() { tool.DefaultRegistry.Register(&webFetchTool{}) }

func (w *webFetchTool) Name() string      { return "WebFetch" }
func (w *webFetchTool) Aliases() []string { return nil }

func (w *webFetchTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"url":     map[string]any{"type": "string", "description": "URL to fetch."},
			"format":  map[string]any{"type": "string", "enum": []string{"markdown", "text"}, "description": "Output format."},
			"timeout": map[string]any{"type": "integer", "description": "Request timeout in seconds (default 30)."},
		},
		"required": []string{"url"},
	}
}

func (w *webFetchTool) ValidateInput(raw json.RawMessage) error {
	var in Input
	return json.Unmarshal(raw, &in)
}

func (w *webFetchTool) CheckPermissions(_ json.RawMessage, _ *tool.Context) tool.PermissionOutcome {
	return tool.PermissionOutcome{Decision: "allow"}
}

func (w *webFetchTool) IsEnabled(_ *tool.Context) bool           { return true }
func (w *webFetchTool) IsConcurrencySafe(_ json.RawMessage) bool { return true }
func (w *webFetchTool) IsReadOnly(_ json.RawMessage) bool        { return true }
func (w *webFetchTool) IsDestructive(_ json.RawMessage) bool     { return false }
func (w *webFetchTool) Prompt(_ tool.PromptOptions) string       { return "" }

func (w *webFetchTool) Call(_ context.Context, _ json.RawMessage, _ *tool.Context) (*tool.Result, error) {
	// TODO(phase2): implement HTTP fetch with HTML-to-Markdown conversion.
	return &tool.Result{Content: "TODO: WebFetch tool not yet implemented"}, nil
}
