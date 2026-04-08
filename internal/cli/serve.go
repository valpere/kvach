package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newServeCmd() *cobra.Command {
	var serveFlags struct {
		Host string
		Port int
	}

	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Start the HTTP API server",
		Long:  `Start the kvach HTTP API server, exposing agent sessions via REST and SSE.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// TODO(phase3): initialise and start server.Server.
			fmt.Fprintf(cmd.OutOrStdout(), "Serving on %s:%d (not yet implemented)\n",
				serveFlags.Host, serveFlags.Port)
			return nil
		},
	}

	cmd.Flags().StringVar(&serveFlags.Host, "host", "127.0.0.1", "Bind address")
	cmd.Flags().IntVar(&serveFlags.Port, "port", 7777, "Listen port")
	return cmd
}
