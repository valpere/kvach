// Package snapshot manages a shadow git repository that tracks every file
// change the agent makes during a session.
//
// A shadow git repository uses a separate GIT_DIR so agent snapshots are
// completely isolated from the project's own git history. This enables
// fine-grained revert (go back to the state before a specific tool call)
// without polluting the project's commits.
//
// The worktree sub-functionality creates isolated git worktrees so that
// multiple subagents can modify files in parallel without conflicts.
package snapshot

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

var (
	lockMu sync.Mutex
	locks  = map[string]*sync.Mutex{}
)

// FilePatch describes one file that changed between two snapshots.
type FilePatch struct {
	Path   string
	Status string // "added" | "modified" | "deleted"
	Diff   string // unified diff
}

// Manager operates a shadow git repository for a single project.
type Manager struct {
	// ShadowGitDir is the path to the shadow .git directory.
	// e.g. ~/.local/share/kvach/snapshot/<projectID>/<hash>
	ShadowGitDir string
	// WorkDir is the project root that the shadow repo tracks.
	WorkDir string
	// ProjectID is used for directory naming.
	ProjectID string
}

// Track stages all current files in the work directory and writes a git tree
// object. Returns the object hash that can be passed to Patch or Restore later.
func (m *Manager) Track(ctx context.Context) (string, error) {
	if err := m.validate(); err != nil {
		return "", err
	}

	mu := getLock(m.ShadowGitDir)
	mu.Lock()
	defer mu.Unlock()

	if err := m.ensureRepo(ctx); err != nil {
		return "", err
	}

	treeHash, err := m.captureCurrentTree(ctx)
	if err != nil {
		return "", err
	}

	parent, _ := m.git(ctx, "rev-parse", "--verify", "refs/snapshots/current")
	parent = strings.TrimSpace(parent)

	args := []string{"commit-tree", treeHash, "-m", "snapshot " + time.Now().UTC().Format(time.RFC3339Nano)}
	if parent != "" {
		args = append(args, "-p", parent)
	}
	commitHash, err := m.git(ctx, args...)
	if err != nil {
		return "", err
	}
	commitHash = strings.TrimSpace(commitHash)

	if _, err := m.git(ctx, "update-ref", "refs/snapshots/current", commitHash); err != nil {
		return "", err
	}
	refName := fmt.Sprintf("refs/snapshots/%d", time.Now().UTC().UnixNano())
	if _, err := m.git(ctx, "update-ref", refName, commitHash); err != nil {
		return "", err
	}

	return commitHash, nil
}

// Patch diffs the current work directory state against the snapshot at hash.
// Returns one FilePatch per changed file.
func (m *Manager) Patch(ctx context.Context, hash string) ([]FilePatch, error) {
	if err := m.validate(); err != nil {
		return nil, err
	}
	if strings.TrimSpace(hash) == "" {
		return nil, errors.New("snapshot hash is required")
	}

	mu := getLock(m.ShadowGitDir)
	mu.Lock()
	defer mu.Unlock()

	if err := m.ensureRepo(ctx); err != nil {
		return nil, err
	}

	currentTree, err := m.captureCurrentTree(ctx)
	if err != nil {
		return nil, err
	}

	statuses, err := m.diffNameStatus(ctx, hash, currentTree)
	if err != nil {
		return nil, err
	}

	out := make([]FilePatch, 0, len(statuses))
	for _, st := range statuses {
		d, err := m.git(ctx, "diff", hash, currentTree, "--", st.Path)
		if err != nil {
			return nil, err
		}
		out = append(out, FilePatch{
			Path:   st.Path,
			Status: st.Status,
			Diff:   d,
		})
	}

	return out, nil
}

// Restore checks out the files from the snapshot at hash into the work
// directory.
func (m *Manager) Restore(ctx context.Context, hash string) error {
	if err := m.validate(); err != nil {
		return err
	}
	if strings.TrimSpace(hash) == "" {
		return errors.New("snapshot hash is required")
	}

	mu := getLock(m.ShadowGitDir)
	mu.Lock()
	defer mu.Unlock()

	if err := m.ensureRepo(ctx); err != nil {
		return err
	}

	currentTree, err := m.captureCurrentTree(ctx)
	if err != nil {
		return err
	}
	statuses, err := m.diffNameStatus(ctx, hash, currentTree)
	if err != nil {
		return err
	}

	for _, st := range statuses {
		if st.Status == "added" {
			if err := m.removePath(st.Path); err != nil {
				return err
			}
		}
	}

	if _, err := m.git(ctx, "checkout", hash, "--", "."); err != nil {
		return err
	}
	return nil
}

// Diff returns a full unified diff between two snapshot hashes.
func (m *Manager) Diff(ctx context.Context, from, to string) (string, error) {
	if err := m.validate(); err != nil {
		return "", err
	}
	if strings.TrimSpace(from) == "" || strings.TrimSpace(to) == "" {
		return "", errors.New("from and to hashes are required")
	}

	mu := getLock(m.ShadowGitDir)
	mu.Lock()
	defer mu.Unlock()

	if err := m.ensureRepo(ctx); err != nil {
		return "", err
	}

	return m.git(ctx, "diff", from, to, "--")
}

// Prune deletes snapshots older than the given duration. Should be called
// periodically (e.g. every hour) to reclaim disk space.
func (m *Manager) Prune(ctx context.Context, olderThan time.Duration) error {
	if err := m.validate(); err != nil {
		return err
	}
	if olderThan <= 0 {
		return errors.New("olderThan must be positive")
	}

	mu := getLock(m.ShadowGitDir)
	mu.Lock()
	defer mu.Unlock()

	if err := m.ensureRepo(ctx); err != nil {
		return err
	}

	cutoff := time.Now().UTC().Add(-olderThan).Unix()
	out, err := m.git(ctx, "for-each-ref", "--format=%(refname) %(creatordate:unix)", "refs/snapshots")
	if err != nil {
		return err
	}
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) != 2 {
			continue
		}
		refName := parts[0]
		if refName == "refs/snapshots/current" {
			continue
		}
		ts, err := strconv.ParseInt(parts[1], 10, 64)
		if err != nil {
			continue
		}
		if ts <= cutoff {
			if _, err := m.git(ctx, "update-ref", "-d", refName); err != nil {
				return err
			}
		}
	}

	_, _ = m.git(ctx, "gc", "--prune=now", "--quiet")
	return nil
}

// Worktree represents a git worktree created for an isolated subagent.
type Worktree struct {
	Path      string
	Branch    string
	ProjectID string
}

// WorktreeManager creates and removes git worktrees.
type WorktreeManager struct {
	// BaseDir is where worktrees are placed.
	// e.g. ~/.local/share/kvach/worktrees/<projectID>/
	BaseDir   string
	ProjectID string
	RepoDir   string
}

// Create creates a new worktree on a fresh branch derived from the current HEAD.
func (w *WorktreeManager) Create(ctx context.Context, branch string) (*Worktree, error) {
	if strings.TrimSpace(branch) == "" {
		return nil, errors.New("branch is required")
	}
	repoDir, err := w.repoDir()
	if err != nil {
		return nil, err
	}
	projectBase := w.projectBase()
	if err := os.MkdirAll(projectBase, 0o755); err != nil {
		return nil, fmt.Errorf("create worktree base directory: %w", err)
	}

	path := filepath.Join(projectBase, sanitizeName(branch)+"-"+time.Now().UTC().Format("20060102-150405.000000000"))
	mu := getLock(repoDir)
	mu.Lock()
	defer mu.Unlock()

	if _, err := runGit(ctx, repoDir, "worktree", "add", "-b", branch, path, "HEAD"); err != nil {
		return nil, err
	}
	return &Worktree{Path: path, Branch: branch, ProjectID: w.ProjectID}, nil
}

// Remove removes the worktree at path and deletes its branch.
func (w *WorktreeManager) Remove(ctx context.Context, path string) error {
	path = strings.TrimSpace(path)
	if path == "" {
		return errors.New("path is required")
	}

	repoDir, err := w.repoDir()
	if err != nil {
		return err
	}

	branchOut, _ := runGit(ctx, path, "rev-parse", "--abbrev-ref", "HEAD")
	branch := strings.TrimSpace(branchOut)

	mu := getLock(repoDir)
	mu.Lock()
	defer mu.Unlock()

	if _, err := runGit(ctx, repoDir, "worktree", "remove", "--force", path); err != nil {
		return err
	}
	if branch != "" && branch != "HEAD" {
		_, _ = runGit(ctx, repoDir, "branch", "-D", branch)
	}
	return nil
}

// Reset hard-resets the worktree at path to its default branch HEAD and runs
// git clean -ffdx to remove untracked files.
func (w *WorktreeManager) Reset(ctx context.Context, path string) error {
	path = strings.TrimSpace(path)
	if path == "" {
		return errors.New("path is required")
	}

	if _, err := runGit(ctx, path, "reset", "--hard"); err != nil {
		return err
	}
	if _, err := runGit(ctx, path, "clean", "-fd"); err != nil {
		return err
	}
	return nil
}

// List returns all active worktrees managed by this WorktreeManager.
func (w *WorktreeManager) List() ([]Worktree, error) {
	repoDir, err := w.repoDir()
	if err != nil {
		return nil, err
	}
	out, err := runGit(context.Background(), repoDir, "worktree", "list", "--porcelain")
	if err != nil {
		return nil, err
	}

	projectBase := w.projectBase()
	var (
		items []Worktree
		cur   Worktree
	)
	flush := func() {
		if strings.TrimSpace(cur.Path) == "" {
			return
		}
		if !isSubpath(projectBase, cur.Path) {
			cur = Worktree{}
			return
		}
		if cur.Branch == "" {
			cur.Branch = "HEAD"
		}
		cur.ProjectID = w.ProjectID
		items = append(items, cur)
		cur = Worktree{}
	}

	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			flush()
			continue
		}
		if strings.HasPrefix(line, "worktree ") {
			flush()
			cur.Path = strings.TrimSpace(strings.TrimPrefix(line, "worktree "))
			continue
		}
		if strings.HasPrefix(line, "branch ") {
			cur.Branch = strings.TrimSpace(strings.TrimPrefix(line, "branch refs/heads/"))
		}
	}
	flush()

	return items, nil
}

func (m *Manager) validate() error {
	if strings.TrimSpace(m.ShadowGitDir) == "" {
		return errors.New("shadow git directory is required")
	}
	if strings.TrimSpace(m.WorkDir) == "" {
		return errors.New("work directory is required")
	}
	return nil
}

func (m *Manager) ensureRepo(ctx context.Context) error {
	if err := os.MkdirAll(filepath.Dir(m.ShadowGitDir), 0o755); err != nil {
		return fmt.Errorf("create shadow parent directory: %w", err)
	}
	if _, err := m.git(ctx, "rev-parse", "--git-dir"); err == nil {
		return nil
	}
	if _, err := m.git(ctx, "init"); err != nil {
		return err
	}
	return nil
}

func (m *Manager) captureCurrentTree(ctx context.Context) (string, error) {
	if _, err := m.git(ctx, "add", "-A", "--", "."); err != nil {
		return "", err
	}
	tree, err := m.git(ctx, "write-tree")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(tree), nil
}

func (m *Manager) git(ctx context.Context, args ...string) (string, error) {
	full := append([]string{"--git-dir=" + m.ShadowGitDir, "--work-tree=" + m.WorkDir}, args...)
	cmd := exec.CommandContext(ctx, "git", full...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return string(out), nil
}

type statusLine struct {
	Path   string
	Status string
}

func (m *Manager) diffNameStatus(ctx context.Context, from, to string) ([]statusLine, error) {
	out, err := m.git(ctx, "diff", "--name-status", from, to, "--")
	if err != nil {
		return nil, err
	}
	lines := strings.Split(strings.TrimSpace(out), "\n")
	result := make([]statusLine, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}
		path := parts[len(parts)-1]
		status := mapStatus(parts[0])
		if status == "" {
			continue
		}
		result = append(result, statusLine{Path: path, Status: status})
	}
	return result, nil
}

func mapStatus(code string) string {
	if code == "" {
		return ""
	}
	switch code[0] {
	case 'A':
		return "added"
	case 'M':
		return "modified"
	case 'D':
		return "deleted"
	case 'R', 'C', 'T', 'U':
		return "modified"
	default:
		return ""
	}
}

func (m *Manager) removePath(rel string) error {
	if strings.TrimSpace(rel) == "" {
		return nil
	}
	rel = filepath.Clean(rel)
	if rel == "." || rel == ".." || rel == "/" {
		return fmt.Errorf("unsafe restore path %q", rel)
	}
	if rel == ".git" || strings.HasPrefix(rel, ".git"+string(os.PathSeparator)) {
		return nil
	}
	workAbs, err := filepath.Abs(m.WorkDir)
	if err != nil {
		return err
	}
	abs, err := filepath.Abs(filepath.Join(m.WorkDir, rel))
	if err != nil {
		return err
	}
	if abs != workAbs && !strings.HasPrefix(abs, workAbs+string(os.PathSeparator)) {
		return fmt.Errorf("path escapes workdir: %s", rel)
	}
	if err := os.RemoveAll(abs); err != nil {
		return fmt.Errorf("remove %s: %w", rel, err)
	}
	return nil
}

func (w *WorktreeManager) repoDir() (string, error) {
	if strings.TrimSpace(w.RepoDir) != "" {
		return w.RepoDir, nil
	}
	wd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("resolve current directory: %w", err)
	}
	return wd, nil
}

func (w *WorktreeManager) projectBase() string {
	if strings.TrimSpace(w.ProjectID) == "" {
		return w.BaseDir
	}
	return filepath.Join(w.BaseDir, w.ProjectID)
}

func sanitizeName(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return "worktree"
	}
	r := strings.NewReplacer("/", "-", "\\", "-", ":", "-", " ", "-")
	return r.Replace(s)
}

func isSubpath(root, path string) bool {
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return false
	}
	pathAbs, err := filepath.Abs(path)
	if err != nil {
		return false
	}
	if pathAbs == rootAbs {
		return true
	}
	return strings.HasPrefix(pathAbs, rootAbs+string(os.PathSeparator))
}

func getLock(key string) *sync.Mutex {
	lockMu.Lock()
	defer lockMu.Unlock()
	if m, ok := locks[key]; ok {
		return m
	}
	m := &sync.Mutex{}
	locks[key] = m
	return m
}

func runGit(ctx context.Context, dir string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", append([]string{"-C", dir}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git -C %s %s: %w: %s", dir, strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return string(out), nil
}
