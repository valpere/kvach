package cli

import (
	"context"
	"fmt"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"github.com/valpere/kvach/internal/provider"
	anthropicProvider "github.com/valpere/kvach/internal/provider/anthropic"
	openaiProvider "github.com/valpere/kvach/internal/provider/openai"
)

func newModelsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "models",
		Short: "List available models from all configured providers",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			out := cmd.OutOrStdout()

			providers := []provider.Provider{
				anthropicProvider.New("", ""),
				openaiProvider.New(""),
			}

			w := tabwriter.NewWriter(out, 0, 4, 2, ' ', 0)
			fmt.Fprintf(w, "PROVIDER\tMODEL\tCONTEXT\tOUTPUT\tINPUT$/M\tOUTPUT$/M\tTOOLS\tVISION\n")

			for _, p := range providers {
				models, err := p.Models(ctx)
				if err != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "warning: %s: %v\n", p.Name(), err)
					continue
				}
				for _, m := range models {
					tools := "-"
					if m.Capabilities.ToolCalling {
						tools = "yes"
					}
					vision := "-"
					if m.Capabilities.Vision {
						vision = "yes"
					}
					fmt.Fprintf(w, "%s\t%s\t%dk\t%dk\t$%.2f\t$%.2f\t%s\t%s\n",
						p.Name(),
						m.ID,
						m.Limits.Context/1000,
						m.Limits.Output/1000,
						m.Cost.InputPerMToken,
						m.Cost.OutputPerMToken,
						tools,
						vision,
					)
				}
			}
			return w.Flush()
		},
	}
}
