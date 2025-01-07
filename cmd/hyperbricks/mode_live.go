package main

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/hyperbricks/hyperbricks/pkg/logging"
)

func live_mode_init() {

	shutdownChan := make(chan struct{})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)

	// Use a WaitGroup to wait for the server to shut down
	var wg sync.WaitGroup
	wg.Add(1)

	// Start the server in a separate goroutine
	go func() {
		defer wg.Done()
		initialisation(ctx, cancel)

	}()
	keyboardActions()
	<-shutdownChan

	cancel()

	// Wait for the server to finish
	wg.Wait()
	fmt.Print("\033[H\033[2J")
	logging.GetLogger().Info("Application exited")

}
