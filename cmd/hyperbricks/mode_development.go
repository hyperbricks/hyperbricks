// main.go
package main

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/hyperbricks/hyperbricks/internal/database"
	"github.com/hyperbricks/hyperbricks/internal/shared"
	"github.com/hyperbricks/hyperbricks/pkg/logging"

	"go.uber.org/zap"
)

func development_mode_init() {

	hbConfig := getHyperBricksConfiguration()

	logging.GetInstance()
	logging.ChangeLevel(zap.InfoLevel)

	// if log directory is given add file log...
	if dir, exists := hbConfig.Directories["logs"]; exists && strings.TrimSpace(dir) != "" {
		logging.AddFileOutput(fmt.Sprintf("./%s/hyperbricks.log", dir))
		logging.GetLogger().Error("WORKS?")
		logging.GetLogger().Debug("WORKS?")
	} else {
		logging.GetLogger().Info("File logging disabled")
	}

	if hbConfig.Development.Watch {
		watchSourceDirectories()
	}

	if hbConfig.Development.Reload && KeyboardEnabled {
		logging.GetLogger().Info("Press 'r' to trigger an action or 'ESC' to exit.")
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
	hbConfig := getHyperBricksConfiguration()
	shutdownChan := make(chan struct{})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	// Use a WaitGroup to wait for the server to shut down
	var wg sync.WaitGroup
	wg.Add(1)

	// Start the server in a separate goroutine
	go func() {
		defer wg.Done()
		initialisation(ctx, cancel)

		// while we're at it, better to use shared.Location for logging the dashboad route (development mode only)
		if hbConfig.Development.Dashboard {
			logging.GetLogger().Info(fmt.Sprintf("dashboard: [http://%s/%s] initialized", shared.Location, "dashboard"))
		}
	}()
	keyboardActions()
	<-shutdownChan

	database.GetDB().Close()
	cancel()

	// Wait for the server to finish
	wg.Wait()
	fmt.Print("\033[H\033[2J")
	logging.GetLogger().Info("Application exited")

}
