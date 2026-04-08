---
name: project_testing_patterns
description: Go testing conventions used in this codebase
type: project
---

# Testing Patterns

## Conventions

- Same package (not `_test` suffix) to access unexported symbols
- Table-driven tests with `t.Run()` for subtests
- `t.Fatal`/`t.Fatalf` for unexpected errors, not `t.Error`
- `t.Helper()` in helper functions
- `t.TempDir()` for filesystem tests — never hardcode paths
- No external test frameworks (no testify, no gomock)
- No test fixtures directory — inline data or t.TempDir

## Provider testing (`internal/provider/failover_test.go`)

```go
type stubProvider struct {
    id, name string
    streamFn func(ctx, req) (<-chan StreamEvent, error)
}
func okStream() (<-chan StreamEvent, error) { /* closed chan with stop event */ }
func failStream() (<-chan StreamEvent, error) { return nil, errors.New(...) }
```

## Tool testing (`internal/tool/task/task_test.go`)

Construct tool struct directly, call methods:
```go
tt := &taskTool{}
raw := json.RawMessage(`{"description": "x", "prompt": "y"}`)
result, err := tt.Call(t.Context(), raw, nil)
```

## Validation testing (`internal/tool/validate_test.go`)

Create FieldValidator with rules, pass json.RawMessage literals:
```go
v := &FieldValidator{Rules: []FieldRule{{Field: "name", Required: true}}}
err := v.Validate(json.RawMessage(`{"name": "test"}`))
```

## Integration tests (`internal/agent/loader_integration_test.go`)

Walk up directories to find repo root, then verify real config files parse.
`t.Skip` if files not found (CI-safe).
