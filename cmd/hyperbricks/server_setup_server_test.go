package main

import (
	"context"
	"testing"
	"time"

	"github.com/hyperbricks/hyperbricks/pkg/shared"
)

func waitForServerState(t *testing.T, wantActive bool) {
	t.Helper()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		serverMu.Lock()
		active := server != nil
		serverMu.Unlock()

		if active == wantActive {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}

	t.Fatalf("server active state did not reach %v before timeout", wantActive)
}

func TestStartServer_ShutsDownOnContextCancel(t *testing.T) {
	shared.Init_configuration()
	hbConfig := shared.GetHyperBricksConfiguration()

	oldMode := hbConfig.Mode
	oldPort := hbConfig.Server.Port

	serverMu.Lock()
	oldServer := server
	server = nil
	serverMu.Unlock()

	hbConfig.Mode = shared.DEBUG_MODE
	hbConfig.Server.Port = 0

	t.Cleanup(func() {
		hbConfig.Mode = oldMode
		hbConfig.Server.Port = oldPort

		serverMu.Lock()
		server = oldServer
		serverMu.Unlock()
	})

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})

	go func() {
		StartServer(ctx)
		close(done)
	}()

	waitForServerState(t, true)

	cancel()

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("StartServer did not return after context cancellation")
	}

	waitForServerState(t, false)
}
