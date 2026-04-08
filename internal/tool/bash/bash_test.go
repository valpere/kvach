package bash

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/valpere/kvach/internal/tool"
)

func TestBashEcho(t *testing.T) {
	tt := &bashTool{}
	raw, _ := json.Marshal(map[string]any{"command": "echo hello"})
	result, err := tt.Call(t.Context(), raw, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result.Content, "hello") {
		t.Fatalf("expected hello, got %q", result.Content)
	}
}

func TestBashNonZeroExit(t *testing.T) {
	tt := &bashTool{}
	raw, _ := json.Marshal(map[string]any{"command": "exit 42"})
	result, err := tt.Call(t.Context(), raw, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result.Content, "error") {
		t.Fatalf("expected error message, got %q", result.Content)
	}
}

func TestBashEmptyCommand(t *testing.T) {
	tt := &bashTool{}
	raw, _ := json.Marshal(map[string]any{"command": ""})
	_, err := tt.Call(t.Context(), raw, nil)
	if err == nil {
		t.Fatal("expected error for empty command")
	}
}

func TestBashWorkDir(t *testing.T) {
	dir := t.TempDir()
	tt := &bashTool{}
	raw, _ := json.Marshal(map[string]any{"command": "pwd"})

	result, err := tt.Call(t.Context(), raw, &tool.Context{WorkDir: dir})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result.Content, dir) {
		t.Fatalf("expected %s in output, got %q", dir, result.Content)
	}
}

func TestBashTimeoutClamped(t *testing.T) {
	tt := &bashTool{}
	raw, _ := json.Marshal(map[string]any{"command": "echo ok", "timeout": 9999})
	result, err := tt.Call(t.Context(), raw, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result.Content, "ok") {
		t.Fatalf("expected ok, got %q", result.Content)
	}
}

func TestBashStderr(t *testing.T) {
	tt := &bashTool{}
	raw, _ := json.Marshal(map[string]any{"command": "echo out && echo err >&2"})
	result, err := tt.Call(t.Context(), raw, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result.Content, "out") || !strings.Contains(result.Content, "err") {
		t.Fatalf("expected both stdout and stderr, got %q", result.Content)
	}
}
