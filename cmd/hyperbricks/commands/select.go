// commands/select.go
package commands

import (
	"fmt"

	"github.com/spf13/cobra"
)

// NewSelectCommand creates the "select" subcommand
func NewSelectCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "select",
		Short: "Select a hyperbricks module",
		Run: func(cmd *cobra.Command, args []string) {
			selected, ok, err := RunModulePicker("Select a module")
			if err != nil {
				fmt.Printf("Error selecting module: %v\n", err)
				return
			}
			if !ok {
				return
			}

			startCmd := NewStartCommand()
			startCmd.Flags().Set("module", selected)
			if err := startCmd.Execute(); err != nil {
				fmt.Printf("Error executing start command: %v\n", err)
			}
		},
	}
	return cmd
}
