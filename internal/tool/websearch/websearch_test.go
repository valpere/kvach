package websearch

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestIsEnabled(t *testing.T) {
	tool := &webSearchTool{}

	t.Setenv("KVACH_WEBSEARCH_PROVIDER", "")
	t.Setenv("TAVILY_API_KEY", "")
	t.Setenv("BRAVE_SEARCH_API_KEY", "")
	if tool.IsEnabled(nil) {
		t.Fatal("expected disabled when no API keys are set")
	}

	t.Setenv("TAVILY_API_KEY", "tavily-key")
	if !tool.IsEnabled(nil) {
		t.Fatal("expected enabled when Tavily key is set")
	}

	t.Setenv("KVACH_WEBSEARCH_PROVIDER", "brave")
	if tool.IsEnabled(nil) {
		t.Fatal("expected disabled when provider is brave but brave key is missing")
	}
}

func TestCallTavily(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", r.Method)
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read body: %v", err)
		}
		if !strings.Contains(string(body), `"query":"golang"`) {
			t.Fatalf("missing query in request body: %s", string(body))
		}
		if !strings.Contains(string(body), `"max_results":3`) {
			t.Fatalf("missing max_results in request body: %s", string(body))
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"results":[{"title":"Go","url":"https://go.dev","content":"The Go programming language"}]}`))
	}))
	defer srv.Close()

	t.Setenv("KVACH_WEBSEARCH_PROVIDER", "tavily")
	t.Setenv("TAVILY_API_KEY", "test-key")
	t.Setenv("KVACH_TAVILY_BASE_URL", srv.URL)
	t.Setenv("BRAVE_SEARCH_API_KEY", "")

	raw, _ := json.Marshal(Input{Query: "golang", NumResults: 3})
	res, err := (&webSearchTool{}).Call(t.Context(), raw, nil)
	if err != nil {
		t.Fatalf("call websearch: %v", err)
	}
	if !strings.Contains(res.Content, "https://go.dev") {
		t.Fatalf("missing URL in response: %s", res.Content)
	}
	if !strings.Contains(res.Content, "The Go programming language") {
		t.Fatalf("missing snippet in response: %s", res.Content)
	}
}

func TestCallBrave(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", r.Method)
		}
		if got := r.Header.Get("X-Subscription-Token"); got != "brave-key" {
			t.Fatalf("unexpected brave token: %q", got)
		}
		if got := r.URL.Query().Get("q"); got != "kvach" {
			t.Fatalf("unexpected query: %q", got)
		}
		if got := r.URL.Query().Get("count"); got != "2" {
			t.Fatalf("unexpected count: %q", got)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"web":{"results":[{"title":"KVACH","url":"https://example.com/kvach","description":"Agent project"}]}}`))
	}))
	defer srv.Close()

	t.Setenv("KVACH_WEBSEARCH_PROVIDER", "brave")
	t.Setenv("TAVILY_API_KEY", "")
	t.Setenv("BRAVE_SEARCH_API_KEY", "brave-key")
	t.Setenv("KVACH_BRAVE_BASE_URL", srv.URL)

	raw, _ := json.Marshal(Input{Query: "kvach", NumResults: 2})
	res, err := (&webSearchTool{}).Call(t.Context(), raw, nil)
	if err != nil {
		t.Fatalf("call websearch: %v", err)
	}
	if !strings.Contains(res.Content, "https://example.com/kvach") {
		t.Fatalf("missing URL in response: %s", res.Content)
	}
	if !strings.Contains(res.Content, "Agent project") {
		t.Fatalf("missing snippet in response: %s", res.Content)
	}
}

func TestCallNoBackendConfigured(t *testing.T) {
	t.Setenv("KVACH_WEBSEARCH_PROVIDER", "")
	t.Setenv("TAVILY_API_KEY", "")
	t.Setenv("BRAVE_SEARCH_API_KEY", "")

	raw, _ := json.Marshal(Input{Query: "anything"})
	_, err := (&webSearchTool{}).Call(t.Context(), raw, nil)
	if err == nil {
		t.Fatal("expected error when no web search backend is configured")
	}
}
