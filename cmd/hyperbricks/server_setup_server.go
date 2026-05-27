package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/hyperbricks/hyperbricks/pkg/logging"
	"github.com/hyperbricks/hyperbricks/pkg/shared"
)

// Global server instance
var (
	server   *http.Server
	serverMu sync.Mutex
)

func currentServer() *http.Server {
	serverMu.Lock()
	defer serverMu.Unlock()
	return server
}

func publishServer(activeServer *http.Server) {
	serverMu.Lock()
	server = activeServer
	serverMu.Unlock()
}

func clearServer(activeServer *http.Server) {
	serverMu.Lock()
	if server == activeServer {
		server = nil
	}
	serverMu.Unlock()
}

// StopServer gracefully shuts down the HTTP server.
func StopServer(ctx context.Context) error {
	activeServer := currentServer()
	if activeServer == nil {
		return nil
	}

	logging.GetLogger().Infow("Shutting down the server gracefully...")
	if err := activeServer.Shutdown(ctx); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

// StartServer initializes and starts the HTTP server based on the selected mode.
func StartServer(ctx context.Context) {
	hbConfig := getHyperBricksConfiguration()
	if err := validateRuntimeGatewayConfig(hbConfig.Server.RuntimeGateway); err != nil {
		log.Fatal("Invalid runtime gateway config:", err)
	}

	var listener net.Listener
	var err error

	// Configure a custom TCP listener for high concurrency
	listener, err = net.Listen("tcp", fmt.Sprintf(":%d", hbConfig.Server.Port))
	if err != nil {
		log.Fatal("Failed to start listener:", err)
	}

	activeServer := &http.Server{
		Addr:         fmt.Sprintf(":%d", hbConfig.Server.Port),
		ReadTimeout:  hbConfig.Server.ReadTimeout,
		WriteTimeout: hbConfig.Server.WriteTimeout,
		IdleTimeout:  hbConfig.Server.IdleTimeout,
	}
	if hbConfig.Mode == shared.LIVE_MODE {
		// Keep the smaller live-mode header limit, but honor configured transport settings.
		activeServer.MaxHeaderBytes = 65536
	}
	activeServer.SetKeepAlivesEnabled(hbConfig.Server.KeepAlivesEnabled)
	publishServer(activeServer)

	go func() {
		<-ctx.Done()

		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := StopServer(shutdownCtx); err != nil {
			logging.GetLogger().Errorw("Server Shutdown Failed", "error", err)
		}
	}()

	// ANSI escape code for green text
	green := "\033[32m"
	// ANSI escape code to reset the text color
	reset := "\033[0m"

	log.Printf("%s Server running in %s mode at http://%s", green+"●"+reset, hbConfig.Mode, shared.Location)
	if os.Getenv("HB_NO_KEYBOARD") == "" {
		log.Printf("Press 'q', ESC or Ctrl+C to stop the server...")
	}

	if err := activeServer.Serve(listener); err != nil && err != http.ErrServerClosed {
		log.Fatal("Server error:", err)
	}

	clearServer(activeServer)
}
