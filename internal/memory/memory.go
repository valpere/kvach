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
	"context"
	"path/filepath"
	"time"
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
	// TODO(phase2): check KVACH_DISABLE_AUTO_MEMORY env, config flag.
	return s.BaseDir != ""
}

// LoadIndexPrompt reads MEMORY.md from the given scope and returns its content
// formatted for injection into the LLM's context. Returns an empty string
// (not an error) when the index file does not exist yet.
//
// agentName selects per-agent memory when non-empty; empty string uses
// project-level memory.
func (s *System) LoadIndexPrompt(agentName string) (string, error) {
	_ = filepath.Join(s.AgentDir(agentName), "MEMORY.md")
	// TODO(phase2): read file, respect MaxIndexLines / MaxIndexBytes.
	return "", nil
}

// ReadTopic loads the topic file named name (without the .md extension)
// from the given scope.
func (s *System) ReadTopic(agentName, name string) (Fact, error) {
	_ = filepath.Join(s.AgentDir(agentName), name+".md")
	// TODO(phase2): parse frontmatter + body.
	return Fact{}, nil
}

// ListTopics returns the names of all topic files in the given scope.
func (s *System) ListTopics(agentName string) ([]string, error) {
	_ = s.AgentDir(agentName)
	// TODO(phase2): read dir, collect *.md filenames (excluding MEMORY.md).
	return nil, nil
}

// WriteTopic writes f to its topic file in the given scope, then rebuilds
// the MEMORY.md index.
func (s *System) WriteTopic(_ context.Context, agentName string, f Fact) error {
	_ = filepath.Join(s.AgentDir(agentName), f.Name+".md")
	// TODO(phase2): write frontmatter + content, then RebuildIndex.
	return nil
}

// DeleteTopic removes the topic file named name from the given scope and
// rebuilds the index.
func (s *System) DeleteTopic(_ context.Context, agentName, name string) error {
	_ = filepath.Join(s.AgentDir(agentName), name+".md")
	// TODO(phase2): remove file, then RebuildIndex.
	return nil
}

// RebuildIndex scans all topic files in the given scope and rewrites
// MEMORY.md enforcing the line and byte limits (evicting the least recently
// updated entries first).
func (s *System) RebuildIndex(agentName string) error {
	_ = s.AgentDir(agentName)
	// TODO(phase2): implement.
	return nil
}

// AppendTranscript appends a JSON line to today's daily log file.
func (s *System) AppendTranscript(_ context.Context, entry []byte) error {
	// TODO(phase2): implement. Transcripts are always project-scoped.
	return nil
}

// SearchTranscripts searches all daily logs for lines matching query using
// a simple substring/regex scan (grep-equivalent). Returns matching raw lines.
func (s *System) SearchTranscripts(_ context.Context, query string) ([]string, error) {
	// TODO(phase2): implement.
	return nil, nil
}
