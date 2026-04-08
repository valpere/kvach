package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/valpere/kvach/internal/permission"
)

func TestLoadMergesSourcesWithEnvOverrides(t *testing.T) {
	tmp := t.TempDir()
	projectDir := filepath.Join(tmp, "project")
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatalf("mkdir project: %v", err)
	}

	globalConfigHome := filepath.Join(tmp, "xdg-config")
	t.Setenv("XDG_CONFIG_HOME", globalConfigHome)
	t.Setenv("KVACH_SYSTEM_CONFIG", filepath.Join(tmp, "system-config"))

	globalConfigPath := filepath.Join(globalConfigHome, "kvach", "config.json")
	if err := os.MkdirAll(filepath.Dir(globalConfigPath), 0o755); err != nil {
		t.Fatalf("mkdir global config dir: %v", err)
	}
	if err := os.WriteFile(globalConfigPath, []byte(`{
		"model": "anthropic/claude-3-5-sonnet",
		"maxTurns": 60,
		"permission": {"mode": "default"},
		"provider": {
			"openai": {
				"baseUrl": "https://api.global.example",
				"models": {
					"gpt-4o": {"name": "global-gpt-4o"}
				}
			}
		},
		"agents": {
			"review": {"description": "global reviewer", "maxTurns": 3}
		},
		"skillDirs": ["/global/skills"]
	}`), 0o644); err != nil {
		t.Fatalf("write global config: %v", err)
	}

	projectConfigPath := filepath.Join(projectDir, ".kvach", "config.jsonc")
	if err := os.MkdirAll(filepath.Dir(projectConfigPath), 0o755); err != nil {
		t.Fatalf("mkdir project config dir: %v", err)
	}
	if err := os.WriteFile(projectConfigPath, []byte(`{
		// project overrides
		"model": "openai/gpt-4.1",
		"permission": {
			"mode": "plan",
			"allow": [{"tool": "Read", "pattern": "**"}]
		},
		"provider": {
			"openai": {
				"apiKey": "project-key",
				"models": {
					"gpt-4o": {"maxOutputTokens": 4096}
				}
			}
		},
		"agents": {
			"review": {"model": "openai/gpt-4.1-mini"}
		},
		"skillDirs": ["./.kvach/skills-extra"],
		"server": {"host": "127.0.0.1", "port": 7777}
	}`), 0o644); err != nil {
		t.Fatalf("write project config: %v", err)
	}

	if err := os.WriteFile(filepath.Join(tmp, "AGENTS.md"), []byte("root instructions"), 0o644); err != nil {
		t.Fatalf("write root instructions: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectDir, "CLAUDE.md"), []byte("project instructions"), 0o644); err != nil {
		t.Fatalf("write project instructions: %v", err)
	}

	t.Setenv("KVACH_MODEL", "anthropic/claude-sonnet-4-5")
	t.Setenv("KVACH_MAX_TURNS", "42")
	t.Setenv("KVACH_AUTO_MEMORY", "true")
	t.Setenv("KVACH_PERMISSION_MODE", "bypassPermissions")
	t.Setenv("KVACH_SKILL_DIRS", "/env/skills1:/env/skills2,/env/skills3")
	t.Setenv("KVACH_SERVER_PORT", "8888")

	cfg, err := Load(projectDir)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	if cfg.Model != "anthropic/claude-sonnet-4-5" {
		t.Fatalf("model override mismatch: %q", cfg.Model)
	}
	if cfg.MaxTurns != 42 {
		t.Fatalf("maxTurns mismatch: got %d", cfg.MaxTurns)
	}
	if cfg.AutoMemory == nil || !*cfg.AutoMemory {
		t.Fatalf("autoMemory expected true, got %#v", cfg.AutoMemory)
	}
	if cfg.Permission.Mode != permission.ModeBypass {
		t.Fatalf("permission mode mismatch: %q", cfg.Permission.Mode)
	}
	if len(cfg.Permission.Allow) != 1 || cfg.Permission.Allow[0].Tool != "Read" {
		t.Fatalf("permission allow merge mismatch: %#v", cfg.Permission.Allow)
	}

	if cfg.Provider["openai"].BaseURL != "https://api.global.example" {
		t.Fatalf("provider base URL was not merged")
	}
	if cfg.Provider["openai"].APIKey != "project-key" {
		t.Fatalf("provider API key was not merged")
	}
	if cfg.Provider["openai"].Models["gpt-4o"].Name != "global-gpt-4o" {
		t.Fatalf("provider model name was not preserved")
	}
	if cfg.Provider["openai"].Models["gpt-4o"].MaxOutputTokens != 4096 {
		t.Fatalf("provider model maxOutputTokens was not overridden")
	}

	reviewAgent := cfg.Agents["review"]
	if reviewAgent.Description != "global reviewer" || reviewAgent.Model != "openai/gpt-4.1-mini" {
		t.Fatalf("agent merge mismatch: %#v", reviewAgent)
	}

	if cfg.Server.Host != "127.0.0.1" || cfg.Server.Port != 8888 {
		t.Fatalf("server merge/env mismatch: %#v", cfg.Server)
	}

	wantSkillDirs := []string{"/env/skills1", "/env/skills2", "/env/skills3"}
	if !reflect.DeepEqual(cfg.SkillDirs, wantSkillDirs) {
		t.Fatalf("skillDirs mismatch: got %#v want %#v", cfg.SkillDirs, wantSkillDirs)
	}

	if !strings.Contains(cfg.Instructions, "project instructions") || !strings.Contains(cfg.Instructions, "root instructions") {
		t.Fatalf("instructions were not discovered: %q", cfg.Instructions)
	}
}

func TestLoadInvalidEnvReturnsError(t *testing.T) {
	tmp := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tmp, "project"), 0o755); err != nil {
		t.Fatalf("mkdir project: %v", err)
	}
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(tmp, "xdg-config"))
	t.Setenv("KVACH_SYSTEM_CONFIG", filepath.Join(tmp, "system-config"))
	t.Setenv("KVACH_MAX_TURNS", "not-an-int")

	_, err := Load(filepath.Join(tmp, "project"))
	if err == nil || !strings.Contains(err.Error(), "KVACH_MAX_TURNS") {
		t.Fatalf("expected KVACH_MAX_TURNS error, got %v", err)
	}
}

func TestStripJSONCommentsPreservesStringContent(t *testing.T) {
	data := []byte(`{
		// line comment
		"url": "https://example.com/path//tail",
		"snippet": "/* keep this */",
		/* block comment */
		"value": 7
	}`)

	stripped, err := stripJSONComments(data)
	if err != nil {
		t.Fatalf("strip comments: %v", err)
	}

	var parsed map[string]any
	if err := json.Unmarshal(stripped, &parsed); err != nil {
		t.Fatalf("unmarshal stripped JSON: %v", err)
	}

	if parsed["url"] != "https://example.com/path//tail" {
		t.Fatalf("url changed after stripping: %#v", parsed["url"])
	}
	if parsed["snippet"] != "/* keep this */" {
		t.Fatalf("snippet changed after stripping: %#v", parsed["snippet"])
	}
}
