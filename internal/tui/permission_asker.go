package tui

import (
	"context"

	"github.com/valpere/kvach/internal/permission"
)

type permissionPrompt struct {
	Request  permission.Request
	Response chan permission.Reply
}

// PermissionAsker bridges permission prompts from the agent into the TUI.
type PermissionAsker struct {
	requests chan permissionPrompt
}

// NewPermissionAsker creates a TUI-compatible permission asker.
func NewPermissionAsker() *PermissionAsker {
	return &PermissionAsker{requests: make(chan permissionPrompt, 16)}
}

func (a *PermissionAsker) Ask(ctx context.Context, req permission.Request) (permission.Reply, error) {
	resp := make(chan permission.Reply, 1)
	prompt := permissionPrompt{Request: req, Response: resp}

	select {
	case <-ctx.Done():
		return permission.Reply{}, ctx.Err()
	case a.requests <- prompt:
	}

	select {
	case <-ctx.Done():
		return permission.Reply{}, ctx.Err()
	case reply := <-resp:
		return reply, nil
	}
}

func (a *PermissionAsker) Requests() <-chan permissionPrompt {
	if a == nil {
		return nil
	}
	return a.requests
}
