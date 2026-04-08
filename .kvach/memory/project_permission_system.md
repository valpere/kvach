---
name: project_permission_system
description: Permission pipeline modes, rules, and pattern matching
type: project
---

# Permission System

## `internal/permission/permission.go`

### Modes
- default: ask for non-read-only
- acceptEdits: auto-approve file edits, ask for shell
- plan: read-only, deny all writes
- bypassPermissions: approve everything (sandbox only)
- dontAsk: never prompt, deny if not explicitly allowed

### Rule
Source (user/project/local/session/cli), Behavior (allow/deny/ask), Tool, Pattern.

### Pipeline (8 stages)
1. Tool.ValidateInput
2. Tool.CheckPermissions (tool-specific)
3. Deny rules
4. Allow rules
5. Ask rules
6. Mode logic
7. PreToolUse hooks
8. Interactive prompt

## `internal/permission/matcher.go`

### MatchRule pattern syntax
- Empty or `*` — match any call to the tool
- `Bash(git:*)` — command starts with "git " (or is exactly "git")
- `WebFetch(domain:docs.github.com)` — URL hostname matches (including subdomains)
- `Read(//home/val/wrk/**)` — file path starts with prefix (** = recursive)
- Bare tool name — case-insensitive match, no pattern filtering

Tool name comparison is case-insensitive. Input fields extracted: command, url,
filePath/path/pattern/directory.
