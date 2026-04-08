// Package openai implements [provider.Provider] for the OpenAI Chat Completions
// API and any compatible endpoint (Groq, Together AI, Perplexity, DeepInfra,
// OpenRouter, Ollama, LM Studio, etc.).
//
// Authentication: reads OPENAI_API_KEY from the environment, or accepts a
// key supplied via ProviderConfig.APIKey. Custom base URLs are supported for
// compatible providers.
package openai

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
	// DefaultBaseURL is the OpenAI Chat Completions API endpoint.
	DefaultBaseURL = "https://api.openai.com/v1"
	// ProviderID is the stable identifier for this provider.
	ProviderID = "openai"
)

// Provider implements [provider.Provider] for OpenAI-compatible APIs.
type Provider struct {
	id      string // may differ from ProviderID for compatible providers
	name    string
	apiKey  string
	baseURL string
	client  *http.Client
}

// New returns a Provider for the canonical OpenAI API.
func New(apiKey string) *Provider {
	return NewCompatible(ProviderID, "OpenAI", apiKey, DefaultBaseURL)
}

// NewCompatible returns a Provider for any OpenAI-compatible API endpoint.
// Use this constructor for Groq, OpenRouter, Ollama, etc.
func NewCompatible(id, name, apiKey, baseURL string) *Provider {
	if apiKey == "" {
		apiKey = os.Getenv("OPENAI_API_KEY")
	}
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	return &Provider{id: id, name: name, apiKey: apiKey, baseURL: baseURL, client: http.DefaultClient}
}

// ID implements [provider.Provider].
func (p *Provider) ID() string { return p.id }

// Name implements [provider.Provider].
func (p *Provider) Name() string { return p.name }

// Models implements [provider.Provider].
func (p *Provider) Models(_ context.Context) ([]provider.Model, error) {
	return []provider.Model{
		{
			ID: "gpt-4o", ProviderID: p.id, Name: "GPT-4o",
			Capabilities: provider.ModelCapabilities{ToolCalling: true, Vision: true},
			Limits:       provider.ModelLimits{Context: 128_000, Output: 16_384},
			Cost:         provider.ModelCost{InputPerMToken: 2.50, OutputPerMToken: 10.0},
			Status:       "active",
		},
		{
			ID: "gpt-4o-mini", ProviderID: p.id, Name: "GPT-4o Mini",
			Capabilities: provider.ModelCapabilities{ToolCalling: true, Vision: true},
			Limits:       provider.ModelLimits{Context: 128_000, Output: 16_384},
			Cost:         provider.ModelCost{InputPerMToken: 0.15, OutputPerMToken: 0.60},
			Status:       "active",
		},
		{
			ID: "o3-mini", ProviderID: p.id, Name: "o3-mini",
			Capabilities: provider.ModelCapabilities{ToolCalling: true, Reasoning: true},
			Limits:       provider.ModelLimits{Context: 200_000, Output: 100_000},
			Cost:         provider.ModelCost{InputPerMToken: 1.10, OutputPerMToken: 4.40},
			Status:       "active",
		},
	}, nil
}

// Stream implements [provider.Provider]. It sends a streaming request to the
// OpenAI-compatible Chat Completions API and returns a channel of StreamEvents.
func (p *Provider) Stream(ctx context.Context, req *provider.StreamRequest) (<-chan provider.StreamEvent, error) {
	if p.apiKey == "" {
		return nil, fmt.Errorf("%s: API key is not set", p.id)
	}

	body, err := buildRequestBody(req)
	if err != nil {
		return nil, fmt.Errorf("%s: build request: %w", p.id, err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", p.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("%s: create request: %w", p.id, err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("%s: request failed: %w", p.id, err)
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("%s: API returned %d: %s", p.id, resp.StatusCode, string(body))
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
	Model      string       `json:"model"`
	Messages   []apiMessage `json:"messages"`
	MaxTokens  int          `json:"max_tokens,omitempty"`
	Stream     bool         `json:"stream"`
	Tools      []apiTool    `json:"tools,omitempty"`
	StreamOpts *streamOpts  `json:"stream_options,omitempty"`
}

type streamOpts struct {
	IncludeUsage bool `json:"include_usage"`
}

type apiMessage struct {
	Role       string  `json:"role"`
	Content    any     `json:"content,omitempty"`
	ToolCalls  []apiTC `json:"tool_calls,omitempty"`
	ToolCallID string  `json:"tool_call_id,omitempty"`
}

type apiTC struct {
	ID       string    `json:"id"`
	Type     string    `json:"type"`
	Function apiTCFunc `json:"function"`
}

type apiTCFunc struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type apiTool struct {
	Type     string      `json:"type"`
	Function apiToolFunc `json:"function"`
}

type apiToolFunc struct {
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	Parameters  map[string]any `json:"parameters"`
}

func buildRequestBody(req *provider.StreamRequest) ([]byte, error) {
	apiReq := apiRequest{
		Model:      req.Model,
		MaxTokens:  req.MaxTokens,
		Stream:     true,
		StreamOpts: &streamOpts{IncludeUsage: true},
	}
	if apiReq.MaxTokens == 0 {
		apiReq.MaxTokens = 8192
	}

	// System message.
	if req.System != "" {
		apiReq.Messages = append(apiReq.Messages, apiMessage{
			Role:    "system",
			Content: req.System,
		})
	}

	// Convert messages.
	for _, msg := range req.Messages {
		am := convertMessage(msg)
		apiReq.Messages = append(apiReq.Messages, am...)
	}

	// Convert tools.
	for _, t := range req.Tools {
		apiReq.Tools = append(apiReq.Tools, apiTool{
			Type: "function",
			Function: apiToolFunc{
				Name:        t.Name,
				Description: t.Description,
				Parameters:  t.InputSchema,
			},
		})
	}

	return json.Marshal(apiReq)
}

func convertMessage(msg provider.Message) []apiMessage {
	switch msg.Role {
	case "user":
		return convertUserMessage(msg)
	case "assistant":
		return convertAssistantMessage(msg)
	default:
		return nil
	}
}

func convertUserMessage(msg provider.Message) []apiMessage {
	var msgs []apiMessage
	for _, part := range msg.Parts {
		switch part.Type {
		case provider.PartTypeText:
			msgs = append(msgs, apiMessage{Role: "user", Content: part.Text})
		case provider.PartTypeToolResult:
			if part.ToolResult != nil {
				msgs = append(msgs, apiMessage{
					Role:       "tool",
					Content:    part.ToolResult.Content,
					ToolCallID: part.ToolResult.ToolUseID,
				})
			}
		}
	}
	return msgs
}

func convertAssistantMessage(msg provider.Message) []apiMessage {
	am := apiMessage{Role: "assistant"}
	var textParts []string

	for _, part := range msg.Parts {
		switch part.Type {
		case provider.PartTypeText:
			textParts = append(textParts, part.Text)
		case provider.PartTypeToolUse:
			if part.ToolUse != nil {
				am.ToolCalls = append(am.ToolCalls, apiTC{
					ID:   part.ToolUse.ID,
					Type: "function",
					Function: apiTCFunc{
						Name:      part.ToolUse.Name,
						Arguments: string(part.ToolUse.Input),
					},
				})
			}
		}
	}

	if len(textParts) > 0 {
		am.Content = strings.Join(textParts, "")
	}
	return []apiMessage{am}
}

// --- SSE parsing ---

type sseChunk struct {
	Choices []sseChoice `json:"choices"`
	Usage   *sseUsage   `json:"usage,omitempty"`
}

type sseChoice struct {
	Delta        sseDelta `json:"delta"`
	FinishReason *string  `json:"finish_reason"`
}

type sseDelta struct {
	Content   string  `json:"content,omitempty"`
	ToolCalls []sseTC `json:"tool_calls,omitempty"`
}

type sseTC struct {
	Index    int       `json:"index"`
	ID       string    `json:"id,omitempty"`
	Type     string    `json:"type,omitempty"`
	Function sseTCFunc `json:"function,omitempty"`
}

type sseTCFunc struct {
	Name      string `json:"name,omitempty"`
	Arguments string `json:"arguments,omitempty"`
}

type sseUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
}

func parseSSE(ctx context.Context, r io.Reader, ch chan<- provider.StreamEvent) {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	ch <- provider.StreamEvent{Type: provider.StreamEventMessageStart}

	for scanner.Scan() {
		if ctx.Err() != nil {
			return
		}

		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			return
		}

		var chunk sseChunk
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue
		}

		// Usage (sent with stream_options.include_usage).
		if chunk.Usage != nil {
			ch <- provider.StreamEvent{
				Type: provider.StreamEventMessageEnd,
				Usage: &provider.UsageStats{
					InputTokens:  chunk.Usage.PromptTokens,
					OutputTokens: chunk.Usage.CompletionTokens,
				},
			}
		}

		if len(chunk.Choices) == 0 {
			continue
		}

		choice := chunk.Choices[0]
		delta := choice.Delta

		// Text content.
		if delta.Content != "" {
			ch <- provider.StreamEvent{Type: provider.StreamEventTextDelta, Text: delta.Content}
		}

		// Tool calls.
		for _, tc := range delta.ToolCalls {
			if tc.ID != "" {
				ch <- provider.StreamEvent{
					Type:      provider.StreamEventToolUseStart,
					ToolUseID: tc.ID,
					ToolName:  tc.Function.Name,
				}
			}
			if tc.Function.Arguments != "" {
				ch <- provider.StreamEvent{
					Type:        provider.StreamEventToolUseDelta,
					PartialJSON: tc.Function.Arguments,
				}
			}
		}

		// Finish reason.
		if choice.FinishReason != nil && *choice.FinishReason != "" {
			ch <- provider.StreamEvent{
				Type:         provider.StreamEventMessageEnd,
				FinishReason: *choice.FinishReason,
			}
		}
	}
}
