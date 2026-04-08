---
applyTo: "**/.kvach/memory/**/*.md,.kvach/memory/**/*.md"
---

# Memory File Instructions

## MEMORY.md (index files)

Index files list links to topic files with one-line descriptions, organized by category:

```markdown
# Agent Memory Title

## Category
- [topic_name.md](topic_name.md) — One-line description of what this topic covers
```

- Keep entries concise — the index is loaded into context every session
- Organize semantically by topic, not chronologically
- Link text is the filename, description after the `—` dash

## Topic files

Topic files use YAML frontmatter:

```yaml
---
name: topic_name
description: One-line summary
type: project    # user, feedback, project, or reference
---

Content...
```

- `name` must match the filename (without `.md`)
- Content should be factual and specific — exact file paths, type names, method signatures
- Do not store opinions or speculation — only confirmed facts
- Update topics when the underlying code changes — stale memory is worse than no memory
