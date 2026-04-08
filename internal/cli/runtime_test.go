package cli

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/valpere/kvach/internal/session"
)

func TestLatestSessionIDSkipsArchived(t *testing.T) {
	ctx := context.Background()
	rt := newTestRuntime(t)

	now := time.Now().UTC()
	archivedAt := now.Add(2 * time.Minute)

	if err := rt.store.CreateSession(ctx, session.Session{
		ID:         "old-active",
		ProjectID:  rt.projectID,
		Directory:  "/tmp/project",
		Title:      "old",
		CreatedAt:  now,
		UpdatedAt:  now,
		ArchivedAt: nil,
	}); err != nil {
		t.Fatalf("create old active session: %v", err)
	}

	if err := rt.store.CreateSession(ctx, session.Session{
		ID:         "new-archived",
		ProjectID:  rt.projectID,
		Directory:  "/tmp/project",
		Title:      "archived",
		CreatedAt:  now.Add(time.Minute),
		UpdatedAt:  now.Add(3 * time.Minute),
		ArchivedAt: &archivedAt,
	}); err != nil {
		t.Fatalf("create archived session: %v", err)
	}

	if err := rt.store.CreateSession(ctx, session.Session{
		ID:         "new-active",
		ProjectID:  rt.projectID,
		Directory:  "/tmp/project",
		Title:      "new",
		CreatedAt:  now.Add(2 * time.Minute),
		UpdatedAt:  now.Add(2 * time.Minute),
		ArchivedAt: nil,
	}); err != nil {
		t.Fatalf("create new active session: %v", err)
	}

	got, err := rt.latestSessionID(ctx)
	if err != nil {
		t.Fatalf("latestSessionID: %v", err)
	}
	if got != "new-active" {
		t.Fatalf("latestSessionID = %q, want %q", got, "new-active")
	}
}

func TestLatestSessionIDNotFound(t *testing.T) {
	rt := newTestRuntime(t)
	_, err := rt.latestSessionID(context.Background())
	if !errors.Is(err, session.ErrNotFound) {
		t.Fatalf("latestSessionID err = %v, want ErrNotFound", err)
	}
}

func TestResolveRunSessionID(t *testing.T) {
	rt := &agentRuntime{}

	if _, err := resolveRunSessionID(context.Background(), rt, "abc", true); err == nil {
		t.Fatal("expected mutual exclusivity error")
	}

	if got, err := resolveRunSessionID(context.Background(), rt, "abc", false); err != nil || got != "abc" {
		t.Fatalf("resume resolution mismatch: got=%q err=%v", got, err)
	}

	if got, err := resolveRunSessionID(context.Background(), rt, "", false); err != nil || got != "" {
		t.Fatalf("fresh resolution mismatch: got=%q err=%v", got, err)
	}
}

func TestResolveRunSessionIDContinueLatest(t *testing.T) {
	rt := newTestRuntime(t)
	ctx := context.Background()

	if _, err := resolveRunSessionID(ctx, rt, "", true); err == nil {
		t.Fatal("expected error when --continue has no sessions")
	}

	if err := rt.store.CreateSession(ctx, session.Session{
		ID:        "latest",
		ProjectID: rt.projectID,
		Directory: "/tmp/project",
		Title:     "latest",
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}); err != nil {
		t.Fatalf("create session: %v", err)
	}

	got, err := resolveRunSessionID(ctx, rt, "", true)
	if err != nil {
		t.Fatalf("resolve continue session: %v", err)
	}
	if got != "latest" {
		t.Fatalf("resolved session = %q, want latest", got)
	}
}

func newTestRuntime(t *testing.T) *agentRuntime {
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

	return &agentRuntime{
		store:     store,
		projectID: "proj",
	}
}
