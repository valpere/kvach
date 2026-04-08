package glob

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/valpere/kvach/internal/tool"
)

func TestGlobFindsGoFiles(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main"), 0o644)
	os.WriteFile(filepath.Join(dir, "README.md"), []byte("# readme"), 0o644)
	os.MkdirAll(filepath.Join(dir, "sub"), 0o755)
	os.WriteFile(filepath.Join(dir, "sub", "util.go"), []byte("package sub"), 0o644)

	tt := &globTool{}
	raw, _ := json.Marshal(map[string]any{"pattern": "*.go"})
	result, err := tt.Call(t.Context(), raw, &tool.Context{WorkDir: dir})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result.Content, "main.go") {
		t.Fatalf("expected main.go, got %q", result.Content)
	}
}

func TestGlobDoubleStarPattern(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "a", "b"), 0o755)
	os.WriteFile(filepath.Join(dir, "a", "b", "deep.go"), []byte(""), 0o644)
	os.WriteFile(filepath.Join(dir, "top.go"), []byte(""), 0o644)

	tt := &globTool{}
	raw, _ := json.Marshal(map[string]any{"pattern": "**/*.go"})
	result, err := tt.Call(t.Context(), raw, &tool.Context{WorkDir: dir})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should find both files.
	if !strings.Contains(result.Content, "deep.go") {
		t.Fatalf("expected deep.go, got %q", result.Content)
	}
	if !strings.Contains(result.Content, "top.go") {
		t.Fatalf("expected top.go, got %q", result.Content)
	}
}

func TestGlobNoMatches(t *testing.T) {
	dir := t.TempDir()

	tt := &globTool{}
	raw, _ := json.Marshal(map[string]any{"pattern": "*.xyz"})
	result, err := tt.Call(t.Context(), raw, &tool.Context{WorkDir: dir})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result.Content, "No files found") {
		t.Fatalf("expected no-match message, got %q", result.Content)
	}
}
