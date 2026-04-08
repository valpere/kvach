package multipatch

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/valpere/kvach/internal/tool"
)

func TestApplyPatch_ModifiesExistingFile(t *testing.T) {
	wd := t.TempDir()
	initGitRepo(t, wd)

	file := filepath.Join(wd, "hello.txt")
	if err := os.WriteFile(file, []byte("hello\n"), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	patch := strings.Join([]string{
		"diff --git a/hello.txt b/hello.txt",
		"--- a/hello.txt",
		"+++ b/hello.txt",
		"@@ -1 +1 @@",
		"-hello",
		"+world",
		"",
	}, "\n")

	raw, _ := json.Marshal(ApplyPatchInput{Patch: patch})
	res, err := (&applyPatchTool{}).Call(t.Context(), raw, &tool.Context{WorkDir: wd})
	if err != nil {
		t.Fatalf("call apply patch: %v", err)
	}
	if res == nil || res.Content == "" {
		t.Fatal("expected non-empty result")
	}

	got, err := os.ReadFile(file)
	if err != nil {
		t.Fatalf("read patched file: %v", err)
	}
	if string(got) != "world\n" {
		t.Fatalf("unexpected patched content: %q", string(got))
	}
}

func TestApplyPatch_CreatesNewFile(t *testing.T) {
	wd := t.TempDir()
	initGitRepo(t, wd)

	patch := strings.Join([]string{
		"diff --git a/new.txt b/new.txt",
		"new file mode 100644",
		"--- /dev/null",
		"+++ b/new.txt",
		"@@ -0,0 +1 @@",
		"+new content",
		"",
	}, "\n")

	raw, _ := json.Marshal(ApplyPatchInput{Patch: patch})
	if _, err := (&applyPatchTool{}).Call(t.Context(), raw, &tool.Context{WorkDir: wd}); err != nil {
		t.Fatalf("call apply patch: %v", err)
	}

	got, err := os.ReadFile(filepath.Join(wd, "new.txt"))
	if err != nil {
		t.Fatalf("read new file: %v", err)
	}
	if string(got) != "new content\n" {
		t.Fatalf("unexpected new file content: %q", string(got))
	}
}

func TestApplyPatch_InvalidPatch(t *testing.T) {
	wd := t.TempDir()
	initGitRepo(t, wd)

	raw, _ := json.Marshal(ApplyPatchInput{Patch: "not a patch"})
	_, err := (&applyPatchTool{}).Call(t.Context(), raw, &tool.Context{WorkDir: wd})
	if err == nil {
		t.Fatal("expected error for invalid patch")
	}
}

func initGitRepo(t *testing.T, dir string) {
	t.Helper()
	cmd := exec.Command("git", "init")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git init failed: %v: %s", err, string(out))
	}
}
