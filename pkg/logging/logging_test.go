package logging

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func TestConsoleFormat(t *testing.T) {
	var output bytes.Buffer
	ls := newLogger(&output, false)
	entry := zapcore.Entry{Time: time.Date(2026, 9, 15, 14, 7, 3, 4000000, time.UTC), Level: zap.InfoLevel, LoggerName: "server", Message: "Listening"}
	if err := ls.logger.Desugar().Core().Write(entry, []zap.Field{zap.Int("port", 8080)}); err != nil {
		t.Fatal(err)
	}
	want := "14:07:03.004  INFO   server  Listening  port=8080\n"
	if output.String() != want {
		t.Fatalf("console output:\n%q\nwant:\n%q", output.String(), want)
	}
}

func TestFileOutputLevelAndContext(t *testing.T) {
	var output bytes.Buffer
	ls := newLogger(&output, false)
	file := filepath.Join(t.TempDir(), "logs", "runtime.jsonl")
	if err := ls.dynamicSyncer.addFile(file); err != nil {
		t.Fatal(err)
	}
	if err := ls.dynamicSyncer.addFile(file); err != nil {
		t.Fatal(err)
	}
	logger := ls.logger.Named("server").With("module", "demo")
	logger.Debug("hidden")
	ls.atomicLevel.SetLevel(zap.DebugLevel)
	logger.Debugw("visible", "route", "index")
	ls.atomicLevel.SetLevel(zap.ErrorLevel)
	logger.Info("hidden too")
	logger.Errorw("failure", "error", errors.New("a cause"))
	if err := ls.dynamicSyncer.close(); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 2 || strings.Contains(string(data), "hidden") {
		t.Fatalf("file output: %s", data)
	}
	for _, line := range lines {
		var event map[string]interface{}
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			t.Fatal(err)
		}
		if event["module"] != "demo" || event["logger"] != "server" {
			t.Fatalf("lost context: %v", event)
		}
		if _, err := time.Parse(time.RFC3339Nano, event["time"].(string)); err != nil {
			t.Fatal(err)
		}
	}
	if len(ls.logBuffer) != 2 || ls.logBuffer[0].Fields["route"] != "index" || ls.logBuffer[0].Fields["module"] != "demo" {
		t.Fatalf("dashboard context: %#v", ls.logBuffer)
	}
	if strings.Contains(output.String(), "hidden") || strings.Contains(string(data), "\x1b") {
		t.Fatal("filter/color leaked")
	}
}

func TestConfigurationValidationAndJSON(t *testing.T) {
	var output bytes.Buffer
	ls := newLogger(&output, true)
	if err := ls.configure("nonsense", "console"); err == nil {
		t.Fatal("invalid level accepted")
	}
	if err := ls.configure("info", "nonsense"); err == nil {
		t.Fatal("invalid format accepted")
	}
	if err := ls.configure("warn", "json"); err != nil {
		t.Fatal(err)
	}
	ls.logger.Info("hidden")
	ls.logger.Warn("line one\nline two\x1b[2J")
	var event map[string]interface{}
	if err := json.Unmarshal(output.Bytes(), &event); err != nil {
		t.Fatal(err)
	}
	if event["message"] != "line one line two" || event["level"] != "warn" || event["logger"] != "runtime" {
		t.Fatalf("JSON event: %v", event)
	}
	parent := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(parent, nil, 0600); err != nil {
		t.Fatal(err)
	}
	if err := ls.dynamicSyncer.addFile(filepath.Join(parent, "not-a-directory.log")); err == nil {
		t.Fatal("file-open failure hidden")
	}
}

func TestContextRedactionAndConcurrentWith(t *testing.T) {
	var output bytes.Buffer
	ls := newLogger(&output, false)
	logger := ls.logger.With("authorization", "Bearer private", "metadata", map[string]string{"password": "private", "name": "kept"})
	logger.Errorw("failed at https://user:private@example.test/run?token=private\nnext", "error", errors.New("https://example.test/?secret=private"), "headers", map[string]string{"Cookie": "private"})
	if strings.Contains(output.String(), "private") || strings.Contains(output.String(), "\nnext") {
		t.Fatalf("unsafe event: %s", output.String())
	}
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(index int) { defer wg.Done(); logger.With("worker", index).Infow("complete", "result", "ok") }(i)
	}
	wg.Wait()
	if len(ls.logBuffer) != 10 {
		t.Fatalf("buffer is not bounded: %d", len(ls.logBuffer))
	}
	for _, record := range ls.logBuffer {
		if record.Fields["authorization"] != "[REDACTED]" || record.Fields["result"] != "ok" {
			t.Fatalf("context: %#v", record)
		}
	}
	fields := cloneFields(ls.logBuffer[0].Fields)
	fields["metadata"].(map[string]interface{})["name"] = "changed"
	if ls.logBuffer[0].Fields["metadata"].(map[string]interface{})["name"] != "kept" {
		t.Fatal("snapshot aliases stored context")
	}
	for len(ls.logsCh) > 0 {
		entry := <-ls.logsCh
		entry.Fields["metadata"].(map[string]interface{})["name"] = "changed"
	}
	if ls.logBuffer[0].Fields["metadata"].(map[string]interface{})["name"] != "kept" {
		t.Fatal("stream aliases stored context")
	}
}

func TestTerminalPolicy(t *testing.T) {
	var output bytes.Buffer
	if IsTerminal(&output) || ColorEnabled(&output) {
		t.Fatal("pipe treated as terminal")
	}
	WriteHeading(&output, "v1.2.5")
	if output.Len() != 0 {
		t.Fatal("heading in pipe")
	}
	WriteError(&output, errors.New("failure\nsecond line\x1b[31m"))
	if output.String() != "error: failure second line\n" {
		t.Fatalf("error output: %q", output.String())
	}
	for _, color := range []bool{false, true} {
		output.Reset()
		ls := newLogger(&output, color)
		ls.logger.Info("ready")
		if strings.Contains(output.String(), "\x1b[") != color {
			t.Fatalf("color=%t output=%q", color, output.String())
		}
	}
}

func TestFatalFlush(t *testing.T) {
	if file := os.Getenv("HB_LOGGING_FATAL_TEST"); file != "" {
		ls := newLogger(os.Stderr, false)
		if err := ls.dynamicSyncer.addFile(file); err != nil {
			panic(err)
		}
		ls.logger.Fatalw("fatal fixture", "detail", "saved")
		return
	}
	file := filepath.Join(t.TempDir(), "fatal.jsonl")
	child := exec.Command(os.Args[0], "-test.run=^TestFatalFlush$")
	child.Env = append(os.Environ(), "HB_LOGGING_FATAL_TEST="+file)
	output, err := child.CombinedOutput()
	var exit *exec.ExitError
	if !errors.As(err, &exit) || exit.ExitCode() != 1 {
		t.Fatalf("exit=%v output=%s", err, output)
	}
	data, err := os.ReadFile(file)
	if err != nil || !json.Valid(data) || !bytes.Contains(data, []byte("saved")) {
		t.Fatalf("fatal file: %s %v", data, err)
	}
}
