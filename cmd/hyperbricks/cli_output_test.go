package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hyperbricks/hyperbricks/cmd/hyperbricks/commands"
)

func TestCLIOutput(t *testing.T) {
	if raw := os.Getenv("HB_CLI_OUTPUT_TEST"); raw != "" {
		var args []string
		if err := json.Unmarshal([]byte(raw), &args); err != nil {
			panic(err)
		}
		os.Args = append([]string{"hyperbricks"}, args...)
		run()
		os.Exit(commands.ExitCode)
	}
	tests := []struct {
		name           string
		args           []string
		code           int
		stdout, stderr string
		json           bool
	}{
		{"version", []string{"version"}, 0, "v", "", false},
		{"short verbose global", []string{"-v", "version"}, 0, "v", "", false},
		{"short verbose version", []string{"version", "-v"}, 0, "v", "", false},
		{"long verbose version", []string{"version", "--verbose"}, 0, "v", "", false},
		{"verbose level validation", []string{"version", "-v", "--log-level", "invalid"}, 1, "", "error: invalid log level", false},
		{"help", []string{"--help"}, 0, "Usage:", "", false},
		{"start help", []string{"start", "--help"}, 0, "Start server", "", false},
		{"doctor help", []string{"doctor", "--help"}, 0, "Diagnose whether a source module is healthy", "", false},
		{"deploy help", []string{"deploy"}, 0, "Available Commands", "", false},
		{"completion", []string{"completion", "bash"}, 0, "bash completion", "", false},
		{"unknown command", []string{"not-a-command"}, 1, "", "error: unknown command", false},
		{"retired scaffold CLI", []string{"scaffold-cli"}, 1, "", "error: unknown command", false},
		{"unknown flag", []string{"start", "--not-a-flag"}, 1, "", "error: unknown flag", false},
		{"missing module", []string{"start", "-m", "missing"}, 1, "", "error:", false},
		{"missing doctor module", []string{"doctor", "-m", "missing", "--json"}, 1, "\"status\":\"unhealthy\"", "", true},
		{"missing static module", []string{"static", "-m", "missing", "--force"}, 1, "", "error: read module config", false},
		{"retired deploy daemon", []string{"deploy-daemon"}, 1, "", "error: unknown command", false},
		{"no deploy daemon subcommand", []string{"deploy", "daemon"}, 1, "", "error: unknown command", false},
		{"no deploy daemon flag", []string{"deploy", "--daemon"}, 1, "", "error: unknown flag", false},
		{"retired start deploy flag", []string{"start", "--deploy-local"}, 1, "", "error: unknown flag", false},
		{"missing archive module", []string{"build", "-m", "missing"}, 1, "", "error:", false},
		{"missing plugin", []string{"plugin", "build", "missing@0.0.0"}, 1, "", "error:", false},
		{"bad plugin argument", []string{"plugin", "build", "missing"}, 1, "", "error:", false},
		{"bad log level", []string{"version", "--log-level", "nope"}, 1, "", "error: invalid log level", false},
		{"bad log format", []string{"version", "--log-format", "nope"}, 1, "", "error: invalid log format", false},
		{"global flags before start", []string{"--log-format", "json", "start", "-m", "demo"}, 1, "", "error: invalid log level", false},
		{"noninteractive wizard", []string{"build", "--non-interactive"}, 1, "", "error:", false},
		{"space log flag JSON", []string{"space", "--json", "--log-format", "invalid"}, 1, "error", "", true},
		{"space error JSON", []string{"space", "--dry-run", "--json"}, 1, "error", "", true},
		{"init", []string{"init", "-m", "demo"}, 0, "Module ready:", "File created", false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			args, _ := json.Marshal(test.args)
			child := exec.Command(os.Args[0], "-test.run=^TestCLIOutput$")
			child.Dir = t.TempDir()
			if test.name == "global flags before start" {
				module := filepath.Join(child.Dir, "modules", "demo")
				if err := os.MkdirAll(module, 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(module, "package.hyperbricks.yaml"), []byte("hyperbricks:\n  logger: {level: invalid}\n"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			child.Env = append(os.Environ(), "HB_CLI_OUTPUT_TEST="+string(args), "HB_NO_KEYBOARD=1", "NO_COLOR=1", "TERM=dumb")
			var stdout, stderr bytes.Buffer
			child.Stdout, child.Stderr = &stdout, &stderr
			_ = child.Run()
			if child.ProcessState == nil || child.ProcessState.ExitCode() != test.code {
				t.Fatalf("exit=%v want=%d stdout=%s stderr=%s", child.ProcessState, test.code, stdout.String(), stderr.String())
			}
			for _, stream := range []struct{ value, want string }{{stdout.String(), test.stdout}, {stderr.String(), test.stderr}} {
				if (stream.want == "" && stream.value != "") || (stream.want != "" && !strings.Contains(stream.value, stream.want)) {
					t.Fatalf("output=%q want=%q", stream.value, stream.want)
				}
				if strings.Contains(stream.value, "\x1b") || strings.Contains(stream.value, "loaded modules") {
					t.Fatalf("runtime/ANSI pollution: %q", stream.value)
				}
			}
			if test.json && !json.Valid(stdout.Bytes()) {
				t.Fatalf("invalid JSON stdout: %s", stdout.String())
			}
			if strings.Count(stderr.String(), "error:") > 1 {
				t.Fatalf("duplicate error: %s", stderr.String())
			}
		})
	}
}
