package server

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

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
