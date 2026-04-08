---
name: project_agent_profiles
description: Agent profile system — model, tools, prompts, discovery
type: project
---

# Agent Profiles

## Model (`internal/agent/profile.go`)

Profile struct fields:
- Name (lowercase, alphanumeric + hyphens, validated)
- Description (shown in Task tool documentation)
- Model (override, empty = inherit session default)
- Tools (allowlist, empty = all), DeniedTools (applied after allowlist)
- SystemPrompt (Markdown body from file)
- MaxTurns (zero = inherit)
- MemoryScope ("project" default, or "agent" for isolated)
- Color (UI hint), Source (builtin/file/config), Disabled

## Registry

- `ProfileRegistry`: Register, Get, All, Names (thread-safe, registration order)
- `DefaultProfileRegistry`: process-wide, populated at init with RegisterBuiltins
- Built-in: general (all tools), explore (read-only), build (all tools, yellow), review (read + Write, blue)

## Loader (`internal/agent/loader.go`)

- `ParseProfileFile`: split YAML frontmatter (---) from Markdown body
- Tools parsed from comma-separated string in YAML
- `LoadProfilesFromDir`: scan *.md files, skip non-parseable with stderr warning
- `DiscoverProfiles`: scan AgentSearchPaths in precedence order (user < project < extra)
- Discovery paths: ~/.kvach/agents/, ~/.agents/agents/, .kvach/agents/, .agents/agents/

## Current project agents (`.kvach/agents/`)

6 agents: code-generator, explorer, reviewer, test-generator, docs-maintainer, refactorer.
Verified by TestLoadKvachAgents integration test.
