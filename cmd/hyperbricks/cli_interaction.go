package main

import (
	"context"
	"os"

	"github.com/eiannone/keyboard"
	"github.com/hyperbricks/hyperbricks/cmd/hyperbricks/commands"
	"github.com/hyperbricks/hyperbricks/pkg/logging"
)

var (
	KeyboardEnabled bool = false
)

func keyboardActions(cancel context.CancelFunc) {
	if os.Getenv("HB_NO_KEYBOARD") != "" || !logging.IsTerminal(os.Stdin) {
		return
	}

	// --production flag
	if commands.Production {
		return
	}

	hbConfig := getHyperBricksConfiguration()

	if hbConfig.Mode == "" {
		return
	}

	// test and open keyboard
	if err := keyboard.Open(); err != nil {
		logging.GetLogger().Warnw("Keyboard unavailable; use Ctrl+C to stop", "error", err)
		return
	} else {
		KeyboardEnabled = true
	}

	defer func() {
		if err := keyboard.Close(); err != nil {
			logging.GetLogger().Warnw("Failed to close keyboard", "error", err)
		}
	}()

	// Channel to signal when "r" is pressed
	rPressed := make(chan bool)
	// Channel to handle program termination (e.g., on ESC key)
	done := make(chan bool)
	// Channel to disable keyboard handling when input is unavailable
	disabled := make(chan bool)

	// Goroutine to listen for key presses
	go func() {
		for {
			char, key, err := keyboard.GetKey()
			if err != nil {
				logging.GetLogger().Warnw("Keyboard input unavailable", "error", err)
				disabled <- true
				return
			}

			// Check if "r" or "R" is pressed
			if char == 'r' || char == 'R' {
				rPressed <- true
			}

			// Optional: Exit on q - Q - ESC key and KeyCtrlC
			if char == 'q' || char == 'Q' || key == keyboard.KeyEsc || key == keyboard.KeyCtrlC {
				done <- true
				return
			}
		}
	}()
	hint := "Press q, Esc or Ctrl+C to stop"
	if hbConfig.Development.Watch {
		hint = "Press r to reload; q, Esc or Ctrl+C to stop"
	}
	logging.GetLogger().Named("server").Info(hint)
	// Main loop to handle events
	for {
		select {
		case <-rPressed:
			if hbConfig.Development.Watch {
				logging.GetLogger().Info("Reloading configuration")
				PreProcessAndPopulateHyperbricksConfigurations()
			}
		// Place your action here
		case <-disabled:
			KeyboardEnabled = false
			return
		case <-done:
			cancel()
			return
		}
	}
}
