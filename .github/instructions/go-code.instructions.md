---
applyTo: "**/*.go"
---

# Go Code Instructions

## Style

- Go 1.24+. Use stdlib when possible.
- Every exported type and function must have a doc comment starting with the symbol name.
- Package-level doc comment at the top of the primary `.go` file in each package.
- Error handling: always wrap with `fmt.Errorf("context: %w", err)`.
- Context: first parameter for blocking/IO functions.
- Use `errors.New` for sentinel errors, `fmt.Errorf` for wrapped errors.
- No naked returns. No named returns unless the function is very short.

## Testing

- Tests live next to source: `foo.go` -> `foo_test.go`, same package.
- Table-driven tests with `t.Run()` subtests.
- Use `t.Fatal`/`t.Fatalf` for unexpected errors, not `t.Error`.
- Use `t.Helper()` in test helpers.
- Use `t.TempDir()` for filesystem tests — never hardcode paths.
- No external test frameworks (no testify, no gomock). Stdlib `testing` only.
- Mock via interfaces, not code generation.

## Tool Packages

Each tool in `internal/tool/<name>/` follows this pattern:
- Unexported struct implementing `tool.Tool` (12 methods)
- `init()` that calls `tool.DefaultRegistry.Register(&myTool{})`
- `Input` struct with JSON tags matching the InputSchema
- `Call` method taking `(context.Context, json.RawMessage, *tool.Context)` returning `(*tool.Result, error)`
- Never import `internal/provider/` from tool packages

## Validation Before Commit

```bash
gofmt -w .
go build ./...
go vet ./...
go test ./...
```

All four must pass. Do not skip any.
