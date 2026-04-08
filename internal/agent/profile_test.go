package agent

import (
	"testing"
)

func TestProfileHasTool(t *testing.T) {
	tests := []struct {
		name     string
		profile  Profile
		toolName string
		want     bool
	}{
		{
			name:     "empty allowlist allows all",
			profile:  Profile{Name: "general"},
			toolName: "Bash",
			want:     true,
		},
		{
			name:     "allowlist includes tool",
			profile:  Profile{Name: "explore", Tools: []string{"Read", "Glob", "Grep"}},
			toolName: "Read",
			want:     true,
		},
		{
			name:     "allowlist excludes tool",
			profile:  Profile{Name: "explore", Tools: []string{"Read", "Glob", "Grep"}},
			toolName: "Bash",
			want:     false,
		},
		{
			name:     "denylist overrides allowlist",
			profile:  Profile{Name: "test", Tools: []string{"Read", "Write"}, DeniedTools: []string{"Write"}},
			toolName: "Write",
			want:     false,
		},
		{
			name:     "denylist on empty allowlist",
			profile:  Profile{Name: "test", DeniedTools: []string{"Bash"}},
			toolName: "Bash",
			want:     false,
		},
		{
			name:     "case insensitive match",
			profile:  Profile{Name: "test", Tools: []string{"read"}},
			toolName: "Read",
			want:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.profile.HasTool(tt.toolName); got != tt.want {
				t.Fatalf("HasTool(%q) = %v, want %v", tt.toolName, got, tt.want)
			}
		})
	}
}

func TestProfileValidate(t *testing.T) {
	tests := []struct {
		name    string
		profile Profile
		wantErr bool
	}{
		{
			name:    "valid",
			profile: Profile{Name: "explore"},
		},
		{
			name:    "empty name",
			profile: Profile{},
			wantErr: true,
		},
		{
			name:    "invalid char in name",
			profile: Profile{Name: "my_agent"},
			wantErr: true,
		},
		{
			name:    "uppercase in name",
			profile: Profile{Name: "MyAgent"},
			wantErr: true,
		},
		{
			name:    "negative max turns",
			profile: Profile{Name: "test", MaxTurns: -1},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.profile.Validate()
			if tt.wantErr && err == nil {
				t.Fatal("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("expected nil error, got %v", err)
			}
		})
	}
}

func TestProfileRegistryBuiltins(t *testing.T) {
	reg := NewProfileRegistry()
	reg.RegisterBuiltins()

	names := reg.Names()
	if len(names) < 4 {
		t.Fatalf("expected at least 4 built-in profiles, got %d: %v", len(names), names)
	}

	for _, want := range []string{"general", "explore", "build", "review"} {
		if _, ok := reg.Get(want); !ok {
			t.Fatalf("missing built-in profile %q", want)
		}
	}
}

func TestProfileRegistryOverride(t *testing.T) {
	reg := NewProfileRegistry()
	reg.Register(Profile{Name: "test", Description: "v1"})
	reg.Register(Profile{Name: "test", Description: "v2"})

	p, ok := reg.Get("test")
	if !ok {
		t.Fatal("expected profile 'test'")
	}
	if p.Description != "v2" {
		t.Fatalf("expected overridden description 'v2', got %q", p.Description)
	}

	// Should not duplicate in order list.
	names := reg.Names()
	count := 0
	for _, n := range names {
		if n == "test" {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("expected 'test' once in names, got %d", count)
	}
}

func TestProfileEffectiveMemoryScope(t *testing.T) {
	p := Profile{Name: "test"}
	if got := p.EffectiveMemoryScope(); got != "project" {
		t.Fatalf("expected default 'project', got %q", got)
	}

	p.MemoryScope = "agent"
	if got := p.EffectiveMemoryScope(); got != "agent" {
		t.Fatalf("expected 'agent', got %q", got)
	}
}
