package commands

import (
	"os"

	"github.com/spf13/cobra"
)

var (
	Exit           = false
	NonInteractive bool
)
var RootCmd = &cobra.Command{
	Use:   "hyperbricks", // Set the correct command name
	Short: "Hyperbricks CLI",
	Long:  `Hyperbricks is a powerful headless cms for managing hypermedia.`,
}

// RegisterSubcommands adds all subcommands to the root command
func RegisterSubcommands() {
	RootCmd.PersistentFlags().BoolVar(&NonInteractive, "non-interactive", false, "Disable keyboard input")
	RootCmd.PersistentPreRun = func(cmd *cobra.Command, args []string) {
		if NonInteractive {
			_ = os.Setenv("HB_NO_KEYBOARD", "1")
		}
	}

	// Add subcommands explicitly
	RootCmd.AddCommand(NewInitCommand())
	RootCmd.AddCommand(NewStartCommand())
	RootCmd.AddCommand(VersionCommand())
	RootCmd.AddCommand(NewSelectCommand())
	RootCmd.AddCommand(NewMakeStaticCommand())
	RootCmd.AddCommand(PluginCommand())
	RootCmd.AddCommand(NewBuildCommand())

}

// Execute runs the root command
func Execute() error {
	return RootCmd.Execute()
}
