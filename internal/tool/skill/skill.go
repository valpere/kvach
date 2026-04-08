// Package skill implements the activate_skill tool.
//
// This tool is the Tier 2 activation mechanism defined by the Agent Skills
// specification (https://agentskills.io/client-implementation/adding-skills-support).
//
// # Activation flow
//
//  1. At session start the agent builds a skill catalog (Tier 1) from all
//     discovered skills and injects it into the system prompt as XML.
//  2. The model reads the catalog and decides which skill is relevant.
//  3. The model calls activate_skill with the skill name.
//  4. This tool returns the full SKILL.md body wrapped in <skill_content> tags
//     plus a <skill_resources> listing (Tier 2).
//  5. The model follows the instructions, calling the agent's file-read tool
//     to load any referenced scripts/references/assets as needed (Tier 3).
//
// The input schema constrains the name parameter to the set of valid skill
// names, preventing the model from hallucinating nonexistent skills.
//
// Skills that are already loaded in the current session context are not
// re-injected (deduplication); the tool returns a short acknowledgement
// instead.
//
// This package self-registers via init().
package skill

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/valpere/kvach/internal/tool"
)

// SystemPromptInstructions is the behavioral guidance added alongside the
// skill catalog in the system prompt. It tells the model how to use
// activate_skill and how to interpret relative paths inside a skill.
const SystemPromptInstructions = `The following skills provide specialized instructions for specific tasks.
When a task matches a skill's description, call the activate_skill tool with the skill's name.
After activation, follow the skill's instructions exactly.
When a skill references relative paths (e.g. scripts/extract.py), resolve them against
the skill directory reported in the activation result and use absolute paths in tool calls.`

// CatalogSystemPrompt builds the full system prompt section for skills:
// the behavioral instructions followed by the XML catalog. Returns an empty
// string when entries is empty (spec: omit the section entirely).
func CatalogSystemPrompt(entries []CatalogEntry) string {
	if len(entries) == 0 {
		return ""
	}
	xml := "<available_skills>\n"
	for _, e := range entries {
		xml += e.CatalogXML() + "\n"
	}
	xml += "</available_skills>"
	return SystemPromptInstructions + "\n\n" + xml
}

// CatalogEntry is a light alias so tool callers don't need to import the skill
// package directly. It re-exports skill.CatalogEntry fields.
type CatalogEntry struct {
	Name        string
	Description string
	Location    string
}

// CatalogXML renders a single catalog entry as the XML recommended by the spec.
func (e CatalogEntry) CatalogXML() string {
	return fmt.Sprintf(
		"  <skill>\n    <name>%s</name>\n    <description>%s</description>\n    <location>%s</location>\n  </skill>",
		e.Name, e.Description, e.Location,
	)
}

// Input is the schema for an activate_skill tool call.
type Input struct {
	// Name must exactly match a skill name from the catalog injected at
	// session start. The JSON schema enum is populated dynamically at session
	// build time so the model cannot invent skill names.
	Name string `json:"name"`
}

// skillTool is the activate_skill tool implementation.
type skillTool struct {
	// validNames is the set of known skill names at session construction time.
	// Used to build the enum constraint in the input schema.
	validNames []string
	// activated tracks names already injected in this session to skip re-injection.
	activated map[string]bool
}

func init() { tool.DefaultRegistry.Register(&skillTool{}) }

func (s *skillTool) Name() string      { return "activate_skill" }
func (s *skillTool) Aliases() []string { return []string{"Skill"} }

// InputSchema returns the JSON Schema for the tool. The name parameter is
// constrained to an enum of valid skill names when validNames is non-empty,
// per the spec recommendation:
//
//	"If you use a dedicated activation tool, constrain the name parameter to
//	the set of valid skill names (e.g., as an enum in the tool schema)."
func (s *skillTool) InputSchema() map[string]any {
	nameProp := map[string]any{
		"type":        "string",
		"description": "Name of the skill to activate, exactly as listed in the available_skills catalog.",
	}
	if len(s.validNames) > 0 {
		nameProp["enum"] = s.validNames
	}
	return map[string]any{
		"type":       "object",
		"properties": map[string]any{"name": nameProp},
		"required":   []string{"name"},
	}
}

func (s *skillTool) ValidateInput(raw json.RawMessage) error {
	var in Input
	return json.Unmarshal(raw, &in)
}

func (s *skillTool) CheckPermissions(_ json.RawMessage, _ *tool.Context) tool.PermissionOutcome {
	// Skills are pre-approved; no permission prompt required.
	// Per the spec: "allowlist skill directories so the model can read
	// bundled resources without triggering user confirmation prompts."
	return tool.PermissionOutcome{Decision: "allow"}
}

// IsEnabled returns false when no skills are in the catalog, per the spec:
// "If no skills are discovered, don't register the tool at all."
// In practice we register but disable; the session builder filters disabled
// tools from the pool it sends to the LLM.
func (s *skillTool) IsEnabled(_ *tool.Context) bool {
	return len(s.validNames) > 0
}

func (s *skillTool) IsConcurrencySafe(_ json.RawMessage) bool { return true }
func (s *skillTool) IsReadOnly(_ json.RawMessage) bool        { return true }
func (s *skillTool) IsDestructive(_ json.RawMessage) bool     { return false }

func (s *skillTool) Prompt(_ tool.PromptOptions) string { return "" }

// Call activates a skill by returning its Tier 2 content wrapped in
// <skill_content> structured tags, per the spec. If the skill was already
// activated in this session it returns a short acknowledgement instead of
// re-injecting the full content.
func (s *skillTool) Call(_ context.Context, raw json.RawMessage, _ *tool.Context) (*tool.Result, error) {
	var in Input
	if err := json.Unmarshal(raw, &in); err != nil {
		return nil, err
	}

	// Deduplication: spec says "if the model attempts to load a skill already
	// in context, skip the re-injection."
	if s.activated == nil {
		s.activated = make(map[string]bool)
	}
	if s.activated[in.Name] {
		return &tool.Result{
			Content: fmt.Sprintf("Skill %q is already active in this session.", in.Name),
		}, nil
	}
	s.activated[in.Name] = true

	// TODO(phase2): look up the skill via the session's Loader, call
	// Loader.Activate(in.Name), and return skill.ActivationXML().
	//
	// Placeholder — returns the structured wrapper with a stub body so the
	// XML format is testable before Loader is wired up.
	return &tool.Result{
		Content: fmt.Sprintf(
			"<skill_content name=%q>\nSkill instructions not yet loaded (Loader not wired).\n\nSkill directory: (unknown)\n</skill_content>",
			in.Name,
		),
	}, nil
}
