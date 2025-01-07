package commands

import (
	"fmt"

	"github.com/spf13/cobra"
)

// NewHelloCommand creates the "hello" subcommand
func NewHelloCommand() *cobra.Command {
	var name string

	cmd := &cobra.Command{
		Use:   "hello",
		Short: "Say hello",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("Hello, %s!\n", name)
		},
	}

	// Add flags
	cmd.Flags().StringVarP(&name, "name", "n", "World", "Name to greet")
	return cmd
}
