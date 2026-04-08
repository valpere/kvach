package cli

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/valpere/kvach/internal/tui"
)

func runTUI(cmd *cobra.Command) error {
	runtime, err := newAgentRuntime(cmd.Context())
	if err != nil {
		return err
	}
	defer runtime.store.Close()

	if err := tui.Run(cmd.Context(), tui.Config{
		Agent: runtime.agent,
		Model: runtime.fullModel,
		In:    cmd.InOrStdin(),
		Out:   cmd.OutOrStdout(),
	}); err != nil {
		return fmt.Errorf("run tui: %w", err)
	}

	return nil
}
