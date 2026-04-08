package grep

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/valpere/kvach/internal/tool"
)

func TestGrepFindsPattern(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "hello.txt"), []byte("hello world\ngoodbye world\nhello again\n"), 0o644)

	tt := &grepTool{}
	raw, _ := json.Marshal(map[string]any{"pattern": "hello"})
	result, err := tt.Call(t.Context(), raw, &tool.Context{WorkDir: dir})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result.Content, "hello.txt:1:hello world") {
		t.Fatalf("expected match at line 1, got %q", result.Content)
	}
	if !strings.Contains(result.Content, "hello.txt:3:hello again") {
		t.Fatalf("expected match at line 3, got %q", result.Content)
	}
}

func TestGrepNoMatches(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "test.txt"), []byte("abc\n"), 0o644)

	tt := &grepTool{}
	raw, _ := json.Marshal(map[string]any{"pattern": "xyz"})
	result, err := tt.Call(t.Context(), raw, &tool.Context{WorkDir: dir})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result.Content, "No matches") {
		t.Fatalf("expected no-match message, got %q", result.Content)
	}
}

func TestGrepIncludeFilter(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "code.go"), []byte("func main() {}\n"), 0o644)
	os.WriteFile(filepath.Join(dir, "doc.md"), []byte("func description\n"), 0o644)

	tt := &grepTool{}
	raw, _ := json.Marshal(map[string]any{"pattern": "func", "include": "*.go"})
	result, err := tt.Call(t.Context(), raw, &tool.Context{WorkDir: dir})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result.Content, "code.go") {
		t.Fatalf("expected code.go match, got %q", result.Content)
	}
	// doc.md should not appear when include=*.go.
	if strings.Contains(result.Content, "doc.md") {
		t.Fatalf("doc.md should be filtered out, got %q", result.Content)
	}
}

func TestGrepInvalidRegex(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "test.txt"), []byte("abc\n"), 0o644)

	tt := &grepTool{}
	raw, _ := json.Marshal(map[string]any{"pattern": "[invalid"})
	_, err := tt.Call(t.Context(), raw, &tool.Context{WorkDir: dir})
	// Either rg handles the invalid regex or the fallback returns an error.
	// Both are acceptable — we just shouldn't panic.
	_ = err
}
