// Package skill implements the Agent Skills standard (https://agentskills.io).
//
// # What is an Agent Skill?
//
// A skill is a directory containing at minimum a SKILL.md file:
//
//	skill-name/
//	├── SKILL.md          # Required: YAML frontmatter + Markdown instructions
//	├── scripts/          # Optional: executable code
//	├── references/       # Optional: documentation
//	└── assets/           # Optional: templates, resources
//
// # Progressive disclosure (three tiers)
//
// Tier 1 — Catalog (~50-100 tokens per skill, loaded at session start):
//
//	Only the name and description are exposed to the model so it can decide
//	when a skill is relevant without paying the cost of all instructions upfront.
//
// Tier 2 — Instructions (<5000 tokens recommended, loaded on activation):
//
//	The full SKILL.md body is injected when the model or user activates a skill.
//
// Tier 3 — Resources (loaded on demand):
//
//	Scripts, references, and assets are read individually when instructions
//	reference them.
//
// # Discovery paths
//
// Skills are scanned in two scopes. Later (project) scopes shadow earlier
// (user) ones when names collide.
//
// User scope (available to all projects):
//
//	~/.kvach/skills/        kvach-native user skills
//	~/.agents/skills/       cross-client interoperability convention
//
// Project scope (relative to working directory, overrides user scope):
//
//	<project>/.kvach/skills/    kvach-native project skills
//	<project>/.agents/skills/   cross-client interoperability convention
//
// Additional directories may be appended via config.SkillDirs.
package skill

import (
	"fmt"
	"path/filepath"
)

// Frontmatter holds the parsed YAML header of a SKILL.md file.
// All fields map directly to the Agent Skills specification.
type Frontmatter struct {
	// Name is the required skill identifier (1-64 chars, lowercase
	// alphanumeric + hyphens, no leading/trailing/consecutive hyphens).
	// Must match the parent directory name.
	Name string `yaml:"name"`

	// Description is the required summary of what the skill does and when to
	// use it (1-1024 chars). This is the only field loaded in Tier 1 (catalog)
	// so it must be self-sufficient for the model to decide relevance.
	Description string `yaml:"description"`

	// License is the optional license name or reference to a bundled file.
	License string `yaml:"license,omitempty"`

	// Compatibility is an optional note about environment requirements
	// (1-500 chars). Only set when the skill has specific prerequisites such
	// as required binaries, network access, or a target agent product.
	Compatibility string `yaml:"compatibility,omitempty"`

	// Metadata is an optional arbitrary key-value map for additional
	// properties not defined by the spec (e.g. author, version).
	Metadata map[string]string `yaml:"metadata,omitempty"`

	// AllowedTools is an optional space-delimited list of pre-approved tools
	// the skill may use without triggering a permission prompt.
	// Experimental — support varies between implementations.
	// Example: "Bash(git:*) Bash(jq:*) Read"
	AllowedTools string `yaml:"allowed-tools,omitempty"`
}

// Skill is a fully parsed Agent Skill, ready to be used by the agent.
type Skill struct {
	// Frontmatter contains all parsed metadata from the SKILL.md header.
	Frontmatter

	// Location is the absolute path to the SKILL.md file.
	// Used to resolve relative file references inside the instructions.
	Location string

	// BaseDir is the skill's root directory (parent of SKILL.md).
	// All relative paths in the instructions are resolved against BaseDir.
	BaseDir string

	// Body is the Markdown content after the frontmatter delimiter.
	// This is Tier 2 content — only loaded when the skill is activated.
	Body string

	// Resources is the list of files found under BaseDir (scripts/,
	// references/, assets/) surfaced to the model on activation so it can
	// load them on demand. Paths are relative to BaseDir.
	Resources []string

	// Config holds the parsed content of a companion config file
	// (config.yaml or config.json) found alongside SKILL.md. Nil when no
	// config file exists. Skills use this for user-tweakable settings
	// (e.g. multi-model review rounds, deployment targets).
	Config map[string]any

	// ConfigPath is the absolute path to the companion config file, or empty.
	ConfigPath string

	// Libraries lists helper scripts found in the skill's lib/ subdirectory.
	// Paths are relative to BaseDir (e.g. "lib/env.sh", "lib/rest.sh").
	// These are surfaced in the activation response so the model can
	// source/read them when the skill instructions reference them.
	Libraries []string

	// Source identifies where the skill was found, for collision logging.
	// One of "user-client", "user-agents", "project-client", "project-agents",
	// or "extra".
	Source string
}

// CatalogEntry is the Tier 1 representation of a skill — only name,
// description, and location. This is what the model sees at session start.
type CatalogEntry struct {
	Name        string
	Description string
	// Location is included so the model can read the SKILL.md directly via a
	// file-read tool if the agent uses file-read activation.
	Location string
}

// CatalogXML renders the catalog entry as the XML block recommended by the
// Agent Skills specification for injection into the system prompt or tool
// description.
func (e CatalogEntry) CatalogXML() string {
	return fmt.Sprintf(
		"  <skill>\n    <name>%s</name>\n    <description>%s</description>\n    <location>%s</location>\n  </skill>",
		e.Name, e.Description, e.Location,
	)
}

// ActivationXML renders the Tier 2 skill content wrapped in the structured
// tags recommended by the spec, including the resource listing, config
// summary, and library listing. This is what the activate_skill tool (or
// equivalent) returns to the model.
func (s *Skill) ActivationXML() string {
	extras := ""

	// Config summary.
	if len(s.Config) > 0 && s.ConfigPath != "" {
		extras += fmt.Sprintf("\n<skill_config path=%q>", s.ConfigPath)
		for k, v := range s.Config {
			extras += fmt.Sprintf("\n  %s: %v", k, v)
		}
		extras += "\n</skill_config>"
	}

	// Libraries.
	if len(s.Libraries) > 0 {
		extras += "\n<skill_libraries>"
		for _, lib := range s.Libraries {
			extras += fmt.Sprintf("\n  <file>%s</file>", lib)
		}
		extras += "\n</skill_libraries>"
	}

	// Resources.
	if len(s.Resources) > 0 {
		extras += "\n<skill_resources>"
		for _, r := range s.Resources {
			extras += fmt.Sprintf("\n  <file>%s</file>", r)
		}
		extras += "\n</skill_resources>"
	}

	return fmt.Sprintf(
		"<skill_content name=%q>\n%s\n\nSkill directory: %s\nRelative paths in this skill are relative to the skill directory.\n%s\n</skill_content>",
		s.Name, s.Body, s.BaseDir, extras,
	)
}

// Loader discovers and activates Agent Skills from the filesystem.
type Loader interface {
	// Discover scans all standard search paths (plus any extra dirs) and
	// returns one CatalogEntry per valid skill, de-duplicated by name with
	// project scope taking precedence over user scope.
	Discover(projectDir string, extraDirs []string) ([]CatalogEntry, error)

	// Activate loads the full Skill (Tier 2 + resources) for the named skill.
	// Returns an error if the name is not in the catalog.
	Activate(name string) (*Skill, error)

	// ParseFile parses a single SKILL.md at the given absolute path.
	// Used by Activate and also useful for testing.
	ParseFile(path string) (*Skill, error)
}

// SearchPaths returns the ordered list of directories to scan for skills,
// given the user home directory, project working directory, and any extra
// paths from config. The returned list is ordered from lowest precedence
// (user scope) to highest (project scope), matching the spec's collision rule.
func SearchPaths(homeDir, projectDir string, extraDirs []string) []struct {
	Dir    string
	Source string
} {
	return []struct {
		Dir    string
		Source string
	}{
		// User scope — lower precedence
		{filepath.Join(homeDir, ".kvach", "skills"), "user-client"},
		{filepath.Join(homeDir, ".agents", "skills"), "user-agents"},
		// Project scope — higher precedence (shadows user scope on collision)
		{filepath.Join(projectDir, ".kvach", "skills"), "project-client"},
		{filepath.Join(projectDir, ".agents", "skills"), "project-agents"},
		// Extra dirs from config — highest precedence
		// (added below from extraDirs)
	}
}

// ValidateName reports whether name is a valid Agent Skills skill name:
//   - 1-64 characters
//   - lowercase letters (a-z), digits (0-9), hyphens (-) only
//   - must not start or end with a hyphen
//   - must not contain consecutive hyphens (--)
func ValidateName(name string) error {
	if len(name) == 0 || len(name) > 64 {
		return fmt.Errorf("skill name %q: must be 1-64 characters", name)
	}
	if name[0] == '-' || name[len(name)-1] == '-' {
		return fmt.Errorf("skill name %q: must not start or end with a hyphen", name)
	}
	for i, r := range name {
		switch {
		case r >= 'a' && r <= 'z':
		case r >= '0' && r <= '9':
		case r == '-':
			if i > 0 && name[i-1] == '-' {
				return fmt.Errorf("skill name %q: must not contain consecutive hyphens", name)
			}
		default:
			return fmt.Errorf("skill name %q: contains invalid character %q (only a-z, 0-9, - allowed)", name, r)
		}
	}
	return nil
}
