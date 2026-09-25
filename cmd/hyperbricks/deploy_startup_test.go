package main

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/hyperbricks/hyperbricks/pkg/shared"
)

// Re-executed through a disposable executable, never the installed HyperBricks
// binary. The normal test run does not set the helper environment variable.
func TestDeployStartupHelper(t *testing.T) {
	behavior := os.Getenv("HB_DEPLOY_STARTUP_TEST_HELPER")
	if behavior == "" {
		return
	}
	if behavior == "exit" {
		fmt.Fprintln(os.Stderr, "test runtime rejected its configuration")
		os.Exit(3)
	}
	if behavior == "parent" {
		executable, err := os.Executable()
		if err != nil {
			os.Exit(5)
		}
		cmd := exec.Command(executable, "-test.run=^TestDeployStartupHelper$")
		cmd.Env = append(os.Environ(), "HB_DEPLOY_STARTUP_TEST_HELPER=listen")
		cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
		var logFile *os.File
		logPath := os.Getenv("HB_DEPLOY_STARTUP_LOG_PATH")
		if logPath != "" {
			logFile, err = os.Create(logPath)
			if err != nil {
				os.Exit(6)
			}
		}
		port, _ := strconv.Atoi(os.Getenv("HB_DEPLOY_PORT"))
		if err := startDeployProcess(cmd, port, logFile, logPath); err != nil {
			os.Exit(7)
		}
		if err := os.WriteFile(os.Getenv("HB_DEPLOY_STARTUP_PID_PATH"), []byte(strconv.Itoa(cmd.Process.Pid)), 0o600); err != nil {
			_ = cmd.Process.Kill()
			os.Exit(8)
		}
		os.Exit(0)
	}
	if marker := os.Getenv("HB_DEPLOY_STARTUP_TEST_MARKER"); marker != "" {
		if err := os.WriteFile(marker, []byte(strconv.Itoa(os.Getpid())), 0o600); err != nil {
			os.Exit(9)
		}
	}
	if gate := os.Getenv("HB_DEPLOY_STARTUP_TEST_GATE"); gate != "" {
		deadline := time.Now().Add(5 * time.Second)
		for {
			if _, err := os.Stat(gate); err == nil {
				break
			}
			if time.Now().After(deadline) {
				os.Exit(10)
			}
			time.Sleep(10 * time.Millisecond)
		}
	}
	listener, err := net.Listen("tcp", net.JoinHostPort("127.0.0.1", os.Getenv("HB_DEPLOY_PORT")))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(4)
	}
	fmt.Fprintln(os.Stdout, "test runtime listener ready")
	for {
		connection, err := listener.Accept()
		if err != nil {
			os.Exit(0)
		}
		fmt.Fprintln(os.Stdout, "test runtime handled connection")
		fmt.Fprintln(os.Stderr, "test runtime remains independent of control plane")
		_ = connection.Close()
	}
}

func deployStartupTestBinary(t *testing.T) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("disposable executable uses a POSIX shell")
	}
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "test-deploy-runtime")
	quoted := "'" + strings.ReplaceAll(executable, "'", "'\\''") + "'"
	body := "#!/bin/sh\nexec " + quoted + " -test.run=^TestDeployStartupHelper$ -- \"$@\"\n"
	if err := os.WriteFile(path, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

type deployStartupFixture struct {
	root   string
	port   int
	local  *deployLocalServer
	remote *deployAPI
}

func newDeployStartupFixture(t *testing.T, logsEnabled bool) deployStartupFixture {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	_ = listener.Close()
	root := t.TempDir()
	archive := filepath.Join(root, "demo", "demo-1.0.0-build-1.hra")
	config := deployPackageFixture + fmt.Sprintf("  server:\n    port: %d\n", port)
	writeDeployHRAFixture(t, archive, config)
	index := deployIndex{Current: "build-1", Port: port, Versions: []deployIndexRow{{
		BuildID: "build-1", File: archive, Format: "hra", RuntimeMode: shared.DEVELOPMENT_MODE,
	}}}
	if err := saveDeployIndex(filepath.Join(root, "demo", deployIndexFile), index); err != nil {
		t.Fatal(err)
	}
	binary := deployStartupTestBinary(t)
	return deployStartupFixture{
		root:   root,
		port:   port,
		local:  &deployLocalServer{buildRoot: root, binaryPath: binary, logsEnabled: logsEnabled, portStart: port},
		remote: &deployAPI{root: root, binaryPath: binary, logsEnabled: logsEnabled, portStart: port},
	}
}

func cleanupDeployStartupProcess(t *testing.T, proc deployProcess) {
	t.Helper()
	t.Cleanup(func() {
		_ = signalProcess(proc.PID, syscall.SIGKILL)
		deadline := time.Now().Add(2 * time.Second)
		for isProcessRunning(proc.PID) && time.Now().Before(deadline) {
			time.Sleep(10 * time.Millisecond)
		}
	})
}

func TestDeploymentStartWaitsForRuntime(t *testing.T) {
	for _, scope := range []string{"local", "remote"} {
		for _, logging := range []bool{false, true} {
			for _, behavior := range []string{"exit", "listen"} {
				t.Run(fmt.Sprintf("%s/logs=%t/%s", scope, logging, behavior), func(t *testing.T) {
					t.Setenv("HB_DEPLOY_STARTUP_TEST_HELPER", behavior)
					fixture := newDeployStartupFixture(t, logging)
					var err error
					var proc deployProcess
					var hasProcess bool
					var pidPath, buildPIDPath, logPath string
					if scope == "local" {
						err = fixture.local.startLocalBuild("demo", "build-1")
						proc, hasProcess = fixture.local.readProcess("demo")
						pidPath = fixture.local.pidPath("demo")
						buildPIDPath = fixture.local.buildPidPath("demo", "build-1")
						logPath = fixture.local.buildLogPath("demo", "build-1")
					} else {
						err = fixture.remote.startManaged("demo", "build-1")
						proc, hasProcess = fixture.remote.readProcess("demo")
						pidPath = fixture.remote.pidPath("demo")
						buildPIDPath = fixture.remote.buildPidPath("demo", "build-1")
						logPath = fixture.remote.buildLogPath("demo", "build-1")
					}
					if hasProcess {
						cleanupDeployStartupProcess(t, proc)
					}
					if behavior == "exit" {
						if err == nil || !strings.Contains(err.Error(), "exit status 3") {
							t.Fatalf("expected useful startup failure, got %v", err)
						}
						if logging && (!strings.Contains(err.Error(), logPath) || !strings.Contains(err.Error(), "test runtime rejected its configuration")) {
							t.Fatalf("startup failure omitted log path: %v", err)
						}
						if !logging && !strings.Contains(err.Error(), "Enable deployment logs") {
							t.Fatalf("startup failure omitted logs-disabled hint: %v", err)
						}
						for _, path := range []string{pidPath, buildPIDPath} {
							if _, err := os.Stat(path); !os.IsNotExist(err) {
								t.Fatalf("failed startup left a process record at %s: %v", path, err)
							}
						}
						return
					}
					if err != nil || !hasProcess {
						t.Fatalf("runtime did not become ready: process=%v, err=%v", hasProcess, err)
					}
					connection, err := net.DialTimeout("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(proc.Port)), time.Second)
					if err != nil {
						t.Fatalf("start returned before a reachable listener: %v", err)
					}
					_ = connection.Close()
					if logging {
						content, err := os.ReadFile(logPath)
						if err != nil || !strings.Contains(string(content), "test runtime listener ready") {
							t.Fatalf("persistent startup output missing: %q, %v", content, err)
						}
					}
				})
			}
		}
	}
}

func TestBuildModeReportsRestartFailureAndKeepsRunningProcess(t *testing.T) {
	for _, scope := range []string{"local", "remote"} {
		t.Run(scope, func(t *testing.T) {
			t.Setenv("HB_DEPLOY_STARTUP_TEST_HELPER", "listen")
			fixture := newDeployStartupFixture(t, false)
			var err error
			var proc deployProcess
			var ok bool
			if scope == "local" {
				err = fixture.local.startLocalBuild("demo", "build-1")
				proc, ok = fixture.local.readProcess("demo")
			} else {
				err = fixture.remote.startManaged("demo", "build-1")
				proc, ok = fixture.remote.readProcess("demo")
			}
			if err != nil || !ok {
				t.Fatalf("start valid fixture: %v (process %v)", err, ok)
			}
			cleanupDeployStartupProcess(t, proc)
			configPath := filepath.Join(fixture.root, "demo", "runtime", "build-1", shared.PackageConfigFileName)
			invalid := "hyperbricks:\n  mode: development\n  development:\n    dashboard: false\n"
			if err := os.WriteFile(configPath, []byte(invalid), 0o644); err != nil {
				t.Fatal(err)
			}
			response := httptest.NewRecorder()
			if scope == "local" {
				fixture.local.updateBuildMode(response, "demo", "build-1", shared.LIVE_MODE)
			} else {
				fixture.remote.updateBuildMode(response, "demo", "build-1", shared.LIVE_MODE)
			}
			if response.Code != http.StatusOK {
				t.Fatalf("mode persistence should succeed: %d %s", response.Code, response.Body.String())
			}
			payload := decodeResponseMap(t, response)
			if payload["runtime_mode"] != shared.LIVE_MODE || payload["restarted"] != false {
				t.Fatalf("unexpected mode response: %#v", payload)
			}
			if message, _ := payload["restart_error"].(string); !strings.Contains(message, "development.dashboard must be a mapping") {
				t.Fatalf("missing actionable restart failure: %#v", payload)
			}
			index, err := loadDeployIndex(filepath.Join(fixture.root, "demo", deployIndexFile))
			if err != nil || len(index.Versions) != 1 || index.Versions[0].RuntimeMode != shared.LIVE_MODE {
				t.Fatalf("mode change was not persisted: %#v, %v", index, err)
			}
			if !isProcessRunning(proc.PID) {
				t.Fatal("configuration preflight stopped the running process")
			}
		})
	}
}

func TestLocalDevStartValidatesBeforeStopping(t *testing.T) {
	root := t.TempDir()
	moduleDir := filepath.Join(root, "modules", "demo")
	if err := os.MkdirAll(moduleDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(moduleDir, shared.PackageConfigFileName), []byte("hyperbricks: {development: {dashboard: false}}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	api := &deployLocalServer{modulesDir: filepath.Join(root, "modules"), buildRoot: filepath.Join(root, "deploy")}
	// An untouched sentinel proves validation precedes process cleanup, without
	// ever risking a signal to another real process.
	if err := api.writeProcess("demo", deployProcess{PID: -1, BuildID: localDevBuildID}); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(api.pidPath("demo"))
	if err != nil {
		t.Fatal(err)
	}
	if err := api.startLocalDev("demo"); err == nil || !strings.Contains(err.Error(), "development.dashboard must be a mapping") {
		t.Fatalf("expected strict source config failure: %v", err)
	}
	after, err := os.ReadFile(api.pidPath("demo"))
	if err != nil || string(after) != string(before) {
		t.Fatalf("preflight modified process state: %q, %v", after, err)
	}
}

func TestDeploymentStartupTimeoutAndOutputBound(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	_ = listener.Close()
	if err := waitDeployProcessReady(make(chan error), port, 50*time.Millisecond); !errors.Is(err, errDeployStartupTimeout) {
		t.Fatalf("expected bounded readiness timeout, got %v", err)
	}
	path := filepath.Join(t.TempDir(), "startup.log")
	if err := os.WriteFile(path, []byte(strings.Repeat("x", deployStartupOutputLimit*2)+"\nlast failure"), 0o600); err != nil {
		t.Fatal(err)
	}
	text := readDeployStartupLog(path)
	if len(text) > deployStartupOutputLimit || !strings.HasSuffix(text, "last failure") {
		t.Fatalf("startup capture is not bounded or lost the final diagnostic: length %d", len(text))
	}
}

func TestDeploymentRuntimeSurvivesControlPlaneExit(t *testing.T) {
	for _, logsEnabled := range []bool{false, true} {
		t.Run(fmt.Sprintf("logs=%t", logsEnabled), func(t *testing.T) {
			fixture := newDeployStartupFixture(t, logsEnabled)
			pidPath := filepath.Join(t.TempDir(), "child.pid")
			logPath := ""
			if logsEnabled {
				logPath = filepath.Join(t.TempDir(), "runtime.log")
			}
			parent := exec.Command(deployStartupTestBinary(t))
			parent.Env = append(os.Environ(),
				"HB_DEPLOY_STARTUP_TEST_HELPER=parent",
				"HB_DEPLOY_PORT="+strconv.Itoa(fixture.port),
				"HB_DEPLOY_STARTUP_PID_PATH="+pidPath,
				"HB_DEPLOY_STARTUP_LOG_PATH="+logPath,
			)
			if err := parent.Run(); err != nil {
				t.Fatalf("disposable control plane failed: %v", err)
			}
			pidBytes, err := os.ReadFile(pidPath)
			if err != nil {
				t.Fatal(err)
			}
			pid, err := strconv.Atoi(string(pidBytes))
			if err != nil {
				t.Fatal(err)
			}
			cleanupDeployStartupProcess(t, deployProcess{PID: pid})
			// Each accepted connection writes stdout and stderr after its parent
			// has exited. A parent-owned output pipe would terminate the runtime.
			for i := 0; i < 3; i++ {
				connection, err := net.DialTimeout("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(fixture.port)), time.Second)
				if err != nil {
					t.Fatalf("runtime lost its listener after control-plane exit: %v", err)
				}
				_ = connection.Close()
				time.Sleep(30 * time.Millisecond)
			}
		})
	}
}

func TestDeploymentStartupConfigSupportsExistingRuntimeAndLegacyArchive(t *testing.T) {
	for _, format := range []string{"hra", "zip", "extracted"} {
		t.Run(format, func(t *testing.T) {
			fixture := newDeployStartupFixture(t, false)
			indexPath := filepath.Join(fixture.root, "demo", deployIndexFile)
			index, err := loadDeployIndex(indexPath)
			if err != nil {
				t.Fatal(err)
			}
			if format == "zip" {
				path := strings.TrimSuffix(index.Versions[0].File, ".hra") + ".zip"
				if err := os.Rename(index.Versions[0].File, path); err != nil {
					t.Fatal(err)
				}
				index.Versions[0].File = path
				index.Versions[0].Format = format
				if err := saveDeployIndex(indexPath, index); err != nil {
					t.Fatal(err)
				}
			}
			location, err := prepareDeployStartupConfig(fixture.root, "demo", "build-1")
			if err != nil {
				t.Fatal(err)
			}
			if format == "extracted" {
				if err := os.Remove(index.Versions[0].File); err != nil {
					t.Fatal(err)
				}
				location, err = prepareDeployStartupConfig(fixture.root, "demo", "build-1")
				if err != nil {
					t.Fatalf("already extracted runtime required archive: %v", err)
				}
			}
			if err := validateDeployStartupConfig(location); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func waitDeployStartupMarker(t *testing.T, path string) int {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if content, err := os.ReadFile(path); err == nil {
			if pid, err := strconv.Atoi(string(content)); err == nil {
				return pid
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("disposable runtime never reached the delayed-listener gate")
	return 0
}

func TestDeploymentStopWaitsForPendingStart(t *testing.T) {
	for _, scope := range []string{"local", "remote"} {
		t.Run(scope, func(t *testing.T) {
			t.Setenv("HB_DEPLOY_STARTUP_TEST_HELPER", "listen")
			fixture := newDeployStartupFixture(t, false)
			marker := filepath.Join(t.TempDir(), "starting.pid")
			gate := filepath.Join(t.TempDir(), "allow-listener")
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
			stopped := make(chan error, 1)
			go func() {
				if scope == "local" {
					_, err := fixture.local.stopLocalModule("demo")
					stopped <- err
				} else {
					stopped <- fixture.remote.stopManaged("demo")
				}
			}()
			select {
			case err := <-stopped:
				t.Fatalf("stop missed a pending start: %v", err)
			case <-time.After(75 * time.Millisecond):
			}
			if err := os.WriteFile(gate, nil, 0o600); err != nil {
				t.Fatal(err)
			}
			for name, result := range map[string]<-chan error{"start": started, "stop": stopped} {
				select {
				case err := <-result:
					if err != nil {
						t.Fatalf("%s failed: %v", name, err)
					}
				case <-time.After(5 * time.Second):
					t.Fatalf("%s remained blocked after listener readiness", name)
				}
			}
			if isProcessRunning(pid) {
				t.Fatal("stop left the newly started runtime alive")
			}
			for _, path := range []string{fixture.local.pidPath("demo"), fixture.local.buildPidPath("demo", "build-1")} {
				if _, err := os.Stat(path); !os.IsNotExist(err) {
					t.Fatalf("stop left a process record: %s, %v", path, err)
				}
			}
		})
	}
}
