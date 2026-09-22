package commands

import (
	"io"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestPluginInstallFlagsCoexistWithGlobalVerbose(t *testing.T) {
	previousRoot := RootCmd
	previousNonInteractive, previousVerbose := NonInteractive, Verbose
	previousLogLevel, previousLogFormat := LogLevel, LogFormat
	previousVersion, previousPath := RequestedHyperbricksVersion, RequestedHyperbricksPath
	t.Cleanup(func() {
		RootCmd = previousRoot
		NonInteractive, Verbose = previousNonInteractive, previousVerbose
		LogLevel, LogFormat = previousLogLevel, previousLogFormat
		RequestedHyperbricksVersion, RequestedHyperbricksPath = previousVersion, previousPath
	})

	RootCmd = &cobra.Command{Use: "hyperbricks"}
	RootCmd.SetOut(io.Discard)
	RootCmd.SetErr(io.Discard)
	RegisterSubcommands()
	RootCmd.SetArgs([]string{"plugin", "install", "--help"})
	if err := RootCmd.Execute(); err != nil {
		t.Fatalf("plugin install --help: %v", err)
	}

	install, _, err := RootCmd.Find([]string{"plugin", "install"})
	if err != nil {
		t.Fatalf("find plugin install: %v", err)
	}
	if err := install.ParseFlags([]string{"-v", "--hyperbricks-version", "v1.2.5-beta"}); err != nil {
		t.Fatalf("parse plugin install flags: %v", err)
	}
	if !Verbose {
		t.Fatal("-v did not enable global verbose logging")
	}
	if RequestedHyperbricksVersion != "v1.2.5-beta" {
		t.Fatalf("hyperbricks version = %q, want v1.2.5-beta", RequestedHyperbricksVersion)
	}
}

func TestPluginUpdateCommandReturnsNotImplemented(t *testing.T) {
	previousExit, previousExitCode := Exit, ExitCode
	t.Cleanup(func() {
		Exit, ExitCode = previousExit, previousExitCode
	})

	Exit = false
	ExitCode = 0
	command := PluginUpdateCommand()
	command.SilenceUsage = true
	command.SetArgs([]string{"example"})

	err := command.Execute()
	if err == nil {
		t.Fatal("plugin update succeeded, want a not implemented error")
	}
	if !strings.Contains(err.Error(), "plugin update is not implemented") {
		t.Fatalf("plugin update error = %q, want a clear not implemented error", err)
	}
	if !Exit || ExitCode != 1 {
		t.Fatalf("exit state = (%t, %d), want failed command exit", Exit, ExitCode)
	}
}
