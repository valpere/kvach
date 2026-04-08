package multiagent

import (
	"testing"
	"time"
)

func TestOptionsNormalize(t *testing.T) {
	var opts Options
	opts.Normalize()

	if opts.Type != SubagentInProcess {
		t.Fatalf("expected default type %q, got %q", SubagentInProcess, opts.Type)
	}
	if opts.Profile != DefaultProfileGeneral {
		t.Fatalf("expected default profile %q, got %q", DefaultProfileGeneral, opts.Profile)
	}
}

func TestOptionsValidate(t *testing.T) {
	tests := []struct {
		name    string
		opts    Options
		wantErr bool
	}{
		{
			name: "valid",
			opts: Options{
				Type:        SubagentInProcess,
				Profile:     "explore",
				Description: "scan repository",
				Prompt:      "find API handlers",
			},
		},
		{
			name: "missing description",
			opts: Options{
				Type:   SubagentInProcess,
				Prompt: "do work",
			},
			wantErr: true,
		},
		{
			name: "missing prompt",
			opts: Options{
				Type:        SubagentInProcess,
				Description: "do work",
			},
			wantErr: true,
		},
		{
			name: "invalid type",
			opts: Options{
				Type:        SubagentType("unknown"),
				Description: "do work",
				Prompt:      "do it",
			},
			wantErr: true,
		},
		{
			name: "negative max turns",
			opts: Options{
				Type:        SubagentInProcess,
				Description: "do work",
				Prompt:      "do it",
				MaxTurns:    -1,
			},
			wantErr: true,
		},
		{
			name: "negative max duration",
			opts: Options{
				Type:        SubagentInProcess,
				Description: "do work",
				Prompt:      "do it",
				MaxDuration: -1 * time.Second,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.opts.Validate()
			if tt.wantErr && err == nil {
				t.Fatal("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("expected nil error, got %v", err)
			}
		})
	}
}
