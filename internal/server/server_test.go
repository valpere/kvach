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
	"github.com/valpere/kvach/internal/permission"
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
	if resp.RunID == "" {
		t.Fatal("expected non-empty run id")
	}
	if resp.FinishReason != "completed" {
		t.Fatalf("finishReason = %q, want %q", resp.FinishReason, "completed")
	}
	if resp.Usage.InputTokens != 11 || resp.Usage.OutputTokens != 7 {
		t.Fatalf("unexpected usage: %#v", resp.Usage)
	}

	runsReq := httptest.NewRequest(http.MethodGet, "/session/sess-prompt/runs", nil)
	runsRR := httptest.NewRecorder()
	h.ServeHTTP(runsRR, runsReq)
	if runsRR.Code != http.StatusOK {
		t.Fatalf("runs status = %d, want %d: %s", runsRR.Code, http.StatusOK, runsRR.Body.String())
	}
	var runs runsResponse
	if err := json.Unmarshal(runsRR.Body.Bytes(), &runs); err != nil {
		t.Fatalf("decode runs response: %v", err)
	}
	if runs.Active != nil {
		t.Fatalf("expected no active run, got %#v", runs.Active)
	}
	if len(runs.Runs) == 0 {
		t.Fatal("expected run history entries")
	}
	if runs.Runs[0].RunID != resp.RunID || runs.Runs[0].Status != runStatusCompleted {
		t.Fatalf("unexpected latest run entry: %#v", runs.Runs[0])
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

	req = httptest.NewRequest(http.MethodGet, "/provider/openai", nil)
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("provider detail status = %d, want %d: %s", rr.Code, http.StatusOK, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "\"id\":\"openai\"") || !strings.Contains(rr.Body.String(), "gpt-4o") {
		t.Fatalf("provider detail response unexpected: %s", rr.Body.String())
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

	req = httptest.NewRequest(http.MethodGet, "/provider/unknown", nil)
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("unknown provider detail status = %d, want %d", rr.Code, http.StatusNotFound)
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

func TestSessionCancelEndpoint(t *testing.T) {
	ctx := context.Background()
	store := openStoreForTest(t)
	workDir := t.TempDir()
	now := time.Now().UTC()

	if err := store.CreateSession(ctx, session.Session{
		ID:        "sess-cancel",
		ProjectID: git.SlugFromRoot(workDir),
		Directory: workDir,
		Title:     "cancel",
		CreatedAt: now,
		UpdatedAt: now,
	}); err != nil {
		t.Fatalf("create session: %v", err)
	}

	factory := func(_ context.Context, _ AgentFactoryArgs) (AgentRunner, error) {
		return blockingRunner{}, nil
	}

	s := New(config.ServerConfig{}, Options{WorkDir: workDir, SessionStore: store, AgentFactory: factory})
	h := s.buildRouter()

	promptReq := httptest.NewRequest(http.MethodPost, "/session/sess-cancel/prompt", bytes.NewBufferString(`{"prompt":"long task"}`))
	promptRR := httptest.NewRecorder()

	done := make(chan struct{})
	go func() {
		h.ServeHTTP(promptRR, promptReq)
		close(done)
	}()

	time.Sleep(30 * time.Millisecond)

	runsReq := httptest.NewRequest(http.MethodGet, "/session/sess-cancel/runs", nil)
	runsRR := httptest.NewRecorder()
	h.ServeHTTP(runsRR, runsReq)
	if runsRR.Code != http.StatusOK {
		t.Fatalf("runs status = %d, want %d: %s", runsRR.Code, http.StatusOK, runsRR.Body.String())
	}
	var activeRuns runsResponse
	if err := json.Unmarshal(runsRR.Body.Bytes(), &activeRuns); err != nil {
		t.Fatalf("decode runs response: %v", err)
	}
	if activeRuns.Active == nil || activeRuns.Active.Status != runStatusRunning {
		t.Fatalf("expected running active run, got %#v", activeRuns.Active)
	}

	cancelReq := httptest.NewRequest(http.MethodPost, "/session/sess-cancel/cancel", nil)
	cancelRR := httptest.NewRecorder()
	h.ServeHTTP(cancelRR, cancelReq)
	if cancelRR.Code != http.StatusOK {
		t.Fatalf("cancel status = %d, want %d: %s", cancelRR.Code, http.StatusOK, cancelRR.Body.String())
	}

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("prompt request did not complete after cancellation")
	}

	if promptRR.Code != http.StatusOK {
		t.Fatalf("prompt status = %d, want %d: %s", promptRR.Code, http.StatusOK, promptRR.Body.String())
	}
	var resp promptResponse
	if err := json.Unmarshal(promptRR.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode prompt response: %v", err)
	}
	if resp.FinishReason != string(agent.ReasonAborted) {
		t.Fatalf("finishReason = %q, want %q", resp.FinishReason, string(agent.ReasonAborted))
	}
	if resp.RunID == "" {
		t.Fatal("expected run id in prompt response")
	}

	runsReq = httptest.NewRequest(http.MethodGet, "/session/sess-cancel/runs", nil)
	runsRR = httptest.NewRecorder()
	h.ServeHTTP(runsRR, runsReq)
	if runsRR.Code != http.StatusOK {
		t.Fatalf("runs status = %d, want %d: %s", runsRR.Code, http.StatusOK, runsRR.Body.String())
	}
	var cancelledRuns runsResponse
	if err := json.Unmarshal(runsRR.Body.Bytes(), &cancelledRuns); err != nil {
		t.Fatalf("decode cancelled runs response: %v", err)
	}
	if cancelledRuns.Active != nil {
		t.Fatalf("expected no active run after cancellation, got %#v", cancelledRuns.Active)
	}
	if len(cancelledRuns.Runs) == 0 {
		t.Fatal("expected cancelled run in history")
	}
	if cancelledRuns.Runs[0].RunID != resp.RunID || cancelledRuns.Runs[0].Status != runStatusCancelled {
		t.Fatalf("unexpected cancelled run entry: %#v", cancelledRuns.Runs[0])
	}

	cancelReq = httptest.NewRequest(http.MethodPost, "/session/sess-cancel/cancel", nil)
	cancelRR = httptest.NewRecorder()
	h.ServeHTTP(cancelRR, cancelReq)
	if cancelRR.Code != http.StatusConflict {
		t.Fatalf("second cancel status = %d, want %d", cancelRR.Code, http.StatusConflict)
	}
}

func TestSessionRunsFilteringAndPagination(t *testing.T) {
	ctx := context.Background()
	store := openStoreForTest(t)
	workDir := t.TempDir()
	now := time.Now().UTC()

	if err := store.CreateSession(ctx, session.Session{
		ID:        "sess-runs-filter",
		ProjectID: git.SlugFromRoot(workDir),
		Directory: workDir,
		Title:     "runs-filter",
		CreatedAt: now,
		UpdatedAt: now,
	}); err != nil {
		t.Fatalf("create session: %v", err)
	}

	s := New(config.ServerConfig{}, Options{WorkDir: workDir, SessionStore: store})
	h := s.buildRouter()

	finished := now.Add(1 * time.Minute)
	s.appendRunHistory("sess-runs-filter", runInfo{
		RunID:      "run-3",
		SessionID:  "sess-runs-filter",
		Status:     runStatusFailed,
		Model:      "openai/gpt-4o",
		Prompt:     "third",
		StartedAt:  now,
		FinishedAt: &finished,
	})
	s.appendRunHistory("sess-runs-filter", runInfo{
		RunID:      "run-2",
		SessionID:  "sess-runs-filter",
		Status:     runStatusCompleted,
		Model:      "openai/gpt-4o",
		Prompt:     "second",
		StartedAt:  now,
		FinishedAt: &finished,
	})
	s.appendRunHistory("sess-runs-filter", runInfo{
		RunID:      "run-1",
		SessionID:  "sess-runs-filter",
		Status:     runStatusCompleted,
		Model:      "openai/gpt-4o",
		Prompt:     "first",
		StartedAt:  now,
		FinishedAt: &finished,
	})

	s.registerRun("sess-runs-filter", activePromptRun{
		RunID:     "run-active",
		SessionID: "sess-runs-filter",
		Prompt:    "active",
		Model:     "openai/gpt-4o",
		Started:   now,
		Cancel:    func() {},
	})
	defer s.clearRun("sess-runs-filter", "run-active")

	req := httptest.NewRequest(http.MethodGet, "/session/sess-runs-filter/runs?status=completed&limit=1&offset=1", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("runs status = %d, want %d: %s", rr.Code, http.StatusOK, rr.Body.String())
	}

	var resp runsResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode runs response: %v", err)
	}
	if resp.Status != "completed" {
		t.Fatalf("status filter = %q, want %q", resp.Status, "completed")
	}
	if resp.Limit != 1 || resp.Offset != 1 {
		t.Fatalf("unexpected pagination fields: limit=%d offset=%d", resp.Limit, resp.Offset)
	}
	if resp.Total != 2 || resp.Count != 1 {
		t.Fatalf("unexpected total/count: total=%d count=%d", resp.Total, resp.Count)
	}
	if resp.Active != nil {
		t.Fatalf("expected active run hidden by completed status filter, got %#v", resp.Active)
	}
	if len(resp.Runs) != 1 || resp.Runs[0].RunID != "run-2" {
		t.Fatalf("unexpected paged runs: %#v", resp.Runs)
	}

	req = httptest.NewRequest(http.MethodGet, "/session/sess-runs-filter/runs", nil)
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("default runs code = %d, want %d", rr.Code, http.StatusOK)
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode default runs response: %v", err)
	}
	if resp.Limit != maxRunHistory {
		t.Fatalf("default limit = %d, want %d", resp.Limit, maxRunHistory)
	}

	req = httptest.NewRequest(http.MethodGet, "/session/sess-runs-filter/runs?status=completed&limit=1&offset=100", nil)
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("high offset runs code = %d, want %d", rr.Code, http.StatusOK)
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode high offset runs response: %v", err)
	}
	if resp.Count != 0 {
		t.Fatalf("high offset count = %d, want 0", resp.Count)
	}
	if !strings.Contains(rr.Body.String(), `"runs":[]`) {
		t.Fatalf("expected empty runs array in response, got: %s", rr.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/session/sess-runs-filter/runs?status=running", nil)
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("running status runs code = %d, want %d", rr.Code, http.StatusOK)
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode running runs response: %v", err)
	}
	if resp.Active == nil || resp.Active.RunID != "run-active" {
		t.Fatalf("expected active run in running filter, got %#v", resp.Active)
	}

	req = httptest.NewRequest(http.MethodGet, "/session/sess-runs-filter/runs?limit=bad", nil)
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("invalid limit status = %d, want %d", rr.Code, http.StatusBadRequest)
	}

	req = httptest.NewRequest(http.MethodGet, "/session/sess-runs-filter/runs?status=oops", nil)
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("invalid status code = %d, want %d", rr.Code, http.StatusBadRequest)
	}

	req = httptest.NewRequest(http.MethodGet, "/session/sess-runs-filter/runs?offset=-1", nil)
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("invalid offset code = %d, want %d", rr.Code, http.StatusBadRequest)
	}

	req = httptest.NewRequest(http.MethodGet, "/session/sess-runs-filter/runs?limit=500", nil)
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("limit clamp code = %d, want %d", rr.Code, http.StatusOK)
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode clamped runs response: %v", err)
	}
	if resp.Limit != 200 {
		t.Fatalf("clamped limit = %d, want 200", resp.Limit)
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

func TestPermissionResolveEndpoint(t *testing.T) {
	ctx := context.Background()
	store := openStoreForTest(t)
	workDir := t.TempDir()
	now := time.Now().UTC()

	if err := store.CreateSession(ctx, session.Session{
		ID:        "sess-perm",
		ProjectID: git.SlugFromRoot(workDir),
		Directory: workDir,
		Title:     "permission",
		CreatedAt: now,
		UpdatedAt: now,
	}); err != nil {
		t.Fatalf("create session: %v", err)
	}

	s := New(config.ServerConfig{}, Options{WorkDir: workDir, SessionStore: store})
	asker := s.newPermissionAsker("sess-perm")

	replyCh := make(chan permission.Reply, 1)
	errCh := make(chan error, 1)
	go func() {
		reply, err := asker.Ask(context.Background(), permission.Request{
			ID:          "perm-1",
			ToolName:    "Bash",
			Description: "run command",
			Risk:        "high",
		})
		if err != nil {
			errCh <- err
			return
		}
		replyCh <- reply
	}()

	deadline := time.Now().Add(1 * time.Second)
	for {
		s.permMu.RLock()
		_, ok := s.pendingPermission["sess-perm"]["perm-1"]
		s.permMu.RUnlock()
		if ok {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("permission request was not registered in time")
		}
		time.Sleep(10 * time.Millisecond)
	}

	h := s.buildRouter()
	req := httptest.NewRequest(http.MethodPost, "/session/sess-perm/permission/perm-1/resolve", bytes.NewBufferString(`{"decision":"allow_always","pattern":"git:*"}`))
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("resolve status = %d, want %d: %s", rr.Code, http.StatusOK, rr.Body.String())
	}

	select {
	case err := <-errCh:
		t.Fatalf("ask returned error: %v", err)
	case reply := <-replyCh:
		if reply.Decision != "allow_always" {
			t.Fatalf("reply decision = %q, want allow_always", reply.Decision)
		}
		if reply.Pattern != "git:*" {
			t.Fatalf("reply pattern = %q, want git:*", reply.Pattern)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for permission reply")
	}
}

func TestPermissionResolveEndpointValidation(t *testing.T) {
	ctx := context.Background()
	store := openStoreForTest(t)
	workDir := t.TempDir()
	now := time.Now().UTC()

	if err := store.CreateSession(ctx, session.Session{
		ID:        "sess-perm-2",
		ProjectID: git.SlugFromRoot(workDir),
		Directory: workDir,
		Title:     "permission-2",
		CreatedAt: now,
		UpdatedAt: now,
	}); err != nil {
		t.Fatalf("create session: %v", err)
	}

	s := New(config.ServerConfig{}, Options{WorkDir: workDir, SessionStore: store})
	h := s.buildRouter()

	req := httptest.NewRequest(http.MethodPost, "/session/sess-perm-2/permission/perm-missing/resolve", bytes.NewBufferString(`{"decision":"allow_once"}`))
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("missing request status = %d, want %d", rr.Code, http.StatusNotFound)
	}

	req = httptest.NewRequest(http.MethodPost, "/session/sess-perm-2/permission/perm-x/resolve", bytes.NewBufferString(`{"decision":"invalid"}`))
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("invalid decision status = %d, want %d", rr.Code, http.StatusBadRequest)
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

type blockingRunner struct{}

func (blockingRunner) Run(ctx context.Context, _ agent.RunOptions) (<-chan agent.Event, error) {
	ch := make(chan agent.Event, 1)
	go func() {
		<-ctx.Done()
		ch <- agent.Event{Type: agent.EventDone, Payload: string(agent.ReasonAborted)}
		close(ch)
	}()
	return ch, nil
}
