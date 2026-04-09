package server

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/valpere/kvach/internal/agent"
	"github.com/valpere/kvach/internal/config"
	"github.com/valpere/kvach/internal/git"
	"github.com/valpere/kvach/internal/session"
)

func TestHealthz(t *testing.T) {
	s := New(config.ServerConfig{}, Options{})

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rr := httptest.NewRecorder()
	s.buildRouter().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}

	var body map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body["ok"] != true {
		t.Fatalf("unexpected health body: %#v", body)
	}
}

func TestBasicAuthMiddleware(t *testing.T) {
	s := New(config.ServerConfig{Password: "secret"}, Options{})
	h := s.withBasicAuth(s.buildRouter())

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("unauth status = %d, want %d", rr.Code, http.StatusUnauthorized)
	}

	req = httptest.NewRequest(http.MethodGet, "/healthz", nil)
	req.Header.Set("Authorization", basicAuthHeader("user", "secret"))
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("auth status = %d, want %d", rr.Code, http.StatusOK)
	}
}

func TestSessionEndpoints(t *testing.T) {
	ctx := context.Background()
	workDir := t.TempDir()
	store := openStoreForTest(t)

	projectID := git.SlugFromRoot(workDir)
	now := time.Now().UTC()
	if err := store.CreateSession(ctx, session.Session{
		ID:        "sess-1",
		ProjectID: projectID,
		Directory: workDir,
		Title:     "demo",
		CreatedAt: now,
		UpdatedAt: now,
	}); err != nil {
		t.Fatalf("create session: %v", err)
	}

	s := New(config.ServerConfig{}, Options{WorkDir: workDir, SessionStore: store})
	h := s.buildRouter()

	req := httptest.NewRequest(http.MethodGet, "/session", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("list status = %d, want %d", rr.Code, http.StatusOK)
	}
	if !strings.Contains(rr.Body.String(), "sess-1") {
		t.Fatalf("list response missing session id: %s", rr.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/session/sess-1", nil)
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("get status = %d, want %d", rr.Code, http.StatusOK)
	}

	req = httptest.NewRequest(http.MethodDelete, "/session/sess-1", nil)
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("archive status = %d, want %d", rr.Code, http.StatusOK)
	}

	sess, err := store.GetSession(ctx, "sess-1")
	if err != nil {
		t.Fatalf("get archived session: %v", err)
	}
	if sess.ArchivedAt == nil {
		t.Fatal("expected archived_at to be set")
	}
}

func TestSessionCreate(t *testing.T) {
	store := openStoreForTest(t)
	workDir := t.TempDir()

	s := New(config.ServerConfig{}, Options{WorkDir: workDir, SessionStore: store})
	h := s.buildRouter()

	body := bytes.NewBufferString(`{"title":"created from api"}`)
	req := httptest.NewRequest(http.MethodPost, "/session", body)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d: %s", rr.Code, http.StatusCreated, rr.Body.String())
	}

	var sess session.Session
	if err := json.Unmarshal(rr.Body.Bytes(), &sess); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if sess.ID == "" {
		t.Fatal("expected session id")
	}
	if sess.Title != "created from api" {
		t.Fatalf("title = %q, want %q", sess.Title, "created from api")
	}
}

func TestSessionMessages(t *testing.T) {
	ctx := context.Background()
	store := openStoreForTest(t)
	workDir := t.TempDir()
	now := time.Now().UTC()

	if err := store.CreateSession(ctx, session.Session{
		ID:        "sess-msg",
		ProjectID: git.SlugFromRoot(workDir),
		Directory: workDir,
		Title:     "messages",
		CreatedAt: now,
		UpdatedAt: now,
	}); err != nil {
		t.Fatalf("create session: %v", err)
	}

	if err := store.AppendMessage(ctx, session.Message{
		ID:        "msg-1",
		SessionID: "sess-msg",
		Role:      "assistant",
		CreatedAt: now,
	}); err != nil {
		t.Fatalf("append message: %v", err)
	}

	text, _ := json.Marshal(session.TextData{Text: "hello"})
	if err := store.AppendPart(ctx, session.Part{
		ID:        "part-1",
		MessageID: "msg-1",
		Type:      session.PartTypeText,
		Data:      text,
		CreatedAt: now,
	}); err != nil {
		t.Fatalf("append part: %v", err)
	}

	s := New(config.ServerConfig{}, Options{WorkDir: workDir, SessionStore: store})
	h := s.buildRouter()

	req := httptest.NewRequest(http.MethodGet, "/session/sess-msg/messages", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", rr.Code, http.StatusOK, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "hello") {
		t.Fatalf("response missing message content: %s", rr.Body.String())
	}
}

func TestSessionPromptAndEvents(t *testing.T) {
	ctx := context.Background()
	store := openStoreForTest(t)
	workDir := t.TempDir()
	now := time.Now().UTC()

	if err := store.CreateSession(ctx, session.Session{
		ID:        "sess-prompt",
		ProjectID: git.SlugFromRoot(workDir),
		Directory: workDir,
		Title:     "prompt",
		CreatedAt: now,
		UpdatedAt: now,
	}); err != nil {
		t.Fatalf("create session: %v", err)
	}

	factory := func(_ context.Context, _ AgentFactoryArgs) (AgentRunner, error) {
		return fakeRunner{events: []agent.Event{
			{Type: agent.EventTextDelta, Payload: "hello from fake"},
			{Type: agent.EventUsageUpdated, Payload: agent.UsageInfo{InputTokens: 11, OutputTokens: 7}},
			{Type: agent.EventDone, Payload: "completed"},
		}}, nil
	}

	s := New(config.ServerConfig{}, Options{WorkDir: workDir, SessionStore: store, AgentFactory: factory})
	h := s.buildRouter()

	eventsCtx, cancelEvents := context.WithCancel(context.Background())
	eventsReq := httptest.NewRequest(http.MethodGet, "/session/sess-prompt/events", nil).WithContext(eventsCtx)
	eventsRR := httptest.NewRecorder()

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		h.ServeHTTP(eventsRR, eventsReq)
	}()
	time.Sleep(20 * time.Millisecond)

	promptReq := httptest.NewRequest(http.MethodPost, "/session/sess-prompt/prompt", bytes.NewBufferString(`{"prompt":"say hi"}`))
	promptRR := httptest.NewRecorder()
	h.ServeHTTP(promptRR, promptReq)
	if promptRR.Code != http.StatusOK {
		t.Fatalf("prompt status = %d, want %d: %s", promptRR.Code, http.StatusOK, promptRR.Body.String())
	}

	var resp promptResponse
	if err := json.Unmarshal(promptRR.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode prompt response: %v", err)
	}
	if resp.Output != "hello from fake" {
		t.Fatalf("output = %q, want %q", resp.Output, "hello from fake")
	}
	if resp.FinishReason != "completed" {
		t.Fatalf("finishReason = %q, want %q", resp.FinishReason, "completed")
	}
	if resp.Usage.InputTokens != 11 || resp.Usage.OutputTokens != 7 {
		t.Fatalf("unexpected usage: %#v", resp.Usage)
	}

	cancelEvents()
	wg.Wait()

	if eventsRR.Code != http.StatusOK {
		t.Fatalf("events status = %d, want %d", eventsRR.Code, http.StatusOK)
	}
	if !strings.Contains(eventsRR.Body.String(), "event: text.delta") {
		t.Fatalf("expected text.delta SSE event, got: %s", eventsRR.Body.String())
	}
}

func TestProviderEndpoints(t *testing.T) {
	s := New(config.ServerConfig{}, Options{})
	h := s.buildRouter()

	req := httptest.NewRequest(http.MethodGet, "/provider", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("provider list status = %d, want %d", rr.Code, http.StatusOK)
	}
	if !strings.Contains(rr.Body.String(), "anthropic") || !strings.Contains(rr.Body.String(), "openai") {
		t.Fatalf("provider list missing expected providers: %s", rr.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/provider/openai/models", nil)
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("provider models status = %d, want %d: %s", rr.Code, http.StatusOK, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "gpt-4o") {
		t.Fatalf("provider models missing expected model: %s", rr.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/provider/unknown/models", nil)
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("unknown provider status = %d, want %d", rr.Code, http.StatusNotFound)
	}
}

func TestSessionPromptStreamingMode(t *testing.T) {
	ctx := context.Background()
	store := openStoreForTest(t)
	workDir := t.TempDir()
	now := time.Now().UTC()

	if err := store.CreateSession(ctx, session.Session{
		ID:        "sess-stream",
		ProjectID: git.SlugFromRoot(workDir),
		Directory: workDir,
		Title:     "stream",
		CreatedAt: now,
		UpdatedAt: now,
	}); err != nil {
		t.Fatalf("create session: %v", err)
	}

	factory := func(_ context.Context, _ AgentFactoryArgs) (AgentRunner, error) {
		return fakeRunner{events: []agent.Event{
			{Type: agent.EventTextDelta, Payload: "streamed chunk"},
			{Type: agent.EventDone, Payload: "completed"},
		}}, nil
	}

	s := New(config.ServerConfig{}, Options{WorkDir: workDir, SessionStore: store, AgentFactory: factory})
	h := s.buildRouter()

	req := httptest.NewRequest(http.MethodPost, "/session/sess-stream/prompt?stream=true", bytes.NewBufferString(`{"prompt":"hi"}`))
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("stream prompt status = %d, want %d: %s", rr.Code, http.StatusOK, rr.Body.String())
	}
	body := rr.Body.String()
	if !strings.Contains(body, "event: text.delta") {
		t.Fatalf("expected text.delta event in stream, got: %s", body)
	}
	if !strings.Contains(body, "event: response.completed") {
		t.Fatalf("expected response.completed event in stream, got: %s", body)
	}
}

func TestSessionEvents(t *testing.T) {
	s := New(config.ServerConfig{}, Options{})
	h := s.buildRouter()

	ctx, cancel := context.WithCancel(context.Background())
	req := httptest.NewRequest(http.MethodGet, "/session/s1/events", nil).WithContext(ctx)
	rr := httptest.NewRecorder()

	go func() {
		time.Sleep(10 * time.Millisecond)
		cancel()
	}()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("events status = %d, want %d", rr.Code, http.StatusOK)
	}
	if !strings.Contains(rr.Body.String(), "event: session.ready") {
		t.Fatalf("events body missing ready event: %s", rr.Body.String())
	}
}

func openStoreForTest(t *testing.T) *session.SQLiteStore {
	t.Helper()
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "sessions.db")
	store, err := session.NewSQLiteStore(ctx, dbPath)
	if err != nil {
		t.Fatalf("open sqlite store: %v", err)
	}
	t.Cleanup(func() {
		_ = store.Close()
	})
	return store
}

func basicAuthHeader(user, pass string) string {
	creds := user + ":" + pass
	return "Basic " + base64.StdEncoding.EncodeToString([]byte(creds))
}

type fakeRunner struct {
	events []agent.Event
	err    error
}

func (f fakeRunner) Run(_ context.Context, _ agent.RunOptions) (<-chan agent.Event, error) {
	if f.err != nil {
		return nil, f.err
	}
	ch := make(chan agent.Event, len(f.events))
	for _, evt := range f.events {
		ch <- evt
	}
	close(ch)
	return ch, nil
}
