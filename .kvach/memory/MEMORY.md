# kvach Project Memory

## Architecture
- [project_package_layout.md](project_package_layout.md) — 38 packages: cmd/kvach (CLI), internal/ (all logic). No public API. 48 source files, 10 test files, 50 TODOs across 4 phases.
- [project_tool_interface.md](project_tool_interface.md) — tool.Tool has 12 methods: Name, Aliases, InputSchema, Call, ValidateInput, CheckPermissions, IsEnabled, IsConcurrencySafe, IsReadOnly, IsDestructive, Prompt. Self-registration via init() + DefaultRegistry.
- [project_provider_interface.md](project_provider_interface.md) — provider.Provider: ID, Name, Models, Stream. Channel-based streaming with 10 StreamEvent types. FailoverProvider wraps primary+failover with configurable failback cooldown.
- [project_session_model.md](project_session_model.md) — Session -> Messages -> Parts. 6 part types: text, reasoning, tool_use, tool_result, file, compaction. Store interface with CRUD. Will be SQLite.

## Agent system
- [project_agent_profiles.md](project_agent_profiles.md) — Profile: name, description, model, tools (allow+deny), system prompt, max_turns, memory scope, color. Built-in: general, explore, build, review. User-defined via .kvach/agents/*.md with YAML frontmatter.
- [project_agent_loader.md](project_agent_loader.md) — ParseProfileFile splits frontmatter (---) from body. Tools are comma-separated in YAML. Discovery: user (~/.kvach/agents/, ~/.agents/agents/) then project (.kvach/agents/, .agents/agents/). Project overrides user.
- [project_agent_definitions.md](project_agent_definitions.md) — 6 agents: code-generator (yellow, full tools + Task/Skill), explorer (cyan, read-only), reviewer (blue, read + Write), test-generator (orange, read + Write), docs-maintainer (gray, read + Write/Edit), refactorer (purple, read + Edit).

## Subagent system
- [project_multiagent_contracts.md](project_multiagent_contracts.md) — Options: TaskID (resume), Type (in_process/teammate/worktree), Profile, AllowedTools, DeniedTools, MaxDuration. Result: TaskState (6 states), OutputContract (summary/findings/changed_files/next_actions), Usage. Runner: Run, Status, Cancel.
- [project_task_tool.md](project_task_tool.md) — Task tool: description + prompt + subagent_type + task_id (resume) + command (audit). Validates description <= 140 chars. Profile resolution: subagent_type > subagent (legacy) > "general" default. Not concurrency-safe.

## Skills
- [project_skill_system.md](project_skill_system.md) — Agent Skills spec (agentskills.io). Frontmatter: name (required), description (required), license, compatibility, metadata, allowed-tools. Three tiers: catalog (name+desc), activation (full body), resources (on-demand). Config map + Libraries list + ActivationXML with config/libs/resources.
- [project_skill_definitions.md](project_skill_definitions.md) — 5 skills: /implement (full pipeline orchestrator), /fix-review (severity-based fix with config.yaml), /housekeeping (8 read-only checks), /improve (6-dimension architecture critique), /test-coverage (measure + fill gaps).

## Infrastructure
- [project_permission_system.md](project_permission_system.md) — 5 modes: default, acceptEdits, plan, bypassPermissions, dontAsk. Rules: tool + pattern. MatchRule supports Bash(git:*), WebFetch(domain:host), Read(//path/**), bare tool, wildcard.
- [project_memory_system.md](project_memory_system.md) — Three layers: index (MEMORY.md, 200 lines, 25KB), topics (*.md with frontmatter), transcripts (JSONL logs). Per-agent scoping via AgentDir(name). 4 types: user, feedback, project, reference.
- [project_prompt_engine.md](project_prompt_engine.md) — {{key}} interpolation. Engine: Register, RegisterDefault, Render, LoadDir (*.md, *.txt). Interpolate exported for one-off use. Unknown keys replaced with empty string.
- [project_validation.md](project_validation.md) — FieldValidator with FieldRule: Required, Enum, MinInt/MaxInt, MaxLen. Supports one level of nested fields (dot notation). Catches "LLM Theater" before tool execution.
- [project_pipeline.md](project_pipeline.md) — Generic Pipeline[I,O] with Handler[I,O] func type. First handler returning (result, true) wins. Prepend/Append for priority control. Used for input preprocessing, output filtering, event routing.

## Workflow
- [project_pr_workflow.md](project_pr_workflow.md) — PR workflow: 1) create branch, 2) stage+commit, 3) push, 4) create PR, 5) review, 6) wait for Copilot review, 7) fix Copilot comments if any, 8) merge. Never merge without waiting for Copilot.
- [project_copilot_instructions.md](project_copilot_instructions.md) — GitHub Copilot instructions in .github/: copilot-instructions.md (repo-wide) + 5 path-specific instruction files in .github/instructions/ (go-code, agent-definitions, skill-definitions, docs, memory).

## Dependencies
- [project_dependencies.md](project_dependencies.md) — gopkg.in/yaml.v3 (YAML parsing), github.com/spf13/cobra (CLI). Stdlib for everything else. No test frameworks — stdlib testing only.
