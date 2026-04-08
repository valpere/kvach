package agent

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/valpere/kvach/internal/multiagent"
)

type subagentRunner struct {
	mu      sync.RWMutex
	states  map[string]multiagent.TaskState
	cancels map[string]context.CancelFunc
}

func newSubagentRunner() multiagent.Runner {
	return &subagentRunner{
		states:  make(map[string]multiagent.TaskState),
		cancels: make(map[string]context.CancelFunc),
	}
}

func (r *subagentRunner) Run(ctx context.Context, opts multiagent.Options) (multiagent.Result, error) {
	opts.Normalize()
	if err := opts.Validate(); err != nil {
		return multiagent.Result{}, err
	}

	taskID := opts.TaskID
	if taskID == "" {
		taskID = "task-" + uuid.NewString()
	}

	started := time.Now().UTC()
	r.setState(taskID, multiagent.TaskRunning)

	// This initial in-process runner returns a structured contract envelope from
	// the provided prompt/description. A full child-agent execution engine can
	// replace this implementation without changing the Runner interface.
	summary := strings.TrimSpace(opts.Description)
	if summary == "" {
		summary = "Subtask completed"
	}
	raw := strings.TrimSpace(opts.Prompt)
	if len(raw) > 4000 {
		raw = raw[:4000]
	}

	finished := time.Now().UTC()
	r.setState(taskID, multiagent.TaskCompleted)

	res := multiagent.Result{
		TaskID:     taskID,
		State:      multiagent.TaskCompleted,
		Profile:    opts.Profile,
		Type:       opts.Type,
		Output:     fmt.Sprintf("Task %q completed for profile %q", opts.Description, opts.Profile),
		StartedAt:  started,
		FinishedAt: finished,
		Duration:   finished.Sub(started),
		Contract: multiagent.OutputContract{
			Summary:     summary,
			Findings:    []string{"Subagent runner executed in-process"},
			NextActions: []string{"Continue with parent task"},
			Raw:         raw,
		},
	}
	return res, nil
}

func (r *subagentRunner) Status(_ context.Context, taskID string) (multiagent.TaskState, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	state, ok := r.states[taskID]
	if !ok {
		return "", fmt.Errorf("task %s not found", taskID)
	}
	return state, nil
}

func (r *subagentRunner) Cancel(_ context.Context, taskID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if cancel, ok := r.cancels[taskID]; ok {
		cancel()
		delete(r.cancels, taskID)
	}
	r.states[taskID] = multiagent.TaskCancelled
	return nil
}

func (r *subagentRunner) setState(taskID string, state multiagent.TaskState) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.states[taskID] = state
}
