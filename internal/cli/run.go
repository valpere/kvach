package cli

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/valpere/kvach/internal/agent"
	"github.com/valpere/kvach/internal/session"
)

func newRunCmd() *cobra.Command {
	var runFlags struct {
		SessionID string
		Continue  bool
	}

	cmd := &cobra.Command{
		Use:   "run [prompt]",
		Short: "Run a single prompt non-interactively",
		Long: `Send a prompt to the agent and stream the response to stdout.
The agent runs the full agentic loop (including tool calls) and exits when complete.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			prompt := args[0]
			ctx := cmd.Context()
			out := cmd.OutOrStdout()

			runtime, err := newAgentRuntime(ctx)
			if err != nil {
				return err
			}
			defer runtime.store.Close()

			sessionID, err := resolveRunSessionID(ctx, runtime, runFlags.SessionID, runFlags.Continue)
			if err != nil {
				return err
			}
			if globalFlags.Debug && sessionID != "" {
				fmt.Fprintf(out, "[session] using %s\n", sessionID)
			}

			// Run and stream events to stdout.
			events, err := runtime.agent.Run(ctx, agent.RunOptions{
				Prompt:    prompt,
				SessionID: sessionID,
			})
			if err != nil {
				return fmt.Errorf("run agent: %w", err)
			}

			for evt := range events {
				switch evt.Type {
				case agent.EventTextDelta:
					if s, ok := evt.Payload.(string); ok {
						fmt.Fprint(out, s)
					}
				case agent.EventToolStarted:
					if info, ok := evt.Payload.(agent.ToolCallInfo); ok {
						if globalFlags.Debug {
							fmt.Fprintf(out, "\n[tool] %s\n", info.Name)
						}
					}
				case agent.EventToolCompleted:
					if info, ok := evt.Payload.(agent.ToolResultInfo); ok {
						if globalFlags.Debug {
							fmt.Fprintf(out, "[tool] %s done (%d bytes)\n", info.Name, len(info.Content))
						}
					}
				case agent.EventToolError:
					if info, ok := evt.Payload.(agent.ToolErrorInfo); ok {
						fmt.Fprintf(out, "\n[error] %s: %s\n", info.Name, info.Message)
					}
				case agent.EventError:
					if s, ok := evt.Payload.(string); ok {
						fmt.Fprintf(out, "\n[error] %s\n", s)
					}
				case agent.EventDone:
					fmt.Fprintln(out)
				}
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&runFlags.SessionID, "resume", "", "Resume an existing session by ID")
	cmd.Flags().BoolVar(&runFlags.Continue, "continue", false, "Continue the most recent session")
	return cmd
}

func resolveRunSessionID(ctx context.Context, runtime *agentRuntime, resumeSessionID string, continueLatest bool) (string, error) {
	resumeSessionID = strings.TrimSpace(resumeSessionID)
	if resumeSessionID != "" && continueLatest {
		return "", fmt.Errorf("--resume and --continue are mutually exclusive")
	}
	if resumeSessionID != "" {
		return resumeSessionID, nil
	}
	if !continueLatest {
		return "", nil
	}

	sessionID, err := runtime.latestSessionID(ctx)
	if err != nil {
		if errors.Is(err, session.ErrNotFound) {
			return "", fmt.Errorf("no previous sessions found for this project")
		}
		return "", fmt.Errorf("resolve latest session: %w", err)
	}
	return sessionID, nil
}

func splitModel(model string) (providerName, modelID string) {
	model = strings.TrimSpace(model)
	if model == "" {
		return "anthropic", "claude-sonnet-4-5"
	}
	if i := strings.IndexByte(model, '/'); i > 0 {
		return strings.ToLower(model[:i]), model[i+1:]
	}
	if strings.HasPrefix(model, "gpt-") || strings.HasPrefix(model, "o") {
		return "openai", model
	}
	return "anthropic", model
}
