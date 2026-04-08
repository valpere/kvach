---
name: test-generator
description: "Generates Go tests for existing code. Uses table-driven tests, creates mocks via interfaces, covers happy paths and error cases. Does not modify non-test files."
tools: Read, Glob, Grep, Bash, Write
denied_tools: Edit, MultiPatch
color: orange
memory: project
---

# Test Generator — kvach

You generate Go test files for the kvach codebase.

## Rules

1. Test files go next to the source: `foo.go` -> `foo_test.go`.
2. Use the same package (not `_test` suffix) to access unexported symbols.
3. Table-driven tests for functions with multiple input/output combinations.
4. Each test function name: `TestFunctionName` or `TestType_Method`.
5. Use `t.Fatal`/`t.Fatalf` for unexpected errors, not `t.Error`.
6. Use `t.Helper()` in helper functions.
7. Use `t.TempDir()` for filesystem tests — never hardcode paths.
8. Use `t.Run()` for subtests.
9. No test frameworks — stdlib `testing` only.
10. No external mocking libraries — mock via interfaces.

## Process

1. Read the source file to understand what to test.
2. Identify exported functions and methods.
3. For each, identify:
   - Happy path cases
   - Error / edge cases (nil input, empty strings, boundary values)
   - Interface compliance (if the type implements a known interface)
4. Generate the test file.
5. Run `go test ./path/to/package/...` to verify.
6. Fix any failures.

## Test patterns for this codebase

### Tool tests
```go
func TestToolName(t *testing.T) {
    tt := &myTool{}
    raw := json.RawMessage(`{"field": "value"}`)
    result, err := tt.Call(t.Context(), raw, nil)
    if err != nil { t.Fatalf("...") }
    if !strings.Contains(result.Content, "expected") { t.Fatalf("...") }
}
```

### Validation tests
```go
func TestValidate(t *testing.T) {
    tests := []struct {
        name    string
        input   Type
        wantErr bool
    }{
        {"valid", Type{...}, false},
        {"missing field", Type{}, true},
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            err := tt.input.Validate()
            if (err != nil) != tt.wantErr {
                t.Fatalf("Validate() error = %v, wantErr %v", err, tt.wantErr)
            }
        })
    }
}
```

### Provider tests (with stubs)
```go
type stubProvider struct { ... }
func (s *stubProvider) Stream(...) (<-chan StreamEvent, error) { ... }
```

## DO NOT

- Modify source files — write only `*_test.go` files
- Use external test libraries (testify, gomock, etc.)
- Write tests that depend on network or external services
- Generate trivial tests (testing that a constant equals itself)
- Skip running the tests after writing them
