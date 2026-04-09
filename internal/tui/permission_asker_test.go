package tui

import (
	"context"
	"testing"
	"time"

	"github.com/valpere/kvach/internal/permission"
)

func TestPermissionAskerRoundTrip(t *testing.T) {
	asker := NewPermissionAsker()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	resultCh := make(chan permission.Reply, 1)
	errCh := make(chan error, 1)
	go func() {
		reply, err := asker.Ask(ctx, permission.Request{ID: "perm-1", ToolName: "Bash", Description: "run command"})
		if err != nil {
			errCh <- err
			return
		}
		resultCh <- reply
	}()

	select {
	case prompt := <-asker.Requests():
		prompt.Response <- permission.Reply{Decision: "allow_once", ToolName: prompt.Request.ToolName, Pattern: "*"}
	case <-ctx.Done():
		t.Fatal("timed out waiting for permission prompt")
	}

	select {
	case err := <-errCh:
		t.Fatalf("ask returned error: %v", err)
	case reply := <-resultCh:
		if reply.Decision != "allow_once" {
			t.Fatalf("decision = %q, want allow_once", reply.Decision)
		}
	case <-ctx.Done():
		t.Fatal("timed out waiting for permission reply")
	}
}
