---
name: test-coverage
description: "Analyze test coverage, identify gaps, and generate tests for uncovered code. Runs go test -cover, parses results, and delegates test generation to the test-generator agent."
---

# /test-coverage — Coverage Analysis + Gap Filling

Analyze test coverage across the codebase and generate tests for uncovered code.

## USAGE

```
/test-coverage                  # analyze entire codebase
/test-coverage <package>        # analyze specific package
```

## STEP 1: Measure Current Coverage

Run coverage analysis:

```bash
go test -coverprofile=coverage.out ./...
go tool cover -func=coverage.out
```

Parse the output. For each package, record:
- Package path
- Coverage percentage
- Uncovered functions (0% coverage)
- Partially covered functions (<80%)

## STEP 2: Prioritize Gaps

Sort packages by importance:

1. `internal/agent/` — core loop, highest priority
2. `internal/tool/` — tool implementations
3. `internal/provider/` — LLM abstraction
4. `internal/permission/` — security boundary
5. `internal/multiagent/` — subagent contracts
6. Everything else

Within a package, prioritize:
1. Exported functions with 0% coverage
2. Exported functions below 80% coverage
3. Unexported functions with 0% coverage

## STEP 3: Generate Tests

For each high-priority uncovered function, launch the test-generator agent:

```
Task(
  description: "Generate tests for <package>/<function>",
  prompt: "Generate tests for function <function> in <file>. Cover: <specific uncovered paths>",
  subagent_type: "test-generator"
)
```

Launch up to 3 test-generator tasks in parallel for different packages.

## STEP 4: Verify Improvement

After tests are generated, re-run coverage:

```bash
go test -coverprofile=coverage.out ./...
go tool cover -func=coverage.out
```

Compare before/after.

## STEP 5: Report

```markdown
## Coverage Report

### Before
| Package | Coverage |
|---------|----------|
| internal/agent | 45% |
| internal/tool | 62% |
| ... | ... |
| **Total** | **52%** |

### After
| Package | Coverage | Delta |
|---------|----------|-------|
| internal/agent | 72% | +27% |
| internal/tool | 78% | +16% |
| ... | ... | ... |
| **Total** | **71%** | **+19%** |

### Tests added
- `internal/agent/profile_test.go` — 3 new test functions
- `internal/tool/validate_test.go` — 5 new test functions

### Still uncovered (deferred)
- `internal/tui/` — requires terminal mock (no stdlib solution)
- `internal/server/` — requires HTTP test server (future phase)
```

## STEP 6: Cleanup

```bash
rm -f coverage.out
```

## RULES

1. Do not generate tests for stub/TODO functions (they have no behavior to test).
2. Do not count `[no test files]` packages as 0% — they're untested, not failed.
3. Generated tests must pass. If a test fails, fix or remove it.
4. Focus on behavioral coverage, not line coverage.
5. Maximum 3 parallel test-generator tasks at once.
