# Code Generator Agent Memory

## Workflow
- [project_pr_workflow.md](project_pr_workflow.md) — PR workflow: branch -> commit -> push -> PR -> review -> wait for Copilot -> fix comments -> merge. NEVER merge without Copilot review.

## Import rules
- [project_import_rules.md](project_import_rules.md) — tool/ packages must NOT import provider/ types. Tool results are plain strings. Interfaces live in consumer package (e.g. skill.Loader in skill/, not tool/skill/). No import cycles.

## Testing conventions
- [project_testing_patterns.md](project_testing_patterns.md) — Table-driven tests with t.Run. t.Fatal not t.Error. t.TempDir for filesystem. t.Helper in helpers. No external test libs. Provider stubs: stubProvider implementing Provider interface. Tool stubs: construct tool struct directly, call methods.

## Phase system
- [project_phase_markers.md](project_phase_markers.md) — TODO(phase1): minimal agent (15 TODOs — loop, providers, core tools, config, CLI, git). TODO(phase2): core features (22 TODOs — memory, tools, TUI). TODO(phase3): advanced (11 TODOs — snapshots, server). TODO(phase4): production quality.

## File creation patterns
- [project_new_package.md](project_new_package.md) — Package doc comment at top. Types in types.go if >3 exported types. Self-registering tools via init(). Validation methods on types (Validate() error). gofmt before commit. All 3 checks must pass: build, vet, test.

## Implemented subsystems
- [project_profile_registry.md](project_profile_registry.md) — ProfileRegistry: Register, Get, All, Names. RegisterBuiltins adds general/explore/build/review. DefaultProfileRegistry is process-wide. Profile.HasTool checks allow then deny lists (case-insensitive).
- [project_loader_mechanics.md](project_loader_mechanics.md) — splitFrontmatter: --- delimiters. splitCommaSeparated for tool lists. DiscoverProfiles scans AgentSearchPaths in order (user < project < extra). Integration test TestLoadKvachAgents verifies .kvach/agents/*.md.
- [project_field_validator.md](project_field_validator.md) — FieldRule: Field (dot path), Required, Enum, MinInt/MaxInt, MaxLen. resolveField handles one nested level. isJSONNull check. Validatable interface for post-Call output validation.
