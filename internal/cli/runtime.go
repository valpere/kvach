package cli

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/valpere/kvach/internal/agent"
	"github.com/valpere/kvach/internal/config"
	"github.com/valpere/kvach/internal/git"
	"github.com/valpere/kvach/internal/permission"
	"github.com/valpere/kvach/internal/provider"
	anthropicProvider "github.com/valpere/kvach/internal/provider/anthropic"
	openaiProvider "github.com/valpere/kvach/internal/provider/openai"
	"github.com/valpere/kvach/internal/session"
	"github.com/valpere/kvach/internal/tool"
)

type agentRuntime struct {
	agent     *agent.Agent
	store     *session.SQLiteStore
	fullModel string
	workDir   string
	projectID string
}

type runtimeOptions struct {
	PermissionAsker permission.Asker
}

func newAgentRuntime(ctx context.Context) (*agentRuntime, error) {
	return newAgentRuntimeWithOptions(ctx, runtimeOptions{})
}

func newAgentRuntimeWithOptions(ctx context.Context, opts runtimeOptions) (*agentRuntime, error) {
	workDir := globalFlags.WorkDir
	if workDir == "" {
		workDir, _ = os.Getwd()
	}

	cfg, err := config.Load(workDir)
	if err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}

	model := cfg.Model
	if globalFlags.Model != "" {
		model = globalFlags.Model
	}

	providerName, modelID := splitModel(model)
	var prov provider.Provider
	switch providerName {
	case "openai", "groq", "openrouter", "together":
		prov = openaiProvider.NewCompatible(providerName, strings.Title(providerName), "", "")
	default:
		prov = anthropicProvider.New("", "")
	}

	store, err := openSessionStore(ctx)
	if err != nil {
		return nil, fmt.Errorf("open session store: %w", err)
	}

	projectID := git.SlugFromRoot(workDir)
	if root, err := git.Root(ctx, workDir); err == nil {
		projectID = git.SlugFromRoot(root)
	}

	systemPrompt := cfg.Instructions
	if systemPrompt == "" {
		systemPrompt = "You are kvach, an AI coding agent. You have access to tools for reading files, writing files, executing shell commands, and searching code. Use them to help the user."
	}

	a := agent.New(prov, tool.DefaultRegistry, store, agent.Config{
		MaxTurns:        cfg.MaxTurns,
		WorkDir:         workDir,
		SystemPrompt:    systemPrompt,
		Model:           modelID,
		PermissionAsker: opts.PermissionAsker,
		PermissionContext: permission.Context{
			Mode:               resolvePermissionMode(cfg.Permission.Mode),
			AllowRules:         append([]permission.Rule(nil), cfg.Permission.Allow...),
			DenyRules:          append([]permission.Rule(nil), cfg.Permission.Deny...),
			WorkingDirectories: []string{workDir},
		},
	})

	return &agentRuntime{
		agent:     a,
		store:     store,
		fullModel: model,
		workDir:   workDir,
		projectID: projectID,
	}, nil
}

func resolvePermissionMode(mode permission.Mode) permission.Mode {
	if mode == "" {
		return permission.ModeBypass
	}
	return mode
}

func (r *agentRuntime) latestSessionID(ctx context.Context) (string, error) {
	sessions, err := r.store.ListSessions(ctx, r.projectID)
	if err != nil {
		return "", err
	}
	for _, sess := range sessions {
		if sess.ArchivedAt == nil {
			return sess.ID, nil
		}
	}
	return "", session.ErrNotFound
}
