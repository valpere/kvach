package cli

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/valpere/kvach/internal/permission"
)

func TestStdioPermissionAskerAllowOnce(t *testing.T) {
	in := strings.NewReader("y\n")
	var out bytes.Buffer

	asker := newStdioPermissionAsker(in, &out)
	reply, err := asker.Ask(context.Background(), permission.Request{
		ToolName:    "Bash",
		Description: "run command",
		Risk:        "high",
	})
	if err != nil {
		t.Fatalf("ask: %v", err)
	}
	if reply.Decision != "allow_once" {
		t.Fatalf("decision = %q, want allow_once", reply.Decision)
	}
}

func TestStdioPermissionAskerAllowAlways(t *testing.T) {
	in := strings.NewReader("a\n")
	var out bytes.Buffer

	asker := newStdioPermissionAsker(in, &out)
	reply, err := asker.Ask(context.Background(), permission.Request{
		ToolName:    "Write",
		Description: "write file",
		Risk:        "medium",
	})
	if err != nil {
		t.Fatalf("ask: %v", err)
	}
	if reply.Decision != "allow_always" {
		t.Fatalf("decision = %q, want allow_always", reply.Decision)
	}
}

func TestStdioPermissionAskerDenyOnEOF(t *testing.T) {
	in := strings.NewReader("")
	var out bytes.Buffer

	asker := newStdioPermissionAsker(in, &out)
	reply, err := asker.Ask(context.Background(), permission.Request{
		ToolName:    "Edit",
		Description: "edit file",
		Risk:        "medium",
	})
	if err != nil {
		t.Fatalf("ask: %v", err)
	}
	if reply.Decision != "deny" {
		t.Fatalf("decision = %q, want deny", reply.Decision)
	}
}
