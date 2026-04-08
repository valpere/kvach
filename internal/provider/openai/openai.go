// Package openai implements [provider.Provider] for the OpenAI Chat Completions
// API and any compatible endpoint (Groq, Together AI, Perplexity, DeepInfra,
// OpenRouter, Ollama, LM Studio, etc.).
//
// Authentication: reads OPENAI_API_KEY from the environment, or accepts a
// key supplied via ProviderConfig.APIKey. Custom base URLs are supported for
// compatible providers.
package openai

import (
	"context"

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
}

// New returns a Provider for the canonical OpenAI API.
func New(apiKey string) *Provider {
	return NewCompatible(ProviderID, "OpenAI", apiKey, DefaultBaseURL)
}

// NewCompatible returns a Provider for any OpenAI-compatible API endpoint.
// Use this constructor for Groq, OpenRouter, Ollama, etc.
func NewCompatible(id, name, apiKey, baseURL string) *Provider {
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	return &Provider{id: id, name: name, apiKey: apiKey, baseURL: baseURL}
}

// ID implements [provider.Provider].
func (p *Provider) ID() string { return p.id }

// Name implements [provider.Provider].
func (p *Provider) Name() string { return p.name }

// Models implements [provider.Provider].
func (p *Provider) Models(_ context.Context) ([]provider.Model, error) {
	// TODO(phase2): fetch from /v1/models endpoint.
	return nil, nil
}

// Stream implements [provider.Provider].
func (p *Provider) Stream(_ context.Context, _ *provider.StreamRequest) (<-chan provider.StreamEvent, error) {
	// TODO(phase1): implement SSE streaming against the Chat Completions API.
	ch := make(chan provider.StreamEvent)
	close(ch)
	return ch, nil
}
