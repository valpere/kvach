// Package prompt implements a simple template engine for agent system prompts
// and skill instructions.
//
// Templates use {{key}} variable interpolation. No logic, no loops — just
// substitution. This is intentional: complex templating is a code smell in
// agent prompts.
//
// Usage:
//
//	engine := prompt.NewEngine()
//	engine.Register("greeting", "Hello {{name}}, you are working on {{project}}.")
//	result := engine.Render("greeting", map[string]string{"name": "val", "project": "kvach"})
//	// -> "Hello val, you are working on kvach."
package prompt

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// Engine manages named templates and renders them with variable substitution.
type Engine struct {
	mu        sync.RWMutex
	templates map[string]string
	defaults  map[string]string
}

// NewEngine returns an Engine with no templates loaded.
func NewEngine() *Engine {
	return &Engine{
		templates: make(map[string]string),
		defaults:  make(map[string]string),
	}
}

// Register stores a named template, replacing any existing one with the same
// name.
func (e *Engine) Register(name, template string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.templates[name] = template
}

// RegisterDefault stores a fallback template used when no explicit template
// has been registered for the given name.
func (e *Engine) RegisterDefault(name, template string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.defaults[name] = template
}

// Get returns the raw template string for name (checking templates first,
// then defaults). Returns empty string and false if not found.
func (e *Engine) Get(name string) (string, bool) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	if t, ok := e.templates[name]; ok {
		return t, true
	}
	if t, ok := e.defaults[name]; ok {
		return t, true
	}
	return "", false
}

// Render looks up the named template and replaces all {{key}} occurrences
// with corresponding values from vars. Unknown keys are replaced with empty
// strings.
//
// Returns empty string when no template is found for name.
func (e *Engine) Render(name string, vars map[string]string) string {
	tmpl, ok := e.Get(name)
	if !ok {
		return ""
	}
	return Interpolate(tmpl, vars)
}

// Interpolate performs {{key}} substitution on tmpl using vars. Unknown keys
// are replaced with empty strings. This is exported for use outside the
// engine (e.g. one-off in-memory templates).
func Interpolate(tmpl string, vars map[string]string) string {
	if vars == nil || !strings.Contains(tmpl, "{{") {
		return tmpl
	}

	var b strings.Builder
	b.Grow(len(tmpl))

	for {
		start := strings.Index(tmpl, "{{")
		if start < 0 {
			b.WriteString(tmpl)
			break
		}
		b.WriteString(tmpl[:start])
		tmpl = tmpl[start+2:]

		end := strings.Index(tmpl, "}}")
		if end < 0 {
			// Unclosed {{ — write literally and stop.
			b.WriteString("{{")
			b.WriteString(tmpl)
			break
		}

		key := strings.TrimSpace(tmpl[:end])
		if val, ok := vars[key]; ok {
			b.WriteString(val)
		}
		// Unknown key — replaced with empty string (omitted).
		tmpl = tmpl[end+2:]
	}

	return b.String()
}

// LoadDir reads all *.md and *.txt files from dir and registers them as
// templates. The template name is the filename without extension.
func (e *Engine) LoadDir(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		ext := filepath.Ext(entry.Name())
		if ext != ".md" && ext != ".txt" {
			continue
		}
		name := strings.TrimSuffix(entry.Name(), ext)
		data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			continue // skip unreadable files
		}
		e.Register(name, string(data))
	}
	return nil
}

// Names returns the names of all registered templates (explicit + defaults),
// deduplicated.
func (e *Engine) Names() []string {
	e.mu.RLock()
	defer e.mu.RUnlock()
	seen := make(map[string]bool)
	var out []string
	for name := range e.templates {
		if !seen[name] {
			seen[name] = true
			out = append(out, name)
		}
	}
	for name := range e.defaults {
		if !seen[name] {
			seen[name] = true
			out = append(out, name)
		}
	}
	return out
}
