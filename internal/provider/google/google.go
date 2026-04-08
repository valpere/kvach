// Package google implements [provider.Provider] for Google Gemini via the
// Generative Language API (generativelanguage.googleapis.com).
//
// Authentication: reads GOOGLE_API_KEY (or GEMINI_API_KEY) from the
// environment, or accepts a key supplied via ProviderConfig.APIKey.
// Google Vertex AI authentication (OAuth / service account) is deferred to
// a later phase.
package google

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/valpere/kvach/internal/provider"
)

const (
	// DefaultBaseURL is the Gemini REST API endpoint.
	DefaultBaseURL = "https://generativelanguage.googleapis.com/v1beta"
	// ProviderID is the stable identifier for this provider.
	ProviderID = "google"
)

// Provider implements [provider.Provider] for Google Gemini.
type Provider struct {
	apiKey  string
	baseURL string
	client  *http.Client
}

// New returns a Provider using the given API key.
func New(apiKey string) *Provider {
	if strings.TrimSpace(apiKey) == "" {
		apiKey = strings.TrimSpace(os.Getenv("GOOGLE_API_KEY"))
	}
	if strings.TrimSpace(apiKey) == "" {
		apiKey = strings.TrimSpace(os.Getenv("GEMINI_API_KEY"))
	}

	baseURL := strings.TrimSpace(os.Getenv("GOOGLE_BASE_URL"))
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}

	return &Provider{apiKey: apiKey, baseURL: baseURL, client: http.DefaultClient}
}

// ID implements [provider.Provider].
func (p *Provider) ID() string { return ProviderID }

// Name implements [provider.Provider].
func (p *Provider) Name() string { return "Google" }

// Models implements [provider.Provider].
func (p *Provider) Models(_ context.Context) ([]provider.Model, error) {
	return []provider.Model{
		{
			ID: "gemini-2.5-pro", ProviderID: ProviderID, Name: "Gemini 2.5 Pro",
			Capabilities: provider.ModelCapabilities{ToolCalling: true, Vision: true, Reasoning: true},
			Limits:       provider.ModelLimits{Context: 1_000_000, Output: 64_000},
			Cost:         provider.ModelCost{},
			Status:       "active",
		},
		{
			ID: "gemini-2.5-flash", ProviderID: ProviderID, Name: "Gemini 2.5 Flash",
			Capabilities: provider.ModelCapabilities{ToolCalling: true, Vision: true},
			Limits:       provider.ModelLimits{Context: 1_000_000, Output: 64_000},
			Cost:         provider.ModelCost{},
			Status:       "active",
		},
	}, nil
}

// Stream implements [provider.Provider].
func (p *Provider) Stream(ctx context.Context, req *provider.StreamRequest) (<-chan provider.StreamEvent, error) {
	if strings.TrimSpace(p.apiKey) == "" {
		return nil, fmt.Errorf("google: GOOGLE_API_KEY or GEMINI_API_KEY is not set")
	}

	body, err := buildRequestBody(req)
	if err != nil {
		return nil, fmt.Errorf("google: build request: %w", err)
	}

	model := strings.TrimSpace(req.Model)
	if model == "" {
		model = "gemini-2.5-flash"
	}

	endpoint := strings.TrimRight(p.baseURL, "/") + "/models/" + model + ":generateContent"
	u, err := url.Parse(endpoint)
	if err != nil {
		return nil, fmt.Errorf("google: parse endpoint: %w", err)
	}
	q := u.Query()
	q.Set("key", p.apiKey)
	u.RawQuery = q.Encode()

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, u.String(), bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("google: create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("google: request failed: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("google: API returned %d: %s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}

	var out generateContentResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		resp.Body.Close()
		return nil, fmt.Errorf("google: decode response: %w", err)
	}
	resp.Body.Close()

	ch := make(chan provider.StreamEvent, 64)
	go func() {
		defer close(ch)
		emitResponseAsEvents(out, ch)
	}()

	return ch, nil
}

type generateContentRequest struct {
	Contents          []geminiContent       `json:"contents,omitempty"`
	Tools             []geminiTool          `json:"tools,omitempty"`
	SystemInstruction *geminiSystemInstr    `json:"system_instruction,omitempty"`
	GenerationConfig  *geminiGenerationConf `json:"generation_config,omitempty"`
}

type geminiGenerationConf struct {
	MaxOutputTokens int      `json:"max_output_tokens,omitempty"`
	Temperature     *float64 `json:"temperature,omitempty"`
}

type geminiSystemInstr struct {
	Parts []geminiPart `json:"parts"`
}

type geminiContent struct {
	Role  string       `json:"role,omitempty"`
	Parts []geminiPart `json:"parts"`
}

type geminiPart struct {
	Text             string              `json:"text,omitempty"`
	FunctionCall     *geminiFunctionCall `json:"functionCall,omitempty"`
	FunctionResponse *geminiFunctionResp `json:"functionResponse,omitempty"`
}

type geminiFunctionCall struct {
	Name string         `json:"name"`
	Args map[string]any `json:"args,omitempty"`
}

type geminiFunctionResp struct {
	Name     string         `json:"name"`
	Response map[string]any `json:"response"`
}

type geminiTool struct {
	FunctionDeclarations []geminiFunctionDecl `json:"functionDeclarations"`
}

type geminiFunctionDecl struct {
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	Parameters  map[string]any `json:"parameters,omitempty"`
}

type generateContentResponse struct {
	Candidates []geminiCandidate `json:"candidates"`
	Usage      *geminiUsage      `json:"usageMetadata,omitempty"`
}

type geminiCandidate struct {
	Content      geminiContent `json:"content"`
	FinishReason string        `json:"finishReason,omitempty"`
}

type geminiUsage struct {
	PromptTokenCount     int `json:"promptTokenCount,omitempty"`
	CandidatesTokenCount int `json:"candidatesTokenCount,omitempty"`
}

func buildRequestBody(req *provider.StreamRequest) ([]byte, error) {
	r := generateContentRequest{}

	if strings.TrimSpace(req.System) != "" {
		r.SystemInstruction = &geminiSystemInstr{Parts: []geminiPart{{Text: req.System}}}
	}

	for _, msg := range req.Messages {
		content, ok := convertMessage(msg)
		if ok {
			r.Contents = append(r.Contents, content)
		}
	}

	if len(req.Tools) > 0 {
		decls := make([]geminiFunctionDecl, 0, len(req.Tools))
		for _, t := range req.Tools {
			decls = append(decls, geminiFunctionDecl{
				Name:        t.Name,
				Description: t.Description,
				Parameters:  t.InputSchema,
			})
		}
		r.Tools = []geminiTool{{FunctionDeclarations: decls}}
	}

	if req.MaxTokens > 0 || req.Temperature != nil {
		r.GenerationConfig = &geminiGenerationConf{}
		if req.MaxTokens > 0 {
			r.GenerationConfig.MaxOutputTokens = req.MaxTokens
		}
		if req.Temperature != nil {
			r.GenerationConfig.Temperature = req.Temperature
		}
	}

	return json.Marshal(r)
}

func convertMessage(msg provider.Message) (geminiContent, bool) {
	content := geminiContent{}
	switch msg.Role {
	case "assistant":
		content.Role = "model"
	default:
		content.Role = "user"
	}

	for _, part := range msg.Parts {
		switch part.Type {
		case provider.PartTypeText:
			if strings.TrimSpace(part.Text) != "" {
				content.Parts = append(content.Parts, geminiPart{Text: part.Text})
			}
		case provider.PartTypeToolUse:
			if msg.Role != "assistant" || part.ToolUse == nil {
				continue
			}
			args := map[string]any{}
			if len(part.ToolUse.Input) > 0 {
				_ = json.Unmarshal(part.ToolUse.Input, &args)
			}
			content.Parts = append(content.Parts, geminiPart{FunctionCall: &geminiFunctionCall{Name: part.ToolUse.Name, Args: args}})
		case provider.PartTypeToolResult:
			if msg.Role != "user" || part.ToolResult == nil {
				continue
			}
			content.Parts = append(content.Parts, geminiPart{FunctionResponse: &geminiFunctionResp{
				Name: part.ToolResult.ToolUseID,
				Response: map[string]any{
					"content":  part.ToolResult.Content,
					"is_error": part.ToolResult.IsError,
				},
			}})
		}
	}

	if len(content.Parts) == 0 {
		return geminiContent{}, false
	}
	return content, true
}

func emitResponseAsEvents(resp generateContentResponse, ch chan<- provider.StreamEvent) {
	toolCallN := 0
	finishReason := "stop"
	if len(resp.Candidates) > 0 {
		if mapped := mapFinishReason(resp.Candidates[0].FinishReason); mapped != "" {
			finishReason = mapped
		}
	}

	for _, cand := range resp.Candidates {
		for _, part := range cand.Content.Parts {
			if part.Text != "" {
				ch <- provider.StreamEvent{Type: provider.StreamEventTextDelta, Text: part.Text}
			}
			if part.FunctionCall != nil {
				toolCallN++
				id := fmt.Sprintf("gemini-tool-%d", toolCallN)
				args, _ := json.Marshal(part.FunctionCall.Args)
				ch <- provider.StreamEvent{Type: provider.StreamEventToolUseStart, ToolUseID: id, ToolName: part.FunctionCall.Name}
				ch <- provider.StreamEvent{Type: provider.StreamEventToolUseDelta, PartialJSON: string(args)}
				ch <- provider.StreamEvent{Type: provider.StreamEventToolUseEnd, ToolUseID: id, ToolName: part.FunctionCall.Name}
				finishReason = "tool_use"
			}
		}
	}

	usage := &provider.UsageStats{}
	if resp.Usage != nil {
		usage.InputTokens = resp.Usage.PromptTokenCount
		usage.OutputTokens = resp.Usage.CandidatesTokenCount
	}

	ch <- provider.StreamEvent{Type: provider.StreamEventMessageEnd, FinishReason: finishReason, Usage: usage}
}

func mapFinishReason(reason string) string {
	switch strings.ToUpper(strings.TrimSpace(reason)) {
	case "", "STOP":
		return "stop"
	case "MAX_TOKENS":
		return "length"
	case "SAFETY", "RECITATION", "OTHER", "BLOCKLIST", "PROHIBITED_CONTENT", "SPII":
		return "stop"
	case "MALFORMED_FUNCTION_CALL":
		return "tool_use"
	default:
		return "stop"
	}
}
