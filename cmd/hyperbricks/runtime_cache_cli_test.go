//go:build darwin || linux

package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"syscall"
	"testing"
	"time"
)

func TestRuntimeCacheCLIPurgeAndShutdown(t *testing.T) {
	// Keep Unix socket names below Darwin's 104-byte limit and isolate control
	// discovery from the developer's own running modules.
	home, err := os.MkdirTemp("/tmp", "hbrc-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(home) })
	t.Setenv("HOME", home)
	t.Setenv("XDG_CACHE_HOME", filepath.Join(home, "cache"))
	var diskCalls, memoryCalls atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		counter := &diskCalls
		if r.URL.Path == "/mem" {
			counter = &memoryCalls
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"value": strings.Trim(r.URL.Path, "/") + "-" + strconv.Itoa(int(counter.Add(1)))})
	}))
	defer upstream.Close()
	address := developmentTestAddress(t)
	_, portText, _ := net.SplitHostPort(address)
	port, _ := strconv.Atoi(portText)
	root, module := runtimeProcessGateFixture(t, "live", port, map[string]any{})
	writeTestFile(t, filepath.Join(module, "hyperbricks", "app.hyperbricks.yaml"), fmt.Sprintf(`disk_page:
  - type: fragment
  - route: disk
  - cache:
      storage: disk
      expire: 1h
  - 10:
      - type: api_render
      - endpoint: %s/disk
      - method: GET
      - inline: '{{.Data.value}}'
memory_page:
  - type: fragment
  - route: mem
  - cache: 1h
  - 10:
      - type: api_render
      - endpoint: %s/mem
      - method: GET
      - inline: '{{.Data.value}}'
`, upstream.URL, upstream.URL))
	args, _ := json.Marshal([]string{"start", "-m", "gates"})
	command := exec.Command(os.Args[0], "-test.run=^TestRuntimeProcessCLIHelper$")
	command.Dir = root
	for _, entry := range os.Environ() {
		name, _, _ := strings.Cut(entry, "=")
		if strings.HasPrefix(name, "HB_DEPLOY_") || name == "HB_PRODUCTION" || name == "HB_PROCESS_TEST_MODE" || name == "HB_RUNTIME_PROCESS_GATE_ARGS" {
			continue
		}
		command.Env = append(command.Env, entry)
	}
	command.Env = append(command.Env, "HB_RUNTIME_PROCESS_GATE_ARGS="+string(args), "HB_NO_KEYBOARD=1", "NO_COLOR=1", "TERM=dumb", "GORACE="+os.Getenv("GORACE")+" atexit_sleep_ms=0")
	logFile, err := os.Create(filepath.Join(root, "runtime.log"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = logFile.Close() })
	command.Stdout, command.Stderr = logFile, logFile
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	var exitErr error
	go func() { exitErr = command.Wait(); close(done) }()
	t.Cleanup(func() {
		select {
		case <-done:
		default:
			_ = command.Process.Signal(syscall.SIGTERM)
			select {
			case <-done:
			case <-time.After(6 * time.Second):
				_ = command.Process.Kill()
				<-done
			}
		}
		if t.Failed() {
			if data, err := os.ReadFile(logFile.Name()); err == nil {
				t.Logf("runtime output:\n%s", data)
			}
		}
	})
	deadline := time.Now().Add(8 * time.Second)
	for {
		connection, err := net.DialTimeout("tcp", address, 50*time.Millisecond)
		if err == nil {
			connection.Close()
			break
		}
		select {
		case <-done:
			t.Fatalf("runtime exited before HTTP listener started: %v", exitErr)
		default:
		}
		if time.Now().After(deadline) {
			t.Fatal("runtime listener did not start")
		}
		time.Sleep(10 * time.Millisecond)
	}
	if _, err := os.Stat(filepath.Join(module, ".cache")); !os.IsNotExist(err) {
		t.Fatalf("starting the runtime eagerly created .cache: %v", err)
	}
	client := &http.Client{Timeout: 3 * time.Second}
	fetch := func(route, etag, wantBody string, wantStatus int) string {
		t.Helper()
		request, err := http.NewRequest(http.MethodGet, "http://"+address+"/"+route, nil)
		if err != nil {
			t.Fatal(err)
		}
		if etag != "" {
			request.Header.Set("If-None-Match", etag)
		}
		response, err := client.Do(request)
		if err != nil {
			t.Fatal(err)
		}
		defer response.Body.Close()
		body, err := io.ReadAll(response.Body)
		if err != nil || response.StatusCode != wantStatus || strings.TrimSpace(string(body)) != wantBody {
			t.Fatalf("%s: status=%d body=%q err=%v, want status=%d body=%q", route, response.StatusCode, body, err, wantStatus, wantBody)
		}
		if response.Header.Get("ETag") == "" {
			t.Fatalf("%s response omitted ETag", route)
		}
		return response.Header.Get("ETag")
	}
	memoryETag := fetch("mem", "", "mem-1", http.StatusOK)
	if _, err := os.Stat(filepath.Join(module, ".cache")); !os.IsNotExist(err) {
		t.Fatalf("memory route created .cache: %v", err)
	}
	diskETag := fetch("disk", "", "disk-1", http.StatusOK)
	fetch("mem", "", "mem-1", http.StatusOK)
	fetch("disk", "", "disk-1", http.StatusOK)
	fetch("mem", memoryETag, "", http.StatusNotModified)
	fetch("disk", diskETag, "", http.StatusNotModified)
	if diskCalls.Load() != 1 || memoryCalls.Load() != 1 {
		t.Fatalf("warm requests rendered again: disk=%d mem=%d", diskCalls.Load(), memoryCalls.Load())
	}
	files, err := filepath.Glob(filepath.Join(module, ".cache", "responses", "runtime-*", "*.entry"))
	if err != nil || len(files) != 1 {
		t.Fatalf("expected one disk response: files=%v err=%v", files, err)
	}
	runtimeDirectory := filepath.Dir(files[0])
	cacheHome, err := os.UserCacheDir()
	if err != nil {
		t.Fatal(err)
	}
	sockets, err := filepath.Glob(filepath.Join(cacheHome, "hyperbricks", "control", "*", "*.sock"))
	if err != nil || len(sockets) != 1 {
		t.Fatalf("expected one private cache-control socket: files=%v err=%v", sockets, err)
	}
	code, output := runRuntimeProcessGateCLI(t, root, nil, "cache", "purge", "--module", "gates", "--route", "disk")
	if code != 0 || !strings.Contains(output, "Purged 0 memory entries and 1 disk entries") {
		t.Fatalf("route purge failed: exit=%d output=%s", code, output)
	}
	diskETag = fetch("disk", diskETag, "disk-2", http.StatusOK)
	fetch("mem", memoryETag, "", http.StatusNotModified)
	code, output = runRuntimeProcessGateCLI(t, root, nil, "cache", "purge", "--module", "gates", "--all")
	if code != 0 || !strings.Contains(output, "Purged 1 memory entries and 1 disk entries") {
		t.Fatalf("full purge failed: exit=%d output=%s", code, output)
	}
	fetch("mem", memoryETag, "mem-2", http.StatusOK)
	fetch("disk", diskETag, "disk-3", http.StatusOK)
	if diskCalls.Load() != 3 || memoryCalls.Load() != 2 {
		t.Fatalf("purged routes did not refresh exactly once: disk=%d mem=%d", diskCalls.Load(), memoryCalls.Load())
	}
	if err := command.Process.Signal(syscall.SIGTERM); err != nil {
		t.Fatal(err)
	}
	select {
	case <-done:
		if exitErr != nil {
			t.Fatalf("graceful shutdown failed: %v", exitErr)
		}
	case <-time.After(6 * time.Second):
		t.Fatal("runtime did not stop after SIGTERM")
	}
	for _, path := range []string{runtimeDirectory, sockets[0]} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("runtime did not remove %s during shutdown: %v", path, err)
		}
	}
}
