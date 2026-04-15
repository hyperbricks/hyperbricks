// main.go
package main

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/hyperbricks/hyperbricks/pkg/logging"

	"go.uber.org/zap"
)

func debug_mode_init() {

	logging.GetInstance()
	logging.ChangeLevel(zap.InfoLevel)

	// if log directory is given add file log...
	hbConfig := getHyperBricksConfiguration()
	if dir, exists := hbConfig.Directories["logs"]; exists && strings.TrimSpace(dir) != "" {
		logging.AddFileOutput(fmt.Sprintf("./%s/hyperbricks.log", dir))
	} else {
		logging.GetLogger().Info("File logging disabled")
	}

	var wg sync.WaitGroup
	wg.Add(1)

	// Start the server in a separate goroutine
	go func() {
		defer wg.Done()
		statusServer()

	}()

	debug_mode()

}

func debug_mode() {
	// Initialize the logger
	logging.GetLogger().Debug("Application started in debug mode...")

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
	fmt.Print("\033[H\033[2J")
	logging.GetLogger().Info("Application exited")

}
