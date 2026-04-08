package session

import (
	"context"
	"path/filepath"
	"testing"
)

func TestSQLiteStoreSessionLifecycle(t *testing.T) {
	ctx := context.Background()
	store, err := NewSQLiteStore(ctx, filepath.Join(t.TempDir(), "sessions.db"))
	if err != nil {
		t.Fatalf("new sqlite store: %v", err)
	}
	defer store.Close()

	s := Session{ID: "s1", ProjectID: "p1", Directory: "/tmp", Title: "test"}
	if err := store.CreateSession(ctx, s); err != nil {
		t.Fatalf("create session: %v", err)
	}

	got, err := store.GetSession(ctx, "s1")
	if err != nil {
		t.Fatalf("get session: %v", err)
	}
	if got.Title != "test" {
		t.Fatalf("expected title test, got %q", got.Title)
	}

	list, err := store.ListSessions(ctx, "p1")
	if err != nil {
		t.Fatalf("list sessions: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 session, got %d", len(list))
	}

	if err := store.ArchiveSession(ctx, "s1"); err != nil {
		t.Fatalf("archive session: %v", err)
	}
}

func TestSQLiteStoreMessageParts(t *testing.T) {
	ctx := context.Background()
	store, err := NewSQLiteStore(ctx, filepath.Join(t.TempDir(), "sessions.db"))
	if err != nil {
		t.Fatalf("new sqlite store: %v", err)
	}
	defer store.Close()

	if err := store.CreateSession(ctx, Session{ID: "s1", ProjectID: "p1", Directory: "/tmp", Title: "test"}); err != nil {
		t.Fatalf("create session: %v", err)
	}

	m := Message{ID: "m1", SessionID: "s1", Role: "user"}
	if err := store.AppendMessage(ctx, m); err != nil {
		t.Fatalf("append message: %v", err)
	}

	p := Part{ID: "p1", MessageID: "m1", Type: PartTypeText, Data: []byte(`{"text":"hello"}`)}
	if err := store.AppendPart(ctx, p); err != nil {
		t.Fatalf("append part: %v", err)
	}

	msgs, err := store.GetMessages(ctx, "s1")
	if err != nil {
		t.Fatalf("get messages: %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}

	parts, err := store.GetParts(ctx, "m1")
	if err != nil {
		t.Fatalf("get parts: %v", err)
	}
	if len(parts) != 1 {
		t.Fatalf("expected 1 part, got %d", len(parts))
	}
}
