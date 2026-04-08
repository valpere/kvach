package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/valpere/kvach/internal/config"
	"github.com/valpere/kvach/internal/server"
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
			ctx := cmd.Context()

			workDir := globalFlags.WorkDir
			if workDir == "" {
				workDir, _ = os.Getwd()
			}

			cfg, err := config.Load(workDir)
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}

			serverCfg := cfg.Server
			if serveFlags.Host != "" {
				serverCfg.Host = serveFlags.Host
			}
			if serveFlags.Port != 0 {
				serverCfg.Port = serveFlags.Port
			}

			store, err := openSessionStore(ctx)
			if err != nil {
				return fmt.Errorf("open session store: %w", err)
			}
			defer store.Close()

			s := server.New(serverCfg, server.Options{WorkDir: workDir, SessionStore: store})
			fmt.Fprintf(cmd.OutOrStdout(), "Serving on %s\n", s.Addr())
			return s.ListenAndServe()
		},
	}

	cmd.Flags().StringVar(&serveFlags.Host, "host", "", "Bind address (default from config)")
	cmd.Flags().IntVar(&serveFlags.Port, "port", 0, "Listen port (default from config)")
	return cmd
}
