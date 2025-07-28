package commands

import (
	"github.com/spf13/cobra"
)

var (
	RenderStatic bool
)

func NewMakeStaticCommand() *cobra.Command {
	var static bool

	cmd := &cobra.Command{
		Use:   "static",
		Short: "Render static content",
		Run: func(cmd *cobra.Command, args []string) {
			RenderStatic = true
		},
	}

	// Add flags
	cmd.Flags().BoolVarP(&static, "static", "s", true, "Render static content")
	cmd.Flags().StringVarP(&StartModule, "module", "m", "default", "module in the ./modules directory")

	return cmd
}
