package cli

import (
	"os"

	"github.com/spf13/cobra"
)

// globalFlags holds flag values bound at the root level and accessible to all
// sub-commands.
var globalFlags struct {
	// WorkDir overrides the working directory for the session.
	WorkDir string
	// Model overrides the default model (e.g. "anthropic/claude-sonnet-4-5").
	Model string
	// Debug enables verbose debug output.
	Debug bool
}

// rootCmd is the cobra root command. Running `kvach` without a sub-command
// launches the interactive TUI.
var rootCmd = &cobra.Command{
	Use:   "kvach",
	Short: "An AI coding agent",
	Long: `kvach is an AI coding agent that helps you write, edit, and understand code.

Run without arguments to open the interactive TUI.
Use sub-commands for scripted or headless operation.`,
	// SilenceUsage prevents cobra from printing usage on every error.
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runTUI(cmd)
	},
}

func init() {
	pf := rootCmd.PersistentFlags()
	pf.StringVarP(&globalFlags.WorkDir, "dir", "C", "", "Working directory (default: current directory)")
	pf.StringVarP(&globalFlags.Model, "model", "m", "", "Model to use, e.g. anthropic/claude-sonnet-4-5")
	pf.BoolVar(&globalFlags.Debug, "debug", false, "Enable debug logging")

	rootCmd.AddCommand(
		newRunCmd(),
		newServeCmd(),
		newSessionCmd(),
		newModelsCmd(),
	)
}

// Execute is the entry point called from main.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
