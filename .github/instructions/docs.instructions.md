---
applyTo: "docs/**/*.md,CLAUDE.md,AGENTS.md"
---

# Documentation Instructions

## CLAUDE.md

- Keep under 150 lines. Loaded into every agent session — brevity matters.
- Must reflect actual code state, not aspirational features.
- Include: commands, package layout, key interfaces, implementation status, rules.

## AGENTS.md

- Keep under 100 lines. Repository guidelines for all AI agents.
- Include: project structure, build/test/vet commands, coding conventions, naming, commit format, architecture constraints.

## docs/*.md

- Use concrete file paths and line numbers, not vague references.
- Do not duplicate information between docs — cross-reference instead.
- Do not document unimplemented features as if they exist.
- Mark implementation status clearly (DONE, TODO, stub).
- Code examples must be valid Go that compiles.
