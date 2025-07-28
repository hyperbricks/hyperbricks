package commands

import (
	"fmt"

	"github.com/hyperbricks/hyperbricks/assets"
	"github.com/spf13/cobra"
)

var (
	Version bool = false
)

func VersionCommand() *cobra.Command {
	var version string

	cmd := &cobra.Command{
		Use:   "version",
		Short: "Show version",
		Run: func(cmd *cobra.Command, args []string) {
			Exit = true
			fmt.Println(assets.VersionMD)
		},
	}

	// Add flags
	cmd.Flags().StringVarP(&version, "version", "v", "Show version", "Show version")
	return cmd
}
