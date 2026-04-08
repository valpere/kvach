package edit

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestEditSingleMatch(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.go")
	os.WriteFile(path, []byte("func main() {\n\tfmt.Println(\"hello\")\n}\n"), 0o644)

	tt := &editTool{}
	raw, _ := json.Marshal(map[string]any{
		"path":       path,
		"old_string": "hello",
		"new_string": "world",
	})
	result, err := tt.Call(t.Context(), raw, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Content == "" {
		t.Fatal("expected non-empty result")
	}

	data, _ := os.ReadFile(path)
	if got := string(data); got != "func main() {\n\tfmt.Println(\"world\")\n}\n" {
		t.Fatalf("unexpected content: %q", got)
	}
}

func TestEditNoMatch(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")
	os.WriteFile(path, []byte("abc"), 0o644)

	tt := &editTool{}
	raw, _ := json.Marshal(map[string]any{
		"path":       path,
		"old_string": "xyz",
		"new_string": "123",
	})
	_, err := tt.Call(t.Context(), raw, nil)
	if err == nil {
		t.Fatal("expected error for no match")
	}
}

func TestEditMultipleMatches(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")
	os.WriteFile(path, []byte("aaa bbb aaa"), 0o644)

	tt := &editTool{}
	raw, _ := json.Marshal(map[string]any{
		"path":       path,
		"old_string": "aaa",
		"new_string": "ccc",
	})
	_, err := tt.Call(t.Context(), raw, nil)
	if err == nil {
		t.Fatal("expected error for multiple matches")
	}
}

func TestEditNonexistentFile(t *testing.T) {
	tt := &editTool{}
	raw, _ := json.Marshal(map[string]any{
		"path":       "/nonexistent/file.txt",
		"old_string": "a",
		"new_string": "b",
	})
	_, err := tt.Call(t.Context(), raw, nil)
	if err == nil {
		t.Fatal("expected error for nonexistent file")
	}
}
