package commands

import (
	"strings"
	"testing"
)

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
