package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/hyperbricks/hyperbricks/pkg/logging"
)

func waitForShutdown(ctx context.Context, cancel context.CancelFunc) {
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(signals)

	go keyboardActions(cancel)

	select {
	case <-ctx.Done():
	case sig := <-signals:
		logging.GetLogger().Infow("Shutdown signal received", "signal", sig.String())
		cancel()
		<-ctx.Done()
	}
}
