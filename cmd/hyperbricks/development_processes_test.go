//go:build darwin || linux

package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/hyperbricks/hyperbricks/pkg/shared"
)

// TestDevelopmentProcessHelper is executed in a real child, so process-group,
// readiness and cancellation assertions exercise OS behavior rather than mocks.
func TestDevelopmentProcessHelper(t *testing.T) {
	mode := os.Getenv("HB_PROCESS_TEST_MODE")
	if mode == "" {
		return
	}
	switch mode {
	case "record":
		cwd, _ := os.Getwd()
		payload, _ := json.Marshal(map[string]interface{}{"cwd": cwd, "args": os.Args,
			"value": os.Getenv("HB_PROCESS_TEST_VALUE"), "root": os.Getenv("HB_MODULE_ROOT"),
			"port": os.Getenv("HB_SERVER_PORT"), "executable": os.Getenv("HB_EXECUTABLE")})
		if err := os.WriteFile(os.Getenv("HB_PROCESS_TEST_FILE"), payload, 0600); err != nil {
			os.Exit(2)
		}
	case "fail":
		fmt.Fprintln(os.Stderr, "a useful failure diagnostic")
		os.Exit(7)
	case "wait":
		if os.Getenv("HB_PROCESS_TEST_IGNORE_TERM") == "1" {
			signal.Ignore(syscall.SIGTERM)
		}
		if path := os.Getenv("HB_PROCESS_TEST_FILE"); path != "" {
			_ = os.WriteFile(path, []byte(strconv.Itoa(os.Getpid())), 0600)
		}
		for {
			time.Sleep(time.Hour)
		}
	case "background":
		cmd := exec.Command(os.Args[0], "-test.run=^TestDevelopmentProcessHelper$")
		cmd.Env = append(os.Environ(), "HB_PROCESS_TEST_MODE=wait")
		cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
		if err := cmd.Start(); err != nil {
			os.Exit(2)
		}
		if err := os.WriteFile(os.Getenv("HB_PROCESS_TEST_FILE"), []byte(strconv.Itoa(cmd.Process.Pid)), 0600); err != nil {
			os.Exit(2)
		}
	case "serve":
		if os.Getenv("HB_PROCESS_TEST_IGNORE_TERM") == "1" {
			signal.Ignore(syscall.SIGTERM)
		}
		listener, err := net.Listen("tcp", os.Getenv("HB_PROCESS_TEST_ADDRESS"))
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
		if path := os.Getenv("HB_PROCESS_TEST_FILE"); path != "" {
			_ = os.WriteFile(path, []byte(strconv.Itoa(os.Getpid())), 0600)
		}
		status, _ := strconv.Atoi(os.Getenv("HB_PROCESS_TEST_STATUS"))
		if status == 0 {
			status = 200
		}
		server := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/exit" {
				go func() { time.Sleep(25 * time.Millisecond); os.Exit(0) }()
			}
			if status == 302 {
				w.Header().Set("Location", "/ready")
			}
			if r.URL.Path == "/ready" {
				w.WriteHeader(200)
			} else {
				w.WriteHeader(status)
			}
		})}
		go server.Serve(listener)
		if os.Getenv("HB_PROCESS_TEST_IGNORE_TERM") == "1" {
			for {
				time.Sleep(time.Hour)
			}
		}
		interrupt := make(chan os.Signal, 1)
		signal.Notify(interrupt, syscall.SIGTERM)
		<-interrupt
		if path := os.Getenv("HB_PROCESS_TEST_STOP_FILE"); path != "" {
			file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
			if err != nil {
				os.Exit(2)
			}
			fmt.Fprintln(file, os.Getenv("HB_PROCESS_TEST_NAME"))
			file.Close()
		}
		_ = server.Close()
	default:
		os.Exit(3)
	}
	os.Exit(0)
}

func developmentTestRunner(t *testing.T) *developmentProcesses {
	t.Helper()
	// Race-instrumented helpers should not spend a second sleeping at exit;
	// production task deadlines are being tested, not the detector's exit delay.
	t.Setenv("GORACE", os.Getenv("GORACE")+" atexit_sleep_ms=0")
	p, err := newDevelopmentProcesses(t.TempDir(), 54321)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = p.Stop() })
	return p
}

func developmentTestCommand() []string {
	return []string{os.Args[0], "-test.run=^TestDevelopmentProcessHelper$", "--"}
}

func developmentTestAddress(t *testing.T) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	address := listener.Addr().String()
	listener.Close()
	return address
}

func developmentTestService(t *testing.T, name string) shared.DevelopmentServiceConfig {
	t.Helper()
	address := developmentTestAddress(t)
	return shared.DevelopmentServiceConfig{Name: name, Command: developmentTestCommand(),
		Env:   map[string]string{"HB_PROCESS_TEST_MODE": "serve", "HB_PROCESS_TEST_ADDRESS": address},
		Ready: shared.DevelopmentServiceReadyConfig{HTTP: "http://" + address + "/health", Timeout: 3 * time.Second}, StopTimeout: time.Second}
}

func TestDevelopmentTasksArgumentsEnvironmentAndOrder(t *testing.T) {
	p := developmentTestRunner(t)
	t.Setenv("HB_PROCESS_TEST_VALUE", "parent-value")
	first := filepath.Join(p.moduleRoot, "first.json")
	second := filepath.Join(p.moduleRoot, "second.json")
	args := append(developmentTestCommand(), "$PORT", "~", "&&", "a b")
	tasks := []shared.DevelopmentTaskConfig{
		{Name: "first", Command: args, Env: map[string]string{"HB_PROCESS_TEST_MODE": "record", "HB_PROCESS_TEST_FILE": first, "HB_PROCESS_TEST_VALUE": "child-value"}, Timeout: time.Second},
		{Name: "second", Command: []string{"sh", "-c", "test -f first.json && cp first.json second.json"}, Timeout: time.Second},
	}
	if err := p.RunTasks(context.Background(), "before_start", tasks); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(second)
	if err != nil {
		t.Fatal(err)
	}
	var got struct {
		Cwd        string
		Args       []string
		Value      string
		Root       string
		Port       string
		Executable string
	}
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	canonicalRoot, err := filepath.EvalSymlinks(p.moduleRoot)
	if err != nil {
		t.Fatal(err)
	}
	if got.Cwd != canonicalRoot || got.Root != p.moduleRoot || got.Port != "54321" || got.Executable != p.executable || got.Value != "child-value" {
		t.Fatalf("unexpected child context: %+v", got)
	}
	if strings.Join(got.Args[len(got.Args)-4:], "|") != "$PORT|~|&&|a b" {
		t.Fatalf("arguments expanded: %q", got.Args)
	}
	if os.Getenv("HB_PROCESS_TEST_VALUE") != "parent-value" {
		t.Fatal("parent environment changed")
	}
	for _, key := range []string{"PATH", "HB_EXECUTABLE", "HB_MODULE_ROOT", "HB_SERVER_PORT"} {
		if _, err := p.childEnvironment(map[string]string{key: "override"}); err == nil {
			t.Errorf("accepted reserved %s", key)
		}
	}
}

func TestDevelopmentTaskFailureIncludesTail(t *testing.T) {
	p := developmentTestRunner(t)
	err := p.RunTasks(context.Background(), "before_start", []shared.DevelopmentTaskConfig{{Name: "fails", Command: developmentTestCommand(), Env: map[string]string{"HB_PROCESS_TEST_MODE": "fail"}, Timeout: time.Second}})
	if err == nil || !strings.Contains(err.Error(), "exit status 7") || !strings.Contains(err.Error(), "useful failure diagnostic") || !strings.Contains(err.Error(), "fails") {
		t.Fatalf("missing diagnostic: %v", err)
	}
}

func TestDevelopmentTaskCancellationCleansChild(t *testing.T) {
	p := developmentTestRunner(t)
	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()
	started := time.Now()
	err := p.RunTasks(ctx, "before_start", []shared.DevelopmentTaskConfig{{Name: "waits", Command: developmentTestCommand(), Env: map[string]string{"HB_PROCESS_TEST_MODE": "wait"}, Timeout: time.Minute}})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("unexpected cancellation: %v", err)
	}
	if time.Since(started) > 2*time.Second {
		t.Fatalf("cancellation was slow: %s", time.Since(started))
	}
	if developmentProcessGroupAlive(p.children[0].cmd.Process.Pid) {
		t.Fatal("cancelled task left its process group")
	}
}

func TestDevelopmentTaskCleansBackgroundDescendants(t *testing.T) {
	p := developmentTestRunner(t)
	pidFile := filepath.Join(p.moduleRoot, "background.pid")
	err := p.RunTasks(context.Background(), "before_start", []shared.DevelopmentTaskConfig{{Name: "background", Command: developmentTestCommand(), Env: map[string]string{"HB_PROCESS_TEST_MODE": "background", "HB_PROCESS_TEST_FILE": pidFile}, Timeout: time.Second}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.ReadFile(pidFile); err != nil {
		t.Fatal(err)
	}
	if developmentProcessGroupAlive(p.children[0].cmd.Process.Pid) {
		t.Fatal("successful task left a background descendant")
	}
}

func TestDevelopmentServicesReadinessAndReverseCleanup(t *testing.T) {
	p := developmentTestRunner(t)
	stopFile := filepath.Join(p.moduleRoot, "stopped")
	first, second := developmentTestService(t, "first"), developmentTestService(t, "second")
	for _, service := range []*shared.DevelopmentServiceConfig{&first, &second} {
		service.Env["HB_PROCESS_TEST_STOP_FILE"] = stopFile
		service.Env["HB_PROCESS_TEST_NAME"] = service.Name
	}
	ctx, cancel := context.WithCancel(context.Background())
	if err := p.StartServices(ctx, []shared.DevelopmentServiceConfig{first, second}); err != nil {
		cancel()
		t.Fatal(err)
	}
	cancel() // Dependencies remain available for the owner's HTTP drain.
	response, err := http.Get(first.Ready.HTTP)
	if err != nil {
		t.Fatalf("cancellation killed dependency before Stop: %v", err)
	}
	response.Body.Close()
	if err := p.Stop(); err != nil {
		t.Fatal(err)
	}
	if err := p.Stop(); err != nil {
		t.Fatalf("Stop not idempotent: %v", err)
	}
	data, err := os.ReadFile(stopFile)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "second\nfirst\n" {
		t.Fatalf("wrong cleanup order: %q", data)
	}
	select {
	case err := <-p.Failures():
		t.Fatalf("intentional shutdown reported failure: %v", err)
	default:
	}
}

func TestDevelopmentServicesRefuseOccupiedReadinessAddress(t *testing.T) {
	p := developmentTestRunner(t)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	service := developmentTestService(t, "occupied")
	service.Ready.HTTP = "http://" + listener.Addr().String() + "/health"
	err = p.StartServices(context.Background(), []shared.DevelopmentServiceConfig{service})
	if err == nil || !strings.Contains(err.Error(), "already occupied") {
		t.Fatalf("unexpected result: %v", err)
	}
	if len(p.children) != 0 {
		t.Fatal("spawned child for occupied address")
	}
	connection, err := net.Dial("tcp", listener.Addr().String())
	if err != nil {
		t.Fatalf("existing service disturbed: %v", err)
	}
	connection.Close()
}

func TestDevelopmentServiceUnexpectedSuccessfulExit(t *testing.T) {
	p := developmentTestRunner(t)
	service := developmentTestService(t, "exits-zero")
	if err := p.StartServices(context.Background(), []shared.DevelopmentServiceConfig{service}); err != nil {
		t.Fatal(err)
	}
	response, err := http.Get(strings.Replace(service.Ready.HTTP, "/health", "/exit", 1))
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	var reported error
	select {
	case reported = <-p.Failures():
		if !strings.Contains(reported.Error(), "exits-zero") || !strings.Contains(reported.Error(), "status 0") {
			t.Fatal(reported)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("successful service exit was not reported")
	}
	if err := p.Stop(); !errors.Is(err, reported) {
		t.Fatalf("original service failure was not preserved: %v", err)
	}
}

func TestDevelopmentServiceExitDuringDrainIsPreserved(t *testing.T) {
	p := developmentTestRunner(t)
	service := developmentTestService(t, "draining-api")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := p.StartServices(ctx, []shared.DevelopmentServiceConfig{service}); err != nil {
		t.Fatal(err)
	}
	// The session owner has received SIGTERM and is draining HTTP, so its
	// failure consumer has stopped. The API has not been asked to stop yet.
	cancel()
	response, err := http.Get(strings.Replace(service.Ready.HTTP, "/health", "/exit", 1))
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	select {
	case <-p.children[0].done:
	case <-time.After(2 * time.Second):
		t.Fatal("helper did not exit")
	}
	err = p.Stop()
	if err == nil || !strings.Contains(err.Error(), "draining-api") || !strings.Contains(err.Error(), "exited unexpectedly") {
		t.Fatalf("drain-phase dependency failure was lost: %v", err)
	}
}

func TestDevelopmentServiceReadinessFailureCleansAll(t *testing.T) {
	for _, status := range []string{"503", "302"} {
		t.Run(status, func(t *testing.T) {
			p := developmentTestRunner(t)
			first, second := developmentTestService(t, "healthy"), developmentTestService(t, "unready")
			second.Env["HB_PROCESS_TEST_STATUS"] = status
			second.Ready.Timeout = 350 * time.Millisecond
			err := p.StartServices(context.Background(), []shared.DevelopmentServiceConfig{first, second})
			if err == nil || !strings.Contains(err.Error(), status) || !errors.Is(err, context.DeadlineExceeded) {
				t.Fatalf("unexpected readiness result: %v", err)
			}
			if err := p.Stop(); err != nil {
				t.Fatal(err)
			}
			for _, child := range p.children {
				if developmentProcessGroupAlive(child.cmd.Process.Pid) {
					t.Fatalf("left %s running", child.name)
				}
			}
		})
	}
}

func TestDevelopmentServiceReadinessCancellationAndEarlyExit(t *testing.T) {
	t.Run("cancel", func(t *testing.T) {
		p := developmentTestRunner(t)
		service := developmentTestService(t, "waiting")
		service.Env["HB_PROCESS_TEST_STATUS"] = "503"
		ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
		defer cancel()
		if err := p.StartServices(ctx, []shared.DevelopmentServiceConfig{service}); !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("unexpected result: %v", err)
		}
		if err := p.Stop(); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("early-exit", func(t *testing.T) {
		p := developmentTestRunner(t)
		service := developmentTestService(t, "exits")
		service.Env["HB_PROCESS_TEST_MODE"] = "fail"
		if err := p.StartServices(context.Background(), []shared.DevelopmentServiceConfig{service}); err == nil || !strings.Contains(err.Error(), "exit status 7") {
			t.Fatalf("unexpected result: %v", err)
		}
	})
}

func TestDevelopmentServiceEscalatesIgnoredTermination(t *testing.T) {
	p := developmentTestRunner(t)
	service := developmentTestService(t, "stubborn")
	service.Env["HB_PROCESS_TEST_IGNORE_TERM"] = "1"
	service.StopTimeout = 100 * time.Millisecond
	if err := p.StartServices(context.Background(), []shared.DevelopmentServiceConfig{service}); err != nil {
		t.Fatal(err)
	}
	err := p.Stop()
	if err == nil || !strings.Contains(err.Error(), "forced termination") {
		t.Fatalf("missing forced termination diagnostic: %v", err)
	}
	if developmentProcessGroupAlive(p.children[0].cmd.Process.Pid) {
		t.Fatal("stubborn service still running")
	}
}

func TestDevelopmentOutputTailIsBounded(t *testing.T) {
	tail := &developmentOutputTail{}
	tail.Write([]byte(strings.Repeat("x", 100000)))
	tail.Write([]byte("last output"))
	got := tail.String()
	if len(got) != developmentOutputTailLimit || !strings.HasSuffix(got, "last output") {
		t.Fatalf("tail length %d suffix missing", len(got))
	}
}

func TestDevelopmentTaskWorkingDirectoryAndGeneratedExecutable(t *testing.T) {
	p := developmentTestRunner(t)
	other := t.TempDir()
	p.invocationDir = filepath.Dir(other)
	tasks := []shared.DevelopmentTaskConfig{
		{Name: "generate", Command: []string{"sh", "-c", "printf '#!/bin/sh\\nprintf prepared > result\\n' > generated; chmod +x generated"}, Cwd: filepath.Base(other), Timeout: time.Second},
		{Name: "execute", Command: []string{"./generated"}, Cwd: filepath.Base(other), Timeout: time.Second},
	}
	if err := p.RunTasks(context.Background(), "before_start", tasks); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(other, "result"))
	if err != nil || string(data) != "prepared" {
		t.Fatalf("generated command result %q: %v", data, err)
	}
}

func TestDevelopmentTaskOwnTimeoutAndShellGrandchild(t *testing.T) {
	p := developmentTestRunner(t)
	err := p.RunTasks(context.Background(), "before_start", []shared.DevelopmentTaskConfig{{Name: "shell", Command: []string{"sh", "-c", "sleep 60 & echo $! > shell.pid; wait"}, Timeout: 150 * time.Millisecond}})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("task timeout: %v", err)
	}
	if _, err := os.ReadFile(filepath.Join(p.moduleRoot, "shell.pid")); err != nil {
		t.Fatal(err)
	}
	if developmentProcessGroupAlive(p.children[0].cmd.Process.Pid) {
		t.Fatal("task left shell grandchild running")
	}
}

func TestDevelopmentReadinessUsesDirectLoopback(t *testing.T) {
	p := developmentTestRunner(t)
	t.Setenv("HTTP_PROXY", "http://127.0.0.1:1")
	t.Setenv("ALL_PROXY", "http://127.0.0.1:1")
	service := developmentTestService(t, "direct")
	if err := p.StartServices(context.Background(), []shared.DevelopmentServiceConfig{service}); err != nil {
		t.Fatal(err)
	}
	for _, raw := range []string{"https://127.0.0.1/", "http://localhost/", "http://user@127.0.0.1/", "http://192.0.2.1/", "http://127.0.0.1/#fragment"} {
		if _, err := developmentReadinessAddress(raw); err == nil {
			t.Errorf("accepted %s", raw)
		}
	}
	for _, raw := range []string{"http://127.0.0.2/", "http://[::1]:3456/"} {
		if _, err := developmentReadinessAddress(raw); err != nil {
			t.Errorf("rejected %s: %v", raw, err)
		}
	}
}
