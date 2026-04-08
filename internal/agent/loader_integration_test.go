package agent

import (
	"os"
	"path/filepath"
	"testing"
)

// TestLoadKvachAgents verifies that the actual .kvach/agents/ definitions
// in this repository parse correctly. This test is skipped in CI if the
// directory doesn't exist relative to the test working directory.
func TestLoadKvachAgents(t *testing.T) {
	// Walk up to find the repo root (where .kvach/ lives).
	dir, err := os.Getwd()
	if err != nil {
		t.Skip("cannot determine working directory")
	}
	for {
		candidate := filepath.Join(dir, ".kvach", "agents")
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			profiles, err := LoadProfilesFromDir(candidate)
			if err != nil {
				t.Fatalf("LoadProfilesFromDir(%s): %v", candidate, err)
			}
			if len(profiles) == 0 {
				t.Fatalf("expected at least 1 profile in %s", candidate)
			}
			for _, p := range profiles {
				t.Logf("loaded profile: name=%q tools=%v color=%q", p.Name, p.Tools, p.Color)
				if err := p.Validate(); err != nil {
					t.Errorf("profile %q failed validation: %v", p.Name, err)
				}
				if p.SystemPrompt == "" {
					t.Errorf("profile %q has empty system prompt", p.Name)
				}
			}
			return
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Skip(".kvach/agents/ not found in parent directories")
		}
		dir = parent
	}
}
