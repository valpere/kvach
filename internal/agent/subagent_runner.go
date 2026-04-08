package agent

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/valpere/kvach/internal/multiagent"
)

type subagentRunner struct {
	parent *Agent

	mu      sync.RWMutex
	states  map[string]multiagent.TaskState
	cancels map[string]context.CancelFunc
	results map[string]multiagent.Result
}

func newSubagentRunner(parent *Agent) multiagent.Runner {
	return &subagentRunner{
		parent:  parent,
		states:  make(map[string]multiagent.TaskState),
		cancels: make(map[string]context.CancelFunc),
		results: make(map[string]multiagent.Result),
	}
}

func (r *subagentRunner) Run(ctx context.Context, opts multiagent.Options) (multiagent.Result, error) {
	opts.Normalize()
	if err := opts.Validate(); err != nil {
		return multiagent.Result{}, err
	}

	if strings.TrimSpace(opts.TaskID) != "" {
		r.mu.RLock()
		res, ok := r.results[opts.TaskID]
		r.mu.RUnlock()
		if ok {
			return res, nil
		}
		return multiagent.Result{}, fmt.Errorf("task %s not found", opts.TaskID)
	}
	taskID := "task-" + uuid.NewString()

	started := time.Now().UTC()
	childCtx, cancel := context.WithCancel(ctx)
	r.mu.Lock()
	r.cancels[taskID] = cancel
	r.mu.Unlock()
	defer func() {
		r.mu.Lock()
		delete(r.cancels, taskID)
		r.mu.Unlock()
	}()

	r.setState(taskID, multiagent.TaskRunning)

	child, err := r.newChildAgent(opts)
	if err != nil {
		r.setState(taskID, multiagent.TaskFailed)
		return multiagent.Result{}, err
	}

	prompt := opts.Prompt
	if strings.TrimSpace(opts.Description) != "" {
		prompt = fmt.Sprintf("Task: %s\n\n%s", strings.TrimSpace(opts.Description), opts.Prompt)
	}

	events, err := child.Run(childCtx, RunOptions{Prompt: prompt})
	if err != nil {
		r.setState(taskID, multiagent.TaskFailed)
		return multiagent.Result{}, err
	}

	var (
		output       strings.Builder
		state        = multiagent.TaskCompleted
		errorMessage string
		usage        multiagent.Usage
	)

	for evt := range events {
		switch evt.Type {
		case EventTextDelta:
			if s, ok := evt.Payload.(string); ok {
				output.WriteString(s)
			}
		case EventUsageUpdated:
			if u, ok := evt.Payload.(UsageInfo); ok {
				usage.InputTokens += u.InputTokens
				usage.OutputTokens += u.OutputTokens
				usage.CostUSD += u.TotalCostUSD
			}
		case EventError:
			state = multiagent.TaskFailed
			if s, ok := evt.Payload.(string); ok {
				errorMessage = strings.TrimSpace(s)
			}
		case EventDone:
			if reason, ok := evt.Payload.(string); ok {
				switch reason {
				case string(ReasonAborted):
					if state == multiagent.TaskCompleted {
						state = multiagent.TaskCancelled
					}
				case string(ReasonMaxTurns):
					if state == multiagent.TaskCompleted {
						state = multiagent.TaskTimeout
					}
				}
			}
		}
	}

	outText := strings.TrimSpace(output.String())
	contract := buildOutputContract(opts.Description, outText)
	if outText == "" {
		outText = contract.Raw
	}

	if errors.Is(childCtx.Err(), context.Canceled) && state == multiagent.TaskCompleted {
		state = multiagent.TaskCancelled
	}
	if state == multiagent.TaskFailed && errorMessage == "" {
		errorMessage = "subagent failed"
	}

	finished := time.Now().UTC()
	r.setState(taskID, state)

	res := multiagent.Result{
		TaskID:     taskID,
		State:      state,
		Profile:    opts.Profile,
		Type:       opts.Type,
		Output:     outText,
		StartedAt:  started,
		FinishedAt: finished,
		Duration:   finished.Sub(started),
		Usage:      usage,
		Contract:   contract,
		Error:      errorMessage,
	}

	r.mu.Lock()
	r.results[taskID] = res
	r.mu.Unlock()

	if state == multiagent.TaskFailed {
		if errorMessage == "" {
			errorMessage = "subagent failed"
		}
		return res, errors.New(errorMessage)
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

func (r *subagentRunner) newChildAgent(opts multiagent.Options) (*Agent, error) {
	if r.parent == nil {
		return nil, errors.New("subagent runner is not configured")
	}

	cfg := r.parent.config
	if strings.TrimSpace(opts.Profile) != "" {
		cfg.AgentName = strings.TrimSpace(opts.Profile)
	}
	if strings.TrimSpace(opts.WorkDir) != "" {
		cfg.WorkDir = strings.TrimSpace(opts.WorkDir)
	}
	if opts.MaxTurns > 0 {
		cfg.MaxTurns = opts.MaxTurns
	}
	cfg.SystemPrompt = strings.TrimSpace(cfg.SystemPrompt + "\n\nYou are a delegated subagent. Complete only the delegated task and return concise, actionable results.")

	child := &Agent{
		provider: r.parent.provider,
		registry: r.parent.registry,
		sessions: r.parent.sessions,
		tasks:    r,
		config:   cfg,
	}
	return child, nil
}

func buildOutputContract(description, output string) multiagent.OutputContract {
	summary := strings.TrimSpace(description)
	if summary == "" {
		summary = "Delegated task completed"
	}

	raw := strings.TrimSpace(output)
	if len(raw) > 4000 {
		raw = raw[:4000]
	}

	findings := make([]string, 0, 5)
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "- ") {
			findings = append(findings, strings.TrimSpace(strings.TrimPrefix(line, "- ")))
		}
		if len(findings) == 5 {
			break
		}
	}

	return multiagent.OutputContract{
		Summary:  summary,
		Findings: findings,
		Raw:      raw,
	}
}
