package agent

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseProfileFile(t *testing.T) {
	content := `---
name: code-reviewer
description: "Reviews pull requests for correctness and style"
tools: Read, Glob, Grep, WebFetch
model: anthropic/claude-sonnet-4-5
color: blue
memory: project
---

# Code Review Instructions

When reviewing code, focus on:
- Correctness
- Performance
- Security
`

	dir := t.TempDir()
	path := filepath.Join(dir, "code-reviewer.md")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	p, err := ParseProfileFile(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if p.Name != "code-reviewer" {
		t.Fatalf("expected name 'code-reviewer', got %q", p.Name)
	}
	if p.Description != "Reviews pull requests for correctness and style" {
		t.Fatalf("wrong description: %q", p.Description)
	}
	if p.Model != "anthropic/claude-sonnet-4-5" {
		t.Fatalf("wrong model: %q", p.Model)
	}
	if p.Color != "blue" {
		t.Fatalf("wrong color: %q", p.Color)
	}
	if p.MemoryScope != "project" {
		t.Fatalf("wrong memory scope: %q", p.MemoryScope)
	}
	if len(p.Tools) != 4 {
		t.Fatalf("expected 4 tools, got %d: %v", len(p.Tools), p.Tools)
	}
	if p.Tools[0] != "Read" {
		t.Fatalf("expected first tool 'Read', got %q", p.Tools[0])
	}
	if p.SystemPrompt == "" {
		t.Fatal("expected non-empty system prompt body")
	}
	if p.SystemPrompt[:20] != "# Code Review Instru" {
		t.Fatalf("unexpected system prompt start: %q", p.SystemPrompt[:20])
	}
}

func TestParseProfileFileMissingName(t *testing.T) {
	content := `---
description: "no name"
---

body
`
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.md")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := ParseProfileFile(path)
	if err == nil {
		t.Fatal("expected error for missing name")
	}
}

func TestParseProfileFileNoFrontmatter(t *testing.T) {
	content := `# Just markdown

No frontmatter here.
`
	dir := t.TempDir()
	path := filepath.Join(dir, "no-fm.md")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := ParseProfileFile(path)
	if err == nil {
		t.Fatal("expected error for missing frontmatter")
	}
}

func TestLoadProfilesFromDir(t *testing.T) {
	dir := t.TempDir()

	// Write two valid profiles.
	writeProfile(t, dir, "alpha.md", "alpha", "Read, Glob")
	writeProfile(t, dir, "beta.md", "beta", "Bash")

	// Write a non-md file (should be skipped).
	if err := os.WriteFile(filepath.Join(dir, "readme.txt"), []byte("not a profile"), 0o644); err != nil {
		t.Fatal(err)
	}

	profiles, err := LoadProfilesFromDir(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(profiles) != 2 {
		t.Fatalf("expected 2 profiles, got %d", len(profiles))
	}
}

func TestLoadProfilesFromNonexistentDir(t *testing.T) {
	profiles, err := LoadProfilesFromDir("/nonexistent/path")
	if err != nil {
		t.Fatalf("expected nil error for nonexistent dir, got %v", err)
	}
	if len(profiles) != 0 {
		t.Fatalf("expected 0 profiles, got %d", len(profiles))
	}
}

func TestDiscoverProfiles(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()

	// Write user-scope profile.
	userDir := filepath.Join(home, ".kvach", "agents")
	if err := os.MkdirAll(userDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeProfile(t, userDir, "shared.md", "shared", "Read")

	// Write project-scope profile that overrides user.
	projectDir := filepath.Join(project, ".kvach", "agents")
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeProfile(t, projectDir, "shared.md", "shared", "Read, Write, Bash")

	reg := NewProfileRegistry()
	if err := DiscoverProfiles(reg, home, project, nil); err != nil {
		t.Fatalf("discover: %v", err)
	}

	p, ok := reg.Get("shared")
	if !ok {
		t.Fatal("expected profile 'shared'")
	}
	// Project scope should override user scope.
	if len(p.Tools) != 3 {
		t.Fatalf("expected 3 tools (project override), got %d: %v", len(p.Tools), p.Tools)
	}
}

func TestSplitFrontmatter(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantFM  string
		wantErr bool
	}{
		{
			name:   "standard",
			input:  "---\nname: test\n---\nbody",
			wantFM: "name: test",
		},
		{
			name:    "no opening",
			input:   "name: test\n---\nbody",
			wantErr: true,
		},
		{
			name:    "no closing",
			input:   "---\nname: test\nbody",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fm, _, err := splitFrontmatter([]byte(tt.input))
			if tt.wantErr && err == nil {
				t.Fatal("expected error")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !tt.wantErr && string(fm) != tt.wantFM {
				t.Fatalf("expected frontmatter %q, got %q", tt.wantFM, string(fm))
			}
		})
	}
}

func writeProfile(t *testing.T, dir, filename, name, tools string) {
	t.Helper()
	content := "---\nname: " + name + "\ntools: " + tools + "\n---\n\nBody for " + name + "\n"
	path := filepath.Join(dir, filename)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
