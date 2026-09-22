package commands

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/hyperbricks/hyperbricks/pkg/logging"
)

// reportedError marks a structured result already written by an authoring command.
type reportedError struct{ error }

var terminalAccent = lipgloss.AdaptiveColor{Light: "#171717", Dark: "#38bdf8"}

func requireTerminal() error {
	if NonInteractive || os.Getenv("HB_NO_KEYBOARD") != "" || !logging.IsTerminal(os.Stdin) || !logging.IsTerminal(os.Stderr) {
		return errors.New("interactive selection requires a terminal; use explicit command options")
	}
	return nil
}

func ReportError(err error) {
	Exit, ExitCode = true, 1
	var reported reportedError
	if !errors.As(err, &reported) {
		logging.WriteError(RootCmd.ErrOrStderr(), err)
	}
}

func failf(format string, args ...interface{}) {
	message := strings.TrimSpace(fmt.Sprintf(format, args...))
	message = strings.TrimPrefix(strings.TrimPrefix(message, "Error: "), "Error ")
	ReportError(errors.New(message))
}
