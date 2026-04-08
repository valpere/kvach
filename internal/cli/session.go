package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newSessionCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "session",
		Short: "Manage agent sessions",
	}

	cmd.AddCommand(
		&cobra.Command{
			Use:   "list",
			Short: "List all sessions for the current project",
			RunE: func(cmd *cobra.Command, args []string) error {
				// TODO(phase2): load from session.Store and print.
				fmt.Fprintln(cmd.OutOrStdout(), "No sessions found (not yet implemented)")
				return nil
			},
		},
		&cobra.Command{
			Use:   "resume [session-id]",
			Short: "Resume a session by ID",
			Args:  cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				// TODO(phase2): resume session via TUI.
				fmt.Fprintf(cmd.OutOrStdout(), "Resuming session %s (not yet implemented)\n", args[0])
				return nil
			},
		},
		&cobra.Command{
			Use:   "show [session-id]",
			Short: "Show the message history of a session",
			Args:  cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				// TODO(phase2): load messages from session.Store and print.
				fmt.Fprintf(cmd.OutOrStdout(), "Session %s (not yet implemented)\n", args[0])
				return nil
			},
		},
	)

	return cmd
}
