package commands

import (
	"fmt"

	assets "github.com/hyperbricks/hyperbricks"
	"github.com/spf13/cobra"
)

func VersionCommand() *cobra.Command {
	var version string

	cmd := &cobra.Command{
		Use:   "version",
		Short: "Show version",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println(assets.VersionMD)
		},
	}

	// Add flags
	cmd.Flags().StringVarP(&version, "version", "v", "Show version", "Show version")
	return cmd
}
