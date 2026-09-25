// main.go
package main

import (
	"context"
	"sync"

	"github.com/hyperbricks/hyperbricks/pkg/logging"
)

func development_mode_init() {

	hbConfig := getHyperBricksConfiguration()

	if hbConfig.Development.Watch {
		watchSourceDirectories()
	}

	var wg sync.WaitGroup
	wg.Add(1)

	// Start the server in a separate goroutine
	go func() {
		defer wg.Done()
		statusServer()

	}()

	development_mode()

}

func development_mode() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	// Use a WaitGroup to wait for the server to shut down
	var wg sync.WaitGroup
	wg.Add(1)

	// Start the server in a separate goroutine
	go func() {
		defer wg.Done()
		initialisation(ctx)

	}()
	waitForShutdown(ctx, cancel)

	// Wait for the server to finish
	wg.Wait()
	logging.GetLogger().Named("server").Info("Stopped")

}
