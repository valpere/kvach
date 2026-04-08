package cli

import (
	"context"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
	"github.com/valpere/kvach/internal/git"
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
				ctx := context.Background()
				store, err := openSessionStore(ctx)
				if err != nil {
					return err
				}
				defer store.Close()

				wd, _ := os.Getwd()
				if globalFlags.WorkDir != "" {
					wd = globalFlags.WorkDir
				}
				projectID := git.SlugFromRoot(wd)
				if root, err := git.Root(ctx, wd); err == nil {
					projectID = git.SlugFromRoot(root)
				}

				sessions, err := store.ListSessions(ctx, projectID)
				if err != nil {
					return err
				}
				if len(sessions) == 0 {
					fmt.Fprintln(cmd.OutOrStdout(), "No sessions found.")
					return nil
				}

				w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 4, 2, ' ', 0)
				fmt.Fprintln(w, "ID\tTITLE\tUPDATED\tDIRECTORY")
				for _, s := range sessions {
					fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", shortID(s.ID), s.Title, s.UpdatedAt.Format("2006-01-02 15:04"), s.Directory)
				}
				return w.Flush()
			},
		},
		&cobra.Command{
			Use:   "resume [session-id]",
			Short: "Resume a session by ID",
			Args:  cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				fmt.Fprintf(cmd.OutOrStdout(), "Use: kvach run --resume %s \"<prompt>\"\n", args[0])
				return nil
			},
		},
		&cobra.Command{
			Use:   "show [session-id]",
			Short: "Show the message history of a session",
			Args:  cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				ctx := context.Background()
				store, err := openSessionStore(ctx)
				if err != nil {
					return err
				}
				defer store.Close()

				sess, err := store.GetSession(ctx, args[0])
				if err != nil {
					return err
				}
				fmt.Fprintf(cmd.OutOrStdout(), "Session: %s\nTitle: %s\nDirectory: %s\nCreated: %s\nUpdated: %s\n\n",
					sess.ID,
					sess.Title,
					sess.Directory,
					sess.CreatedAt.Format(time.RFC3339),
					sess.UpdatedAt.Format(time.RFC3339),
				)

				msgs, err := store.GetMessages(ctx, sess.ID)
				if err != nil {
					return err
				}
				for _, m := range msgs {
					fmt.Fprintf(cmd.OutOrStdout(), "[%s] %s (%s)\n", m.CreatedAt.Format("15:04:05"), strings.ToUpper(m.Role), m.ID)
					parts, err := store.GetParts(ctx, m.ID)
					if err != nil {
						return err
					}
					for _, p := range parts {
						fmt.Fprintf(cmd.OutOrStdout(), "  - %s (%d bytes)\n", p.Type, len(p.Data))
					}
				}
				return nil
			},
		},
	)

	return cmd
}

func shortID(id string) string {
	if len(id) <= 8 {
		return id
	}
	return id[:8]
}
