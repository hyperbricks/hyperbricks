//go:build darwin || linux

package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/hyperbricks/hyperbricks/cmd/hyperbricks/commands"
)

// Register one controllable request before entering the real CLI. The runtime
// still owns configuration, the managed API, HTTP serving and SIGTERM cleanup.
func TestRuntimeServiceDrainCLIHelper(t *testing.T) {
	raw := os.Getenv("HB_RUNTIME_SERVICE_DRAIN_ARGS")
	if raw == "" {
		return
	}
	var args []string
	if err := json.Unmarshal([]byte(raw), &args); err != nil {
		t.Fatal(err)
	}
	http.DefaultServeMux.HandleFunc("/__test-managed-service-drain", func(w http.ResponseWriter, r *http.Request) {
		if err := os.WriteFile(os.Getenv("HB_RUNTIME_SERVICE_DRAIN_STARTED"), []byte("started"), 0600); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		deadline := time.Now().Add(4 * time.Second)
		for {
			if _, err := os.Stat(os.Getenv("HB_RUNTIME_SERVICE_DRAIN_RELEASE")); err == nil {
				break
			}
			if time.Now().After(deadline) {
				http.Error(w, "test did not release request", http.StatusGatewayTimeout)
				return
			}
			time.Sleep(5 * time.Millisecond)
		}
		response, err := (&http.Client{Timeout: time.Second}).Get(os.Getenv("HB_RUNTIME_SERVICE_DRAIN_UPSTREAM"))
		if err != nil {
			http.Error(w, "managed API stopped before request finished: "+err.Error(), http.StatusBadGateway)
			return
		}
		response.Body.Close()
		if response.StatusCode != http.StatusOK {
			http.Error(w, response.Status, http.StatusBadGateway)
			return
		}
		fmt.Fprint(w, "managed API reachable while draining")
	})
	os.Args = append([]string{"hyperbricks"}, args...)
	run()
	os.Exit(commands.ExitCode)
}

func TestRuntimeServiceDrainKeepsManagedAPIUntilRequestFinishes(t *testing.T) {
	mainAddress := developmentTestAddress(t)
	_, portText, _ := net.SplitHostPort(mainAddress)
	mainPort, _ := strconv.Atoi(portText)
	apiAddress := developmentTestAddress(t)
	markers := t.TempDir()
	pidFile := filepath.Join(markers, "api.pid")
	startedFile, releaseFile := filepath.Join(markers, "request.started"), filepath.Join(markers, "request.release")
	service := runtimeProcessGateTask(t, pidFile, "serve")
	delete(service, "timeout")
	service["name"] = "drain-api"
	service["env"].(map[string]string)["HB_PROCESS_TEST_ADDRESS"] = apiAddress
	service["ready"] = map[string]any{"http": "http://" + apiAddress + "/health", "timeout": "3s"}
	service["stop_timeout"] = "1s"
	root, _ := runtimeProcessGateFixture(t, "development", mainPort, map[string]any{"services": []any{service}})
	args, _ := json.Marshal([]string{"start", "-m", "gates", "--with-processes"})
	command := exec.Command(os.Args[0], "-test.run=^TestRuntimeServiceDrainCLIHelper$")
	command.Dir = root
	for _, entry := range os.Environ() {
		name, _, _ := strings.Cut(entry, "=")
		if strings.HasPrefix(name, "HB_DEPLOY_") || name == "HB_PRODUCTION" || name == "HB_PROCESS_TEST_MODE" {
			continue
		}
		command.Env = append(command.Env, entry)
	}
	command.Env = append(command.Env,
		"HB_RUNTIME_SERVICE_DRAIN_ARGS="+string(args),
		"HB_RUNTIME_SERVICE_DRAIN_STARTED="+startedFile,
		"HB_RUNTIME_SERVICE_DRAIN_RELEASE="+releaseFile,
		"HB_RUNTIME_SERVICE_DRAIN_UPSTREAM=http://"+apiAddress+"/health",
		"HB_NO_KEYBOARD=1", "NO_COLOR=1", "TERM=dumb", "GORACE="+os.Getenv("GORACE")+" atexit_sleep_ms=0")
	logFile, err := os.Create(filepath.Join(markers, "runtime.log"))
	if err != nil {
		t.Fatal(err)
	}
	defer logFile.Close()
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
		if data, err := os.ReadFile(pidFile); err == nil {
			if pid, err := strconv.Atoi(strings.TrimSpace(string(data))); err == nil && pid > 0 && syscall.Kill(pid, 0) == nil {
				_ = syscall.Kill(-pid, syscall.SIGKILL)
			}
		}
		if t.Failed() {
			if data, err := os.ReadFile(logFile.Name()); err == nil {
				t.Logf("runtime output:\n%s", data)
			}
		}
	})
	wait := func(description string, condition func() bool) {
		t.Helper()
		budget := 6 * time.Second
		// Race instrumentation makes component registration slower; the assertion
		// concerns shutdown ordering, not startup throughput.
		if description == "main HTTP listener" {
			budget = 20 * time.Second
		}
		deadline := time.Now().Add(budget)
		for !condition() {
			select {
			case <-done:
				t.Fatalf("CLI exited before %s: %v", description, exitErr)
			default:
			}
			if time.Now().After(deadline) {
				t.Fatalf("timed out waiting for %s", description)
			}
			time.Sleep(10 * time.Millisecond)
		}
	}
	listening := func() bool {
		connection, err := net.DialTimeout("tcp", mainAddress, 50*time.Millisecond)
		if err != nil {
			return false
		}
		connection.Close()
		return true
	}
	wait("main HTTP listener", listening)
	type requestResult struct {
		status int
		body   string
		err    error
	}
	requestDone := make(chan requestResult, 1)
	go func() {
		response, err := (&http.Client{Timeout: 7 * time.Second}).Get("http://" + mainAddress + "/__test-managed-service-drain")
		if err != nil {
			requestDone <- requestResult{err: err}
			return
		}
		defer response.Body.Close()
		body, err := io.ReadAll(response.Body)
		requestDone <- requestResult{status: response.StatusCode, body: string(body), err: err}
	}()
	wait("active request", func() bool { _, err := os.Stat(startedFile); return err == nil })
	if err := command.Process.Signal(syscall.SIGTERM); err != nil {
		t.Fatal(err)
	}
	wait("HTTP listener to close for draining", func() bool { return !listening() })
	// The CLI has entered its actual HTTP shutdown, but its managed child must
	// still answer both this probe and the in-flight application's later call.
	response, err := (&http.Client{Timeout: time.Second}).Get("http://" + apiAddress + "/health")
	if err != nil {
		t.Fatalf("managed API stopped at start of HTTP drain: %v", err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("API health during drain: %s", response.Status)
	}
	if err := os.WriteFile(releaseFile, []byte("finish request"), 0600); err != nil {
		t.Fatal(err)
	}
	result := <-requestDone
	if result.err != nil || result.status != http.StatusOK || result.body != "managed API reachable while draining" {
		t.Fatalf("in-flight request failed during shutdown: status=%d body=%q err=%v", result.status, result.body, result.err)
	}
	select {
	case <-done:
		if exitErr != nil {
			t.Fatalf("clean SIGTERM shutdown failed: %v", exitErr)
		}
	case <-time.After(4 * time.Second):
		t.Fatal("CLI did not finish after request drained")
	}
	data, err := os.ReadFile(pidFile)
	if err != nil {
		t.Fatal(err)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		t.Fatal(err)
	}
	if err := syscall.Kill(pid, 0); err != syscall.ESRCH {
		t.Fatalf("managed API remains after CLI exit: %v", err)
	}
	listener, err := net.Listen("tcp", apiAddress)
	if err != nil {
		t.Fatalf("API port was not released: %v", err)
	}
	listener.Close()
}
