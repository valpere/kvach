// Package google implements [provider.Provider] for Google Gemini via the
// Generative Language API (generativelanguage.googleapis.com).
//
// Authentication: reads GOOGLE_API_KEY (or GEMINI_API_KEY) from the
// environment, or accepts a key supplied via ProviderConfig.APIKey.
// Google Vertex AI authentication (OAuth / service account) is deferred to
// a later phase.
package google

import (
	"context"

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
}

// New returns a Provider using the given API key.
func New(apiKey string) *Provider {
	return &Provider{apiKey: apiKey, baseURL: DefaultBaseURL}
}

// ID implements [provider.Provider].
func (p *Provider) ID() string { return ProviderID }

// Name implements [provider.Provider].
func (p *Provider) Name() string { return "Google" }

// Models implements [provider.Provider].
func (p *Provider) Models(_ context.Context) ([]provider.Model, error) {
	// TODO(phase2): return hardcoded Gemini model list; fetch from API later.
	return nil, nil
}

// Stream implements [provider.Provider].
func (p *Provider) Stream(_ context.Context, _ *provider.StreamRequest) (<-chan provider.StreamEvent, error) {
	// TODO(phase2): implement Gemini streaming (different SSE format from
	// Anthropic/OpenAI).
	ch := make(chan provider.StreamEvent)
	close(ch)
	return ch, nil
}
