---
name: housekeeping
description: "Repository health check. Runs 8 read-only checks and produces a pass/fail table. Never modifies files."
---

# /housekeeping — Repo Health Check

Run a battery of health checks against the kvach repository. This skill is
read-only and never modifies any files.

## USAGE

```
/housekeeping
```

## CHECKS

### 1. Build passes

```bash
go build ./...
```

PASS if exit code 0. FAIL with error output otherwise.

### 2. Tests pass

```bash
go test ./...
```

PASS if exit code 0. FAIL with failing test names.

### 3. Vet passes

```bash
go vet ./...
```

PASS if exit code 0. FAIL with vet warnings.

### 4. Format clean

```bash
gofmt -l .
```

PASS if no output (all files formatted). FAIL with list of unformatted files.

### 5. No console debug statements

Search for `fmt.Print` in non-test Go files (excluding `cmd/`):

```bash
grep -rn 'fmt\.Print' internal/ --include='*.go' | grep -v '_test.go'
```

PASS if no matches. INFO with count if matches found. (These may be
intentional in some cases.)

### 6. No TODO without phase marker

Search for bare TODOs without `(phaseN)` or `(phase N)`:

```bash
grep -rn 'TODO[^(]' internal/ --include='*.go' | grep -v 'TODO(phase'
```

PASS if no matches. INFO with count and locations.

### 7. Doc comments on exported types

For each `*.go` file (excluding tests), check that every `type Foo struct`
and `func Foo(` has a preceding `// Foo ...` comment. Use Grep to find
exported declarations without doc comments.

PASS if all covered. FAIL with list of undocumented exports.

### 8. Stale branches

```bash
git branch --merged main | grep -v '^\*\|main'
```

PASS if 3 or fewer merged branches. INFO with branch count and names.

## OUTPUT FORMAT

```markdown
## Housekeeping Report

| # | Check              | Status | Detail           |
|---|--------------------|--------|------------------|
| 1 | Build              | PASS   |                  |
| 2 | Tests              | PASS   |                  |
| 3 | Vet                | PASS   |                  |
| 4 | Format             | FAIL   | 3 files          |
| 5 | Debug statements   | INFO   | 2 found          |
| 6 | TODO markers       | PASS   |                  |
| 7 | Doc comments       | FAIL   | 5 missing        |
| 8 | Stale branches     | PASS   |                  |

**Result: 6/8 passed, 2 issues found**
```

## RULES

1. This skill is **read-only**. Do NOT modify any files.
2. Report what you find, do not fix anything.
3. SKIP a check if it cannot be performed (e.g. no git repo). Mark as SKIP.
4. INFO status is not a failure — it's informational.
5. Exit after producing the table. Do not offer to fix issues.
