package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/hyperbricks/hyperbricks/pkg/shared"
)

func TestBuildModeWaitsForPendingStartup(t *testing.T) {
	for _, scope := range []string{"local", "remote"} {
		t.Run(scope, func(t *testing.T) {
			fixture := newDeployStartupFixture(t, false)
			marker := filepath.Join(t.TempDir(), "starting")
			gate := filepath.Join(t.TempDir(), "ready")
			t.Setenv("HB_DEPLOY_STARTUP_TEST_HELPER", "listen")
			t.Setenv("HB_DEPLOY_STARTUP_TEST_MARKER", marker)
			t.Setenv("HB_DEPLOY_STARTUP_TEST_GATE", gate)
			started := make(chan error, 1)
			go func() {
				if scope == "local" {
					started <- fixture.local.startLocalBuild("demo", "build-1")
				} else {
					started <- fixture.remote.startManaged("demo", "build-1")
				}
			}()
			pid := waitDeployStartupMarker(t, marker)
			cleanupDeployStartupProcess(t, deployProcess{PID: pid})
			response := httptest.NewRecorder()
			modeUpdated := make(chan struct{})
			go func() {
				defer close(modeUpdated)
				if scope == "local" {
					fixture.local.updateBuildMode(response, "demo", "build-1", shared.LIVE_MODE)
				} else {
					fixture.remote.updateBuildMode(response, "demo", "build-1", shared.LIVE_MODE)
				}
			}()
			select {
			case <-modeUpdated:
				t.Fatal("mode update skipped the pending runtime and returned before startup")
			case <-time.After(100 * time.Millisecond):
			}
			if err := os.WriteFile(gate, nil, 0o600); err != nil {
				t.Fatal(err)
			}
			if err := <-started; err != nil {
				t.Fatalf("initial startup: %v", err)
			}
			select {
			case <-modeUpdated:
			case <-time.After(10 * time.Second):
				t.Fatal("mode update did not finish after startup")
			}
			var proc deployProcess
			var running bool
			if scope == "local" {
				proc, running = fixture.local.readProcess("demo")
			} else {
				proc, running = fixture.remote.readProcess("demo")
			}
			if running {
				cleanupDeployStartupProcess(t, proc)
			}
			payload := decodeResponseMap(t, response)
			if response.Code != http.StatusOK || payload["restarted"] != true || payload["restart_error"] != "" {
				t.Fatalf("mode update did not restart pending build: %d %#v", response.Code, payload)
			}
			if !running || proc.RuntimeMode != shared.LIVE_MODE || proc.PID == pid {
				t.Fatalf("running process does not match saved mode: %#v (running %v)", proc, running)
			}
		})
	}
}
