package commands

import (
	"github.com/spf13/cobra"
)

var (
	Exit = false
)
var RootCmd = &cobra.Command{
	Use:   "hyperbricks", // Set the correct command name
	Short: "Hyperbricks CLI",
	Long:  `Hyperbricks is a powerful headless cms for managing hypermedia.`,
}

// RegisterSubcommands adds all subcommands to the root command
func RegisterSubcommands() {
	// Add subcommands explicitly
	RootCmd.AddCommand(NewInitCommand())
	RootCmd.AddCommand(NewStartCommand())
	RootCmd.AddCommand(VersionCommand())
	RootCmd.AddCommand(NewSelectCommand())
	RootCmd.AddCommand(NewMakeStaticCommand())
	RootCmd.AddCommand(PluginCommand())

}

// Execute runs the root command
func Execute() error {
	return RootCmd.Execute()
}
