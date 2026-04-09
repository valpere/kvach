package agent

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/valpere/kvach/internal/permission"
	"github.com/valpere/kvach/internal/tool"
)

func TestExecuteToolCallsPermissionPlanDeniesWrite(t *testing.T) {
	toolName := "PermWrite"
	r := tool.NewRegistry()
	r.Register(&fakePermissionTool{name: toolName, readOnly: false, content: "ok"})

	a := New(fakeSubagentProvider{}, r, nil, Config{
		WorkDir: t.TempDir(),
		PermissionContext: permission.Context{
			Mode: permission.ModePlan,
		},
	})

	events := make(chan Event, 16)
	parts := a.executeToolCalls(context.Background(), "sess-1", []toolCall{makeToolCall("call-1", toolName, `{"path":"x"}`)}, events)
	close(events)

	if len(parts) != 1 || parts[0].ToolResult == nil {
		t.Fatalf("expected one tool result part, got %#v", parts)
	}
	if !parts[0].ToolResult.IsError {
		t.Fatal("expected permission denial tool result error")
	}
	if !strings.Contains(parts[0].ToolResult.Content, "read-only") {
		t.Fatalf("unexpected denial message: %q", parts[0].ToolResult.Content)
	}
}

func TestExecuteToolCallsPermissionDenyRule(t *testing.T) {
	toolName := "PermRead"
	r := tool.NewRegistry()
	r.Register(&fakePermissionTool{name: toolName, readOnly: true, content: "ok"})

	a := New(fakeSubagentProvider{}, r, nil, Config{
		WorkDir: t.TempDir(),
		PermissionContext: permission.Context{
			Mode:      permission.ModeBypass,
			DenyRules: []permission.Rule{{Tool: toolName, Pattern: "*"}},
		},
	})

	events := make(chan Event, 16)
	parts := a.executeToolCalls(context.Background(), "sess-1", []toolCall{makeToolCall("call-1", toolName, `{"path":"x"}`)}, events)
	close(events)

	if len(parts) != 1 || parts[0].ToolResult == nil {
		t.Fatalf("expected one tool result part, got %#v", parts)
	}
	if !parts[0].ToolResult.IsError {
		t.Fatal("expected denial from deny rule")
	}
	if !strings.Contains(parts[0].ToolResult.Content, "deny rule") {
		t.Fatalf("unexpected deny-rule message: %q", parts[0].ToolResult.Content)
	}
}

func TestExecuteToolCallsPermissionDefaultWithAskerAllows(t *testing.T) {
	toolName := "PermWriteAsk"
	r := tool.NewRegistry()
	r.Register(&fakePermissionTool{name: toolName, readOnly: false, content: "ok"})

	a := New(fakeSubagentProvider{}, r, nil, Config{
		WorkDir: t.TempDir(),
		PermissionContext: permission.Context{
			Mode: permission.ModeDefault,
		},
		PermissionAsker: fakeAsker{reply: permission.Reply{Decision: "allow_once"}},
	})

	events := make(chan Event, 32)
	parts := a.executeToolCalls(context.Background(), "sess-1", []toolCall{makeToolCall("call-1", toolName, `{"path":"x"}`)}, events)
	close(events)

	if len(parts) != 1 || parts[0].ToolResult == nil {
		t.Fatalf("expected one tool result part, got %#v", parts)
	}
	if parts[0].ToolResult.IsError {
		t.Fatalf("expected allow_once to succeed, got error: %q", parts[0].ToolResult.Content)
	}

	hasAsked := false
	hasResolved := false
	for evt := range events {
		if evt.Type == EventPermissionAsked {
			hasAsked = true
		}
		if evt.Type == EventPermissionResolved {
			hasResolved = true
		}
	}
	if !hasAsked || !hasResolved {
		t.Fatalf("expected permission ask+resolve events, asked=%v resolved=%v", hasAsked, hasResolved)
	}
}

func TestExecuteToolCallsPermissionDefaultWithoutAskerDenies(t *testing.T) {
	toolName := "PermWriteNoAsker"
	r := tool.NewRegistry()
	r.Register(&fakePermissionTool{name: toolName, readOnly: false, content: "ok"})

	a := New(fakeSubagentProvider{}, r, nil, Config{
		WorkDir: t.TempDir(),
		PermissionContext: permission.Context{
			Mode: permission.ModeDefault,
		},
	})

	events := make(chan Event, 32)
	parts := a.executeToolCalls(context.Background(), "sess-1", []toolCall{makeToolCall("call-1", toolName, `{"path":"x"}`)}, events)
	close(events)

	if len(parts) != 1 || parts[0].ToolResult == nil {
		t.Fatalf("expected one tool result part, got %#v", parts)
	}
	if !parts[0].ToolResult.IsError {
		t.Fatal("expected ask-without-asker to deny")
	}
	if !strings.Contains(parts[0].ToolResult.Content, "no permission asker configured") {
		t.Fatalf("unexpected no-asker message: %q", parts[0].ToolResult.Content)
	}
}

func makeToolCall(id, name, input string) toolCall {
	tc := toolCall{ID: id, Name: name}
	tc.inputJSON.WriteString(input)
	return tc
}

type fakePermissionTool struct {
	name     string
	readOnly bool
	content  string
}

func (f *fakePermissionTool) Name() string      { return f.name }
func (f *fakePermissionTool) Aliases() []string { return nil }

func (f *fakePermissionTool) InputSchema() map[string]any {
	return map[string]any{"type": "object"}
}

func (f *fakePermissionTool) Call(_ context.Context, _ json.RawMessage, _ *tool.Context) (*tool.Result, error) {
	return &tool.Result{Content: f.content}, nil
}

func (f *fakePermissionTool) ValidateInput(_ json.RawMessage) error { return nil }

func (f *fakePermissionTool) CheckPermissions(_ json.RawMessage, _ *tool.Context) tool.PermissionOutcome {
	return tool.PermissionOutcome{Decision: "allow"}
}

func (f *fakePermissionTool) IsEnabled(_ *tool.Context) bool           { return true }
func (f *fakePermissionTool) IsConcurrencySafe(_ json.RawMessage) bool { return true }
func (f *fakePermissionTool) IsReadOnly(_ json.RawMessage) bool        { return f.readOnly }
func (f *fakePermissionTool) IsDestructive(_ json.RawMessage) bool     { return false }
func (f *fakePermissionTool) Prompt(_ tool.PromptOptions) string       { return "" }

type fakeAsker struct {
	reply permission.Reply
	err   error
}

func (f fakeAsker) Ask(_ context.Context, _ permission.Request) (permission.Reply, error) {
	return f.reply, f.err
}
