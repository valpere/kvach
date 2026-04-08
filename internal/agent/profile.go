package agent

import (
	"fmt"
	"strings"
	"sync"
)

// Profile defines a named agent configuration with its model, tool
// restrictions, system prompt, and behavioral metadata.
//
// Built-in profiles are registered in Go code. Users can add or override
// profiles via markdown files in standard discovery paths or the config file's
// agents map.
type Profile struct {
	// Name is the unique profile identifier (lowercase, alphanumeric + hyphens).
	// Examples: "general", "explore", "build", "review".
	Name string `yaml:"name" json:"name"`

	// Description explains when this profile should be used. Shown in the Task
	// tool's subagent_type documentation and in the /agents list command.
	Description string `yaml:"description" json:"description,omitempty"`

	// Model overrides the session-level model for this profile. Empty means
	// inherit the session default.
	Model string `yaml:"model" json:"model,omitempty"`

	// Tools is the allowlist of tool names available to this profile.
	// Empty means all enabled tools are available.
	Tools []string `yaml:"tools" json:"tools,omitempty"`

	// DeniedTools is a denylist applied after the allowlist. Use this to
	// remove specific tools from an otherwise-unrestricted profile.
	DeniedTools []string `yaml:"denied_tools" json:"denied_tools,omitempty"`

	// SystemPrompt is the full system prompt body (Markdown). Injected as the
	// system message for sessions using this profile. Empty means use the
	// default system prompt.
	SystemPrompt string `yaml:"-" json:"-"`

	// MaxTurns overrides the global turn limit for this profile. Zero means
	// inherit the session or global default.
	MaxTurns int `yaml:"max_turns" json:"max_turns,omitempty"`

	// MemoryScope controls which memory directory this profile reads/writes.
	// "project" (default) uses the project-level memory; "agent" uses a
	// per-agent subdirectory.
	MemoryScope string `yaml:"memory" json:"memory,omitempty"`

	// Color is a UI hint for TUI display (e.g. "yellow", "blue", "#ff0000").
	Color string `yaml:"color" json:"color,omitempty"`

	// Source records where this profile was loaded from, for diagnostics.
	// One of "builtin", "user", "project", "config".
	Source string `yaml:"-" json:"-"`

	// Disabled removes this profile from the available profiles list.
	Disabled bool `yaml:"disabled" json:"disabled,omitempty"`
}

// EffectiveMemoryScope returns the resolved memory scope, defaulting to
// "project" when MemoryScope is empty.
func (p Profile) EffectiveMemoryScope() string {
	if p.MemoryScope == "" {
		return "project"
	}
	return p.MemoryScope
}

// HasTool reports whether toolName is available to this profile after
// applying both the allow and deny lists.
func (p Profile) HasTool(toolName string) bool {
	// Check denylist first — explicit deny always wins.
	for _, d := range p.DeniedTools {
		if strings.EqualFold(d, toolName) {
			return false
		}
	}
	// Empty allowlist means all enabled tools are available.
	if len(p.Tools) == 0 {
		return true
	}
	for _, a := range p.Tools {
		if strings.EqualFold(a, toolName) {
			return true
		}
	}
	return false
}

// Validate returns an error if the profile has invalid fields.
func (p Profile) Validate() error {
	if strings.TrimSpace(p.Name) == "" {
		return fmt.Errorf("profile name is required")
	}
	for _, r := range p.Name {
		switch {
		case r >= 'a' && r <= 'z':
		case r >= '0' && r <= '9':
		case r == '-':
		default:
			return fmt.Errorf("profile name %q: invalid character %q (only a-z, 0-9, - allowed)", p.Name, r)
		}
	}
	if p.MaxTurns < 0 {
		return fmt.Errorf("profile %q: max_turns cannot be negative", p.Name)
	}
	return nil
}

// ProfileRegistry holds all known agent profiles, keyed by name.
type ProfileRegistry struct {
	mu       sync.RWMutex
	profiles map[string]Profile
	order    []string // registration order for deterministic iteration
}

// NewProfileRegistry returns an empty registry.
func NewProfileRegistry() *ProfileRegistry {
	return &ProfileRegistry{profiles: make(map[string]Profile)}
}

// Register adds or replaces a profile. Profiles registered later override
// earlier ones with the same name (this is the mechanism for user/project
// overrides of built-in profiles).
func (r *ProfileRegistry) Register(p Profile) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.profiles[p.Name]; !exists {
		r.order = append(r.order, p.Name)
	}
	r.profiles[p.Name] = p
}

// Get returns the profile with the given name, or false if not found.
func (r *ProfileRegistry) Get(name string) (Profile, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	p, ok := r.profiles[name]
	return p, ok
}

// All returns all non-disabled profiles in registration order.
func (r *ProfileRegistry) All() []Profile {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]Profile, 0, len(r.order))
	for _, name := range r.order {
		p := r.profiles[name]
		if !p.Disabled {
			out = append(out, p)
		}
	}
	return out
}

// Names returns the names of all non-disabled profiles in registration order.
func (r *ProfileRegistry) Names() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]string, 0, len(r.order))
	for _, name := range r.order {
		if !r.profiles[name].Disabled {
			out = append(out, name)
		}
	}
	return out
}

// RegisterBuiltins registers the default profiles that ship with kvach.
func (r *ProfileRegistry) RegisterBuiltins() {
	r.Register(Profile{
		Name:        "general",
		Description: "General-purpose agent for complex multi-step tasks. Has access to all tools. Use when no specialist profile is a better fit.",
		Source:      "builtin",
	})
	r.Register(Profile{
		Name:        "explore",
		Description: "Fast read-only agent for codebase exploration. Use for finding files, searching content, answering questions about code structure. Cannot modify files.",
		Tools:       []string{"Read", "Glob", "Grep", "Bash", "WebFetch"},
		DeniedTools: []string{"Write", "Edit", "MultiPatch"},
		Source:      "builtin",
		Color:       "cyan",
	})
	r.Register(Profile{
		Name:        "build",
		Description: "Implementation agent with full tool access. Use for writing code, running builds, creating files, and executing shell commands.",
		Source:      "builtin",
		Color:       "yellow",
	})
	r.Register(Profile{
		Name:        "review",
		Description: "Code review agent with read-only tools plus Write for generating reports. Cannot execute shell commands or modify source files directly.",
		Tools:       []string{"Read", "Glob", "Grep", "WebFetch", "Write"},
		DeniedTools: []string{"Bash", "Edit", "MultiPatch"},
		Source:      "builtin",
		Color:       "blue",
	})
}

// DefaultRegistry is the process-wide profile registry.
var DefaultProfileRegistry = NewProfileRegistry()

func init() {
	DefaultProfileRegistry.RegisterBuiltins()
}
