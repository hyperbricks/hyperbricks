package main

import (
	"context"
	"fmt"

	"net/http"

	"github.com/hyperbricks/hyperbricks/internal/shared"
	"github.com/hyperbricks/hyperbricks/pkg/logging"
)

var server *http.Server

func StopServer(ctx context.Context) {
	logging.GetLogger().Infow("Shutting down the server gracefully...")
	if err := server.Shutdown(ctx); err != nil {
		logging.GetLogger().Fatalw("Server Shutdown Failed", "error", err)
	}
}

func StartServer(ctx context.Context) {
	hbConfig := getHyperBricksConfiguration()

	server = &http.Server{
		Addr: fmt.Sprintf(":%d", hbConfig.Server.Port),
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

		if hbConfig.Mode == shared.DEVELOPMENT_MODE {
			logging.GetLogger().Info(orangeTrueColor, fmt.Sprintf("Dashboard running at http://%s/dashboard", shared.Location), reset)
		}

		logging.GetLogger().Info(orangeTrueColor, fmt.Sprintf("Server is listening at http://%s", shared.Location), reset)

		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logging.GetLogger().Fatalw("Server failed to start", "error", err)
		}
	}()
}
