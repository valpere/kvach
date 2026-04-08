package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newModelsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "models",
		Short: "List available models from all configured providers",
		RunE: func(cmd *cobra.Command, args []string) error {
			// TODO(phase1): enumerate providers from registry and print models.
			fmt.Fprintln(cmd.OutOrStdout(), "Model listing not yet implemented.")
			return nil
		},
	}
}
