---
name: implement
description: "Full implementation pipeline. Takes a feature description, designs the interface, implements it, generates tests, runs review, and updates docs. Orchestrates code-generator, test-generator, reviewer, and docs-maintainer agents."
---

# /implement — Full Feature Pipeline

Orchestrate the complete implementation of a feature or change.

## USAGE

```
/implement <description of what to build>
```

## PIPELINE

```
STEP 1: Understand + Design
STEP 2: Implement (code-generator agent)
STEP 3: Parallel review (test-generator + reviewer agents)
STEP 4: Fix review findings
STEP 5: Update docs (docs-maintainer agent)
STEP 6: Final verification
```

## STEP 1: Understand + Design

1. Parse the feature description.
2. Identify affected packages by searching the codebase.
3. Design the interface(s) — write them out as Go code in the response.
4. Ask the user to confirm the design if it touches more than 2 packages.

If the user says "just do it" or passes `--yes`, skip confirmation.

## STEP 2: Implement

Launch the `code-generator` agent with the Task tool:

```
Task(
  description: "Implement <feature>",
  prompt: "<full design + instructions>",
  subagent_type: "code-generator"
)
```

Wait for completion. Extract the list of modified files from the result.

## STEP 3: Parallel Review

Launch THREE agents in parallel as a single batch of Task calls:

```
Task(description: "Generate tests for <feature>", subagent_type: "test-generator", ...)
Task(description: "Review <feature> implementation", subagent_type: "reviewer", ...)
```

Also run static analysis directly:

```bash
go vet ./...
```

Do NOT run these sequentially — all three must be parallel.

## STEP 4: Fix Review Findings

Read the reviewer's report. For each CRITICAL or HIGH finding:

1. Read the relevant file.
2. Apply the fix via Edit.
3. Run `go test ./affected/package/...` to verify.

For MEDIUM findings: fix if the fix is less than 10 lines. Otherwise note
them for later.

For LOW/INFO findings: skip.

## STEP 5: Update Docs

Launch the `docs-maintainer` agent:

```
Task(
  description: "Update docs for <feature>",
  prompt: "The following files were modified: <list>. Check if docs need updating.",
  subagent_type: "docs-maintainer"
)
```

## STEP 6: Final Verification

Run the full check suite:

```bash
gofmt -w .
go build ./...
go vet ./...
go test ./...
```

All four must pass. If any fail, fix and re-run.

## OUTPUT

Produce a summary:

```markdown
## Implementation Complete: <feature>

### Files modified
- `path/to/file.go` — <what changed>

### Tests added
- `path/to/file_test.go` — <what's tested>

### Review findings addressed
- [HIGH] <finding> — fixed
- [MEDIUM] <finding> — deferred

### Verification
- [x] gofmt
- [x] go build
- [x] go vet
- [x] go test
```

## RULES

1. Never skip the test-generator step.
2. Never skip the reviewer step.
3. Always run final verification.
4. If verification fails after 3 fix attempts, stop and report the failure.
5. Every new exported type must have a doc comment.
6. Every new package must have a package-level doc comment.
