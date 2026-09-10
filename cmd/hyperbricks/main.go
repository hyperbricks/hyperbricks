package main

import (
	"os"

	"github.com/hyperbricks/hyperbricks/cmd/hyperbricks/commands"
)

func main() {
	if commands.ExitCode != 0 {
		os.Exit(commands.ExitCode)
	}
}
