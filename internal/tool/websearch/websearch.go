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
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/valpere/kvach/internal/tool"
)

const defaultSearchTimeout = 20 * time.Second

type searchBackend string

const (
	backendTavily searchBackend = "tavily"
	backendBrave  searchBackend = "brave"
)

type searchResult struct {
	Title   string
	URL     string
	Snippet string
}

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
func (w *webSearchTool) Aliases() []string { return []string{"web_search"} }

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
	if err := json.Unmarshal(raw, &in); err != nil {
		return err
	}
	if strings.TrimSpace(in.Query) == "" {
		return fmt.Errorf("query is required")
	}
	if in.NumResults < 0 {
		return fmt.Errorf("num_results must be >= 0")
	}
	if in.NumResults > 20 {
		return fmt.Errorf("num_results must be <= 20")
	}
	return nil
}

func (w *webSearchTool) CheckPermissions(_ json.RawMessage, _ *tool.Context) tool.PermissionOutcome {
	return tool.PermissionOutcome{Decision: "allow"}
}

func (w *webSearchTool) IsEnabled(tctx *tool.Context) bool {
	_, ok := configuredBackend()
	return ok
}

func (w *webSearchTool) IsConcurrencySafe(_ json.RawMessage) bool { return true }
func (w *webSearchTool) IsReadOnly(_ json.RawMessage) bool        { return true }
func (w *webSearchTool) IsDestructive(_ json.RawMessage) bool     { return false }
func (w *webSearchTool) Prompt(_ tool.PromptOptions) string {
	return `## WebSearch tool

Use WebSearch for up-to-date web information when repository files are insufficient.
Returns ranked results with title, URL, and snippet.`
}

func (w *webSearchTool) Call(ctx context.Context, raw json.RawMessage, _ *tool.Context) (*tool.Result, error) {
	var in Input
	if err := json.Unmarshal(raw, &in); err != nil {
		return nil, fmt.Errorf("parse input: %w", err)
	}
	if strings.TrimSpace(in.Query) == "" {
		return nil, fmt.Errorf("query is required")
	}

	n := in.NumResults
	if n <= 0 {
		n = 5
	}
	if n > 20 {
		n = 20
	}

	backend, ok := configuredBackend()
	if !ok {
		return nil, fmt.Errorf("web search is not configured (set TAVILY_API_KEY or BRAVE_SEARCH_API_KEY)")
	}

	results, err := search(ctx, backend, strings.TrimSpace(in.Query), n)
	if err != nil {
		return nil, err
	}
	if len(results) == 0 {
		return &tool.Result{Content: "No web results found."}, nil
	}

	var b strings.Builder
	for i, r := range results {
		title := strings.TrimSpace(r.Title)
		if title == "" {
			title = "(untitled)"
		}
		snippet := compactWhitespace(r.Snippet)
		if len(snippet) > 500 {
			snippet = snippet[:500]
		}

		fmt.Fprintf(&b, "%d. %s\n", i+1, title)
		if u := strings.TrimSpace(r.URL); u != "" {
			fmt.Fprintf(&b, "URL: %s\n", u)
		}
		if snippet != "" {
			fmt.Fprintf(&b, "Snippet: %s\n", snippet)
		}
		if i != len(results)-1 {
			b.WriteString("\n")
		}
	}

	return &tool.Result{Content: b.String()}, nil
}

func configuredBackend() (searchBackend, bool) {
	want := strings.ToLower(strings.TrimSpace(os.Getenv("KVACH_WEBSEARCH_PROVIDER")))
	hasTavily := strings.TrimSpace(os.Getenv("TAVILY_API_KEY")) != ""
	hasBrave := strings.TrimSpace(os.Getenv("BRAVE_SEARCH_API_KEY")) != ""

	switch want {
	case string(backendTavily):
		if hasTavily {
			return backendTavily, true
		}
		return "", false
	case string(backendBrave):
		if hasBrave {
			return backendBrave, true
		}
		return "", false
	}

	if hasTavily {
		return backendTavily, true
	}
	if hasBrave {
		return backendBrave, true
	}
	return "", false
}

func search(ctx context.Context, backend searchBackend, query string, numResults int) ([]searchResult, error) {
	client := &http.Client{Timeout: defaultSearchTimeout}

	switch backend {
	case backendTavily:
		return searchTavily(ctx, client, query, numResults)
	case backendBrave:
		return searchBrave(ctx, client, query, numResults)
	default:
		return nil, fmt.Errorf("unsupported web search backend %q", backend)
	}
}

func searchTavily(ctx context.Context, client *http.Client, query string, numResults int) ([]searchResult, error) {
	apiKey := strings.TrimSpace(os.Getenv("TAVILY_API_KEY"))
	if apiKey == "" {
		return nil, fmt.Errorf("TAVILY_API_KEY is not set")
	}

	baseURL := strings.TrimSpace(os.Getenv("KVACH_TAVILY_BASE_URL"))
	if baseURL == "" {
		baseURL = "https://api.tavily.com/search"
	}

	body := map[string]any{
		"api_key":             apiKey,
		"query":               query,
		"max_results":         numResults,
		"search_depth":        "basic",
		"include_answer":      false,
		"include_images":      false,
		"include_raw_content": false,
	}
	rawBody, _ := json.Marshal(body)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL, strings.NewReader(string(rawBody)))
	if err != nil {
		return nil, fmt.Errorf("create Tavily request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Tavily request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 8192))
		return nil, fmt.Errorf("Tavily request failed: HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var payload struct {
		Results []struct {
			Title   string `json:"title"`
			URL     string `json:"url"`
			Content string `json:"content"`
		} `json:"results"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("decode Tavily response: %w", err)
	}

	results := make([]searchResult, 0, len(payload.Results))
	for _, r := range payload.Results {
		results = append(results, searchResult{Title: r.Title, URL: r.URL, Snippet: r.Content})
	}
	return results, nil
}

func searchBrave(ctx context.Context, client *http.Client, query string, numResults int) ([]searchResult, error) {
	apiKey := strings.TrimSpace(os.Getenv("BRAVE_SEARCH_API_KEY"))
	if apiKey == "" {
		return nil, fmt.Errorf("BRAVE_SEARCH_API_KEY is not set")
	}

	baseURL := strings.TrimSpace(os.Getenv("KVACH_BRAVE_BASE_URL"))
	if baseURL == "" {
		baseURL = "https://api.search.brave.com/res/v1/web/search"
	}

	u, err := url.Parse(baseURL)
	if err != nil {
		return nil, fmt.Errorf("parse Brave URL: %w", err)
	}
	q := u.Query()
	q.Set("q", query)
	q.Set("count", strconv.Itoa(numResults))
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("create Brave request: %w", err)
	}
	req.Header.Set("X-Subscription-Token", apiKey)
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Brave request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 8192))
		return nil, fmt.Errorf("Brave request failed: HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var payload struct {
		Web struct {
			Results []struct {
				Title       string `json:"title"`
				URL         string `json:"url"`
				Description string `json:"description"`
			} `json:"results"`
		} `json:"web"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("decode Brave response: %w", err)
	}

	results := make([]searchResult, 0, len(payload.Web.Results))
	for _, r := range payload.Web.Results {
		results = append(results, searchResult{Title: r.Title, URL: r.URL, Snippet: r.Description})
	}
	return results, nil
}

func compactWhitespace(s string) string {
	return strings.Join(strings.Fields(s), " ")
}
