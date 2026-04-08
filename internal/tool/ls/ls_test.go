package ls

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/valpere/kvach/internal/tool"
)

func TestLsFlat(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "a.txt"), []byte("hello"), 0o644)
	os.Mkdir(filepath.Join(dir, "subdir"), 0o755)

	tt := &lsTool{}
	raw, _ := json.Marshal(map[string]any{"path": dir})
	result, err := tt.Call(t.Context(), raw, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result.Content, "a.txt") {
		t.Fatalf("expected a.txt, got %q", result.Content)
	}
	if !strings.Contains(result.Content, "subdir/") {
		t.Fatalf("expected subdir/, got %q", result.Content)
	}
	if !strings.Contains(result.Content, "5 bytes") {
		t.Fatalf("expected file size, got %q", result.Content)
	}
}

func TestLsRecursive(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "a", "b"), 0o755)
	os.WriteFile(filepath.Join(dir, "a", "b", "deep.txt"), []byte("x"), 0o644)
	os.WriteFile(filepath.Join(dir, "top.txt"), []byte("y"), 0o644)

	tt := &lsTool{}
	raw, _ := json.Marshal(map[string]any{"path": dir, "recursive": true})
	result, err := tt.Call(t.Context(), raw, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result.Content, "deep.txt") {
		t.Fatalf("expected deep.txt, got %q", result.Content)
	}
	if !strings.Contains(result.Content, "top.txt") {
		t.Fatalf("expected top.txt, got %q", result.Content)
	}
}

func TestLsWorkDir(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "work.txt"), []byte(""), 0o644)

	tt := &lsTool{}
	raw, _ := json.Marshal(map[string]any{})
	result, err := tt.Call(t.Context(), raw, &tool.Context{WorkDir: dir})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result.Content, "work.txt") {
		t.Fatalf("expected work.txt, got %q", result.Content)
	}
}

func TestLsNonexistent(t *testing.T) {
	tt := &lsTool{}
	raw, _ := json.Marshal(map[string]any{"path": "/nonexistent/dir"})
	_, err := tt.Call(t.Context(), raw, nil)
	if err == nil {
		t.Fatal("expected error for nonexistent directory")
	}
}
