// Package ollama implements [provider.Provider] for Ollama, the local model
// runner (ollama.com).
//
// Ollama exposes an OpenAI-compatible /api/chat endpoint. This package wraps
// the openai provider with Ollama-specific defaults (no API key, local base
// URL, dynamic model discovery from the running Ollama daemon).
package ollama

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/valpere/kvach/internal/provider"
	compat "github.com/valpere/kvach/internal/provider/openai"
)

const (
	// DefaultBaseURL is the default Ollama API endpoint.
	DefaultBaseURL = "http://localhost:11434/v1"
	// ProviderID is the stable identifier for this provider.
	ProviderID = "ollama"
)

// Provider implements [provider.Provider] for Ollama.
type Provider struct {
	inner   *compat.Provider
	baseURL string
}

// New returns a Provider that communicates with a local Ollama daemon.
func New(baseURL string) *Provider {
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	return &Provider{
		inner:   compat.NewCompatible(ProviderID, "Ollama", "", baseURL),
		baseURL: baseURL,
	}
}

// ID implements [provider.Provider].
func (p *Provider) ID() string { return ProviderID }

// Name implements [provider.Provider].
func (p *Provider) Name() string { return "Ollama" }

// Models queries the running Ollama daemon for installed models.
func (p *Provider) Models(ctx context.Context) ([]provider.Model, error) {
	url := fmt.Sprintf("%s/../api/tags", p.baseURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		// Ollama is not running — return empty list, not an error.
		return nil, nil
	}
	defer resp.Body.Close()

	var result struct {
		Models []struct {
			Name string `json:"name"`
		} `json:"models"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	models := make([]provider.Model, 0, len(result.Models))
	for _, m := range result.Models {
		models = append(models, provider.Model{
			ID:         m.Name,
			ProviderID: ProviderID,
			Name:       m.Name,
			Capabilities: provider.ModelCapabilities{
				ToolCalling: true, // assume tool support; may not hold for all models
			},
			Status: "active",
		})
	}
	return models, nil
}

// Stream delegates to the OpenAI-compatible inner provider.
func (p *Provider) Stream(ctx context.Context, req *provider.StreamRequest) (<-chan provider.StreamEvent, error) {
	return p.inner.Stream(ctx, req)
}
