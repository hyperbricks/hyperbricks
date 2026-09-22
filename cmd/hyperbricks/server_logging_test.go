package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/hyperbricks/hyperbricks/cmd/hyperbricks/commands"
	"github.com/hyperbricks/hyperbricks/pkg/logging"
	"github.com/hyperbricks/hyperbricks/pkg/shared"
	"go.uber.org/zap/zapcore"
)

func TestRenderFailureLogsInEveryModeWithoutRetainingLiveOutcomes(t *testing.T) {
	for _, mode := range []string{shared.LIVE_MODE, shared.DEVELOPMENT_MODE, shared.DEBUG_MODE} {
		t.Run(mode, func(t *testing.T) {
			setupDevelopmentModeServeContentTest(t, false)
			getHyperBricksConfiguration().Mode = mode
			installDiagnosticTestRoute(t, func(context.Context) (string, []error) {
				return "partial", []error{shared.ComponentError{Type: "<TEMPLATE>", File: "page.hyperbricks.yaml", Path: "page.content", Line: 8, Column: 3, Resource: "page.html", Phase: "render", Err: "template execution failed"}}
			})
			request := httptest.NewRequest(http.MethodGet, "/page?token=do-not-log", nil)
			request.Header.Set("Authorization", "Bearer do-not-log")
			response := httptest.NewRecorder()
			started := time.Now()
			ServeContent(response, request)
			if response.Header().Get(renderErrorCountHeader) != "1" || response.Header().Get("Cache-Control") != "no-store" {
				t.Fatalf("failure headers: %v", response.Header())
			}
			count := 0
			for _, entry := range logging.GetLogs() {
				if entry.Time.Before(started) || entry.Fields["request_id"] != response.Header().Get(requestIDHeader) {
					continue
				}
				count++
				if entry.Level != zapcore.ErrorLevel || entry.Fields["file"] != "page.hyperbricks.yaml" || entry.Fields["path"] != "page.content" || entry.Fields["error"] != "template execution failed" {
					t.Fatalf("incomplete event: %#v", entry)
				}
				encoded, _ := json.Marshal(entry)
				if strings.Contains(string(encoded), "do-not-log") {
					t.Fatalf("secret in event: %s", encoded)
				}
				if mode == shared.LIVE_MODE && entry.Fields["diagnostics_url"] != nil {
					t.Fatal("live diagnostics link")
				}
			}
			if count != 1 {
				t.Fatalf("failure log count=%d want=1", count)
			}
			if mode == shared.LIVE_MODE && (len(renderDiagnostics) != 0 || len(renderDiagnosticsOrder) != 0) {
				t.Fatal("live diagnostics retained")
			}
		})
	}
}

func TestRuntimeLoggingConfiguration(t *testing.T) {
	oldLevel, oldFormat, oldNonInteractive := commands.LogLevel, commands.LogFormat, commands.NonInteractive
	oldVerbose := commands.Verbose
	commands.Verbose = false
	commands.LogLevel, commands.LogFormat, commands.NonInteractive = "", "", true
	t.Cleanup(func() {
		_ = logging.Close()
		_ = logging.Configure("info", "console")
		commands.LogLevel, commands.LogFormat, commands.NonInteractive = oldLevel, oldFormat, oldNonInteractive
		commands.Verbose = oldVerbose
	})
	file := filepath.Join(t.TempDir(), "configured.jsonl")
	config := &shared.Config{Mode: shared.DEBUG_MODE, Logger: shared.LoggerConfig{Path: file}}
	if err := configureRuntimeLogging(config); err != nil {
		t.Fatal(err)
	}
	if !logging.GetLogger().Desugar().Core().Enabled(zapcore.DebugLevel) {
		t.Fatal("debug default ignored")
	}
	logging.GetLogger().Debugw("configured output", "module", "fixture")
	if err := logging.Close(); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(file)
	if err != nil || !json.Valid(data) || !strings.Contains(string(data), "configured output") {
		t.Fatalf("file=%s error=%v", data, err)
	}
	config.Logger.Level = "error"
	config.Logger.Path = ""
	if err := configureRuntimeLogging(config); err != nil {
		t.Fatal(err)
	}
	if logging.GetLogger().Desugar().Core().Enabled(zapcore.InfoLevel) {
		t.Fatal("package level ignored")
	}
	commands.Verbose = true
	if err := configureRuntimeLogging(config); err != nil {
		t.Fatal(err)
	}
	if !logging.GetLogger().Desugar().Core().Enabled(zapcore.DebugLevel) {
		t.Fatal("verbose must override package level")
	}
	commands.LogLevel = "warn"
	if err := configureRuntimeLogging(config); err != nil {
		t.Fatal(err)
	}
	if logging.GetLogger().Desugar().Core().Enabled(zapcore.InfoLevel) {
		t.Fatal("explicit log level must override verbose")
	}
	commands.LogLevel = "debug"
	commands.LogFormat = "json"
	if err := configureRuntimeLogging(config); err != nil {
		t.Fatal(err)
	}
	if !logging.GetLogger().Desugar().Core().Enabled(zapcore.DebugLevel) {
		t.Fatal("CLI level override ignored")
	}
	commands.LogLevel = "nope"
	if err := configureRuntimeLogging(config); err == nil {
		t.Fatal("invalid level accepted")
	}
	commands.LogLevel = "info"
	config.Logger.Path = filepath.Join(file, "not-a-directory")
	if err := configureRuntimeLogging(config); err == nil {
		t.Fatal("file-open error hidden")
	}
}

func TestRenderDiagnosticSeverity(t *testing.T) {
	setupDevelopmentModeServeContentTest(t, false)
	for _, test := range []struct {
		level    string
		rejected bool
		want     zapcore.Level
	}{{"WARNING", false, zapcore.WarnLevel}, {"WARN", false, zapcore.WarnLevel}, {"INFO", false, zapcore.InfoLevel}, {"WARN", true, zapcore.ErrorLevel}} {
		logRenderDiagnostics(nil, "severity-test", "page", []error{shared.ComponentError{Err: "fixture", Level: test.level, Rejected: test.rejected}})
		entries := logging.GetLogs()
		last := entries[len(entries)-1]
		if last.Fields["request_id"] != "severity-test" || last.Level != test.want {
			t.Fatalf("level=%s rejected=%t: %#v", test.level, test.rejected, last)
		}
	}
}
