package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
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

	logging.GetLogger().Named("server").Info("Stopping")
	if err := activeServer.Shutdown(ctx); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

// runtimeHTTPServer retains the serving result until the lifecycle owner has
// drained requests. Closing Serve is not the same as finishing Shutdown.
type runtimeHTTPServer struct {
	server *http.Server
	done   chan struct{}
	err    error // written before closing done
}

func startRuntimeHTTPServer() (*runtimeHTTPServer, error) {
	hbConfig := getHyperBricksConfiguration()
	if err := validateRuntimeGatewayConfig(hbConfig.Server.RuntimeGateway); err != nil {
		return nil, fmt.Errorf("invalid runtime gateway configuration: %w", err)
	}
	listener, err := listenRuntimeHTTP(hbConfig.Server.Port)
	if err != nil {
		return nil, err
	}
	activeServer := &http.Server{
		Addr:         fmt.Sprintf(":%d", hbConfig.Server.Port),
		ReadTimeout:  hbConfig.Server.ReadTimeout,
		WriteTimeout: hbConfig.Server.WriteTimeout,
		IdleTimeout:  hbConfig.Server.IdleTimeout,
	}
	if hbConfig.Mode == shared.LIVE_MODE {
		activeServer.MaxHeaderBytes = 65536
	}
	activeServer.SetKeepAlivesEnabled(hbConfig.Server.KeepAlivesEnabled)
	publishServer(activeServer)
	running := &runtimeHTTPServer{server: activeServer, done: make(chan struct{})}
	go func() {
		running.err = activeServer.Serve(listener)
		close(running.done)
	}()
	logging.GetLogger().Named("server").Infow("Listening", "url", "http://"+shared.Location, "mode", hbConfig.Mode)
	return running, nil
}

func listenRuntimeHTTP(port int) (net.Listener, error) {
	// On macOS, the reusable wildcard socket may bind successfully while a
	// loopback listener still receives requests on this port. Check active
	// loopback listeners before advertising the server or running after_start.
	// Keep normal socket reuse so restarting after a request still works.
	// This check complements bind; it does not atomically reserve the port.
	if port > 0 {
		for _, host := range []string{"127.0.0.1", "::1"} {
			address := net.JoinHostPort(host, fmt.Sprint(port))
			connection, err := net.DialTimeout("tcp", address, 200*time.Millisecond)
			if err == nil {
				connection.Close()
				return nil, fmt.Errorf("start listener on port %d: address %s is already occupied; choose another --port or stop the existing server", port, address)
			}
		}
	}
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return nil, fmt.Errorf("start listener on port %d: %w", port, err)
	}
	return listener, nil
}

func (running *runtimeHTTPServer) shutdown(ctx context.Context) error {
	defer clearServer(running.server)
	err := running.server.Shutdown(ctx)
	if err != nil {
		// Shutdown does not close active connections on deadline expiry.
		err = errors.Join(err, running.server.Close())
	}
	<-running.done
	if running.err != nil && !errors.Is(running.err, http.ErrServerClosed) {
		err = errors.Join(err, running.err)
	}
	return err
}

// StartServer is also used by transport tests. Runtime startup uses the same
// server handle so after_start and service cleanup share one lifetime owner.
func StartServer(ctx context.Context) error {
	running, err := startRuntimeHTTPServer()
	if err != nil {
		return err
	}
	select {
	case <-ctx.Done():
	case <-running.done:
	}
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return running.shutdown(shutdownCtx)
}
