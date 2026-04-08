package permission

import "testing"

func TestMatchRuleBash(t *testing.T) {
	rule := Rule{Tool: "Bash", Pattern: "git:*"}

	tests := []struct {
		name  string
		input map[string]any
		want  bool
	}{
		{"git status", map[string]any{"command": "git status"}, true},
		{"git push origin", map[string]any{"command": "git push origin main"}, true},
		{"bare git", map[string]any{"command": "git"}, true},
		{"gitignore (no space)", map[string]any{"command": "gitignore"}, false},
		{"npm run", map[string]any{"command": "npm run test"}, false},
		{"empty command", map[string]any{"command": ""}, false},
		{"no command field", map[string]any{}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MatchRule(rule, "Bash", tt.input)
			if got != tt.want {
				t.Fatalf("MatchRule(Bash, %v) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestMatchRuleWebFetch(t *testing.T) {
	rule := Rule{Tool: "WebFetch", Pattern: "domain:docs.github.com"}

	tests := []struct {
		name  string
		input map[string]any
		want  bool
	}{
		{"exact match", map[string]any{"url": "https://docs.github.com/en/rest"}, true},
		{"with port", map[string]any{"url": "https://docs.github.com:443/en/rest"}, true},
		{"different domain", map[string]any{"url": "https://api.github.com/repos"}, false},
		{"subdomain", map[string]any{"url": "https://sub.docs.github.com/page"}, true},
		{"no url", map[string]any{}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MatchRule(rule, "WebFetch", tt.input)
			if got != tt.want {
				t.Fatalf("MatchRule(WebFetch, %v) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestMatchRuleFilePath(t *testing.T) {
	rule := Rule{Tool: "Read", Pattern: "///home/val/wrk/**"}

	tests := []struct {
		name  string
		input map[string]any
		want  bool
	}{
		{"within path", map[string]any{"filePath": "/home/val/wrk/project/main.go"}, true},
		{"exact dir", map[string]any{"filePath": "/home/val/wrk/foo"}, true},
		{"outside path", map[string]any{"filePath": "/etc/passwd"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MatchRule(rule, "Read", tt.input)
			if got != tt.want {
				t.Fatalf("MatchRule(Read, %v) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestMatchRuleEmptyPattern(t *testing.T) {
	rule := Rule{Tool: "Bash", Pattern: ""}
	got := MatchRule(rule, "Bash", map[string]any{"command": "rm -rf /"})
	if !got {
		t.Fatal("empty pattern should match any call")
	}
}

func TestMatchRuleWildcard(t *testing.T) {
	rule := Rule{Tool: "Read", Pattern: "*"}
	got := MatchRule(rule, "Read", map[string]any{"filePath": "/any/path"})
	if !got {
		t.Fatal("wildcard pattern should match any call")
	}
}

func TestMatchRuleToolNameMismatch(t *testing.T) {
	rule := Rule{Tool: "Bash", Pattern: "git:*"}
	got := MatchRule(rule, "Read", map[string]any{"command": "git status"})
	if got {
		t.Fatal("should not match when tool names differ")
	}
}

func TestMatchRuleCaseInsensitive(t *testing.T) {
	rule := Rule{Tool: "bash", Pattern: "git:*"}
	got := MatchRule(rule, "Bash", map[string]any{"command": "git status"})
	if !got {
		t.Fatal("tool name match should be case-insensitive")
	}
}

func TestExtractHost(t *testing.T) {
	tests := []struct {
		url  string
		want string
	}{
		{"https://example.com/path", "example.com"},
		{"http://user:pass@host.io/path", "host.io"},
		{"https://example.com:8080/path", "example.com"},
		{"example.com", "example.com"},
	}

	for _, tt := range tests {
		t.Run(tt.url, func(t *testing.T) {
			got := extractHost(tt.url)
			if got != tt.want {
				t.Fatalf("extractHost(%q) = %q, want %q", tt.url, got, tt.want)
			}
		})
	}
}
