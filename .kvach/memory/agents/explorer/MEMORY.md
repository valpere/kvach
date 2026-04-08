# Explorer Agent Memory

## Package inventory
- [project_package_inventory.md](project_package_inventory.md) — 38 packages total. 8 have tests: agent (4 test files), provider (1), tool (1), tool/task (1), permission (1), prompt (1), multiagent (1). 30 packages have no tests. 50 TODOs across phases 1-3.

## Key entry points
- [project_entry_points.md](project_entry_points.md) — main: cmd/kvach/main.go -> internal/cli/root.go (Cobra). Loop: agent.Agent.Run() -> agent.loop() (TODO). Tools: tool.DefaultRegistry (process-wide). Profiles: agent.DefaultProfileRegistry (process-wide, RegisterBuiltins at init).

## Type hierarchy
- [project_type_hierarchy.md](project_type_hierarchy.md) — Provider interface: 4 methods (ID, Name, Models, Stream). Tool interface: 12 methods (Name through Prompt). Session.Store: 9 methods. Runner: 3 methods (Run, Status, Cancel). Permission.Checker: 1 method (Check). Skill.Loader: 3 methods (Discover, Activate, ParseFile).

## Package relationships
- [project_package_deps.md](project_package_deps.md) — agent imports: provider, session, tool. tool/task imports: multiagent, tool. tool/skill imports: tool (only). config imports: hook, mcp, permission. No cycles. provider sub-packages import only provider parent.

## File structure conventions
- [project_file_conventions.md](project_file_conventions.md) — Agent defs: .kvach/agents/*.md (YAML frontmatter + Markdown body). Skills: .kvach/skills/<name>/SKILL.md + optional config.yaml + lib/. Memory: .kvach/memory/MEMORY.md (index) + topic files. Agent memory: .kvach/memory/agents/<name>/MEMORY.md.
