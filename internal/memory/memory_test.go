package memory

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteReadListTopic(t *testing.T) {
	sys := NewSystem(t.TempDir())
	ctx := context.Background()

	f := Fact{Name: "topic1", Description: "desc", Type: TypeProject, Content: "hello"}
	if err := sys.WriteTopic(ctx, "", f); err != nil {
		t.Fatalf("write topic: %v", err)
	}

	got, err := sys.ReadTopic("", "topic1")
	if err != nil {
		t.Fatalf("read topic: %v", err)
	}
	if got.Content != "hello" {
		t.Fatalf("expected hello, got %q", got.Content)
	}

	topics, err := sys.ListTopics("")
	if err != nil {
		t.Fatalf("list topics: %v", err)
	}
	if len(topics) != 1 || topics[0] != "topic1" {
		t.Fatalf("unexpected topics: %v", topics)
	}
}

func TestLoadIndexPrompt(t *testing.T) {
	sys := NewSystem(t.TempDir())
	ctx := context.Background()

	if err := sys.WriteTopic(ctx, "", Fact{Name: "one", Description: "first", Type: TypeProject, Content: "x"}); err != nil {
		t.Fatalf("write topic: %v", err)
	}

	idx, err := sys.LoadIndexPrompt("")
	if err != nil {
		t.Fatalf("load index: %v", err)
	}
	if !strings.Contains(idx, "one.md") {
		t.Fatalf("expected one.md in index, got %q", idx)
	}
}

func TestTranscriptAppendSearch(t *testing.T) {
	sys := NewSystem(t.TempDir())
	ctx := context.Background()

	line := []byte(`{"message":"hello world"}`)
	if err := sys.AppendTranscript(ctx, line); err != nil {
		t.Fatalf("append transcript: %v", err)
	}

	matches, err := sys.SearchTranscripts(ctx, "hello world")
	if err != nil {
		t.Fatalf("search transcripts: %v", err)
	}
	if len(matches) == 0 {
		t.Fatal("expected at least one transcript match")
	}
}

func TestAgentDir(t *testing.T) {
	base := t.TempDir()
	sys := NewSystem(base)
	if got := sys.AgentDir(""); got != base {
		t.Fatalf("expected base dir, got %q", got)
	}
	if got := sys.AgentDir("explore"); got != filepath.Join(base, "agents", "explore") {
		t.Fatalf("unexpected agent dir: %q", got)
	}
}
