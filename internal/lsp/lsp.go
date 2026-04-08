// Package lsp provides a minimal LSP (Language Server Protocol) client.
//
// The LSP client lets tools request code intelligence (diagnostics, hover
// information, symbol lists) from language servers running in the project.
// This enriches tool results — for example, the Edit tool can report
// compilation errors after a write.
//
// This package is intentionally thin in Phase 1; it is a placeholder for
// Phase 3 implementation.
package lsp

import "context"

// DiagnosticSeverity mirrors the LSP DiagnosticSeverity enum.
type DiagnosticSeverity int

const (
	SeverityError       DiagnosticSeverity = 1
	SeverityWarning     DiagnosticSeverity = 2
	SeverityInformation DiagnosticSeverity = 3
	SeverityHint        DiagnosticSeverity = 4
)

// Diagnostic is a single LSP diagnostic message.
type Diagnostic struct {
	File     string
	Line     int
	Col      int
	Severity DiagnosticSeverity
	Message  string
	Source   string
}

// Client communicates with a language server.
type Client interface {
	// Diagnostics returns the current diagnostics for file.
	Diagnostics(ctx context.Context, file string) ([]Diagnostic, error)

	// Shutdown stops the language server process.
	Shutdown(ctx context.Context) error
}
