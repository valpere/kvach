package todo

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/valpere/kvach/internal/bus"
	"github.com/valpere/kvach/internal/session"
	"github.com/valpere/kvach/internal/tool"
)

func TestTodoWritePersistsStateAndPublishesEvent(t *testing.T) {
	ctx := t.Context()
	store := openStore(t)

	if err := store.CreateSession(ctx, session.Session{
		ID:        "sess-1",
		ProjectID: "proj",
		Directory: t.TempDir(),
		Title:     "test",
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}); err != nil {
		t.Fatalf("create session: %v", err)
	}

	eventBus := bus.New()
	events, cancel := eventBus.Subscribe(func(e bus.Event) bool { return e.Type == EventTypeTodoUpdated })
	defer cancel()

	raw, _ := json.Marshal(Input{Todos: []Item{
		{Content: "Implement API", Status: StatusInProgress, Priority: PriorityHigh},
		{Content: "Write tests", Status: StatusPending, Priority: PriorityMedium},
	}})

	tt := &todoTool{}
	res, err := tt.Call(ctx, raw, &tool.Context{
		SessionID:    "sess-1",
		SessionStore: store,
		EventBus:     eventBus,
	})
	if err != nil {
		t.Fatalf("call todo tool: %v", err)
	}
	if !strings.Contains(res.Content, "2 items") {
		t.Fatalf("unexpected result summary: %q", res.Content)
	}

	msgs, err := store.GetMessages(ctx, "sess-1")
	if err != nil {
		t.Fatalf("get messages: %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("message count = %d, want 1", len(msgs))
	}
	if msgs[0].Role != "system" || msgs[0].AgentName != "TodoWrite" {
		t.Fatalf("unexpected persisted message: %#v", msgs[0])
	}

	parts, err := store.GetParts(ctx, msgs[0].ID)
	if err != nil {
		t.Fatalf("get parts: %v", err)
	}
	if len(parts) != 1 {
		t.Fatalf("part count = %d, want 1", len(parts))
	}
	if parts[0].Type != session.PartTypeTodo {
		t.Fatalf("part type = %q, want %q", parts[0].Type, session.PartTypeTodo)
	}

	var payload session.TodoData
	if err := json.Unmarshal(parts[0].Data, &payload); err != nil {
		t.Fatalf("decode todo payload: %v", err)
	}
	if len(payload.Todos) != 2 || payload.Todos[0].Content != "Implement API" {
		t.Fatalf("unexpected todo payload: %#v", payload)
	}

	select {
	case evt := <-events:
		payload, ok := evt.Payload.(UpdatedEvent)
		if !ok {
			t.Fatalf("unexpected event payload type: %T", evt.Payload)
		}
		if payload.SessionID != "sess-1" || len(payload.Todos) != 2 {
			t.Fatalf("unexpected event payload: %#v", payload)
		}
	default:
		t.Fatal("expected todo.updated event")
	}
}

func TestTodoWriteRejectsInvalidStatus(t *testing.T) {
	raw, _ := json.Marshal(Input{Todos: []Item{{Content: "x", Status: Status("bad"), Priority: PriorityHigh}}})
	_, err := (&todoTool{}).Call(t.Context(), raw, nil)
	if err == nil || !strings.Contains(err.Error(), "invalid status") {
		t.Fatalf("expected invalid status error, got %v", err)
	}
}

func openStore(t *testing.T) *session.SQLiteStore {
	t.Helper()
	store, err := session.NewSQLiteStore(t.Context(), filepath.Join(t.TempDir(), "sessions.db"))
	if err != nil {
		t.Fatalf("open sqlite store: %v", err)
	}
	t.Cleanup(func() {
		_ = store.Close()
	})
	return store
}
