package google

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/valpere/kvach/internal/provider"
)

func TestModels(t *testing.T) {
	p := New("test-key")
	models, err := p.Models(t.Context())
	if err != nil {
		t.Fatalf("models: %v", err)
	}
	if len(models) == 0 {
		t.Fatal("expected at least one model")
	}
}

func TestStreamTextResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", r.Method)
		}
		if !strings.Contains(r.URL.Path, "/models/gemini-2.5-flash:generateContent") {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if r.URL.Query().Get("key") != "test-key" {
			t.Fatalf("unexpected API key query: %q", r.URL.Query().Get("key"))
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read body: %v", err)
		}
		if !strings.Contains(string(body), "hello") {
			t.Fatalf("expected input text in request body: %s", string(body))
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"candidates":[{"content":{"parts":[{"text":"hi from gemini"}]},"finishReason":"STOP"}],
			"usageMetadata":{"promptTokenCount":11,"candidatesTokenCount":7}
		}`))
	}))
	defer srv.Close()

	t.Setenv("GOOGLE_BASE_URL", srv.URL)
	p := New("test-key")

	stream, err := p.Stream(context.Background(), &provider.StreamRequest{
		Model: "gemini-2.5-flash",
		Messages: []provider.Message{{
			Role:  "user",
			Parts: []provider.Part{{Type: provider.PartTypeText, Text: "hello"}},
		}},
	})
	if err != nil {
		t.Fatalf("stream: %v", err)
	}

	events := collectEvents(stream)
	if len(events) < 2 {
		t.Fatalf("expected at least 2 events, got %d", len(events))
	}
	if events[0].Type != provider.StreamEventTextDelta || events[0].Text != "hi from gemini" {
		t.Fatalf("unexpected first event: %#v", events[0])
	}
	last := events[len(events)-1]
	if last.Type != provider.StreamEventMessageEnd {
		t.Fatalf("expected message_end as last event, got %#v", last)
	}
	if last.Usage == nil || last.Usage.InputTokens != 11 || last.Usage.OutputTokens != 7 {
		t.Fatalf("unexpected usage: %#v", last.Usage)
	}
}

func TestStreamFunctionCallResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"candidates":[{
				"content":{"parts":[{"functionCall":{"name":"Read","args":{"path":"README.md"}}}]},
				"finishReason":"STOP"
			}],
			"usageMetadata":{"promptTokenCount":20,"candidatesTokenCount":5}
		}`))
	}))
	defer srv.Close()

	t.Setenv("GOOGLE_BASE_URL", srv.URL)
	p := New("test-key")

	stream, err := p.Stream(context.Background(), &provider.StreamRequest{
		Model: "gemini-2.5-flash",
		Messages: []provider.Message{{
			Role:  "user",
			Parts: []provider.Part{{Type: provider.PartTypeText, Text: "read readme"}},
		}},
	})
	if err != nil {
		t.Fatalf("stream: %v", err)
	}

	events := collectEvents(stream)
	if len(events) < 4 {
		t.Fatalf("expected tool events + message_end, got %d", len(events))
	}

	if events[0].Type != provider.StreamEventToolUseStart || events[0].ToolName != "Read" {
		t.Fatalf("unexpected tool start event: %#v", events[0])
	}
	if events[1].Type != provider.StreamEventToolUseDelta {
		t.Fatalf("unexpected tool delta event: %#v", events[1])
	}
	var args map[string]any
	if err := json.Unmarshal([]byte(events[1].PartialJSON), &args); err != nil {
		t.Fatalf("tool delta is not valid JSON: %v (%q)", err, events[1].PartialJSON)
	}
	if args["path"] != "README.md" {
		t.Fatalf("unexpected tool args: %#v", args)
	}
	if events[2].Type != provider.StreamEventToolUseEnd {
		t.Fatalf("unexpected tool end event: %#v", events[2])
	}
	last := events[len(events)-1]
	if last.Type != provider.StreamEventMessageEnd || last.FinishReason != "tool_use" {
		t.Fatalf("unexpected final event: %#v", last)
	}
}

func TestStreamMissingAPIKey(t *testing.T) {
	t.Setenv("GOOGLE_API_KEY", "")
	t.Setenv("GEMINI_API_KEY", "")
	t.Setenv("GOOGLE_BASE_URL", "")

	p := New("")
	_, err := p.Stream(context.Background(), &provider.StreamRequest{})
	if err == nil {
		t.Fatal("expected error when API key is missing")
	}
}

func collectEvents(ch <-chan provider.StreamEvent) []provider.StreamEvent {
	var events []provider.StreamEvent
	for evt := range ch {
		events = append(events, evt)
	}
	return events
}
