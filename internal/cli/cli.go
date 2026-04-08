// Package cli assembles the cobra command tree for the kvach binary.
//
// The root command launches the interactive TUI when no sub-command is given.
// Sub-commands cover non-interactive operation, server mode, session
// management, and provider/model inspection.
//
// Commands are defined in separate files within this package:
//
//	root.go    — root command, global flags, bootstrap
//	run.go     — `kvach run "prompt"` (non-interactive single prompt)
//	serve.go   — `kvach serve` (start the HTTP API server)
//	session.go — `kvach session list|resume|show`
//	config.go  — `kvach config`
//	models.go  — `kvach models`
//	mcp.go     — `kvach mcp list|connect`
package cli
