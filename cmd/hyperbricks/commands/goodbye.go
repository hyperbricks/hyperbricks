package commands

import (
	"fmt"

	"github.com/spf13/cobra"
)

// NewGoodbyeCommand creates the "goodbye" subcommand
func NewGoodbyeCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "goodbye",
		Short: "Say goodbye",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("Goodbye, world!")

		},
	}
}
