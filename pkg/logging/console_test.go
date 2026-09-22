package logging

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func TestConsoleModuleSummary(t *testing.T) {
	var output bytes.Buffer
	ls := newLogger(&output, false)
	entry := zapcore.Entry{Time: time.Date(2026, 9, 15, 23, 17, 57, 836000000, time.UTC), Level: zap.InfoLevel, LoggerName: "runtime", Message: "Module configured"}
	if err := ls.logger.Desugar().Core().Write(entry, []zap.Field{
		zap.String("module", "hyperbricks-landing"),
		zap.String("config", "profiles/development.hyperbricks.yaml"),
		zap.String("mode", "development"), zap.Int("gomaxprocs", 8),
	}); err != nil {
		t.Fatal(err)
	}
	want := "23:17:57.836  INFO   runtime  Module configured  module=hyperbricks-landing config=profiles/development.hyperbricks.yaml mode=development gomaxprocs=8\n"
	if output.String() != want {
		t.Fatalf("module summary:\n%s\nwant:\n%s", output.String(), want)
	}
}

func TestConsoleGroupedRoutes(t *testing.T) {
	var output bytes.Buffer
	ls := newLogger(&output, false)
	file := filepath.Join(t.TempDir(), "events.jsonl")
	if err := ls.dynamicSyncer.addFile(file); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ls.dynamicSyncer.close() })
	entry := zapcore.Entry{Time: time.Date(2026, 9, 15, 23, 5, 3, 841000000, time.UTC), Level: zap.InfoLevel, LoggerName: "routes", Message: "Routes registered"}
	fields := []zap.Field{zap.Any("sources", map[string][]map[string]string{"hyperbricks/landing.hyperbricks.yaml": {
		{"route": "/", "type": "<HYPERMEDIA>", "static": "index.html"},
		{"route": "/de", "type": "<HYPERMEDIA>", "static": "de/index.html"},
		{"route": "/status", "type": "<FRAGMENT>"},
	}})}
	if err := ls.logger.Desugar().Core().Write(entry, fields); err != nil {
		t.Fatal(err)
	}
	want := "23:05:03.841  INFO   routes  Routes registered  sources={1}\n"
	if output.String() != want {
		t.Fatalf("console:\n%s\nwant:\n%s", output.String(), want)
	}
	if err := ls.dynamicSyncer.Sync(); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	var event map[string]interface{}
	if err := json.Unmarshal(data, &event); err != nil {
		t.Fatalf("file lost JSON: %s: %v", data, err)
	}
	sources, ok := event["sources"].(map[string]interface{})
	if !ok {
		t.Fatalf("missing source groups: %v", event)
	}
	rows, ok := sources["hyperbricks/landing.hyperbricks.yaml"].([]interface{})
	if !ok || len(rows) != 3 || rows[0].(map[string]interface{})["route"] != "/" {
		t.Fatalf("incomplete JSON: %v", event)
	}
	if len(ls.logBuffer) != 1 || len(ls.logBuffer[0].Fields["sources"].(map[string]interface{})["hyperbricks/landing.hyperbricks.yaml"].([]interface{})) != 3 {
		t.Fatalf("dashboard lost grouped data: %#v", ls.logBuffer)
	}
	output.Reset()
	if err := ls.configure("info", "json"); err != nil {
		t.Fatal(err)
	}
	if err := ls.logger.Desugar().Core().Write(entry, fields); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(data, output.Bytes()) {
		t.Fatalf("explicit JSON differs from file:\n%s\n%s", output.Bytes(), data)
	}
}

func TestConsoleNestedContextAndLongRows(t *testing.T) {
	var output bytes.Buffer
	ls := newLogger(&output, false)
	long := "hyperbricks/" + strings.Repeat("nested/", 15) + "page.hyperbricks.yaml"
	ls.logger.With("module", "demo").Named("render").Errorw("Render failed\nunsafe\x1b[2J",
		"file", "hyperbricks/page.hyperbricks.yaml", "request_id", "hb-12", "line", 42,
		"directories", []string{"hyperbricks", "templates"}, "empty", []string{},
		"metadata", map[string]interface{}{"password": "private", "nested": map[string]interface{}{"cause": "missing template"}},
		"rows", []map[string]interface{}{{"file": long, "line": 42}}, "bad\nkey\x1b[2J", "kept")
	text := output.String()
	for _, want := range []string{"Module:", "demo", "File:", "hyperbricks/page.hyperbricks.yaml", "Request ID:", "hb-12", "Line:", "42", "- hyperbricks\n", "- templates\n", "(none)", "Password:", "[REDACTED]", "Cause:", "missing template", "Rows:", "  1.\n", long, "bad key:"} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %q in:\n%s", want, text)
		}
	}
	for _, unwanted := range []string{"private", "\x1b", "map[", "{\"", "\nunsafe", "bad\nkey"} {
		if strings.Contains(text, unwanted) {
			t.Fatalf("unexpected %q in:\n%s", unwanted, text)
		}
	}
}

func TestConsoleConcurrentBlocks(t *testing.T) {
	var output bytes.Buffer
	ls := newLogger(&output, false)
	var workers sync.WaitGroup
	for i := 0; i < 40; i++ {
		workers.Add(1)
		go func(i int) {
			defer workers.Done()
			ls.logger.With("worker", i).Infow(fmt.Sprintf("Worker %d", i), "result", "complete")
		}(i)
	}
	workers.Wait()
	lines := strings.Split(strings.TrimSuffix(output.String(), "\n"), "\n")
	if len(lines) != 40 {
		t.Fatalf("got %d lines", len(lines))
	}
	for _, line := range lines {
		if !strings.Contains(line, "Worker ") || !strings.Contains(line, "worker=") || !strings.HasSuffix(line, "result=complete") {
			t.Fatalf("incomplete line: %q", line)
		}
	}
}

func TestConsoleSeverityColor(t *testing.T) {
	for _, level := range []zapcore.Level{zap.DebugLevel, zap.InfoLevel, zap.WarnLevel, zap.ErrorLevel} {
		var output bytes.Buffer
		ls := newLogger(&output, true)
		if err := ls.configure("debug", "console"); err != nil {
			t.Fatal(err)
		}
		ls.logger.Desugar().Log(level, "message", zap.String("file", "page.yaml"))
		line := strings.Split(strings.TrimSuffix(output.String(), "\n"), "\n")[0]
		if !strings.Contains(line, consoleLevel(level, true)) {
			t.Fatalf("severity color: %q", line)
		}
	}
}

func TestConsoleUsesOneInfoColor(t *testing.T) {
	var output bytes.Buffer
	ls := newLogger(&output, true)
	var workers sync.WaitGroup
	for i := 0; i < 40; i++ {
		workers.Add(1)
		go func(i int) {
			defer workers.Done()
			ls.logger.Named("runtime").With("worker", i).Infow("Group", "result", "done")
		}(i)
	}
	workers.Wait()
	lines := strings.Split(strings.TrimSuffix(output.String(), "\n"), "\n")
	if len(lines) != 40 {
		t.Fatalf("line count=%d", len(lines))
	}
	for _, line := range lines {
		if !strings.Contains(line, "\x1b[36mINFO ") || strings.Contains(line, "\x1b[94m") || strings.Contains(line, "\x1b[2m") || strings.Contains(line, "\x1b[22m") {
			t.Fatalf("incorrect INFO color: %q", line)
		}
	}
}
