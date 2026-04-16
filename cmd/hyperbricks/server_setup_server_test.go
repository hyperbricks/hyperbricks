package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
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

func reserveTCPPort(t *testing.T) int {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to reserve test port: %v", err)
	}
	defer listener.Close()

	return listener.Addr().(*net.TCPAddr).Port
}

func waitForServerReady(t *testing.T, address string) {
	t.Helper()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", address, 50*time.Millisecond)
		if err == nil {
			conn.Close()
			return
		}
		time.Sleep(10 * time.Millisecond)
	}

	t.Fatalf("server was not reachable at %s before timeout", address)
}

func sendRawHTTPGet(t *testing.T, conn net.Conn, reader *bufio.Reader, host string) *http.Response {
	t.Helper()

	if _, err := fmt.Fprintf(conn, "GET / HTTP/1.1\r\nHost: %s\r\n\r\n", host); err != nil {
		t.Fatalf("failed to write request: %v", err)
	}

	resp, err := http.ReadResponse(reader, &http.Request{
		Method: http.MethodGet,
	})
	if err != nil {
		t.Fatalf("failed to read response: %v", err)
	}

	return resp
}

func startTransportTestServer(t *testing.T, configure func(*shared.Config), mux *http.ServeMux) (string, *http.Server, func()) {
	t.Helper()

	shared.Init_configuration()
	hbConfig := shared.GetHyperBricksConfiguration()

	oldMode := hbConfig.Mode
	oldPort := hbConfig.Server.Port
	oldReadTimeout := hbConfig.Server.ReadTimeout
	oldWriteTimeout := hbConfig.Server.WriteTimeout
	oldIdleTimeout := hbConfig.Server.IdleTimeout
	oldKeepAlivesEnabled := hbConfig.Server.KeepAlivesEnabled
	oldMux := http.DefaultServeMux

	serverMu.Lock()
	oldServer := server
	server = nil
	serverMu.Unlock()

	http.DefaultServeMux = mux

	hbConfig.Mode = shared.LIVE_MODE
	hbConfig.Server.Port = reserveTCPPort(t)
	configure(hbConfig)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})

	go func() {
		StartServer(ctx)
		close(done)
	}()

	waitForServerState(t, true)

	serverMu.Lock()
	activeServer := server
	serverMu.Unlock()
	if activeServer == nil {
		t.Fatal("expected active server to be set")
	}

	address := fmt.Sprintf("127.0.0.1:%d", hbConfig.Server.Port)
	waitForServerReady(t, address)

	cleanup := func() {
		cancel()

		select {
		case <-done:
		case <-time.After(3 * time.Second):
			t.Fatal("StartServer did not return after context cancellation")
		}

		waitForServerState(t, false)

		hbConfig.Mode = oldMode
		hbConfig.Server.Port = oldPort
		hbConfig.Server.ReadTimeout = oldReadTimeout
		hbConfig.Server.WriteTimeout = oldWriteTimeout
		hbConfig.Server.IdleTimeout = oldIdleTimeout
		hbConfig.Server.KeepAlivesEnabled = oldKeepAlivesEnabled
		http.DefaultServeMux = oldMux

		serverMu.Lock()
		server = oldServer
		serverMu.Unlock()
	}

	return address, activeServer, cleanup
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

func TestStartServer_LiveMode_HonorsConfiguredTimeoutsAndKeepAlives(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "ok")
	})

	wantReadTimeout := 7 * time.Second
	wantWriteTimeout := 11 * time.Second
	wantIdleTimeout := 13 * time.Second

	address, activeServer, cleanup := startTransportTestServer(t, func(hbConfig *shared.Config) {
		hbConfig.Server.ReadTimeout = wantReadTimeout
		hbConfig.Server.WriteTimeout = wantWriteTimeout
		hbConfig.Server.IdleTimeout = wantIdleTimeout
		hbConfig.Server.KeepAlivesEnabled = true
	}, mux)
	defer cleanup()

	if activeServer.ReadTimeout != wantReadTimeout {
		t.Fatalf("live mode read timeout = %v, want %v", activeServer.ReadTimeout, wantReadTimeout)
	}
	if activeServer.WriteTimeout != wantWriteTimeout {
		t.Fatalf("live mode write timeout = %v, want %v", activeServer.WriteTimeout, wantWriteTimeout)
	}
	if activeServer.IdleTimeout != wantIdleTimeout {
		t.Fatalf("live mode idle timeout = %v, want %v", activeServer.IdleTimeout, wantIdleTimeout)
	}

	conn, err := net.Dial("tcp", address)
	if err != nil {
		t.Fatalf("failed to dial test server: %v", err)
	}
	defer conn.Close()

	reader := bufio.NewReader(conn)

	firstResponse := sendRawHTTPGet(t, conn, reader, address)
	if firstResponse.Close {
		t.Fatalf("expected keep-alives to stay enabled in live mode")
	}
	if _, err := io.ReadAll(firstResponse.Body); err != nil {
		t.Fatalf("failed to read first response body: %v", err)
	}
	firstResponse.Body.Close()

	secondResponse := sendRawHTTPGet(t, conn, reader, address)
	if secondResponse.Close {
		t.Fatalf("expected reused connection to remain open in live mode")
	}
	if _, err := io.ReadAll(secondResponse.Body); err != nil {
		t.Fatalf("failed to read second response body: %v", err)
	}
	secondResponse.Body.Close()
}

func TestStartServer_LiveMode_DisablesKeepAlivesWhenConfigured(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "ok")
	})

	address, _, cleanup := startTransportTestServer(t, func(hbConfig *shared.Config) {
		hbConfig.Server.KeepAlivesEnabled = false
	}, mux)
	defer cleanup()

	conn, err := net.Dial("tcp", address)
	if err != nil {
		t.Fatalf("failed to dial test server: %v", err)
	}
	defer conn.Close()

	reader := bufio.NewReader(conn)
	response := sendRawHTTPGet(t, conn, reader, address)
	if !response.Close {
		t.Fatalf("expected keep-alives to be disabled when configured")
	}
	if _, err := io.ReadAll(response.Body); err != nil {
		t.Fatalf("failed to read response body: %v", err)
	}
	response.Body.Close()
}
