// Package websearch implements the WebSearch tool.
//
// The WebSearch tool issues a web search query and returns a ranked list of
// results (title, URL, snippet). It supports Brave Search and Tavily as
// backends, selected by configuration.
//
// This package self-registers via init().
package websearch

import (
	"context"
	"encoding/json"

	"github.com/valpere/kvach/internal/tool"
)

// Input is the schema for a WebSearch tool call.
type Input struct {
	// Query is the search query string.
	Query string `json:"query"`
	// NumResults controls how many results to return. Default 5.
	NumResults int `json:"num_results,omitempty"`
}

type webSearchTool struct{}

func init() { tool.DefaultRegistry.Register(&webSearchTool{}) }

func (w *webSearchTool) Name() string      { return "WebSearch" }
func (w *webSearchTool) Aliases() []string { return nil }

func (w *webSearchTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"query":       map[string]any{"type": "string", "description": "Search query."},
			"num_results": map[string]any{"type": "integer", "description": "Number of results (default 5)."},
		},
		"required": []string{"query"},
	}
}

func (w *webSearchTool) ValidateInput(raw json.RawMessage) error {
	var in Input
	return json.Unmarshal(raw, &in)
}

func (w *webSearchTool) CheckPermissions(_ json.RawMessage, _ *tool.Context) tool.PermissionOutcome {
	return tool.PermissionOutcome{Decision: "allow"}
}

func (w *webSearchTool) IsEnabled(tctx *tool.Context) bool {
	// TODO(phase2): return false if no search API key is configured.
	return true
}

func (w *webSearchTool) IsConcurrencySafe(_ json.RawMessage) bool { return true }
func (w *webSearchTool) IsReadOnly(_ json.RawMessage) bool        { return true }
func (w *webSearchTool) IsDestructive(_ json.RawMessage) bool     { return false }
func (w *webSearchTool) Prompt(_ tool.PromptOptions) string       { return "" }

func (w *webSearchTool) Call(_ context.Context, _ json.RawMessage, _ *tool.Context) (*tool.Result, error) {
	// TODO(phase2): implement Brave / Tavily search.
	return &tool.Result{Content: "TODO: WebSearch tool not yet implemented"}, nil
}
