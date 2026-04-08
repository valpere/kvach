---
applyTo: "**/.kvach/agents/*.md,.kvach/agents/*.md"
---

# Agent Definition Instructions

Agent definitions use YAML frontmatter + Markdown body:

```yaml
---
name: lowercase-hyphenated
description: "When to invoke this agent — include example triggers"
tools: Read, Glob, Grep, Bash    # comma-separated tool names
model: anthropic/claude-sonnet-4-5  # optional, empty = inherit
color: blue                       # UI hint
memory: project                   # "project" or "agent"
---

# System Prompt Body

Markdown instructions for the agent...
```

## Required frontmatter fields

- `name` — lowercase, alphanumeric + hyphens only, 1-64 chars
- `tools` — comma-separated list of tool names from the registry

## Conventions

- Tool names must match registered tool names: Read, Write, Edit, Bash, Glob, Grep, LS, Task, Skill, Question, TodoWrite, WebFetch, WebSearch, MultiEdit, ApplyPatch
- Use `denied_tools` to remove specific tools from an otherwise-unrestricted profile
- System prompt body should include: role description, allowed/forbidden actions, verification checklist
- End with a Persistent Agent Memory section if `memory: agent` is set
