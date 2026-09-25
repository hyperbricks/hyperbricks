package main

import (
	"context"
	"errors"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

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
		logging.GetLogger().Infof("Shutdown signal received (%s)", sig.String())
		cancel()
		<-ctx.Done()
	}
}

// Deploy servers share the CLI shutdown boundary so file logs are closed on exit.
func serveDeployHTTP(server *http.Server, mode string) error {
	listener, err := net.Listen("tcp", server.Addr)
	if err != nil {
		return err
	}
	defer server.Close()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	logger := logging.GetLogger().Named("deploy")
	logger.Infow("Listening", "url", "http://"+listener.Addr().String(), "mode", mode)
	done := make(chan error, 1)
	go func() { done <- server.Serve(listener) }()
	select {
	case err := <-done:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-ctx.Done():
		logger.Info("Stopping")
		shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdown); err != nil {
			return err
		}
		if err := <-done; err != nil && !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		logger.Info("Stopped")
		return nil
	}
}
