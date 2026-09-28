package commands

import (
	"os"

	"github.com/hyperbricks/hyperbricks/pkg/logging"

	"github.com/spf13/cobra"
)

var (
	Exit           = false
	ExitCode       = 0
	NonInteractive bool
	Verbose        bool
	LogLevel       string
	LogFormat      string
)
var RootCmd = &cobra.Command{
	Use:           "hyperbricks", // Set the correct command name
	Short:         "HyperBricks CLI",
	Long:          `HyperBricks is a Web App Build System for managing hypermedia.`,
	SilenceErrors: true,
	SilenceUsage:  true,
}

// RegisterSubcommands adds all subcommands to the root command
func RegisterSubcommands() {
	RootCmd.PersistentFlags().BoolVar(&NonInteractive, "non-interactive", false, "Disable keyboard input")
	RootCmd.PersistentFlags().BoolVarP(&Verbose, "verbose", "v", false, "Include detailed DEBUG logs (explicit --log-level takes precedence)")
	RootCmd.PersistentFlags().StringVar(&LogLevel, "log-level", "", "Log level (overrides package logger.level)")
	RootCmd.PersistentFlags().StringVar(&LogFormat, "log-format", "", "Log format: console or json (overrides package logger.format)")
	RootCmd.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		if NonInteractive {
			_ = os.Setenv("HB_NO_KEYBOARD", "1")
		}
		level, format := EffectiveLogLevel("info"), LogFormat
		if format == "" {
			format = "console"
		}
		if err := logging.Configure(level, format); err != nil {
			return cmd.FlagErrorFunc()(cmd, err)
		}
		return nil
	}

	// Add subcommands explicitly
	RootCmd.AddCommand(NewInitCommand())
	RootCmd.AddCommand(NewScaffoldCommand())
	RootCmd.AddCommand(NewAuthorCommand())
	RootCmd.AddCommand(NewSpaceCommand())
	RootCmd.AddCommand(NewDoctorCommand())
	RootCmd.AddCommand(NewLanguageServerCommand())
	RootCmd.AddCommand(NewStartCommand())
	RootCmd.AddCommand(NewDeployCommand())
	RootCmd.AddCommand(VersionCommand())
	RootCmd.AddCommand(NewSelectCommand())
	RootCmd.AddCommand(NewMakeStaticCommand())
	RootCmd.AddCommand(PluginCommand())
	RootCmd.AddCommand(InitStarterCommand())
	RootCmd.AddCommand(NewBuildCommand())

}

// EffectiveLogLevel applies CLI choices after the package/mode default.
func EffectiveLogLevel(configured string) string {
	if LogLevel != "" {
		return LogLevel
	}
	if Verbose {
		return "debug"
	}
	return configured
}

// Execute runs the root command
func Execute() error {
	_, err := RootCmd.ExecuteC()
	if err != nil {
		Exit, ExitCode = true, 1
	}
	return err
}
