// Package config loads and merges agent configuration from multiple sources.
//
// Sources are applied in order; later sources override earlier ones:
//
//  1. Built-in defaults
//  2. System config  (/etc/kvach/config.json)
//  3. Global user    (~/.config/kvach/config.json or config.jsonc)
//  4. Project        (.kvach/config.json or .kvach/config.jsonc)
//  5. Environment    (KVACH_* variables)
//  6. CLI flags
//
// JSONC (JSON with // and /* */ comments) is supported in all file sources.
// CLAUDE.md / AGENTS.md files are discovered by walking upward from the
// project root and their content is concatenated into Config.Instructions.
package config

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/valpere/kvach/internal/hook"
	"github.com/valpere/kvach/internal/mcp"
	"github.com/valpere/kvach/internal/permission"
)

// Config is the merged configuration for a kvach session.
type Config struct {
	// Model is the default model identifier, e.g. "anthropic/claude-sonnet-4-5".
	Model string `json:"model,omitempty"`

	// Provider holds per-provider overrides and custom model definitions.
	Provider map[string]ProviderConfig `json:"provider,omitempty"`

	// Permission controls how tool calls are approved.
	Permission PermissionConfig `json:"permission,omitempty"`

	// MCP holds named MCP server configurations.
	MCP MCPConfig `json:"mcp,omitempty"`

	// Hooks maps hook event names to their matcher lists.
	Hooks map[string][]hook.Matcher `json:"hooks,omitempty"`

	// Agents holds named agent configuration overrides.
	Agents map[string]AgentConfig `json:"agents,omitempty"`

	// Server configures the HTTP API server.
	Server ServerConfig `json:"server,omitempty"`

	// SkillDirs lists extra directories to append to the standard Agent Skills
	// search paths after the built-in scopes have been scanned.
	//
	// Standard paths scanned automatically (no config needed):
	//   User scope:    ~/.kvach/skills/  and  ~/.agents/skills/
	//   Project scope: <project>/.kvach/skills/  and  <project>/.agents/skills/
	//
	// Project scope always overrides user scope on name collisions.
	// Entries in SkillDirs are appended last and therefore have the highest
	// precedence.
	//
	// See https://agentskills.io/client-implementation/adding-skills-support
	SkillDirs []string `json:"skillDirs,omitempty"`

	// Instructions is the concatenated content of all CLAUDE.md / AGENTS.md
	// files discovered for the current project. Injected into every LLM call.
	Instructions string `json:"-"`

	// MaxTurns is the default maximum agentic turns per run. Default 50.
	MaxTurns int `json:"maxTurns,omitempty"`

	// AutoMemory enables the persistent memory system. Default true.
	AutoMemory *bool `json:"autoMemory,omitempty"`
}

// ProviderConfig holds user overrides for a single provider.
type ProviderConfig struct {
	// APIKey overrides the environment-variable API key for this provider.
	APIKey string `json:"apiKey,omitempty"`
	// BaseURL overrides the provider's default API endpoint.
	BaseURL string `json:"baseUrl,omitempty"`
	// Models lists additional or overridden model definitions.
	Models map[string]ModelOverride `json:"models,omitempty"`
}

// ModelOverride allows users to tweak model metadata (e.g. context size,
// pricing) without changing the provider implementation.
type ModelOverride struct {
	Name            string  `json:"name,omitempty"`
	ContextTokens   int     `json:"contextTokens,omitempty"`
	MaxOutputTokens int     `json:"maxOutputTokens,omitempty"`
	InputCostPer1M  float64 `json:"inputCostPer1M,omitempty"`
	OutputCostPer1M float64 `json:"outputCostPer1M,omitempty"`
}

// PermissionConfig mirrors [permission.Mode] and default rule lists.
type PermissionConfig struct {
	Mode  permission.Mode   `json:"mode,omitempty"`
	Allow []permission.Rule `json:"allow,omitempty"`
	Deny  []permission.Rule `json:"deny,omitempty"`
}

// MCPConfig is the top-level MCP configuration block.
type MCPConfig struct {
	Servers map[string]mcp.ServerConfig `json:"servers,omitempty"`
}

// AgentConfig allows users to override the built-in agent definitions.
type AgentConfig struct {
	Description string `json:"description,omitempty"`
	// Model overrides the model used by this agent.
	Model string `json:"model,omitempty"`
	// Prompt replaces the agent's default system prompt when non-empty.
	Prompt string `json:"prompt,omitempty"`
	// MaxTurns overrides the global MaxTurns for this agent.
	MaxTurns int `json:"maxTurns,omitempty"`
	// Disabled removes the agent from the available agents list.
	Disabled bool `json:"disabled,omitempty"`
}

// ServerConfig controls the built-in HTTP API server.
type ServerConfig struct {
	// Host is the bind address. Default "127.0.0.1".
	Host string `json:"host,omitempty"`
	// Port is the listen port. Default 7777.
	Port int `json:"port,omitempty"`
	// Password enables HTTP Basic Auth when non-empty.
	Password string `json:"password,omitempty"`
}

// Paths holds the platform-specific file-system paths resolved at startup.
type Paths struct {
	// ConfigHome is the user-level config directory.
	// e.g. ~/.config/kvach
	ConfigHome string
	// DataHome is the user-level data directory.
	// e.g. ~/.local/share/kvach
	DataHome string
	// CacheHome is the user-level cache directory.
	// e.g. ~/.cache/kvach
	CacheHome string
	// SystemConfig is the system-wide config directory.
	// e.g. /etc/kvach
	SystemConfig string
}

// Load reads and merges configuration from all sources for the given project
// directory. It also discovers CLAUDE.md / AGENTS.md files and populates
// Config.Instructions.
func Load(projectDir string) (*Config, error) {
	cfg := &Config{
		MaxTurns: 50,
		Model:    "anthropic/claude-sonnet-4-5",
	}

	// Discover CLAUDE.md / AGENTS.md by walking upward from projectDir.
	cfg.Instructions = discoverInstructions(projectDir)

	// TODO(phase2): implement multi-source merge, JSONC parsing, env overrides.
	// For now we load the model from KVACH_MODEL env if set.
	if m := os.Getenv("KVACH_MODEL"); m != "" {
		cfg.Model = m
	}

	return cfg, nil
}

// discoverInstructions walks upward from dir to /, collecting CLAUDE.md and
// AGENTS.md files. Returns concatenated content with source headers.
func discoverInstructions(dir string) string {
	var parts []string
	seen := make(map[string]bool)

	for {
		for _, name := range []string{"CLAUDE.md", "AGENTS.md"} {
			p := filepath.Join(dir, name)
			if seen[p] {
				continue
			}
			seen[p] = true
			data, err := os.ReadFile(p)
			if err != nil {
				continue
			}
			content := strings.TrimSpace(string(data))
			if content != "" {
				parts = append(parts, "Instructions from: "+p+"\n"+content)
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return strings.Join(parts, "\n\n")
}

// ResolvePaths returns the platform-appropriate config/data/cache directories.
func ResolvePaths() Paths {
	home, _ := os.UserHomeDir()
	if home == "" {
		home = "/tmp"
	}

	// XDG-style paths with kvach subdirectory.
	configHome := os.Getenv("XDG_CONFIG_HOME")
	if configHome == "" {
		configHome = filepath.Join(home, ".config")
	}

	dataHome := os.Getenv("XDG_DATA_HOME")
	if dataHome == "" {
		dataHome = filepath.Join(home, ".local", "share")
	}

	cacheHome := os.Getenv("XDG_CACHE_HOME")
	if cacheHome == "" {
		cacheHome = filepath.Join(home, ".cache")
	}

	return Paths{
		ConfigHome:   filepath.Join(configHome, "kvach"),
		DataHome:     filepath.Join(dataHome, "kvach"),
		CacheHome:    filepath.Join(cacheHome, "kvach"),
		SystemConfig: "/etc/kvach",
	}
}
