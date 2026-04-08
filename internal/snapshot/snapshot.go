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
	"time"
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
func (m *Manager) Track(_ context.Context) (string, error) {
	// TODO(phase3): shell out to git with GIT_DIR override.
	return "", nil
}

// Patch diffs the current work directory state against the snapshot at hash.
// Returns one FilePatch per changed file.
func (m *Manager) Patch(_ context.Context, hash string) ([]FilePatch, error) {
	// TODO(phase3): implement.
	return nil, nil
}

// Restore checks out the files from the snapshot at hash into the work
// directory.
func (m *Manager) Restore(_ context.Context, hash string) error {
	// TODO(phase3): implement.
	return nil
}

// Diff returns a full unified diff between two snapshot hashes.
func (m *Manager) Diff(_ context.Context, from, to string) (string, error) {
	// TODO(phase3): implement.
	return "", nil
}

// Prune deletes snapshots older than the given duration. Should be called
// periodically (e.g. every hour) to reclaim disk space.
func (m *Manager) Prune(_ context.Context, olderThan time.Duration) error {
	// TODO(phase3): implement.
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
}

// Create creates a new worktree on a fresh branch derived from the current HEAD.
func (w *WorktreeManager) Create(_ context.Context, branch string) (*Worktree, error) {
	// TODO(phase3): shell out to git worktree add.
	return nil, nil
}

// Remove removes the worktree at path and deletes its branch.
func (w *WorktreeManager) Remove(_ context.Context, path string) error {
	// TODO(phase3): implement.
	return nil
}

// Reset hard-resets the worktree at path to its default branch HEAD and runs
// git clean -ffdx to remove untracked files.
func (w *WorktreeManager) Reset(_ context.Context, path string) error {
	// TODO(phase3): implement.
	return nil
}

// List returns all active worktrees managed by this WorktreeManager.
func (w *WorktreeManager) List() ([]Worktree, error) {
	// TODO(phase3): implement.
	return nil, nil
}
