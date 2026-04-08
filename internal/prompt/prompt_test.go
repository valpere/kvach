package prompt

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInterpolate(t *testing.T) {
	tests := []struct {
		name string
		tmpl string
		vars map[string]string
		want string
	}{
		{
			name: "simple substitution",
			tmpl: "Hello {{name}}!",
			vars: map[string]string{"name": "val"},
			want: "Hello val!",
		},
		{
			name: "multiple variables",
			tmpl: "{{greeting}} {{name}}, welcome to {{project}}.",
			vars: map[string]string{"greeting": "Hi", "name": "val", "project": "kvach"},
			want: "Hi val, welcome to kvach.",
		},
		{
			name: "unknown key replaced with empty",
			tmpl: "Hello {{name}} from {{city}}.",
			vars: map[string]string{"name": "val"},
			want: "Hello val from .",
		},
		{
			name: "no variables",
			tmpl: "No variables here.",
			vars: nil,
			want: "No variables here.",
		},
		{
			name: "empty vars map",
			tmpl: "Hello {{name}}!",
			vars: map[string]string{},
			want: "Hello !",
		},
		{
			name: "spaces in key",
			tmpl: "Hello {{ name }}!",
			vars: map[string]string{"name": "val"},
			want: "Hello val!",
		},
		{
			name: "unclosed brace",
			tmpl: "Hello {{name",
			vars: map[string]string{"name": "val"},
			want: "Hello {{name",
		},
		{
			name: "no match bracket",
			tmpl: "No match here",
			vars: map[string]string{"name": "val"},
			want: "No match here",
		},
		{
			name: "adjacent variables",
			tmpl: "{{a}}{{b}}",
			vars: map[string]string{"a": "1", "b": "2"},
			want: "12",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Interpolate(tt.tmpl, tt.vars)
			if got != tt.want {
				t.Fatalf("Interpolate(%q, %v) = %q, want %q", tt.tmpl, tt.vars, got, tt.want)
			}
		})
	}
}

func TestEngineRender(t *testing.T) {
	e := NewEngine()
	e.Register("greet", "Hello {{name}}!")

	got := e.Render("greet", map[string]string{"name": "val"})
	if got != "Hello val!" {
		t.Fatalf("expected 'Hello val!', got %q", got)
	}

	// Missing template returns empty.
	got = e.Render("nonexistent", nil)
	if got != "" {
		t.Fatalf("expected empty for unknown template, got %q", got)
	}
}

func TestEngineDefaults(t *testing.T) {
	e := NewEngine()
	e.RegisterDefault("greet", "Default: {{name}}")

	// Default is used when no explicit template.
	got := e.Render("greet", map[string]string{"name": "val"})
	if got != "Default: val" {
		t.Fatalf("expected default, got %q", got)
	}

	// Explicit overrides default.
	e.Register("greet", "Override: {{name}}")
	got = e.Render("greet", map[string]string{"name": "val"})
	if got != "Override: val" {
		t.Fatalf("expected override, got %q", got)
	}
}

func TestEngineLoadDir(t *testing.T) {
	dir := t.TempDir()

	if err := os.WriteFile(filepath.Join(dir, "hello.md"), []byte("Hello {{name}}!"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "bye.txt"), []byte("Bye {{name}}."), 0o644); err != nil {
		t.Fatal(err)
	}
	// Non-template file should be ignored.
	if err := os.WriteFile(filepath.Join(dir, "data.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}

	e := NewEngine()
	if err := e.LoadDir(dir); err != nil {
		t.Fatalf("LoadDir: %v", err)
	}

	got := e.Render("hello", map[string]string{"name": "val"})
	if got != "Hello val!" {
		t.Fatalf("expected 'Hello val!', got %q", got)
	}

	got = e.Render("bye", map[string]string{"name": "val"})
	if got != "Bye val." {
		t.Fatalf("expected 'Bye val.', got %q", got)
	}

	// json file should not be loaded.
	_, found := e.Get("data")
	if found {
		t.Fatal("data.json should not have been loaded as a template")
	}
}

func TestEngineLoadDirNonexistent(t *testing.T) {
	e := NewEngine()
	if err := e.LoadDir("/nonexistent/dir"); err != nil {
		t.Fatalf("expected nil error for nonexistent dir, got %v", err)
	}
}

func TestEngineNames(t *testing.T) {
	e := NewEngine()
	e.Register("alpha", "a")
	e.RegisterDefault("beta", "b")
	e.Register("gamma", "c")

	names := e.Names()
	if len(names) != 3 {
		t.Fatalf("expected 3 names, got %d: %v", len(names), names)
	}

	// Check all present (order is not guaranteed due to map iteration).
	seen := make(map[string]bool)
	for _, n := range names {
		seen[n] = true
	}
	for _, want := range []string{"alpha", "beta", "gamma"} {
		if !seen[want] {
			t.Fatalf("missing template name %q", want)
		}
	}
}
