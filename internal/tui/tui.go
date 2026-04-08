// Package tui implements the interactive terminal user interface.
//
// The TUI is built with Bubble Tea (github.com/charmbracelet/bubbletea) and
// follows its Model/Update/View architecture. The root model delegates to
// focused child models:
//
//   - [ConversationModel]  — scrollable message history with tool visualisation
//   - [InputModel]         — multi-line text input with history
//   - [StatusBarModel]     — current model, token usage, active tool indicator
//   - [PermissionModel]    — permission prompt overlay (blocks until resolved)
//
// The TUI connects to the agent via the event channel returned by
// [agent.Agent.Run]; each event type drives a specific UI update.
package tui

// Model is the root Bubble Tea model for the kvach TUI.
// It is intentionally left as a placeholder until Phase 2 when bubbletea
// is added as a dependency.
type Model struct {
	// TODO(phase2): add bubbletea model fields.
}
