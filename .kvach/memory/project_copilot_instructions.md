---
name: project_copilot_instructions
description: GitHub Copilot custom instructions file structure
type: project
---

# Copilot Instructions

Located in `.github/` per GitHub docs convention.

## Repository-wide

`.github/copilot-instructions.md` — loaded for every Copilot request in the repo.
Contains: project description, build/test/vet commands, package layout, coding
conventions, naming, commit format, key interfaces, "what NOT to do".

## Path-specific (`.github/instructions/`)

| File | applyTo | Content |
|------|---------|---------|
| `go-code.instructions.md` | `**/*.go` | Go style, testing patterns, tool package structure |
| `agent-definitions.instructions.md` | `.kvach/agents/*.md` | YAML frontmatter format, required fields, tool names |
| `skill-definitions.instructions.md` | `.kvach/skills/**/SKILL.md` | Agent Skills spec, STEP/RULES/OUTPUT sections |
| `docs.instructions.md` | `docs/**/*.md`, `CLAUDE.md`, `AGENTS.md` | Line limits, accuracy, no-duplication |
| `memory.instructions.md` | `.kvach/memory/**/*.md` | Index format, topic frontmatter, staleness policy |

## Format

Path-specific files use YAML frontmatter with `applyTo` glob:

```yaml
---
applyTo: "**/*.go"
---
```

Optional `excludeAgent: "code-review"` or `"cloud-agent"` to restrict scope.
