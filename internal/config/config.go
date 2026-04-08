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
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
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

	paths := ResolvePaths()

	systemConfigPath, err := firstExistingFile(
		filepath.Join(paths.SystemConfig, "config.json"),
		filepath.Join(paths.SystemConfig, "config.jsonc"),
	)
	if err != nil {
		return nil, fmt.Errorf("resolve system config: %w", err)
	}
	if systemConfigPath != "" {
		if err := loadAndMergeFile(cfg, systemConfigPath); err != nil {
			return nil, fmt.Errorf("load system config: %w", err)
		}
	}

	globalConfigPath, err := firstExistingFile(
		filepath.Join(paths.ConfigHome, "config.json"),
		filepath.Join(paths.ConfigHome, "config.jsonc"),
	)
	if err != nil {
		return nil, fmt.Errorf("resolve user config: %w", err)
	}
	if globalConfigPath != "" {
		if err := loadAndMergeFile(cfg, globalConfigPath); err != nil {
			return nil, fmt.Errorf("load user config: %w", err)
		}
	}

	projectConfigPath, err := firstExistingFile(
		filepath.Join(projectDir, ".kvach", "config.json"),
		filepath.Join(projectDir, ".kvach", "config.jsonc"),
	)
	if err != nil {
		return nil, fmt.Errorf("resolve project config: %w", err)
	}
	if projectConfigPath != "" {
		if err := loadAndMergeFile(cfg, projectConfigPath); err != nil {
			return nil, fmt.Errorf("load project config: %w", err)
		}
	}

	if err := applyEnvOverrides(cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

func firstExistingFile(paths ...string) (string, error) {
	for _, p := range paths {
		st, err := os.Stat(p)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return "", fmt.Errorf("stat %s: %w", p, err)
		}
		if st.IsDir() {
			continue
		}
		return p, nil
	}
	return "", nil
}

func loadAndMergeFile(cfg *Config, filePath string) error {
	raw, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("read %s: %w", filePath, err)
	}

	jsonData, err := stripJSONComments(raw)
	if err != nil {
		return fmt.Errorf("parse %s as JSONC: %w", filePath, err)
	}

	var src Config
	if err := json.Unmarshal(jsonData, &src); err != nil {
		return fmt.Errorf("decode %s: %w", filePath, err)
	}

	mergeConfig(cfg, &src)
	return nil
}

func applyEnvOverrides(cfg *Config) error {
	if m := strings.TrimSpace(os.Getenv("KVACH_MODEL")); m != "" {
		cfg.Model = m
	}

	if raw := strings.TrimSpace(os.Getenv("KVACH_MAX_TURNS")); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n <= 0 {
			return fmt.Errorf("invalid KVACH_MAX_TURNS %q", raw)
		}
		cfg.MaxTurns = n
	}

	if raw := strings.TrimSpace(os.Getenv("KVACH_AUTO_MEMORY")); raw != "" {
		b, err := strconv.ParseBool(raw)
		if err != nil {
			return fmt.Errorf("invalid KVACH_AUTO_MEMORY %q", raw)
		}
		cfg.AutoMemory = &b
	}

	if mode := strings.TrimSpace(os.Getenv("KVACH_PERMISSION_MODE")); mode != "" {
		cfg.Permission.Mode = permission.Mode(mode)
	}

	if dirs := splitPathList(strings.TrimSpace(os.Getenv("KVACH_SKILL_DIRS"))); len(dirs) > 0 {
		cfg.SkillDirs = dirs
	}

	if host := strings.TrimSpace(os.Getenv("KVACH_SERVER_HOST")); host != "" {
		cfg.Server.Host = host
	}

	if raw := strings.TrimSpace(os.Getenv("KVACH_SERVER_PORT")); raw != "" {
		p, err := strconv.Atoi(raw)
		if err != nil || p <= 0 {
			return fmt.Errorf("invalid KVACH_SERVER_PORT %q", raw)
		}
		cfg.Server.Port = p
	}

	return nil
}

func splitPathList(value string) []string {
	if value == "" {
		return nil
	}
	parts := strings.FieldsFunc(value, func(r rune) bool {
		return r == os.PathListSeparator || r == ','
	})
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func mergeConfig(dst, src *Config) {
	if src.Model != "" {
		dst.Model = src.Model
	}
	if src.MaxTurns != 0 {
		dst.MaxTurns = src.MaxTurns
	}
	if src.AutoMemory != nil {
		b := *src.AutoMemory
		dst.AutoMemory = &b
	}

	if src.Permission.Mode != "" {
		dst.Permission.Mode = src.Permission.Mode
	}
	if len(src.Permission.Allow) > 0 {
		dst.Permission.Allow = append([]permission.Rule(nil), src.Permission.Allow...)
	}
	if len(src.Permission.Deny) > 0 {
		dst.Permission.Deny = append([]permission.Rule(nil), src.Permission.Deny...)
	}

	if len(src.SkillDirs) > 0 {
		dst.SkillDirs = append([]string(nil), src.SkillDirs...)
	}

	if src.Server.Host != "" {
		dst.Server.Host = src.Server.Host
	}
	if src.Server.Port != 0 {
		dst.Server.Port = src.Server.Port
	}
	if src.Server.Password != "" {
		dst.Server.Password = src.Server.Password
	}

	if src.Provider != nil {
		if dst.Provider == nil {
			dst.Provider = make(map[string]ProviderConfig)
		}
		for name, p := range src.Provider {
			current, ok := dst.Provider[name]
			if !ok {
				dst.Provider[name] = p
				continue
			}
			mergeProviderConfig(&current, &p)
			dst.Provider[name] = current
		}
	}

	if src.MCP.Servers != nil {
		if dst.MCP.Servers == nil {
			dst.MCP.Servers = make(map[string]mcp.ServerConfig)
		}
		for name, server := range src.MCP.Servers {
			dst.MCP.Servers[name] = server
		}
	}

	if src.Hooks != nil {
		if dst.Hooks == nil {
			dst.Hooks = make(map[string][]hook.Matcher)
		}
		for event, matchers := range src.Hooks {
			dst.Hooks[event] = append([]hook.Matcher(nil), matchers...)
		}
	}

	if src.Agents != nil {
		if dst.Agents == nil {
			dst.Agents = make(map[string]AgentConfig)
		}
		for name, agentCfg := range src.Agents {
			current, ok := dst.Agents[name]
			if !ok {
				dst.Agents[name] = agentCfg
				continue
			}
			mergeAgentConfig(&current, &agentCfg)
			dst.Agents[name] = current
		}
	}
}

func mergeProviderConfig(dst, src *ProviderConfig) {
	if src.APIKey != "" {
		dst.APIKey = src.APIKey
	}
	if src.BaseURL != "" {
		dst.BaseURL = src.BaseURL
	}

	if src.Models != nil {
		if dst.Models == nil {
			dst.Models = make(map[string]ModelOverride)
		}
		for name, model := range src.Models {
			current, ok := dst.Models[name]
			if !ok {
				dst.Models[name] = model
				continue
			}
			mergeModelOverride(&current, &model)
			dst.Models[name] = current
		}
	}
}

func mergeModelOverride(dst, src *ModelOverride) {
	if src.Name != "" {
		dst.Name = src.Name
	}
	if src.ContextTokens != 0 {
		dst.ContextTokens = src.ContextTokens
	}
	if src.MaxOutputTokens != 0 {
		dst.MaxOutputTokens = src.MaxOutputTokens
	}
	if src.InputCostPer1M != 0 {
		dst.InputCostPer1M = src.InputCostPer1M
	}
	if src.OutputCostPer1M != 0 {
		dst.OutputCostPer1M = src.OutputCostPer1M
	}
}

func mergeAgentConfig(dst, src *AgentConfig) {
	if src.Description != "" {
		dst.Description = src.Description
	}
	if src.Model != "" {
		dst.Model = src.Model
	}
	if src.Prompt != "" {
		dst.Prompt = src.Prompt
	}
	if src.MaxTurns != 0 {
		dst.MaxTurns = src.MaxTurns
	}
	if src.Disabled {
		dst.Disabled = true
	}
}

func stripJSONComments(data []byte) ([]byte, error) {
	out := make([]byte, 0, len(data))
	inString := false
	escaped := false
	inLineComment := false
	inBlockComment := false

	for i := 0; i < len(data); i++ {
		c := data[i]

		if inLineComment {
			if c == '\n' {
				inLineComment = false
				out = append(out, c)
			}
			continue
		}

		if inBlockComment {
			if c == '*' && i+1 < len(data) && data[i+1] == '/' {
				inBlockComment = false
				i++
			}
			continue
		}

		if inString {
			out = append(out, c)
			if escaped {
				escaped = false
				continue
			}
			if c == '\\' {
				escaped = true
				continue
			}
			if c == '"' {
				inString = false
			}
			continue
		}

		if c == '"' {
			inString = true
			out = append(out, c)
			continue
		}

		if c == '/' && i+1 < len(data) {
			next := data[i+1]
			if next == '/' {
				inLineComment = true
				i++
				continue
			}
			if next == '*' {
				inBlockComment = true
				i++
				continue
			}
		}

		out = append(out, c)
	}

	if inBlockComment {
		return nil, errors.New("unterminated block comment")
	}

	return out, nil
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

	systemConfig := os.Getenv("KVACH_SYSTEM_CONFIG")
	if systemConfig == "" {
		systemConfig = "/etc/kvach"
	}

	return Paths{
		ConfigHome:   filepath.Join(configHome, "kvach"),
		DataHome:     filepath.Join(dataHome, "kvach"),
		CacheHome:    filepath.Join(cacheHome, "kvach"),
		SystemConfig: systemConfig,
	}
}
