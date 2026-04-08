---
name: fix-review
description: "Fix all review comments on a branch. Reads review findings, groups by file, applies fixes in priority order, runs tests after each batch, and produces a summary."
---

# /fix-review — Fix Review Findings

Systematically fix review comments from a code review.

## USAGE

```
/fix-review                    # fix findings in current branch
/fix-review <review-file>      # fix findings from a specific review report
```

## STEP 0: Collect Findings

If a review file is provided, read it. Otherwise look for the most recent
reviewer output.

Parse findings into a list:
- severity (CRITICAL, HIGH, MEDIUM, LOW, INFO)
- file path and line number
- description of the issue
- suggested fix (if provided)

## STEP 1: Prioritize

Sort findings:
1. CRITICAL first
2. HIGH second
3. MEDIUM third
4. LOW / INFO — skip unless trivial (1-2 line fix)

Group by file to minimize context switches.

## STEP 2: Fix Each Group

For each file group:

1. Read the file.
2. Apply fixes for all findings in that file.
3. Run `go build ./...` — revert if it breaks.
4. Run `go test ./affected/package/...` — revert if it breaks.

If a fix causes a test failure:
- Read the failing test.
- Either fix the test (if the review finding is correct and the test is wrong)
  or revert the fix (if the test is correct and the review finding is wrong).

## STEP 3: Run Full Suite

```bash
gofmt -w .
go build ./...
go vet ./...
go test ./...
```

## STEP 4: Summary

```markdown
## Fix-Review Summary

### Fixed
- [HIGH] <description> in `file.go:line`
- [MEDIUM] <description> in `file.go:line`

### Skipped
- [LOW] <description> — trivial, deferred
- [INFO] <description> — informational only

### Reverted
- [HIGH] <description> — fix caused test failure in `test_file.go`

### Verification
- [x] go build
- [x] go vet
- [x] go test
```

## RULES

1. Never apply a fix that breaks an existing test without understanding why.
2. If a CRITICAL finding cannot be fixed, stop and escalate to the user.
3. Do not refactor code beyond what the review finding requires.
4. Keep fixes minimal — each fix should address exactly one finding.
5. Run tests after each file group, not only at the end.
