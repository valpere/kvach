// Package multiagent implements the contracts for supervisor/subagent
// orchestration.
//
// The primary agent (supervisor) delegates focused work to short-lived
// subagents. Each subagent runs an independent loop with isolated context and a
// constrained tool set, then returns a structured result envelope.
//
// This package defines contracts only; concrete scheduling and execution live
// in a later phase.
package multiagent

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

// SubagentType identifies how a subagent is created.
type SubagentType string

const (
	// SubagentInProcess spawns the subagent as a goroutine in the same process.
	SubagentInProcess SubagentType = "in_process"
	// SubagentTeammate spawns the subagent as a separate goroutine with its
	// own message history.
	SubagentTeammate SubagentType = "teammate"
	// SubagentWorktree spawns the subagent in an isolated git worktree.
	SubagentWorktree SubagentType = "worktree"
)

const (
	// DefaultProfileGeneral is the catch-all profile used when no specialist is
	// requested.
	DefaultProfileGeneral = "general"
)

// TaskState describes the lifecycle state of a delegated task.
type TaskState string

const (
	TaskQueued    TaskState = "queued"
	TaskRunning   TaskState = "running"
	TaskCompleted TaskState = "completed"
	TaskFailed    TaskState = "failed"
	TaskCancelled TaskState = "cancelled"
	TaskTimeout   TaskState = "timeout"
)

// Options parameterises a subagent run.
type Options struct {
	// TaskID resumes an existing task when non-empty.
	TaskID string

	// Type selects the subagent spawn strategy.
	Type SubagentType

	// Profile selects the named specialist profile (for example: "general",
	// "explore").
	Profile string

	// Description is a short, user-facing label for progress UI.
	Description string

	// Prompt is the task description given to the subagent.
	Prompt string

	// AllowedTools restricts the tool set available to the subagent.
	// Nil means inherit from the selected profile.
	AllowedTools []string

	// DeniedTools removes tools even if profile defaults include them.
	DeniedTools []string

	// WorkDir overrides the working directory. Defaults to the parent's WorkDir.
	WorkDir string

	// ParentSessionID links the subagent's session to its parent for lineage
	// tracking.
	ParentSessionID string

	// ParentTaskID links to the parent delegated task when nested delegation is
	// allowed.
	ParentTaskID string

	// Command records the supervisor command that triggered this delegation.
	Command string

	// MaxTurns overrides the default turn limit for this subagent.
	// Zero means use the global default.
	MaxTurns int

	// MaxDuration bounds total wall-clock runtime. Zero means default runtime
	// limit.
	MaxDuration time.Duration

	// Metadata holds optional audit tags (trace IDs, planner strategy, etc.).
	Metadata map[string]string
}

// Normalize applies defaults to the options in-place.
func (o *Options) Normalize() {
	if o.Type == "" {
		o.Type = SubagentInProcess
	}
	if strings.TrimSpace(o.Profile) == "" {
		o.Profile = DefaultProfileGeneral
	}
}

// Validate returns an error when required options are invalid.
func (o Options) Validate() error {
	if strings.TrimSpace(o.Description) == "" {
		return errors.New("description is required")
	}
	if strings.TrimSpace(o.Prompt) == "" {
		return errors.New("prompt is required")
	}
	switch o.Type {
	case SubagentInProcess, SubagentTeammate, SubagentWorktree:
		// Valid.
	default:
		return fmt.Errorf("invalid subagent type %q", o.Type)
	}
	if o.MaxTurns < 0 {
		return errors.New("max turns cannot be negative")
	}
	if o.MaxDuration < 0 {
		return errors.New("max duration cannot be negative")
	}
	return nil
}

// Usage captures aggregate token and cost accounting for a task.
type Usage struct {
	InputTokens  int
	OutputTokens int
	CostUSD      float64
}

// OutputContract defines the canonical payload a subagent should return.
//
// The supervisor can render Summary directly and reason over Findings,
// ChangedFiles, and NextActions without re-parsing free-form text.
type OutputContract struct {
	Summary      string
	Findings     []string
	ChangedFiles []string
	NextActions  []string
	Raw          string
}

// Result is returned when a subagent run completes or terminates.
type Result struct {
	// TaskID identifies this delegated task.
	TaskID string

	// State is the terminal state for the run.
	State TaskState

	// Profile/Type echo the resolved execution configuration.
	Profile string
	Type    SubagentType

	// Output is the final text produced by the subagent.
	Output string

	// Contract is the structured extraction from Output.
	Contract OutputContract

	// SessionID is the ID of the session the subagent created.
	SessionID string

	// StartedAt and FinishedAt provide run timing boundaries.
	StartedAt  time.Time
	FinishedAt time.Time

	// Duration is the wall-clock time taken.
	Duration time.Duration

	// Usage contains token and cost accounting.
	Usage Usage

	// Error is set for failed/cancelled runs.
	Error string
}

// Runner manages delegated tasks and subagent lifecycle.
type Runner interface {
	// Run starts a new subagent run or resumes one when Options.TaskID is set.
	Run(ctx context.Context, opts Options) (Result, error)

	// Status returns the latest known lifecycle state for taskID.
	Status(ctx context.Context, taskID string) (TaskState, error)

	// Cancel requests cancellation of taskID.
	Cancel(ctx context.Context, taskID string) error
}
