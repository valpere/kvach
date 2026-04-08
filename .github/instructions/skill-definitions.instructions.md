---
applyTo: "**/.kvach/skills/**/SKILL.md,.kvach/skills/**/SKILL.md"
---

# Skill Definition Instructions

Skills follow the Agent Skills spec (https://agentskills.io).

## SKILL.md format

```yaml
---
name: lowercase-hyphenated
description: "When to use this skill and what it does"
---

# /skill-name — Title

Step-by-step instructions...
```

## Conventions

- Name must match the parent directory name
- Description is the only field shown in the catalog — make it self-sufficient
- Body should use numbered STEP sections for multi-step procedures
- Include a RULES section at the end with hard constraints
- Include an OUTPUT FORMAT section showing expected output structure
- Reference agents by profile name when delegating via Task tool
- Include `## USAGE` section showing invocation syntax

## Optional companion files

- `config.yaml` — user-configurable settings, parsed into Skill.Config map
- `lib/*.sh` — helper scripts, listed in activation response
- `scripts/`, `references/`, `assets/` — on-demand resources (Tier 3)
