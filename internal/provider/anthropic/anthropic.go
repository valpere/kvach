// Package anthropic implements [provider.Provider] for the Anthropic Messages
// API.
//
// It handles streaming SSE responses, tool calling via the Anthropic tool-use
// format, and prompt caching headers.
//
// Authentication: reads ANTHROPIC_API_KEY from the environment, or accepts a
// key supplied via ProviderConfig.APIKey.
package anthropic

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/valpere/kvach/internal/provider"
)

const (
	// DefaultBaseURL is the Anthropic Messages API endpoint.
	DefaultBaseURL = "https://api.anthropic.com/v1"
	// ProviderID is the stable identifier for this provider.
	ProviderID = "anthropic"
	// APIVersion is the Anthropic API version header value.
	APIVersion = "2023-06-01"
)

// Provider implements [provider.Provider] for Anthropic.
type Provider struct {
	apiKey  string
	baseURL string
	client  *http.Client
}

// New returns a Provider using the given API key.
// If apiKey is empty the ANTHROPIC_API_KEY environment variable is used.
func New(apiKey, baseURL string) *Provider {
	if apiKey == "" {
		apiKey = os.Getenv("ANTHROPIC_API_KEY")
	}
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	return &Provider{apiKey: apiKey, baseURL: baseURL, client: http.DefaultClient}
}

// ID implements [provider.Provider].
func (p *Provider) ID() string { return ProviderID }

// Name implements [provider.Provider].
func (p *Provider) Name() string { return "Anthropic" }

// Models implements [provider.Provider].
func (p *Provider) Models(_ context.Context) ([]provider.Model, error) {
	return []provider.Model{
		{
			ID: "claude-sonnet-4-5", ProviderID: ProviderID, Name: "Claude Sonnet 4.5",
			Capabilities: provider.ModelCapabilities{ToolCalling: true, Vision: true},
			Limits:       provider.ModelLimits{Context: 200_000, Output: 8_192},
			Cost:         provider.ModelCost{InputPerMToken: 3.0, OutputPerMToken: 15.0, CacheRead: 0.30, CacheWrite: 3.75},
			Status:       "active",
		},
		{
			ID: "claude-opus-4-5", ProviderID: ProviderID, Name: "Claude Opus 4.5",
			Capabilities: provider.ModelCapabilities{ToolCalling: true, Vision: true, Reasoning: true},
			Limits:       provider.ModelLimits{Context: 200_000, Output: 8_192},
			Cost:         provider.ModelCost{InputPerMToken: 15.0, OutputPerMToken: 75.0, CacheRead: 1.50, CacheWrite: 18.75},
			Status:       "active",
		},
		{
			ID: "claude-haiku-4-5", ProviderID: ProviderID, Name: "Claude Haiku 4.5",
			Capabilities: provider.ModelCapabilities{ToolCalling: true, Vision: true},
			Limits:       provider.ModelLimits{Context: 200_000, Output: 8_192},
			Cost:         provider.ModelCost{InputPerMToken: 0.80, OutputPerMToken: 4.0, CacheRead: 0.08, CacheWrite: 1.0},
			Status:       "active",
		},
	}, nil
}

// Stream implements [provider.Provider]. It sends a streaming request to the
// Anthropic Messages API and returns a channel of StreamEvents parsed from
// the SSE response.
func (p *Provider) Stream(ctx context.Context, req *provider.StreamRequest) (<-chan provider.StreamEvent, error) {
	if p.apiKey == "" {
		return nil, fmt.Errorf("anthropic: ANTHROPIC_API_KEY is not set")
	}

	body, err := buildRequestBody(req)
	if err != nil {
		return nil, fmt.Errorf("anthropic: build request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", p.baseURL+"/messages", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("anthropic: create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-API-Key", p.apiKey)
	httpReq.Header.Set("Anthropic-Version", APIVersion)

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("anthropic: request failed: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("anthropic: API returned %d: %s", resp.StatusCode, string(body))
	}

	ch := make(chan provider.StreamEvent, 64)
	go func() {
		defer close(ch)
		defer resp.Body.Close()
		parseSSE(ctx, resp.Body, ch)
	}()
	return ch, nil
}

// --- Request building ---

type apiRequest struct {
	Model     string       `json:"model"`
	Messages  []apiMessage `json:"messages"`
	System    string       `json:"system,omitempty"`
	MaxTokens int          `json:"max_tokens"`
	Stream    bool         `json:"stream"`
	Tools     []apiTool    `json:"tools,omitempty"`
}

type apiMessage struct {
	Role    string     `json:"role"`
	Content []apiBlock `json:"content"`
}

type apiBlock struct {
	Type      string          `json:"type"`
	Text      string          `json:"text,omitempty"`
	ID        string          `json:"id,omitempty"`
	Name      string          `json:"name,omitempty"`
	Input     json.RawMessage `json:"input,omitempty"`
	ToolUseID string          `json:"tool_use_id,omitempty"`
	Content   string          `json:"content,omitempty"`
	IsError   bool            `json:"is_error,omitempty"`
}

type apiTool struct {
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	InputSchema map[string]any `json:"input_schema"`
}

func buildRequestBody(req *provider.StreamRequest) ([]byte, error) {
	apiReq := apiRequest{
		Model:     req.Model,
		System:    req.System,
		MaxTokens: req.MaxTokens,
		Stream:    true,
	}
	if apiReq.MaxTokens == 0 {
		apiReq.MaxTokens = 8192
	}

	// Convert messages.
	for _, msg := range req.Messages {
		am := apiMessage{Role: msg.Role}
		for _, part := range msg.Parts {
			switch part.Type {
			case provider.PartTypeText:
				am.Content = append(am.Content, apiBlock{Type: "text", Text: part.Text})
			case provider.PartTypeToolUse:
				if part.ToolUse != nil {
					am.Content = append(am.Content, apiBlock{
						Type:  "tool_use",
						ID:    part.ToolUse.ID,
						Name:  part.ToolUse.Name,
						Input: part.ToolUse.Input,
					})
				}
			case provider.PartTypeToolResult:
				if part.ToolResult != nil {
					am.Content = append(am.Content, apiBlock{
						Type:      "tool_result",
						ToolUseID: part.ToolResult.ToolUseID,
						Content:   part.ToolResult.Content,
						IsError:   part.ToolResult.IsError,
					})
				}
			}
		}
		if len(am.Content) == 0 {
			// Empty content — add a text block to satisfy the API.
			am.Content = []apiBlock{{Type: "text", Text: ""}}
		}
		apiReq.Messages = append(apiReq.Messages, am)
	}

	// Convert tools.
	for _, t := range req.Tools {
		apiReq.Tools = append(apiReq.Tools, apiTool{
			Name:        t.Name,
			Description: t.Description,
			InputSchema: t.InputSchema,
		})
	}

	return json.Marshal(apiReq)
}

// --- SSE parsing ---

// parseSSE reads an SSE stream and sends provider.StreamEvent values to ch.
func parseSSE(ctx context.Context, r io.Reader, ch chan<- provider.StreamEvent) {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	var eventType string

	for scanner.Scan() {
		if ctx.Err() != nil {
			return
		}

		line := scanner.Text()

		if line == "" {
			eventType = ""
			continue
		}

		if strings.HasPrefix(line, "event: ") {
			eventType = strings.TrimPrefix(line, "event: ")
			continue
		}

		if strings.HasPrefix(line, "data: ") {
			data := strings.TrimPrefix(line, "data: ")
			if data == "[DONE]" {
				return
			}
			handleSSEData(eventType, []byte(data), ch)
		}
	}
}

// SSE event data structures.
type sseContentBlockStart struct {
	Index        int `json:"index"`
	ContentBlock struct {
		Type string `json:"type"`
		ID   string `json:"id,omitempty"`
		Name string `json:"name,omitempty"`
		Text string `json:"text,omitempty"`
	} `json:"content_block"`
}

type sseContentBlockDelta struct {
	Index int `json:"index"`
	Delta struct {
		Type        string `json:"type"`
		Text        string `json:"text,omitempty"`
		PartialJSON string `json:"partial_json,omitempty"`
	} `json:"delta"`
}

type sseMessageDelta struct {
	Delta struct {
		StopReason string `json:"stop_reason"`
	} `json:"delta"`
	Usage struct {
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
}

type sseMessageStart struct {
	Message struct {
		Usage struct {
			InputTokens int `json:"input_tokens"`
		} `json:"usage"`
	} `json:"message"`
}

func handleSSEData(eventType string, data []byte, ch chan<- provider.StreamEvent) {
	switch eventType {
	case "message_start":
		var msg sseMessageStart
		if json.Unmarshal(data, &msg) == nil {
			ch <- provider.StreamEvent{
				Type:  provider.StreamEventMessageStart,
				Usage: &provider.UsageStats{InputTokens: msg.Message.Usage.InputTokens},
			}
		}

	case "content_block_start":
		var block sseContentBlockStart
		if json.Unmarshal(data, &block) == nil {
			switch block.ContentBlock.Type {
			case "tool_use":
				ch <- provider.StreamEvent{
					Type:      provider.StreamEventToolUseStart,
					ToolUseID: block.ContentBlock.ID,
					ToolName:  block.ContentBlock.Name,
				}
			case "text":
				// Text block start — emit any initial text.
				if block.ContentBlock.Text != "" {
					ch <- provider.StreamEvent{Type: provider.StreamEventTextDelta, Text: block.ContentBlock.Text}
				}
			}
		}

	case "content_block_delta":
		var delta sseContentBlockDelta
		if json.Unmarshal(data, &delta) == nil {
			switch delta.Delta.Type {
			case "text_delta":
				ch <- provider.StreamEvent{Type: provider.StreamEventTextDelta, Text: delta.Delta.Text}
			case "input_json_delta":
				ch <- provider.StreamEvent{Type: provider.StreamEventToolUseDelta, PartialJSON: delta.Delta.PartialJSON}
			case "thinking_delta":
				ch <- provider.StreamEvent{Type: provider.StreamEventReasoningDelta, Reasoning: delta.Delta.Text}
			}
		}

	case "content_block_stop":
		ch <- provider.StreamEvent{Type: provider.StreamEventToolUseEnd}

	case "message_delta":
		var msg sseMessageDelta
		if json.Unmarshal(data, &msg) == nil {
			ch <- provider.StreamEvent{
				Type:         provider.StreamEventMessageEnd,
				FinishReason: msg.Delta.StopReason,
				Usage:        &provider.UsageStats{OutputTokens: msg.Usage.OutputTokens},
			}
		}

	case "message_stop":
		// Final event — nothing to emit beyond what message_delta sent.

	case "error":
		var errData struct {
			Error struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		if json.Unmarshal(data, &errData) == nil {
			ch <- provider.StreamEvent{Type: provider.StreamEventError, Error: errData.Error.Message}
		}
	}
}
