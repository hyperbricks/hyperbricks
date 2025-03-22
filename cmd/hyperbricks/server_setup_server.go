package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"os/exec"
	"runtime"
	"time"

	"github.com/hyperbricks/hyperbricks/internal/shared"
	"github.com/hyperbricks/hyperbricks/pkg/logging"
)

// Global server instance
var server *http.Server

// StopServer gracefully shuts down the HTTP server.
func StopServer(ctx context.Context) {
	logging.GetLogger().Infow("Shutting down the server gracefully...")
	if err := server.Shutdown(ctx); err != nil {
		logging.GetLogger().Fatalw("Server Shutdown Failed", "error", err)
	}
}

// StartServer initializes and starts the HTTP server based on the selected mode.
func StartServer(ctx context.Context) {
	hbConfig := getHyperBricksConfiguration()

	var server *http.Server
	var listener net.Listener
	var err error

	// Configure a custom TCP listener for high concurrency
	listener, err = net.Listen("tcp", fmt.Sprintf(":%d", hbConfig.Server.Port))
	if err != nil {
		log.Fatal("Failed to start listener:", err)
	}

	switch hbConfig.Mode {
	case shared.LIVE_MODE:
		// High-performance settings for production (cache enabled)
		server = &http.Server{
			Addr:           fmt.Sprintf(":%d", hbConfig.Server.Port),
			ReadTimeout:    0,                // No read timeout (cached responses)
			WriteTimeout:   0,                // No write timeout (fast processing)
			IdleTimeout:    60 * time.Second, // Keep connections alive for efficiency
			MaxHeaderBytes: 65536,            // 64KB headers for high-throughput requests
		}

		// Ensure we don’t keep too many idle connections
		server.SetKeepAlivesEnabled(false)

	case shared.DEVELOPMENT_MODE, shared.DEBUG_MODE:
		// More relaxed settings for development
		server = &http.Server{
			Addr:         fmt.Sprintf(":%d", hbConfig.Server.Port),
			ReadTimeout:  hbConfig.Server.ReadTimeout,
			WriteTimeout: hbConfig.Server.WriteTimeout,
			IdleTimeout:  hbConfig.Server.IdleTimeout,
		}
	}

	// Run server in a separate Goroutine
	go func() {
		// ANSI escape code for green text
		green := "\033[32m"
		// ANSI escape code to reset the text color
		reset := "\033[0m"

		// Print a green dot using a Unicode bullet character

		log.Printf("%s Server running in %s mode at http://%s", green+"●"+reset, hbConfig.Mode, shared.Location)

		// Start the HTTP server
		if err := server.Serve(listener); err != nil && err != http.ErrServerClosed {
			log.Fatal("Server error:", err)
		}
	}()
}

// StartServer initializes and runs the HTTP server.
func StartServerV1(ctx context.Context) {
	hbConfig := getHyperBricksConfiguration()

	switch hbConfig.Mode {

	case shared.LIVE_MODE:
		// Initialize the server
		server = &http.Server{
			Addr:           fmt.Sprintf(":%d", hbConfig.Server.Port),
			ReadTimeout:    0, // Disable read timeout since it's cached
			WriteTimeout:   0, // No delay in writing responses
			IdleTimeout:    60 * time.Second,
			MaxHeaderBytes: 65536, // 64KB header limit
		}
	case shared.DEVELOPMENT_MODE, shared.DEBUG_MODE:
		// Initialize the server
		server = &http.Server{
			Addr: fmt.Sprintf(":%d", hbConfig.Server.Port),

			ReadTimeout:  hbConfig.Server.ReadTimeout,
			WriteTimeout: hbConfig.Server.WriteTimeout,
			IdleTimeout:  hbConfig.Server.IdleTimeout,
		}
	}

	go func() {
		ips, err := getHostIPv4s()
		if err != nil {
			logging.GetLogger().Errorw("Error retrieving host IPs", "error", err)
			return
		}
		if len(ips) == 0 {
			logging.GetLogger().Errorw("No IPv4 addresses found for the host")
			return
		}

		shared.Location = fmt.Sprintf("%s:%d", ips[0], hbConfig.Server.Port)
		orangeTrueColor := "\033[38;2;255;165;0m"
		reset := "\033[0m"

		// Open dashboard if in development mode
		if hbConfig.Mode == shared.DEVELOPMENT_MODE && hbConfig.Development.Dashboard {
			logging.GetLogger().Info(orangeTrueColor, fmt.Sprintf("Dashboard running at http://%s/dashboard", shared.Location), reset)
			url := fmt.Sprintf("http://%s/dashboard", shared.Location)
			err := openBrowser(url)
			if err != nil {
				fmt.Println("Error opening browser:", err)
			}
		}

		logging.GetLogger().Info(orangeTrueColor, fmt.Sprintf("Server is listening at http://%s", shared.Location), reset)

		// Start the server
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logging.GetLogger().Fatalw("Server failed to start", "error", err)
		}
	}()
}

// openBrowser opens the dashboard URL in a browser.
func openBrowser(url string) error {
	var cmd string
	var args []string

	switch runtime.GOOS {
	case "windows":
		cmd = "rundll32"
		args = []string{"url.dll,FileProtocolHandler", url}
	case "darwin":
		cmd = "open"
		args = []string{url}
	default: // Linux, BSD, etc.
		cmd = "xdg-open"
		args = []string{url}
	}

	return exec.Command(cmd, args...).Start()
}
