---
name: project_skill_system
description: Agent Skills spec implementation and project skills
type: project
---

# Skill System

## Spec compliance (agentskills.io)

`internal/skill/skill.go` implements the Agent Skills standard.

Frontmatter fields: name (required, 1-64 chars, validated), description (required),
license, compatibility, metadata (map), allowed-tools.

Three-tier progressive disclosure:
1. Catalog: name + description + location (injected at session start)
2. Instructions: full SKILL.md body (on activation)
3. Resources: scripts/, references/, assets/ (on demand)

## Extensions beyond spec

- `Config map[string]any` — parsed from companion config.yaml/config.json
- `ConfigPath string` — absolute path to config file
- `Libraries []string` — helper scripts in lib/ subdirectory
- `ActivationXML()` includes `<skill_config>`, `<skill_libraries>`, `<skill_resources>` sections

## Tool (`internal/tool/skill/skill.go`)

- Name: `activate_skill`, alias: `Skill`
- Constrains name parameter to enum of valid skill names (prevents hallucination)
- Deduplication: already-activated skills return short acknowledgement
- Disabled when no skills in catalog
- Always auto-approved (no permission prompt)

## Project skills (`.kvach/skills/`)

5 skills: implement, fix-review (+ config.yaml), housekeeping, improve, test-coverage.
Shared helper: lib/go-check.sh (build/test/vet/fmt functions).
