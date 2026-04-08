// Package webfetch implements the WebFetch tool.
//
// The WebFetch tool fetches a URL and returns its content as plain text.
// HTML tags are stripped to produce a readable text representation. For full
// HTML-to-Markdown conversion a future iteration may use a library, but the
// current approach (strip tags, normalize whitespace) is zero-dependency and
// sufficient for most use cases.
//
// This package self-registers via init().
package webfetch

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

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
	if err := json.Unmarshal(raw, &in); err != nil {
		return err
	}
	if strings.TrimSpace(in.URL) == "" {
		return fmt.Errorf("url is required")
	}
	return nil
}

func (w *webFetchTool) CheckPermissions(_ json.RawMessage, _ *tool.Context) tool.PermissionOutcome {
	return tool.PermissionOutcome{Decision: "allow"}
}

func (w *webFetchTool) IsEnabled(_ *tool.Context) bool           { return true }
func (w *webFetchTool) IsConcurrencySafe(_ json.RawMessage) bool { return true }
func (w *webFetchTool) IsReadOnly(_ json.RawMessage) bool        { return true }
func (w *webFetchTool) IsDestructive(_ json.RawMessage) bool     { return false }
func (w *webFetchTool) Prompt(_ tool.PromptOptions) string       { return "" }

func (w *webFetchTool) Call(ctx context.Context, raw json.RawMessage, _ *tool.Context) (*tool.Result, error) {
	var in Input
	if err := json.Unmarshal(raw, &in); err != nil {
		return nil, fmt.Errorf("parse input: %w", err)
	}

	url := strings.TrimSpace(in.URL)
	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
		url = "https://" + url
	}

	timeout := in.Timeout
	if timeout <= 0 {
		timeout = 30
	}
	if timeout > 120 {
		timeout = 120
	}

	client := &http.Client{Timeout: time.Duration(timeout) * time.Second}
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("User-Agent", "kvach/1.0 (AI coding agent)")
	req.Header.Set("Accept", "text/html, application/xhtml+xml, text/plain, */*")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch %s: %w", in.URL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 400 {
		return nil, fmt.Errorf("fetch %s: HTTP %d %s", in.URL, resp.StatusCode, resp.Status)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, int64(MaxOutputBytes*2)))
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	content := string(body)

	// Strip HTML tags for a readable text representation.
	contentType := resp.Header.Get("Content-Type")
	if strings.Contains(contentType, "html") || strings.Contains(content, "<html") {
		content = stripHTML(content)
	}

	truncated := false
	if len(content) > MaxOutputBytes {
		content = content[:MaxOutputBytes]
		truncated = true
	}

	return &tool.Result{Content: content, Truncated: truncated}, nil
}

var (
	reScript = regexp.MustCompile(`(?is)<script[^>]*>.*?</script>`)
	reStyle  = regexp.MustCompile(`(?is)<style[^>]*>.*?</style>`)
	reTag    = regexp.MustCompile(`<[^>]+>`)
	reSpace  = regexp.MustCompile(`[ \t]+`)
	reLines  = regexp.MustCompile(`\n{3,}`)
)

// stripHTML removes script/style blocks, then all HTML tags, then normalizes
// whitespace. This is a lightweight tag stripper, not a full parser.
func stripHTML(s string) string {
	s = reScript.ReplaceAllString(s, "")
	s = reStyle.ReplaceAllString(s, "")
	s = reTag.ReplaceAllString(s, " ")
	s = strings.ReplaceAll(s, "&amp;", "&")
	s = strings.ReplaceAll(s, "&lt;", "<")
	s = strings.ReplaceAll(s, "&gt;", ">")
	s = strings.ReplaceAll(s, "&quot;", "\"")
	s = strings.ReplaceAll(s, "&#39;", "'")
	s = strings.ReplaceAll(s, "&nbsp;", " ")
	s = reSpace.ReplaceAllString(s, " ")
	s = reLines.ReplaceAllString(s, "\n\n")
	return strings.TrimSpace(s)
}
