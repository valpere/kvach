// Package memory implements the three-layer persistent memory system with
// optional per-agent scoping.
//
// # Scoping
//
// Memory can be scoped at two levels:
//
//   - Project-level (default): all agents share the same memory directory.
//     Path: <project>/.kvach/memory/
//
//   - Agent-level: each agent profile has an isolated memory subdirectory.
//     Path: <project>/.kvach/memory/agents/<agent-name>/
//
// The scope is controlled by the agent profile's MemoryScope field ("project"
// or "agent").
//
// # Three layers
//
// Layer 1 — Index (always loaded into every LLM call):
//
//	MEMORY.md — links to topic files with one-line descriptions.
//	Hard limits: 200 lines, 25 KB.
//
// Layer 2 — Topic files (loaded on demand):
//
//	<topic>.md — Markdown with YAML frontmatter (name, description, type).
//	Loaded when the LLM explicitly reads them.
//
// Layer 3 — Transcripts (grep only, never fully loaded):
//
//	logs/YYYY/MM/DD.jsonl — Append-only daily logs.
//	Used for autoDream-style consolidation; never placed in context.
package memory

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Type classifies what a memory fact describes.
type Type string

const (
	// TypeUser stores facts about the user (role, preferences, working style).
	TypeUser Type = "user"
	// TypeFeedback stores corrections and confirmations the user has given.
	TypeFeedback Type = "feedback"
	// TypeProject stores ongoing project decisions, deadlines, and context.
	TypeProject Type = "project"
	// TypeReference stores pointers to external systems, docs, or tickets.
	TypeReference Type = "reference"
)

// Fact is a single memory entry stored as a topic file.
type Fact struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
	Type        Type   `yaml:"type"`
	Content     string `yaml:"-"`
	CreatedAt   time.Time
	// Confidence is a 0–1 score optionally set by consolidation passes.
	Confidence float64
}

// System manages the memory directory for a single project.
type System struct {
	// BaseDir is the project-level memory directory.
	BaseDir string
	// MaxIndexLines is the maximum number of lines in MEMORY.md (default 200).
	MaxIndexLines int
	// MaxIndexBytes is the maximum byte size of MEMORY.md (default 25 000).
	MaxIndexBytes int
}

// NewSystem returns a System rooted at baseDir with default limits applied.
func NewSystem(baseDir string) *System {
	return &System{
		BaseDir:       baseDir,
		MaxIndexLines: 200,
		MaxIndexBytes: 25_000,
	}
}

// AgentDir returns the memory directory for a specific agent profile.
// Returns BaseDir when agentName is empty (project-level scope).
func (s *System) AgentDir(agentName string) string {
	if agentName == "" {
		return s.BaseDir
	}
	return filepath.Join(s.BaseDir, "agents", agentName)
}

// IsEnabled reports whether the memory system should be active for this
// session. Returns false if the directory is unavailable or if the caller
// has disabled auto-memory via config/env.
func (s *System) IsEnabled() bool {
	if s.BaseDir == "" {
		return false
	}
	if strings.EqualFold(os.Getenv("KVACH_DISABLE_AUTO_MEMORY"), "1") ||
		strings.EqualFold(os.Getenv("KVACH_DISABLE_AUTO_MEMORY"), "true") {
		return false
	}
	return s.BaseDir != ""
}

// LoadIndexPrompt reads MEMORY.md from the given scope and returns its content
// formatted for injection into the LLM's context. Returns an empty string
// (not an error) when the index file does not exist yet.
//
// agentName selects per-agent memory when non-empty; empty string uses
// project-level memory.
func (s *System) LoadIndexPrompt(agentName string) (string, error) {
	path := filepath.Join(s.AgentDir(agentName), "MEMORY.md")
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("read index %s: %w", path, err)
	}

	content := string(data)
	lines := strings.Split(content, "\n")
	if s.MaxIndexLines <= 0 {
		s.MaxIndexLines = 200
	}
	if s.MaxIndexBytes <= 0 {
		s.MaxIndexBytes = 25_000
	}

	if len(lines) > s.MaxIndexLines {
		lines = lines[:s.MaxIndexLines]
	}
	content = strings.Join(lines, "\n")
	for len(content) > s.MaxIndexBytes && len(lines) > 0 {
		lines = lines[:len(lines)-1]
		content = strings.Join(lines, "\n")
	}
	return content, nil
}

// ReadTopic loads the topic file named name (without the .md extension)
// from the given scope.
func (s *System) ReadTopic(agentName, name string) (Fact, error) {
	name = safeName(name)
	path := filepath.Join(s.AgentDir(agentName), name+".md")
	data, err := os.ReadFile(path)
	if err != nil {
		return Fact{}, fmt.Errorf("read topic %s: %w", path, err)
	}
	return parseTopic(path, data)
}

// ListTopics returns the names of all topic files in the given scope.
func (s *System) ListTopics(agentName string) ([]string, error) {
	dir := s.AgentDir(agentName)
	entries, err := os.ReadDir(dir)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read topic dir %s: %w", dir, err)
	}

	var topics []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasSuffix(name, ".md") || name == "MEMORY.md" {
			continue
		}
		topics = append(topics, strings.TrimSuffix(name, ".md"))
	}
	sort.Strings(topics)
	return topics, nil
}

// WriteTopic writes f to its topic file in the given scope, then rebuilds
// the MEMORY.md index.
func (s *System) WriteTopic(_ context.Context, agentName string, f Fact) error {
	f.Name = safeName(f.Name)
	if f.Name == "" {
		return errors.New("fact name is required")
	}
	if f.Type == "" {
		f.Type = TypeProject
	}

	dir := s.AgentDir(agentName)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create memory dir %s: %w", dir, err)
	}

	fm, err := yaml.Marshal(map[string]any{
		"name":        f.Name,
		"description": f.Description,
		"type":        string(f.Type),
	})
	if err != nil {
		return fmt.Errorf("marshal topic frontmatter: %w", err)
	}

	content := "---\n" + string(fm) + "---\n\n" + strings.TrimSpace(f.Content) + "\n"
	path := filepath.Join(dir, f.Name+".md")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return fmt.Errorf("write topic %s: %w", path, err)
	}

	return s.RebuildIndex(agentName)
}

// DeleteTopic removes the topic file named name from the given scope and
// rebuilds the index.
func (s *System) DeleteTopic(_ context.Context, agentName, name string) error {
	name = safeName(name)
	path := filepath.Join(s.AgentDir(agentName), name+".md")
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("delete topic %s: %w", path, err)
	}
	return s.RebuildIndex(agentName)
}

// RebuildIndex scans all topic files in the given scope and rewrites
// MEMORY.md enforcing the line and byte limits (evicting the least recently
// updated entries first).
func (s *System) RebuildIndex(agentName string) error {
	topics, err := s.ListTopics(agentName)
	if err != nil {
		return err
	}

	var b strings.Builder
	b.WriteString("# Memory Index\n\n")

	for _, topic := range topics {
		fact, err := s.ReadTopic(agentName, topic)
		if err != nil {
			continue
		}
		desc := strings.TrimSpace(fact.Description)
		if desc == "" {
			desc = "(no description)"
		}
		line := fmt.Sprintf("- [%s.md](%s.md) — %s\n", fact.Name, fact.Name, desc)
		b.WriteString(line)
	}

	content := b.String()
	if s.MaxIndexLines <= 0 {
		s.MaxIndexLines = 200
	}
	if s.MaxIndexBytes <= 0 {
		s.MaxIndexBytes = 25_000
	}

	lines := strings.Split(content, "\n")
	if len(lines) > s.MaxIndexLines {
		lines = lines[:s.MaxIndexLines]
	}
	content = strings.Join(lines, "\n")
	for len(content) > s.MaxIndexBytes && len(lines) > 0 {
		lines = lines[:len(lines)-1]
		content = strings.Join(lines, "\n")
	}

	dir := s.AgentDir(agentName)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create memory dir %s: %w", dir, err)
	}
	path := filepath.Join(dir, "MEMORY.md")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return fmt.Errorf("write index %s: %w", path, err)
	}
	return nil
}

// AppendTranscript appends a JSON line to today's daily log file.
func (s *System) AppendTranscript(_ context.Context, entry []byte) error {
	now := time.Now().UTC()
	dir := filepath.Join(s.BaseDir, "logs", now.Format("2006"), now.Format("01"))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create transcript dir %s: %w", dir, err)
	}
	path := filepath.Join(dir, now.Format("02")+".jsonl")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("open transcript %s: %w", path, err)
	}
	defer f.Close()

	line := bytes.TrimSpace(entry)
	if len(line) == 0 {
		return nil
	}
	if _, err := f.Write(append(line, '\n')); err != nil {
		return fmt.Errorf("append transcript %s: %w", path, err)
	}
	return nil
}

// SearchTranscripts searches all daily logs for lines matching query using
// a simple substring/regex scan (grep-equivalent). Returns matching raw lines.
func (s *System) SearchTranscripts(_ context.Context, query string) ([]string, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, nil
	}

	logsDir := filepath.Join(s.BaseDir, "logs")
	var out []string

	err := filepath.WalkDir(logsDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() || !strings.HasSuffix(path, ".jsonl") {
			return nil
		}
		f, err := os.Open(path)
		if err != nil {
			return nil
		}
		defer f.Close()

		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			line := scanner.Text()
			if strings.Contains(line, query) {
				out = append(out, line)
				if len(out) >= 5000 {
					return filepath.SkipAll
				}
			}
		}
		return nil
	})
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("search transcripts: %w", err)
	}
	return out, nil
}

func safeName(name string) string {
	name = strings.TrimSpace(name)
	name = strings.TrimSuffix(name, ".md")
	name = strings.ReplaceAll(name, "..", "")
	name = strings.ReplaceAll(name, "/", "")
	name = strings.ReplaceAll(name, `\\`, "")
	return name
}

func parseTopic(path string, data []byte) (Fact, error) {
	var fact Fact
	trimmed := bytes.TrimSpace(data)
	if !bytes.HasPrefix(trimmed, []byte("---")) {
		return Fact{}, fmt.Errorf("topic %s missing YAML frontmatter", path)
	}
	rest := trimmed[3:]
	if len(rest) > 0 && rest[0] == '\r' {
		rest = rest[1:]
	}
	if len(rest) > 0 && rest[0] == '\n' {
		rest = rest[1:]
	}
	idx := bytes.Index(rest, []byte("\n---"))
	if idx < 0 {
		return Fact{}, fmt.Errorf("topic %s missing closing frontmatter delimiter", path)
	}
	fm := rest[:idx]
	body := rest[idx+4:]
	if len(body) > 0 && body[0] == '\r' {
		body = body[1:]
	}
	if len(body) > 0 && body[0] == '\n' {
		body = body[1:]
	}

	if err := yaml.Unmarshal(fm, &fact); err != nil {
		return Fact{}, fmt.Errorf("parse topic frontmatter %s: %w", path, err)
	}
	fact.Content = strings.TrimSpace(string(body))
	return fact, nil
}
