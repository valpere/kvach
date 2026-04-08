package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/valpere/kvach/internal/agent"
	"github.com/valpere/kvach/internal/config"
	anthropicProvider "github.com/valpere/kvach/internal/provider/anthropic"
	"github.com/valpere/kvach/internal/tool"
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

			// Resolve working directory.
			workDir := globalFlags.WorkDir
			if workDir == "" {
				workDir, _ = os.Getwd()
			}

			// Load configuration.
			cfg, err := config.Load(workDir)
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}

			// Model selection: CLI flag > config > default.
			model := cfg.Model
			if globalFlags.Model != "" {
				model = globalFlags.Model
			}

			// Create provider. For now, only Anthropic is fully implemented.
			prov := anthropicProvider.New("", "")

			// Build system prompt from discovered instructions.
			systemPrompt := cfg.Instructions
			if systemPrompt == "" {
				systemPrompt = "You are kvach, an AI coding agent. You have access to tools for reading files, writing files, executing shell commands, and searching code. Use them to help the user."
			}

			// Create the agent.
			a := agent.New(prov, tool.DefaultRegistry, nil, agent.Config{
				MaxTurns:     cfg.MaxTurns,
				WorkDir:      workDir,
				SystemPrompt: systemPrompt,
				Model:        model,
			})

			// Run and stream events to stdout.
			events, err := a.Run(ctx, agent.RunOptions{
				Prompt:    prompt,
				SessionID: runFlags.SessionID,
			})
			if err != nil {
				return fmt.Errorf("run agent: %w", err)
			}

			out := cmd.OutOrStdout()
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
