package agent

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// loaderFrontmatter is the subset of Profile fields that appear in the YAML
// frontmatter of agent definition markdown files. SystemPrompt comes from the
// body after the frontmatter.
type loaderFrontmatter struct {
	Name        string   `yaml:"name"`
	Description string   `yaml:"description"`
	Model       string   `yaml:"model"`
	Tools       string   `yaml:"tools"`        // comma-separated in YAML, split into slice
	DeniedTools string   `yaml:"denied_tools"` // comma-separated
	MaxTurns    int      `yaml:"max_turns"`
	Memory      string   `yaml:"memory"`
	Color       string   `yaml:"color"`
	Disabled    bool     `yaml:"disabled"`
	ToolsList   []string `yaml:"-"` // parsed from Tools
	DeniedList  []string `yaml:"-"` // parsed from DeniedTools
}

// ParseProfileFile parses a single agent definition markdown file (YAML
// frontmatter + Markdown body) into a Profile.
//
// Expected format:
//
//	---
//	name: my-agent
//	description: "What this agent does"
//	tools: Read, Glob, Grep, Bash
//	model: anthropic/claude-sonnet-4-5
//	color: blue
//	memory: project
//	---
//
//	# System Prompt Body
//
//	Instructions for the agent...
func ParseProfileFile(path string) (Profile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Profile{}, fmt.Errorf("read agent file %s: %w", path, err)
	}
	return parseProfileContent(data, path)
}

func parseProfileContent(data []byte, sourcePath string) (Profile, error) {
	// Split frontmatter from body.
	frontmatter, body, err := splitFrontmatter(data)
	if err != nil {
		return Profile{}, fmt.Errorf("parse agent file %s: %w", sourcePath, err)
	}

	var fm loaderFrontmatter
	if err := yaml.Unmarshal(frontmatter, &fm); err != nil {
		return Profile{}, fmt.Errorf("parse agent frontmatter %s: %w", sourcePath, err)
	}

	if fm.Name == "" {
		return Profile{}, fmt.Errorf("parse agent file %s: name is required in frontmatter", sourcePath)
	}

	// Parse comma-separated tool lists.
	tools := splitCommaSeparated(fm.Tools)
	denied := splitCommaSeparated(fm.DeniedTools)

	p := Profile{
		Name:         fm.Name,
		Description:  fm.Description,
		Model:        fm.Model,
		Tools:        tools,
		DeniedTools:  denied,
		SystemPrompt: strings.TrimSpace(string(body)),
		MaxTurns:     fm.MaxTurns,
		MemoryScope:  fm.Memory,
		Color:        fm.Color,
		Disabled:     fm.Disabled,
		Source:       "file:" + sourcePath,
	}

	if err := p.Validate(); err != nil {
		return Profile{}, fmt.Errorf("validate agent file %s: %w", sourcePath, err)
	}

	return p, nil
}

// LoadProfilesFromDir scans dir for *.md files and parses each as an agent
// profile. Returns all successfully parsed profiles. Files that fail to parse
// are skipped with a warning logged to stderr.
func LoadProfilesFromDir(dir string) ([]Profile, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("scan agent dir %s: %w", dir, err)
	}

	var profiles []Profile
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		p, err := ParseProfileFile(path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "kvach: skip agent file %s: %v\n", path, err)
			continue
		}
		profiles = append(profiles, p)
	}
	return profiles, nil
}

// DiscoverProfiles scans standard and extra directories for agent profiles
// and registers them in the given registry. Later scopes override earlier ones
// (project overrides user, extra overrides project).
func DiscoverProfiles(reg *ProfileRegistry, homeDir, projectDir string, extraDirs []string) error {
	dirs := AgentSearchPaths(homeDir, projectDir, extraDirs)
	for _, d := range dirs {
		profiles, err := LoadProfilesFromDir(d.Dir)
		if err != nil {
			return err
		}
		for _, p := range profiles {
			if p.Source == "" || strings.HasPrefix(p.Source, "file:") {
				p.Source = d.Source
			}
			reg.Register(p)
		}
	}
	return nil
}

// AgentSearchPath is a directory + source tag pair for agent discovery.
type AgentSearchPath struct {
	Dir    string
	Source string
}

// AgentSearchPaths returns the ordered list of directories to scan for agent
// profiles, from lowest to highest precedence.
func AgentSearchPaths(homeDir, projectDir string, extraDirs []string) []AgentSearchPath {
	paths := []AgentSearchPath{
		{filepath.Join(homeDir, ".kvach", "agents"), "user-client"},
		{filepath.Join(homeDir, ".agents", "agents"), "user-agents"},
		{filepath.Join(projectDir, ".kvach", "agents"), "project-client"},
		{filepath.Join(projectDir, ".agents", "agents"), "project-agents"},
	}
	for _, d := range extraDirs {
		paths = append(paths, AgentSearchPath{Dir: d, Source: "extra"})
	}
	return paths
}

// splitFrontmatter splits a document into YAML frontmatter and Markdown body.
// The document must start with "---\n", followed by YAML, then "---\n" or
// "---\r\n", then the body. Returns an error if frontmatter delimiters are
// not found.
func splitFrontmatter(data []byte) (frontmatter, body []byte, err error) {
	const delimiter = "---"

	trimmed := bytes.TrimLeft(data, " \t\r\n")
	if !bytes.HasPrefix(trimmed, []byte(delimiter)) {
		return nil, nil, fmt.Errorf("missing opening frontmatter delimiter (---)")
	}

	// Skip first delimiter line.
	afterFirst := trimmed[len(delimiter):]
	if len(afterFirst) > 0 && afterFirst[0] == '\r' {
		afterFirst = afterFirst[1:]
	}
	if len(afterFirst) > 0 && afterFirst[0] == '\n' {
		afterFirst = afterFirst[1:]
	}

	// Find closing delimiter.
	idx := bytes.Index(afterFirst, []byte("\n"+delimiter))
	if idx < 0 {
		return nil, nil, fmt.Errorf("missing closing frontmatter delimiter (---)")
	}

	frontmatter = afterFirst[:idx]
	rest := afterFirst[idx+1+len(delimiter):]

	// Skip the rest of the delimiter line.
	if len(rest) > 0 && rest[0] == '\r' {
		rest = rest[1:]
	}
	if len(rest) > 0 && rest[0] == '\n' {
		rest = rest[1:]
	}

	return frontmatter, rest, nil
}

// splitCommaSeparated splits a comma-separated string, trims each element,
// and returns non-empty elements.
func splitCommaSeparated(s string) []string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
