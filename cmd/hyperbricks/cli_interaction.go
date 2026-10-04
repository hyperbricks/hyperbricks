package main

import (
	"context"
	"os"

	"github.com/eiannone/keyboard"
	"github.com/hyperbricks/hyperbricks/cmd/hyperbricks/commands"
	"github.com/hyperbricks/hyperbricks/pkg/logging"
)

func keyboardActions(ctx context.Context, cancel context.CancelCauseFunc, reloads *runtimeReload) {
	if commands.NonInteractive || commands.Production || os.Getenv("HB_NO_KEYBOARD") != "" || !logging.IsTerminal(os.Stdin) {
		return
	}
	keys, err := keyboard.GetKeys(10)
	if err != nil {
		logging.GetLogger().Warnw("Keyboard unavailable; use Ctrl+C to stop", "error", err)
		return
	}
	defer keyboard.Close()
	logger := logging.GetLogger().Named("server")
	logger.Info("Press r to reload when watching; q, Esc or Ctrl+C to stop")
	for {
		select {
		case <-ctx.Done():
			return
		case event, ok := <-keys:
			if !ok || event.Err != nil {
				return
			}
			switch {
			case event.Key == keyboard.KeyCtrlC:
				cancel(errRuntimeInterrupt)
				return
			case event.Rune == 'q' || event.Rune == 'Q' || event.Key == keyboard.KeyEsc:
				cancel(context.Canceled)
				return
			case event.Rune == 'r' || event.Rune == 'R':
				if !reloads.Request() {
					logger.Info("Reload unavailable during startup/shutdown or when source watching is disabled")
				}
			}
		}
	}
}
