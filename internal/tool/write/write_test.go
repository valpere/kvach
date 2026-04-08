package write

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/valpere/kvach/internal/tool"
)

func TestWriteNewFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "out.txt")

	tt := &writeTool{}
	raw, _ := json.Marshal(map[string]any{"path": path, "content": "hello world"})
	result, err := tt.Call(t.Context(), raw, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result.Content, "11 bytes") {
		t.Fatalf("expected byte count, got %q", result.Content)
	}

	data, _ := os.ReadFile(path)
	if string(data) != "hello world" {
		t.Fatalf("expected 'hello world', got %q", string(data))
	}
}

func TestWriteCreatesParentDirs(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "a", "b", "c.txt")

	tt := &writeTool{}
	raw, _ := json.Marshal(map[string]any{"path": path, "content": "nested"})
	_, err := tt.Call(t.Context(), raw, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, _ := os.ReadFile(path)
	if string(data) != "nested" {
		t.Fatalf("expected 'nested', got %q", string(data))
	}
}

func TestWriteOverwrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "over.txt")
	os.WriteFile(path, []byte("old"), 0o644)

	tt := &writeTool{}
	raw, _ := json.Marshal(map[string]any{"path": path, "content": "new"})
	_, err := tt.Call(t.Context(), raw, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, _ := os.ReadFile(path)
	if string(data) != "new" {
		t.Fatalf("expected 'new', got %q", string(data))
	}
}

func TestWriteRelativeWithWorkDir(t *testing.T) {
	dir := t.TempDir()

	tt := &writeTool{}
	raw, _ := json.Marshal(map[string]any{"path": "rel.txt", "content": "relative"})
	_, err := tt.Call(t.Context(), raw, &tool.Context{WorkDir: dir})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, _ := os.ReadFile(filepath.Join(dir, "rel.txt"))
	if string(data) != "relative" {
		t.Fatalf("expected 'relative', got %q", string(data))
	}
}
