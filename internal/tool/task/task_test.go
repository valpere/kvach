package task

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestInputProfile(t *testing.T) {
	tests := []struct {
		name string
		in   Input
		want string
	}{
		{
			name: "default general",
			in:   Input{},
			want: "general",
		},
		{
			name: "uses legacy alias",
			in: Input{
				Subagent: "explore",
			},
			want: "explore",
		},
		{
			name: "subagent_type wins",
			in: Input{
				SubagentType: "general",
				Subagent:     "explore",
			},
			want: "general",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.in.profile(); got != tt.want {
				t.Fatalf("expected %q, got %q", tt.want, got)
			}
		})
	}
}

func TestValidateInput(t *testing.T) {
	tt := &taskTool{}

	valid := map[string]any{
		"description": "inspect api",
		"prompt":      "find all api handlers",
	}
	raw, err := json.Marshal(valid)
	if err != nil {
		t.Fatalf("marshal valid input: %v", err)
	}
	if err := tt.ValidateInput(raw); err != nil {
		t.Fatalf("expected valid input, got error: %v", err)
	}

	invalid := map[string]any{
		"description": "",
		"prompt":      "find all api handlers",
	}
	raw, err = json.Marshal(invalid)
	if err != nil {
		t.Fatalf("marshal invalid input: %v", err)
	}
	if err := tt.ValidateInput(raw); err == nil {
		t.Fatal("expected validation error for empty description")
	}
}

func TestCallIncludesModeAndProfile(t *testing.T) {
	tt := &taskTool{}
	input := map[string]any{
		"description":   "search routes",
		"prompt":        "find all HTTP routes",
		"subagent_type": "explore",
		"task_id":       "task-123",
		"command":       "scan routes",
	}

	raw, err := json.Marshal(input)
	if err != nil {
		t.Fatalf("marshal input: %v", err)
	}

	res, err := tt.Call(t.Context(), raw, nil)
	if err != nil {
		t.Fatalf("call returned error: %v", err)
	}
	if res == nil {
		t.Fatal("expected non-nil result")
	}

	if !strings.Contains(res.Content, "mode=resume") {
		t.Fatalf("expected resume mode in output, got %q", res.Content)
	}
	if !strings.Contains(res.Content, "profile=\"explore\"") {
		t.Fatalf("expected profile in output, got %q", res.Content)
	}
}
