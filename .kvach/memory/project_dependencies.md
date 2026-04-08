---
name: project_dependencies
description: External dependencies and justification
type: project
---

# External Dependencies

## go.mod

- `gopkg.in/yaml.v3 v3.0.1` — YAML parsing for agent profile frontmatter and
  skill frontmatter. Stdlib has no YAML parser. Well-maintained, zero transitive deps.
- `github.com/spf13/cobra v1.9.1` — CLI command tree. Standard Go CLI library.
  Transitively pulls pflag only.

## Node (tooling only, not runtime)

- `@commitlint/cli`, `@commitlint/config-conventional` — commit message linting
- `husky` — git hook management for commitlint

## Policy

No new external deps without explicit justification for why stdlib is insufficient.
No test frameworks — stdlib `testing` only. No mocking libraries — mock via interfaces.
