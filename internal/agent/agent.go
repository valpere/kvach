// Package agent implements the core agentic loop.
//
// The agent maintains a conversation with an LLM provider, dispatches tool
// calls returned by the model, feeds results back, and repeats until the
// model stops requesting tools or a termination condition is reached.
//
// The primary entry point is [Agent.Run], which returns a channel of [Event]
// values that callers (CLI, TUI, HTTP server) consume to drive their UIs.
package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
	"github.com/valpere/kvach/internal/bus"
	"github.com/valpere/kvach/internal/git"
	"github.com/valpere/kvach/internal/memory"
	"github.com/valpere/kvach/internal/multiagent"
	"github.com/valpere/kvach/internal/permission"
	"github.com/valpere/kvach/internal/provider"
	"github.com/valpere/kvach/internal/session"
	"github.com/valpere/kvach/internal/tool"
)

// Agent orchestrates one or more turns of LLM <-> tool interaction.
type Agent struct {
	provider provider.Provider
	registry *tool.Registry
	sessions session.Store
	tasks    multiagent.Runner
	bus      *bus.Bus
	config   Config
}

// Config holds the runtime settings for an Agent.
type Config struct {
	// MaxTurns is the maximum number of LLM->tool round-trips per Run call.
	// Zero means use the default (50).
	MaxTurns int

	// AgentName selects the named agent configuration (e.g. "build", "plan").
	// Empty defaults to "build".
	AgentName string

	// WorkDir is the working directory for tool execution.
	// Defaults to the process working directory.
	WorkDir string

	// SystemPrompt is the system message sent to the LLM.
	SystemPrompt string

	// Model overrides the provider-level model selection.
	Model string

	// PermissionContext controls whether tool calls are allowed, denied, or
	// require user confirmation.
	PermissionContext permission.Context

	// PermissionAsker is used when a tool call requires interactive approval.
	PermissionAsker permission.Asker
}

// New creates an Agent with the given provider, tool registry, session store,
// and optional configuration overrides.
func New(p provider.Provider, r *tool.Registry, s session.Store, cfg Config) *Agent {
	if cfg.MaxTurns == 0 {
		cfg.MaxTurns = 50
	}
	if cfg.AgentName == "" {
		cfg.AgentName = "build"
	}
	a := &Agent{
		provider: p,
		registry: r,
		sessions: s,
		bus:      bus.New(),
		config:   cfg,
	}
	if a.config.PermissionContext.Mode == "" {
		a.config.PermissionContext.Mode = permission.ModeBypass
	}
	if strings.TrimSpace(a.config.WorkDir) != "" && len(a.config.PermissionContext.WorkingDirectories) == 0 {
		a.config.PermissionContext.WorkingDirectories = []string{a.config.WorkDir}
	}
	a.tasks = newSubagentRunner(a)
	return a
}

// RunOptions parameterises a single Run call.
type RunOptions struct {
	// SessionID resumes an existing session when non-empty.
	// An empty value starts a new session.
	SessionID string

	// Prompt is the user message that starts this run.
	Prompt string
}

// Run executes the agentic loop for the given prompt and returns a channel of
// events. The channel is closed when the loop terminates (successfully or not).
// The caller must drain the channel; cancelling ctx aborts the run.
func (a *Agent) Run(ctx context.Context, opts RunOptions) (<-chan Event, error) {
	ch := make(chan Event, 64)
	go func() {
		defer close(ch)
		if err := a.loop(ctx, opts, ch); err != nil {
			ch <- Event{Type: EventError, Payload: err.Error()}
		}
	}()
	return ch, nil
}

// loop is the core agentic while-loop.
func (a *Agent) loop(ctx context.Context, opts RunOptions, events chan<- Event) error {
	sessionID, err := a.ensureSession(ctx, opts.SessionID)
	if err != nil {
		return err
	}

	// Build initial messages from the user prompt.
	messages := []provider.Message{
		{
			Role: "user",
			Parts: []provider.Part{
				{Type: provider.PartTypeText, Text: opts.Prompt},
			},
		},
	}
	_ = a.persistProviderMessage(ctx, sessionID, messages[0], "", 0, 0, 0)

	// Build the tool schemas for the LLM.
	tools := a.buildToolSchemas()

	// Load project memory index and append it to the system prompt when present.
	systemPrompt := a.config.SystemPrompt
	if strings.TrimSpace(a.config.WorkDir) != "" {
		mem := memory.NewSystem(filepath.Join(a.config.WorkDir, ".kvach", "memory"))
		if mem.IsEnabled() {
			if idx, err := mem.LoadIndexPrompt(""); err == nil && strings.TrimSpace(idx) != "" {
				systemPrompt = strings.TrimSpace(systemPrompt) + "\n\n# Memory Index\n\n" + idx
			}
		}
	}

	for turn := 0; turn < a.config.MaxTurns; turn++ {
		if ctx.Err() != nil {
			events <- Event{Type: EventDone, Payload: string(ReasonAborted)}
			return nil
		}

		// Stream the LLM response.
		req := &provider.StreamRequest{
			Model:     a.config.Model,
			Messages:  messages,
			Tools:     tools,
			System:    systemPrompt,
			MaxTokens: 8192,
		}

		stream, err := a.provider.Stream(ctx, req)
		if err != nil {
			return fmt.Errorf("stream turn %d: %w", turn, err)
		}

		// Process the streamed response.
		assistantMsg, toolCalls, finishReason, err := a.processStream(stream, events)
		if err != nil {
			return fmt.Errorf("process stream turn %d: %w", turn, err)
		}

		// Append assistant message to history.
		messages = append(messages, assistantMsg)
		_ = a.persistProviderMessage(ctx, sessionID, assistantMsg, finishReason, 0, 0, 0)

		// If no tool calls, we're done.
		if len(toolCalls) == 0 {
			events <- Event{Type: EventDone, Payload: string(ReasonCompleted)}
			return nil
		}

		// Check for end_turn or stop finish reason with tool calls — unusual
		// but some models do this.
		if finishReason != "tool_use" && finishReason != "" && len(toolCalls) == 0 {
			events <- Event{Type: EventDone, Payload: string(ReasonCompleted)}
			return nil
		}

		// Execute tool calls and build result message.
		resultParts := a.executeToolCalls(ctx, sessionID, toolCalls, events)

		resultMsg := provider.Message{
			Role:  "user",
			Parts: resultParts,
		}
		messages = append(messages, resultMsg)
		_ = a.persistProviderMessage(ctx, sessionID, resultMsg, "", 0, 0, 0)
	}

	events <- Event{Type: EventDone, Payload: string(ReasonMaxTurns)}
	return nil
}

func (a *Agent) ensureSession(ctx context.Context, sessionID string) (string, error) {
	if a.sessions == nil {
		return "", nil
	}
	if strings.TrimSpace(sessionID) != "" {
		if _, err := a.sessions.GetSession(ctx, sessionID); err != nil {
			return "", fmt.Errorf("load session %s: %w", sessionID, err)
		}
		return sessionID, nil
	}

	id := uuid.NewString()
	projectID := "project"
	if strings.TrimSpace(a.config.WorkDir) != "" {
		if root, err := git.Root(ctx, a.config.WorkDir); err == nil {
			projectID = git.SlugFromRoot(root)
		} else {
			projectID = git.SlugFromRoot(a.config.WorkDir)
		}
	}

	err := a.sessions.CreateSession(ctx, session.Session{
		ID:        id,
		ProjectID: projectID,
		Directory: a.config.WorkDir,
		Title:     "kvach run",
	})
	if err != nil {
		return "", fmt.Errorf("create session: %w", err)
	}
	return id, nil
}

func (a *Agent) persistProviderMessage(ctx context.Context, sessionID string, msg provider.Message, finishReason string, inputTokens, outputTokens int, cost float64) error {
	if a.sessions == nil || sessionID == "" {
		return nil
	}

	messageID := uuid.NewString()
	err := a.sessions.AppendMessage(ctx, session.Message{
		ID:           messageID,
		SessionID:    sessionID,
		Role:         msg.Role,
		AgentName:    a.config.AgentName,
		ModelID:      a.config.Model,
		InputTokens:  inputTokens,
		OutputTokens: outputTokens,
		CostUSD:      cost,
		FinishReason: finishReason,
	})
	if err != nil {
		return err
	}

	for _, part := range msg.Parts {
		payload, ptype := mapProviderPart(part)
		if payload == nil {
			continue
		}
		data, err := json.Marshal(payload)
		if err != nil {
			continue
		}
		_ = a.sessions.AppendPart(ctx, session.Part{
			ID:        uuid.NewString(),
			MessageID: messageID,
			Type:      ptype,
			Data:      data,
		})
	}
	return nil
}

func mapProviderPart(part provider.Part) (any, session.PartType) {
	switch part.Type {
	case provider.PartTypeText:
		return session.TextData{Text: part.Text}, session.PartTypeText
	case provider.PartTypeReasoning:
		return session.ReasoningData{Reasoning: part.Text}, session.PartTypeReasoning
	case provider.PartTypeToolUse:
		if part.ToolUse == nil {
			return nil, ""
		}
		return session.ToolUseData{
			ID:    part.ToolUse.ID,
			Name:  part.ToolUse.Name,
			Input: part.ToolUse.Input,
			State: "completed",
		}, session.PartTypeToolUse
	case provider.PartTypeToolResult:
		if part.ToolResult == nil {
			return nil, ""
		}
		return session.ToolResultData{
			ToolUseID: part.ToolResult.ToolUseID,
			Content:   part.ToolResult.Content,
			IsError:   part.ToolResult.IsError,
		}, session.PartTypeToolResult
	case provider.PartTypeFile:
		if part.File == nil {
			return nil, ""
		}
		return session.FileData{
			Path:     part.File.Path,
			MimeType: part.File.MimeType,
			URL:      part.File.URL,
		}, session.PartTypeFile
	default:
		return nil, ""
	}
}

// processStream reads all events from the stream channel and assembles
// the assistant message + tool calls.
func (a *Agent) processStream(stream <-chan provider.StreamEvent, events chan<- Event) (provider.Message, []toolCall, string, error) {
	var (
		textParts    strings.Builder
		toolCalls    []toolCall
		current      *toolCall
		finishReason string
	)

	for evt := range stream {
		switch evt.Type {
		case provider.StreamEventTextDelta:
			textParts.WriteString(evt.Text)
			events <- Event{Type: EventTextDelta, Payload: evt.Text}

		case provider.StreamEventReasoningDelta:
			events <- Event{Type: EventReasoningDelta, Payload: evt.Reasoning}

		case provider.StreamEventToolUseStart:
			current = &toolCall{ID: evt.ToolUseID, Name: evt.ToolName}
			events <- Event{Type: EventToolStarted, Payload: ToolCallInfo{
				ID: evt.ToolUseID, Name: evt.ToolName,
			}}

		case provider.StreamEventToolUseDelta:
			if current != nil {
				current.inputJSON.WriteString(evt.PartialJSON)
			}

		case provider.StreamEventToolUseEnd:
			if current != nil {
				toolCalls = append(toolCalls, *current)
				current = nil
			}

		case provider.StreamEventMessageEnd:
			finishReason = evt.FinishReason
			if evt.Usage != nil {
				events <- Event{Type: EventUsageUpdated, Payload: UsageInfo{
					InputTokens:  evt.Usage.InputTokens,
					OutputTokens: evt.Usage.OutputTokens,
					CacheRead:    evt.Usage.CacheRead,
					CacheWrite:   evt.Usage.CacheWrite,
				}}
			}

		case provider.StreamEventError:
			return provider.Message{}, nil, "", fmt.Errorf("stream error: %s", evt.Error)
		}
	}

	// Build assistant message.
	msg := provider.Message{Role: "assistant"}
	if textParts.Len() > 0 {
		msg.Parts = append(msg.Parts, provider.Part{
			Type: provider.PartTypeText,
			Text: textParts.String(),
		})
	}
	for _, tc := range toolCalls {
		msg.Parts = append(msg.Parts, provider.Part{
			Type: provider.PartTypeToolUse,
			ToolUse: &provider.ToolUsePart{
				ID:    tc.ID,
				Name:  tc.Name,
				Input: json.RawMessage(tc.inputJSON.String()),
			},
		})
	}

	return msg, toolCalls, finishReason, nil
}

// executeToolCalls runs each tool call and returns the result parts.
func (a *Agent) executeToolCalls(ctx context.Context, sessionID string, calls []toolCall, events chan<- Event) []provider.Part {
	tctx := &tool.Context{
		SessionID:    sessionID,
		WorkDir:      a.config.WorkDir,
		Permissions:  &a.config.PermissionContext,
		Asker:        a.config.PermissionAsker,
		TaskRunner:   a.tasks,
		SessionStore: a.sessions,
		EventBus:     a.bus,
	}

	var parts []provider.Part

	for _, tc := range calls {
		t, ok := a.registry.Get(tc.Name)
		if !ok {
			// Tool not found — return error to LLM.
			parts = append(parts, provider.Part{
				Type: provider.PartTypeToolResult,
				ToolResult: &provider.ToolResultPart{
					ToolUseID: tc.ID,
					Content:   fmt.Sprintf("Error: tool %q not found", tc.Name),
					IsError:   true,
				},
			})
			events <- Event{Type: EventToolError, Payload: ToolErrorInfo{
				ID: tc.ID, Name: tc.Name, Message: "tool not found",
			}}
			continue
		}

		input := json.RawMessage(tc.inputJSON.String())

		// Validate input.
		if err := t.ValidateInput(input); err != nil {
			parts = append(parts, provider.Part{
				Type: provider.PartTypeToolResult,
				ToolResult: &provider.ToolResultPart{
					ToolUseID: tc.ID,
					Content:   fmt.Sprintf("Validation error: %v", err),
					IsError:   true,
				},
			})
			events <- Event{Type: EventToolError, Payload: ToolErrorInfo{
				ID: tc.ID, Name: tc.Name, Message: err.Error(),
			}}
			continue
		}

		permissionInput := decodePermissionInput(input)

		if err := a.checkToolPermissions(ctx, t, tc, permissionInput, input, tctx, events); err != nil {
			parts = append(parts, provider.Part{
				Type: provider.PartTypeToolResult,
				ToolResult: &provider.ToolResultPart{
					ToolUseID: tc.ID,
					Content:   fmt.Sprintf("Permission denied: %v", err),
					IsError:   true,
				},
			})
			events <- Event{Type: EventToolError, Payload: ToolErrorInfo{
				ID: tc.ID, Name: tc.Name, Message: err.Error(),
			}}
			continue
		}

		// Execute the tool.
		result, err := t.Call(ctx, input, tctx)
		if err != nil {
			parts = append(parts, provider.Part{
				Type: provider.PartTypeToolResult,
				ToolResult: &provider.ToolResultPart{
					ToolUseID: tc.ID,
					Content:   fmt.Sprintf("Error: %v", err),
					IsError:   true,
				},
			})
			events <- Event{Type: EventToolError, Payload: ToolErrorInfo{
				ID: tc.ID, Name: tc.Name, Message: err.Error(),
			}}
			continue
		}

		parts = append(parts, provider.Part{
			Type: provider.PartTypeToolResult,
			ToolResult: &provider.ToolResultPart{
				ToolUseID: tc.ID,
				Content:   result.Content,
			},
		})
		events <- Event{Type: EventToolCompleted, Payload: ToolResultInfo{
			ID: tc.ID, Name: tc.Name, Content: result.Content,
		}}
	}

	return parts
}

func decodePermissionInput(input json.RawMessage) map[string]any {
	var decoded map[string]any
	if err := json.Unmarshal(input, &decoded); err != nil || decoded == nil {
		return map[string]any{}
	}
	return decoded
}

func (a *Agent) checkToolPermissions(
	ctx context.Context,
	t tool.Tool,
	tc toolCall,
	inputMap map[string]any,
	rawInput json.RawMessage,
	tctx *tool.Context,
	events chan<- Event,
) error {
	toolDecision := t.CheckPermissions(rawInput, tctx)
	if strings.EqualFold(toolDecision.Decision, "deny") {
		reason := strings.TrimSpace(toolDecision.Reason)
		if reason == "" {
			reason = "blocked by tool-specific permission checks"
		}
		return errors.New(reason)
	}
	if strings.EqualFold(toolDecision.Decision, "ask") {
		return a.askPermission(ctx, tc, inputMap, t, toolDecision.Reason, events)
	}

	pctx := &a.config.PermissionContext

	for _, rule := range pctx.DenyRules {
		if permission.MatchRule(rule, tc.Name, inputMap) {
			return fmt.Errorf("blocked by deny rule (%s %s)", rule.Tool, rule.Pattern)
		}
	}

	for _, rule := range pctx.AllowRules {
		if permission.MatchRule(rule, tc.Name, inputMap) {
			return nil
		}
	}

	if t.IsDestructive(rawInput) {
		return a.askPermission(ctx, tc, inputMap, t, "destructive tool call", events)
	}

	switch pctx.Mode {
	case permission.ModeBypass:
		return nil
	case permission.ModePlan:
		if !t.IsReadOnly(rawInput) {
			return fmt.Errorf("mode %q allows read-only tools only", pctx.Mode)
		}
		return nil
	case permission.ModeDontAsk:
		if t.IsReadOnly(rawInput) {
			return nil
		}
		return fmt.Errorf("mode %q requires explicit allow rules", pctx.Mode)
	case permission.ModeAcceptEdits:
		if t.IsReadOnly(rawInput) {
			return nil
		}
		if strings.EqualFold(tc.Name, "Bash") {
			return a.askPermission(ctx, tc, inputMap, t, "shell execution requires confirmation", events)
		}
		return nil
	default:
		if t.IsReadOnly(rawInput) {
			return nil
		}
		return a.askPermission(ctx, tc, inputMap, t, "tool call requires confirmation", events)
	}
}

func (a *Agent) askPermission(
	ctx context.Context,
	tc toolCall,
	input map[string]any,
	t tool.Tool,
	reason string,
	events chan<- Event,
) error {
	requestID := "perm-" + uuid.NewString()
	risk := "medium"
	if t.IsDestructive(json.RawMessage(tc.inputJSON.String())) {
		risk = "destructive"
	}
	description := strings.TrimSpace(reason)
	if description == "" {
		description = "permission required"
	}

	events <- Event{Type: EventPermissionAsked, Payload: PermissionInfo{
		ID:          requestID,
		ToolName:    tc.Name,
		Description: description,
		Risk:        risk,
	}}

	asker := a.config.PermissionAsker
	if asker == nil {
		events <- Event{Type: EventPermissionResolved, Payload: PermissionResolutionInfo{
			ID:       requestID,
			Decision: "deny",
			Reason:   "no permission asker configured",
		}}
		return fmt.Errorf("%s: no permission asker configured", description)
	}

	reply, err := asker.Ask(ctx, permission.Request{
		ID:          requestID,
		ToolName:    tc.Name,
		Description: description,
		Input:       input,
		Risk:        risk,
	})
	if err != nil {
		events <- Event{Type: EventPermissionResolved, Payload: PermissionResolutionInfo{
			ID:       requestID,
			Decision: "deny",
			Reason:   err.Error(),
		}}
		return err
	}

	switch strings.ToLower(strings.TrimSpace(reply.Decision)) {
	case "allow_once":
		events <- Event{Type: EventPermissionResolved, Payload: PermissionResolutionInfo{ID: requestID, Decision: "allow"}}
		return nil
	case "allow_always":
		pattern := strings.TrimSpace(reply.Pattern)
		if pattern == "" {
			pattern = "*"
		}
		a.config.PermissionContext.AllowRules = append(a.config.PermissionContext.AllowRules, permission.Rule{
			Source:   permission.RuleSourceSession,
			Behavior: "allow",
			Tool:     tc.Name,
			Pattern:  pattern,
		})
		events <- Event{Type: EventPermissionResolved, Payload: PermissionResolutionInfo{ID: requestID, Decision: "allow_always"}}
		return nil
	default:
		events <- Event{Type: EventPermissionResolved, Payload: PermissionResolutionInfo{
			ID:       requestID,
			Decision: "deny",
			Reason:   "denied by user",
		}}
		return fmt.Errorf("denied by user")
	}
}

// buildToolSchemas converts the registry's tools into provider.ToolSchema
// values for the LLM request.
func (a *Agent) buildToolSchemas() []provider.ToolSchema {
	tools := a.registry.All()
	schemas := make([]provider.ToolSchema, 0, len(tools))
	for _, t := range tools {
		schemas = append(schemas, provider.ToolSchema{
			Name:        t.Name(),
			Description: t.Prompt(tool.PromptOptions{}),
			InputSchema: t.InputSchema(),
		})
	}
	return schemas
}

// toolCall accumulates a single tool call from streaming events.
type toolCall struct {
	ID        string
	Name      string
	inputJSON strings.Builder
}
