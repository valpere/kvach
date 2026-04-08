package snapshot

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestManagerTrackPatchDiffRestorePrune(t *testing.T) {
	t.Setenv("GIT_AUTHOR_NAME", "kvach-test")
	t.Setenv("GIT_AUTHOR_EMAIL", "kvach-test@example.com")
	t.Setenv("GIT_COMMITTER_NAME", "kvach-test")
	t.Setenv("GIT_COMMITTER_EMAIL", "kvach-test@example.com")

	ctx := context.Background()
	base := t.TempDir()
	workDir := filepath.Join(base, "work")
	if err := os.MkdirAll(workDir, 0o755); err != nil {
		t.Fatalf("mkdir work dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(workDir, "a.txt"), []byte("one\n"), 0o644); err != nil {
		t.Fatalf("write initial file: %v", err)
	}

	mgr := &Manager{
		ShadowGitDir: filepath.Join(base, "shadow", "repo.git"),
		WorkDir:      workDir,
		ProjectID:    "proj",
	}

	h1, err := mgr.Track(ctx)
	if err != nil {
		t.Fatalf("track h1: %v", err)
	}
	if h1 == "" {
		t.Fatal("expected non-empty snapshot hash")
	}

	if err := os.WriteFile(filepath.Join(workDir, "a.txt"), []byte("two\n"), 0o644); err != nil {
		t.Fatalf("modify tracked file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(workDir, "b.txt"), []byte("new\n"), 0o644); err != nil {
		t.Fatalf("create new file: %v", err)
	}

	patches, err := mgr.Patch(ctx, h1)
	if err != nil {
		t.Fatalf("patch against h1: %v", err)
	}
	if len(patches) < 2 {
		t.Fatalf("expected at least 2 patches, got %d", len(patches))
	}

	statusByPath := map[string]string{}
	for _, p := range patches {
		statusByPath[p.Path] = p.Status
	}
	if statusByPath["a.txt"] != "modified" {
		t.Fatalf("a.txt status = %q, want modified", statusByPath["a.txt"])
	}
	if statusByPath["b.txt"] != "added" {
		t.Fatalf("b.txt status = %q, want added", statusByPath["b.txt"])
	}

	h2, err := mgr.Track(ctx)
	if err != nil {
		t.Fatalf("track h2: %v", err)
	}
	diff, err := mgr.Diff(ctx, h1, h2)
	if err != nil {
		t.Fatalf("diff h1..h2: %v", err)
	}
	if !strings.Contains(diff, "a.txt") || !strings.Contains(diff, "b.txt") {
		t.Fatalf("unexpected diff output: %s", diff)
	}

	if err := mgr.Restore(ctx, h1); err != nil {
		t.Fatalf("restore h1: %v", err)
	}
	aData, err := os.ReadFile(filepath.Join(workDir, "a.txt"))
	if err != nil {
		t.Fatalf("read restored a.txt: %v", err)
	}
	if string(aData) != "one\n" {
		t.Fatalf("restored a.txt = %q, want %q", string(aData), "one\n")
	}
	if _, err := os.Stat(filepath.Join(workDir, "b.txt")); !os.IsNotExist(err) {
		t.Fatalf("expected b.txt to be removed, stat err=%v", err)
	}

	if err := mgr.Prune(ctx, 1*time.Nanosecond); err != nil {
		t.Fatalf("prune snapshots: %v", err)
	}

	refs, err := mgr.git(ctx, "for-each-ref", "--format=%(refname)", "refs/snapshots")
	if err != nil {
		t.Fatalf("list snapshot refs: %v", err)
	}
	lines := strings.Fields(strings.TrimSpace(refs))
	if len(lines) == 0 {
		t.Fatal("expected at least refs/snapshots/current to remain")
	}
}

func TestWorktreeManagerLifecycle(t *testing.T) {
	ctx := context.Background()
	repoDir := t.TempDir()
	runGitTest(t, ctx, repoDir, "init")
	runGitTest(t, ctx, repoDir, "config", "user.email", "kvach-test@example.com")
	runGitTest(t, ctx, repoDir, "config", "user.name", "kvach-test")

	if err := os.WriteFile(filepath.Join(repoDir, "main.txt"), []byte("base\n"), 0o644); err != nil {
		t.Fatalf("write main file: %v", err)
	}
	runGitTest(t, ctx, repoDir, "add", "main.txt")
	runGitTest(t, ctx, repoDir, "commit", "-m", "init")

	wm := &WorktreeManager{
		BaseDir:   filepath.Join(t.TempDir(), "worktrees"),
		ProjectID: "proj",
		RepoDir:   repoDir,
	}

	wt, err := wm.Create(ctx, "feat-snap")
	if err != nil {
		t.Fatalf("create worktree: %v", err)
	}
	if wt == nil || wt.Path == "" {
		t.Fatal("expected worktree path")
	}

	if err := os.WriteFile(filepath.Join(wt.Path, "main.txt"), []byte("changed\n"), 0o644); err != nil {
		t.Fatalf("modify worktree tracked file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(wt.Path, "tmp.txt"), []byte("temp\n"), 0o644); err != nil {
		t.Fatalf("create untracked worktree file: %v", err)
	}

	if err := wm.Reset(ctx, wt.Path); err != nil {
		t.Fatalf("reset worktree: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(wt.Path, "main.txt"))
	if err != nil {
		t.Fatalf("read reset tracked file: %v", err)
	}
	if string(data) != "base\n" {
		t.Fatalf("reset tracked file = %q, want %q", string(data), "base\n")
	}
	if _, err := os.Stat(filepath.Join(wt.Path, "tmp.txt")); !os.IsNotExist(err) {
		t.Fatalf("expected tmp.txt removed after reset, err=%v", err)
	}

	items, err := wm.List()
	if err != nil {
		t.Fatalf("list worktrees: %v", err)
	}
	found := false
	for _, item := range items {
		if item.Path == wt.Path {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("worktree %s not found in list %#v", wt.Path, items)
	}

	if err := wm.Remove(ctx, wt.Path); err != nil {
		t.Fatalf("remove worktree: %v", err)
	}
	if _, err := os.Stat(wt.Path); !os.IsNotExist(err) {
		t.Fatalf("expected worktree path removed, err=%v", err)
	}
}

func runGitTest(t *testing.T, ctx context.Context, dir string, args ...string) {
	t.Helper()
	cmd := exec.CommandContext(ctx, "git", append([]string{"-C", dir}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git -C %s %s failed: %v: %s", dir, strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
}
