// Package git provides utilities for reading git repository state.
//
// These are read-only operations used to build the agent's context (branch
// name, recent commits, file status). Mutations (worktree management,
// snapshot commits) live in internal/snapshot.
package git

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// Status holds a summary of the repository's current state.
type Status struct {
	// Branch is the current branch name, or the short SHA when detached.
	Branch string
	// Ahead is the number of commits ahead of the upstream branch.
	Ahead int
	// Behind is the number of commits behind the upstream branch.
	Behind int
	// Modified lists files with unstaged changes.
	Modified []string
	// Staged lists files with staged changes.
	Staged []string
	// Untracked lists untracked files.
	Untracked []string
}

// RecentCommit is a single entry from the recent commit log.
type RecentCommit struct {
	Hash    string
	Subject string
	Author  string
	Date    string
}

// Root returns the absolute path of the git repository root for the given
// directory, or an error if dir is not inside a git repository.
func Root(ctx context.Context, dir string) (string, error) {
	out, err := run(ctx, dir, "rev-parse", "--show-toplevel")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

// GetStatus returns the current branch and working-tree status for the
// repository rooted at dir.
func GetStatus(ctx context.Context, dir string) (Status, error) {
	var s Status

	// Branch name.
	branch, err := run(ctx, dir, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return s, err
	}
	s.Branch = strings.TrimSpace(branch)

	// Porcelain status for file changes.
	out, err := run(ctx, dir, "status", "--porcelain=v1")
	if err != nil {
		return s, nil // non-fatal, return what we have
	}
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		if len(line) < 4 {
			continue
		}
		x, y := line[0], line[1]
		file := strings.TrimSpace(line[3:])
		if x != ' ' && x != '?' {
			s.Staged = append(s.Staged, file)
		}
		if y != ' ' && y != '?' {
			s.Modified = append(s.Modified, file)
		}
		if x == '?' && y == '?' {
			s.Untracked = append(s.Untracked, file)
		}
	}

	return s, nil
}

// RecentCommits returns the last n commits from the log of the repository at
// dir.
func RecentCommits(ctx context.Context, dir string, n int) ([]RecentCommit, error) {
	if n <= 0 {
		n = 10
	}
	format := "%H%x00%s%x00%an%x00%ai"
	out, err := run(ctx, dir, "log", fmt.Sprintf("-n%d", n), fmt.Sprintf("--format=%s", format))
	if err != nil {
		return nil, err
	}

	var commits []RecentCommit
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "\x00", 4)
		if len(parts) < 4 {
			continue
		}
		commits = append(commits, RecentCommit{
			Hash:    parts[0][:minLen(len(parts[0]), 12)],
			Subject: parts[1],
			Author:  parts[2],
			Date:    parts[3],
		})
	}
	return commits, nil
}

func minLen(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// SlugFromRoot returns a filesystem-safe slug derived from the git root path.
// Used to construct per-project config and memory directories.
func SlugFromRoot(root string) string {
	// Replace path separators and other special characters with underscores,
	// then truncate to avoid overly long directory names.
	r := strings.NewReplacer("/", "_", "\\", "_", ":", "_", " ", "_")
	slug := r.Replace(strings.TrimPrefix(root, "/"))
	if len(slug) > 64 {
		slug = slug[len(slug)-64:]
	}
	return slug
}

// run executes a git subcommand in dir and returns its combined stdout output.
func run(ctx context.Context, dir string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	return string(out), err
}
