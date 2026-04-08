package read

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/valpere/kvach/internal/tool"
)

func TestReadFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")
	os.WriteFile(path, []byte("line1\nline2\nline3\nline4\nline5\n"), 0o644)

	tt := &readTool{}
	raw, _ := json.Marshal(map[string]any{"path": path})
	result, err := tt.Call(t.Context(), raw, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result.Content, "1: line1") {
		t.Fatalf("expected line-numbered output, got %q", result.Content)
	}
	if !strings.Contains(result.Content, "5: line5") {
		t.Fatalf("expected line 5, got %q", result.Content)
	}
}

func TestReadFileLineRange(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")
	os.WriteFile(path, []byte("a\nb\nc\nd\ne\n"), 0o644)

	tt := &readTool{}
	raw, _ := json.Marshal(map[string]any{"path": path, "start_line": 2, "end_line": 4})
	result, err := tt.Call(t.Context(), raw, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result.Content, "2: b") {
		t.Fatalf("expected line 2, got %q", result.Content)
	}
	if !strings.Contains(result.Content, "4: d") {
		t.Fatalf("expected line 4, got %q", result.Content)
	}
	if strings.Contains(result.Content, "1: a") {
		t.Fatal("should not contain line 1")
	}
	if strings.Contains(result.Content, "5: e") {
		t.Fatal("should not contain line 5")
	}
}

func TestReadDirectory(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "a.txt"), []byte(""), 0o644)
	os.Mkdir(filepath.Join(dir, "subdir"), 0o755)

	tt := &readTool{}
	raw, _ := json.Marshal(map[string]any{"path": dir})
	result, err := tt.Call(t.Context(), raw, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result.Content, "a.txt") {
		t.Fatalf("expected a.txt in listing, got %q", result.Content)
	}
	if !strings.Contains(result.Content, "subdir/") {
		t.Fatalf("expected subdir/ in listing, got %q", result.Content)
	}
}

func TestReadRelativeWithWorkDir(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "rel.txt"), []byte("hello\n"), 0o644)

	tt := &readTool{}
	raw, _ := json.Marshal(map[string]any{"path": "rel.txt"})
	result, err := tt.Call(t.Context(), raw, &tool.Context{WorkDir: dir})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result.Content, "hello") {
		t.Fatalf("expected hello, got %q", result.Content)
	}
}

func TestReadNonexistent(t *testing.T) {
	tt := &readTool{}
	raw, _ := json.Marshal(map[string]any{"path": "/nonexistent/file.txt"})
	_, err := tt.Call(t.Context(), raw, nil)
	if err == nil {
		t.Fatal("expected error for nonexistent file")
	}
}
