package skill

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFSLoaderDiscoverAndActivate(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()

	// user skill
	userSkillDir := filepath.Join(home, ".kvach", "skills", "alpha")
	if err := os.MkdirAll(userSkillDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(userSkillDir, "SKILL.md"), []byte(`---
name: alpha
description: alpha skill
---

Use alpha.
`), 0o644); err != nil {
		t.Fatal(err)
	}

	// project override
	projectSkillDir := filepath.Join(project, ".kvach", "skills", "alpha")
	if err := os.MkdirAll(projectSkillDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(projectSkillDir, "SKILL.md"), []byte(`---
name: alpha
description: project alpha skill
---

Use project alpha.
`), 0o644); err != nil {
		t.Fatal(err)
	}

	loader := NewFSLoader(home)
	entries, err := loader.Discover(project, nil)
	if err != nil {
		t.Fatalf("discover: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].Description != "project alpha skill" {
		t.Fatalf("expected project override description, got %q", entries[0].Description)
	}

	sk, err := loader.Activate("alpha")
	if err != nil {
		t.Fatalf("activate: %v", err)
	}
	if sk.Name != "alpha" {
		t.Fatalf("expected name alpha, got %q", sk.Name)
	}
	if sk.Body == "" {
		t.Fatal("expected non-empty skill body")
	}
}

func TestParseFileWithConfigAndLib(t *testing.T) {
	d := t.TempDir()
	skillDir := filepath.Join(d, "beta")
	if err := os.MkdirAll(filepath.Join(skillDir, "lib"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(`---
name: beta
description: beta skill
---

Do beta.
`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "config.yaml"), []byte("flag: true\nvalue: 3\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "lib", "helpers.sh"), []byte("#!/bin/sh\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	loader := NewFSLoader(t.TempDir())
	sk, err := loader.ParseFile(filepath.Join(skillDir, "SKILL.md"))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if sk.Config == nil || len(sk.Config) == 0 {
		t.Fatal("expected config to be parsed")
	}
	if len(sk.Libraries) != 1 {
		t.Fatalf("expected 1 library file, got %d", len(sk.Libraries))
	}
}
