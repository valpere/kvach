package agent

import (
	"context"
	"testing"

	"github.com/valpere/kvach/internal/multiagent"
	"github.com/valpere/kvach/internal/provider"
	"github.com/valpere/kvach/internal/tool"
)

func TestSubagentRunnerRunAndResume(t *testing.T) {
	a := New(fakeSubagentProvider{}, tool.DefaultRegistry, nil, Config{
		WorkDir:      t.TempDir(),
		SystemPrompt: "base prompt",
		MaxTurns:     4,
	})

	runner, ok := a.tasks.(*subagentRunner)
	if !ok {
		t.Fatal("expected subagentRunner implementation")
	}

	res, err := runner.Run(context.Background(), multiagent.Options{
		Description: "inspect repository",
		Prompt:      "summarize current state",
		Profile:     "explore",
	})
	if err != nil {
		t.Fatalf("run delegated task: %v", err)
	}
	if res.State != multiagent.TaskCompleted {
		t.Fatalf("state = %q, want %q", res.State, multiagent.TaskCompleted)
	}
	if res.Output != "delegated output" {
		t.Fatalf("output = %q, want %q", res.Output, "delegated output")
	}
	if res.Contract.Summary != "inspect repository" {
		t.Fatalf("summary = %q, want %q", res.Contract.Summary, "inspect repository")
	}

	resumed, err := runner.Run(context.Background(), multiagent.Options{
		TaskID:      res.TaskID,
		Description: "resume",
		Prompt:      "resume",
	})
	if err != nil {
		t.Fatalf("resume delegated task: %v", err)
	}
	if resumed.TaskID != res.TaskID {
		t.Fatalf("resumed task id = %q, want %q", resumed.TaskID, res.TaskID)
	}
	if resumed.Output != res.Output {
		t.Fatalf("resumed output = %q, want %q", resumed.Output, res.Output)
	}
}

func TestSubagentRunnerResumeMissingTask(t *testing.T) {
	a := New(fakeSubagentProvider{}, tool.DefaultRegistry, nil, Config{WorkDir: t.TempDir()})
	runner, ok := a.tasks.(*subagentRunner)
	if !ok {
		t.Fatal("expected subagentRunner implementation")
	}

	_, err := runner.Run(context.Background(), multiagent.Options{
		TaskID:      "task-missing",
		Description: "resume",
		Prompt:      "resume",
	})
	if err == nil {
		t.Fatal("expected missing task error")
	}
}

type fakeSubagentProvider struct{}

func (fakeSubagentProvider) ID() string   { return "fake" }
func (fakeSubagentProvider) Name() string { return "Fake" }
func (fakeSubagentProvider) Models(context.Context) ([]provider.Model, error) {
	return nil, nil
}

func (fakeSubagentProvider) Stream(_ context.Context, _ *provider.StreamRequest) (<-chan provider.StreamEvent, error) {
	ch := make(chan provider.StreamEvent, 2)
	ch <- provider.StreamEvent{Type: provider.StreamEventTextDelta, Text: "delegated output"}
	ch <- provider.StreamEvent{Type: provider.StreamEventMessageEnd, FinishReason: "stop", Usage: &provider.UsageStats{InputTokens: 10, OutputTokens: 5}}
	close(ch)
	return ch, nil
}
