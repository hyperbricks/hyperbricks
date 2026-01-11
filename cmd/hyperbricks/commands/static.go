package commands

import (
	"github.com/spf13/cobra"
)

var (
	RenderStatic bool
	ServeStatic  bool
	ForceStatic  bool
)

func NewMakeStaticCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "static",
		Short: "Render static content",
		Run: func(cmd *cobra.Command, args []string) {
			RenderStatic = true
		},
	}

	// Add flags
	cmd.Flags().BoolVar(&ServeStatic, "serve", false, "Serve rendered static files")
	cmd.Flags().BoolVar(&ForceStatic, "force", false, "Overwrite rendered output without confirmation")
	cmd.Flags().StringVarP(&StartModule, "module", "m", "default", "module in the ./modules directory")

	return cmd
}
