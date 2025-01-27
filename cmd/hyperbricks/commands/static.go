package commands

import (
	"github.com/spf13/cobra"
)

var (
	RenderStatic bool
)

// NewHelloCommand creates the "hello" subcommand
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
	return cmd
}
