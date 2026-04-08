package question

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/valpere/kvach/internal/tool"
)

type mockAsker struct {
	answer string
	err    error
}

func (m mockAsker) AskQuestion(_ context.Context, _ string, _ []string) (string, error) {
	return m.answer, m.err
}

func TestQuestionCallUsesAsker(t *testing.T) {
	qt := &questionTool{}
	raw, _ := json.Marshal(map[string]any{
		"question": "Choose",
		"options":  []string{"a", "b"},
	})

	res, err := qt.Call(t.Context(), raw, &tool.Context{Asker: mockAsker{answer: "b"}})
	if err != nil {
		t.Fatalf("call: %v", err)
	}
	if res.Content != "b" {
		t.Fatalf("expected answer b, got %q", res.Content)
	}
}

func TestQuestionValidateInput(t *testing.T) {
	qt := &questionTool{}
	if err := qt.ValidateInput(json.RawMessage(`{"question":"hi"}`)); err != nil {
		t.Fatalf("expected valid input, got %v", err)
	}
	if err := qt.ValidateInput(json.RawMessage(`{"question":""}`)); err == nil {
		t.Fatal("expected validation error for empty question")
	}
}
