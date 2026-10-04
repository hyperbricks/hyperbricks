//go:build darwin || linux

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/hyperbricks/hyperbricks/assets"
	"github.com/hyperbricks/hyperbricks/cmd/hyperbricks/commands"
	"go.yaml.in/yaml/v4"
)

// Re-enter the actual command/runtime boundary in an isolated process. Hooks
// themselves run the existing real-process helper, never this CLI entrypoint.
func TestRuntimeProcessCLIHelper(t *testing.T) {
	raw := os.Getenv("HB_RUNTIME_PROCESS_GATE_ARGS")
	if raw == "" {
		return
	}
	var args []string
	if err := json.Unmarshal([]byte(raw), &args); err != nil {
		t.Fatal(err)
	}
	os.Args = append([]string{"hyperbricks"}, args...)
	run()
	os.Exit(commands.ExitCode)
}

func runtimeProcessGateFixture(t *testing.T, mode string, port int, development map[string]any) (string, string) {
	t.Helper()
	root := t.TempDir()
	module := filepath.Join(root, "modules", "gates")
	for _, directory := range []string{"hyperbricks", "templates", "resources", "static", "rendered"} {
		if err := os.MkdirAll(filepath.Join(module, directory), 0755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.MkdirAll(filepath.Join(root, "bin", "plugins"), 0755); err != nil {
		t.Fatal(err)
	}
	development["watch"] = false
	development["dashboard"] = map[string]any{"enabled": false}
	development["frontend_editing"] = map[string]any{"enabled": false}
	config := map[string]any{"hyperbricks": map[string]any{
		"mode":     mode,
		"metadata": map[string]any{"module": "gates", "moduleversion": "1.0.0", "hyperbricks": strings.TrimSpace(assets.VersionMD)},
		"server":   map[string]any{"port": port}, "rate_limit": map[string]any{"enabled": false},
		"development": development,
	}}
	content, err := yaml.Marshal(config)
	if err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(module, "package.hyperbricks.yaml"), string(content))
	writeTestFile(t, filepath.Join(module, "hyperbricks", "app.hyperbricks.yaml"), `page:
  - type: hypermedia
  - route: index
  - title: Lifecycle fixture
  - content:
      - type: text
      - value: Process gate fixture
`)
	return root, module
}

func runtimeProcessGateTask(t *testing.T, marker, mode string) map[string]any {
	t.Helper()
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	return map[string]any{"name": "prepare", "command": []string{executable, "-test.run=^TestDevelopmentProcessHelper$", "--"},
		"timeout": "3s", "env": map[string]string{"HB_PROCESS_TEST_MODE": mode, "HB_PROCESS_TEST_FILE": marker}}
}

func runtimeProcessGateDevelopment(t *testing.T, marker string) map[string]any {
	t.Helper()
	before := runtimeProcessGateTask(t, marker, "record")
	after := runtimeProcessGateTask(t, marker, "record")
	after["name"] = "verify"
	service := runtimeProcessGateTask(t, marker, "record")
	service["name"] = "local-api"
	delete(service, "timeout")
	service["ready"] = map[string]any{"http": "http://127.0.0.1:1/health"}
	return map[string]any{
		"hooks":    map[string]any{"before_start": []any{before}, "after_start": []any{after}},
		"services": []any{service},
	}
}

func runRuntimeProcessGateCLI(t *testing.T, dir string, environment []string, args ...string) (int, string) {
	t.Helper()
	encoded, err := json.Marshal(args)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestRuntimeProcessCLIHelper$")
	command.Cancel = func() error { return command.Process.Signal(syscall.SIGTERM) }
	command.Dir = dir
	for _, value := range os.Environ() {
		name, _, _ := strings.Cut(value, "=")
		if strings.HasPrefix(name, "HB_DEPLOY_") || name == "HB_PRODUCTION" || name == "HB_RUNTIME_PROCESS_GATE_ARGS" || name == "HB_PROCESS_TEST_MODE" {
			continue
		}
		command.Env = append(command.Env, value)
	}
	command.Env = append(command.Env, "HB_RUNTIME_PROCESS_GATE_ARGS="+string(encoded), "HB_NO_KEYBOARD=1", "NO_COLOR=1", "TERM=dumb", "GORACE="+os.Getenv("GORACE")+" atexit_sleep_ms=0")
	command.Env = append(command.Env, environment...)
	command.WaitDelay = 7 * time.Second
	var output bytes.Buffer
	command.Stdout, command.Stderr = &output, &output
	err = command.Run()
	if ctx.Err() != nil {
		t.Fatalf("CLI failed to finish: %v\n%s", ctx.Err(), output.String())
	}
	if command.ProcessState == nil {
		t.Fatalf("CLI did not start: %v\n%s", err, output.String())
	}
	return command.ProcessState.ExitCode(), output.String()
}

func runtimeProcessGateNoMarker(t *testing.T, marker string) {
	t.Helper()
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("disabled process created marker or marker could not be checked: %v", err)
	}
}

func TestRuntimeProcessCLIGates(t *testing.T) {
	for _, tc := range []struct {
		name, mode string
		optIn      bool
		env        []string
		want       string
	}{
		{"disabled direct start", "development", false, nil, "Development hooks and services are disabled"},
		{"disabled managed start", "development", false, []string{"HB_DEPLOY_BUILD_ID=gate-test"}, "Development hooks and services are disabled"},
		{"live rejects opt-in", "live", true, nil, "--with-processes requires development or debug mode"},
		{"managed rejects opt-in", "development", true, []string{"HB_DEPLOY_BUILD_ID=gate-test"}, "--with-processes is unavailable for deployment-managed runtimes"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// The occupied main port ends a disabled start after initialization,
			// proving its hook was skipped without requiring a long-lived CLI.
			occupied, err := net.Listen("tcp", ":0")
			if err != nil {
				t.Fatal(err)
			}
			defer occupied.Close()
			marker := filepath.Join(t.TempDir(), "ran.json")
			development := runtimeProcessGateDevelopment(t, marker)
			root, _ := runtimeProcessGateFixture(t, tc.mode, occupied.Addr().(*net.TCPAddr).Port, development)
			args := []string{"start", "-m", "gates"}
			if tc.optIn {
				args = append(args, "--with-processes")
			}
			code, output := runRuntimeProcessGateCLI(t, root, tc.env, args...)
			if code != 1 || !strings.Contains(output, tc.want) {
				t.Fatalf("exit=%d; want 1 and %q\n%s", code, tc.want, output)
			}
			if !tc.optIn && !strings.Contains(output, "start listener on port") {
				t.Fatalf("disabled start never reached ordinary HTTP startup\n%s", output)
			}
			runtimeProcessGateNoMarker(t, marker)
		})
	}
}

func TestRuntimeProcessCLIInspectionDoesNotExecute(t *testing.T) {
	for _, tc := range []struct {
		name string
		args []string
	}{
		{"author", []string{"author", "context", "-m", "gates", "--json"}},
		{"doctor", []string{"doctor", "-m", "gates", "--json"}},
		{"static", []string{"static", "-m", "gates", "--force"}},
		{"build", []string{"build", "-m", "gates", "--force"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			marker := filepath.Join(t.TempDir(), "ran.json")
			development := runtimeProcessGateDevelopment(t, marker)
			root, _ := runtimeProcessGateFixture(t, "development", 8080, development)
			code, output := runRuntimeProcessGateCLI(t, root, nil, tc.args...)
			if code != 0 {
				t.Fatalf("inspection/export exit=%d\n%s", code, output)
			}
			runtimeProcessGateNoMarker(t, marker)
		})
	}
}

func TestRuntimeProcessCLIInvalidConfigurationWithoutOptIn(t *testing.T) {
	for _, tc := range []struct {
		name, field, want string
		value             any
	}{
		{"scalar command", "command", "command must be a nonempty string argument list", "echo marker"},
		{"numeric argument", "command", "command[1] must be a string", []any{"echo", 42}},
		{"nonpositive duration", "timeout", "timeout must be a positive", "0s"},
		{"unknown field", "typo", "typo is an unknown field", "value"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			marker := filepath.Join(t.TempDir(), "ran.json")
			task := runtimeProcessGateTask(t, marker, "record")
			task[tc.field] = tc.value
			root, _ := runtimeProcessGateFixture(t, "development", 8080, map[string]any{"hooks": map[string]any{"before_start": []any{task}}})
			code, output := runRuntimeProcessGateCLI(t, root, nil, "start", "-m", "gates")
			if code != 1 || !strings.Contains(output, "hyperbricks.development.hooks.before_start[0]."+tc.want) {
				t.Fatalf("exit=%d; want full field error %q\n%s", code, tc.want, output)
			}
			runtimeProcessGateNoMarker(t, marker)
		})
	}
}

func TestRuntimeProcessCLIBeforeStartFailureDoesNotListen(t *testing.T) {
	address := developmentTestAddress(t)
	_, rawPort, _ := net.SplitHostPort(address)
	port, _ := strconv.Atoi(rawPort)
	root, _ := runtimeProcessGateFixture(t, "development", port, map[string]any{
		"hooks": map[string]any{"before_start": []any{runtimeProcessGateTask(t, filepath.Join(t.TempDir(), "unused"), "fail")}},
	})
	code, output := runRuntimeProcessGateCLI(t, root, nil, "start", "-m", "gates", "--with-processes")
	if code != 1 || !strings.Contains(output, "exit status 7") || !strings.Contains(output, "useful failure diagnostic") {
		t.Fatalf("exit=%d; want hook failure\n%s", code, output)
	}
	if strings.Contains(output, "Listening") {
		t.Fatalf("HTTP started despite failed preparation\n%s", output)
	}
	listener, err := net.Listen("tcp", address)
	if err != nil {
		t.Fatalf("HTTP address remains occupied after preparation failure: %v", err)
	}
	listener.Close()
}

func TestRuntimeProcessCLIRequiredResolverFailsBeforeExecution(t *testing.T) {
	for _, optIn := range []bool{false, true} {
		t.Run(strconv.FormatBool(optIn), func(t *testing.T) {
			marker := filepath.Join(t.TempDir(), "ran.json")
			task := runtimeProcessGateTask(t, marker, "record")
			environment := map[string]any{}
			for key, value := range task["env"].(map[string]string) {
				environment[key] = value
			}
			// HB_DEPLOY_* is removed from the subprocess environment by the
			// harness, making this missing required value deterministic.
			environment["REQUIRED_VALUE"] = map[string]any{"env": map[string]any{"name": "HB_DEPLOY_MISSING_GATE_VALUE", "required": true}}
			task["env"] = environment
			root, _ := runtimeProcessGateFixture(t, "development", 8080, map[string]any{"hooks": map[string]any{"before_start": []any{task}}})
			args := []string{"start", "-m", "gates"}
			if optIn {
				args = append(args, "--with-processes")
			}
			code, output := runRuntimeProcessGateCLI(t, root, nil, args...)
			if code != 1 || !strings.Contains(output, "env_missing at hyperbricks.development.hooks.before_start[0].env.REQUIRED_VALUE:") {
				t.Fatalf("exit=%d; required input did not retain its source diagnostic\n%s", code, output)
			}
			if strings.Contains(output, "Listening") {
				t.Fatalf("HTTP started despite required input failure\n%s", output)
			}
			runtimeProcessGateNoMarker(t, marker)
		})
	}
}

func TestRuntimeProcessCLIListenerFailureCleansServices(t *testing.T) {
	for _, tc := range []struct{ name, network, address string }{
		{"wildcard", "tcp", ":0"},
		{"IPv4 loopback", "tcp4", "127.0.0.1:0"},
		{"IPv6 loopback", "tcp6", "[::1]:0"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			occupied, err := net.Listen(tc.network, tc.address)
			if err != nil {
				if tc.network == "tcp6" {
					t.Skipf("IPv6 unavailable: %v", err)
				}
				t.Fatal(err)
			}
			defer occupied.Close()
			address := developmentTestAddress(t)
			pidFile := filepath.Join(t.TempDir(), "service.pid")
			t.Cleanup(func() {
				// Keep the suite contained even when a broken implementation fails
				// before the normal child-reaping assertions can run.
				if raw, err := os.ReadFile(pidFile); err == nil {
					if pid, err := strconv.Atoi(strings.TrimSpace(string(raw))); err == nil && pid > 0 && syscall.Kill(pid, 0) == nil {
						_ = syscall.Kill(-pid, syscall.SIGKILL)
					}
				}
			})
			service := runtimeProcessGateTask(t, pidFile, "serve")
			delete(service, "timeout")
			service["name"] = "local-api"
			service["env"].(map[string]string)["HB_PROCESS_TEST_ADDRESS"] = address
			service["ready"] = map[string]any{"http": "http://" + address + "/health", "timeout": "3s"}
			service["stop_timeout"] = "1s"
			afterMarker := filepath.Join(t.TempDir(), "after-start.json")
			after := runtimeProcessGateTask(t, afterMarker, "record")
			after["name"] = "verify"
			root, _ := runtimeProcessGateFixture(t, "development", occupied.Addr().(*net.TCPAddr).Port, map[string]any{
				"services": []any{service},
				"hooks":    map[string]any{"after_start": []any{after}},
			})
			code, output := runRuntimeProcessGateCLI(t, root, nil, "start", "-m", "gates", "--with-processes")
			if code != 1 || !strings.Contains(output, "start listener on port") {
				t.Fatalf("exit=%d; want listener failure after readiness\n%s", code, output)
			}
			runtimeProcessGateNoMarker(t, afterMarker)
			if strings.Contains(output, "Listening") {
				t.Fatalf("announced HTTP serving despite occupied port\n%s", output)
			}
			pidBytes, err := os.ReadFile(pidFile)
			if err != nil {
				t.Fatalf("managed API did not start before bind failure: %v\n%s", err, output)
			}
			pid, err := strconv.Atoi(strings.TrimSpace(string(pidBytes)))
			if err != nil || pid <= 0 {
				t.Fatalf("invalid owned child PID %q", pidBytes)
			}
			if err := syscall.Kill(pid, 0); err != syscall.ESRCH {
				// This exact child PID was created by this isolated test invocation.
				_ = syscall.Kill(-pid, syscall.SIGKILL)
				t.Fatalf("managed API %d still exists after CLI cleanup: %v\n%s", pid, err, output)
			}
			listener, err := net.Listen("tcp", address)
			if err != nil {
				t.Fatalf("managed API port was not released: %v\n%s", err, output)
			}
			listener.Close()
			connection, err := net.DialTimeout("tcp", occupied.Addr().String(), time.Second)
			if err != nil {
				t.Fatalf("unrelated occupied listener was disturbed: %v", err)
			}
			connection.Close()
		})
	}
}
