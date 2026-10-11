package commands

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/hyperbricks/hyperbricks/pkg/language"
)

func executeLintJSON(t *testing.T, args ...string) (lintReport, string, int) {
	t.Helper()
	previousExit, previousCode := Exit, ExitCode
	t.Cleanup(func() { Exit, ExitCode = previousExit, previousCode })
	cmd := NewLintCommand()
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs(append(args, "--json"))
	if err := cmd.Execute(); err != nil {
		t.Fatalf("lint failed: %v; stderr=%s", err, stderr.String())
	}
	var report lintReport
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("invalid JSON: %v; stdout=%s", err, stdout.String())
	}
	if stderr.Len() != 0 || !Exit {
		t.Fatalf("stderr=%s; Exit=%v", stderr.String(), Exit)
	}
	return report, stdout.String(), ExitCode
}

func writeLintFixture(t *testing.T, path, source string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(source), 0o640); err != nil {
		t.Fatal(err)
	}
}

func TestLintCommandModuleJSONIsSharedDeterministicAndReadOnly(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)
	source := "page:\n  - type: 'hypermedia'\n  - route: \"events\"\n  - title: Event calendar\n"
	writeLintFixture(t, "modules/default/hyperbricks/pages/events.hyperbricks.yaml", source)
	writeLintFixture(t, "modules/default/package.hyperbricks.yaml", "hyperbricks:\n  mode: development\n  metadata:\n    moduleversion: 1.0.0\n")
	writeLintFixture(t, "modules/default/resources/ignored.yaml", "value: 'not a HyperBricks source file'\n")
	before := snapshotDoctorTree(t, root)
	report, raw, code := executeLintJSON(t)
	if report.SchemaVersion != 1 || report.Status != "warning" || report.FilesChecked != 2 || report.Warnings != 2 || report.Errors != 0 || code != 0 {
		t.Fatalf("report=%#v; exit=%d", report, code)
	}
	for i, diagnostic := range language.LintDocument(source) {
		got := report.Diagnostics[i]
		if got.File != "modules/default/hyperbricks/pages/events.hyperbricks.yaml" || got.Range != diagnostic.Range || got.Code != diagnostic.Code || got.Message != diagnostic.Message || got.Severity != "warning" || got.Line != diagnostic.Range.Start.Line+1 || got.Column != diagnostic.Range.Start.Character+1 {
			t.Fatalf("CLI/shared mismatch: %#v; want %#v", got, diagnostic)
		}
	}
	_, repeated, _ := executeLintJSON(t)
	if repeated != raw {
		t.Fatalf("output is not deterministic:\n%s\n%s", raw, repeated)
	}
	strictReport, _, code := executeLintJSON(t, "--strict")
	if !strictReport.Strict || strictReport.Status != "warning" || code != 1 {
		t.Fatalf("strict report=%#v; exit=%d", strictReport, code)
	}
	if after := snapshotDoctorTree(t, root); !reflect.DeepEqual(before, after) {
		t.Fatal("lint changed source files, permissions, or modification times")
	}
}

func TestLintCommandAcceptsGeneratedInitMetadata(t *testing.T) {
	t.Chdir(t.TempDir())
	if err := initializeModule("demo"); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join("modules", "demo", PackageConfigFileName)
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(content), "moduleversion: 1.0.0\n") {
		t.Fatalf("init did not write a plain module version:\n%s", content)
	}
	report, raw, code := executeLintJSON(t, path, "--strict")
	if report.Status != "clean" || report.FilesChecked != 1 || code != 0 {
		t.Fatalf("generated metadata should pass lint: %s; exit=%d", raw, code)
	}
}

func TestLintCommandExplicitFilesSyntaxErrorsAndNecessaryQuotes(t *testing.T) {
	t.Chdir(t.TempDir())
	writeLintFixture(t, "a.yaml", "value: 'hello'\n")
	writeLintFixture(t, "b.yaml", "value: 'unterminated\n")
	writeLintFixture(t, "clean.yaml", "value: '001'\nother: '#target'\n")
	report, _, code := executeLintJSON(t, "b.yaml", "a.yaml", "./a.yaml")
	if report.FilesChecked != 2 || report.Warnings != 1 || report.Errors != 1 || report.Status != "error" || code != 1 {
		t.Fatalf("report=%#v; exit=%d", report, code)
	}
	if report.Diagnostics[0].File != "a.yaml" || report.Diagnostics[1].Code != "yaml.syntax" {
		t.Fatalf("diagnostics=%#v", report.Diagnostics)
	}
	report, raw, code := executeLintJSON(t, "clean.yaml", "--strict")
	if report.Status != "clean" || report.FilesChecked != 1 || code != 0 || !strings.Contains(raw, `"diagnostics":[]`) {
		t.Fatalf("clean report=%s; exit=%d", raw, code)
	}
}

func TestLintCommandModuleVersionTypeErrorsAndQuoteWarnings(t *testing.T) {
	t.Chdir(t.TempDir())
	for _, test := range []struct {
		value    string
		errors   int
		warnings int
	}{
		{"1.0.0", 0, 0},
		{`"1.0.0"`, 0, 1},
		{`'1.0.0'`, 0, 1},
		{`"1.0"`, 0, 0},
		{`'1.0'`, 0, 0},
		{"1.2.3-beta.2+build.4", 0, 0},
		{"1.0", 1, 0},
		{"1", 1, 0},
		{"true", 1, 0},
		{"null", 1, 0},
		{"[1.0.0]", 1, 0},
	} {
		t.Run(test.value, func(t *testing.T) {
			source := "hyperbricks:\n  metadata:\n    moduleversion: " + test.value + "\n"
			path := "modules/demo/package.hyperbricks.yaml"
			writeLintFixture(t, path, source)
			before := snapshotDoctorTree(t, ".")
			for _, strict := range []bool{false, true} {
				args := []string{"-m", "demo"}
				wantCode := 0
				if strict {
					args = append(args, "--strict")
				}
				if test.errors > 0 || strict && test.warnings > 0 {
					wantCode = 1
				}
				report, raw, code := executeLintJSON(t, args...)
				if report.Errors != test.errors || report.Warnings != test.warnings || code != wantCode || report.FilesChecked != 1 {
					t.Fatalf("report=%s, exit=%d; want errors=%d warnings=%d exit=%d", raw, code, test.errors, test.warnings, wantCode)
				}
				for i, want := range language.LintDocument(source) {
					got := report.Diagnostics[i]
					if got.Code != want.Code || got.Range != want.Range || got.Message != want.Message {
						t.Fatalf("CLI/shared diagnostic mismatch: got %#v, want %#v", got, want)
					}
				}
				if test.errors != 0 && (report.Status != "error" || report.Diagnostics[0].Severity != "error") {
					t.Fatalf("wrong type must be an error: %s", raw)
				}
			}
			if after := snapshotDoctorTree(t, "."); !reflect.DeepEqual(before, after) {
				t.Fatal("type check modified the source")
			}
		})
	}
}

func TestLintCommandSelectionFailuresRemainJSON(t *testing.T) {
	t.Chdir(t.TempDir())
	if err := os.Mkdir("empty", 0o755); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{}, {"--module", "./empty"}, {"missing.yaml"}, {"empty"}} {
		report, raw, code := executeLintJSON(t, args...)
		if code != 1 || report.Status != "error" || report.Errors != 1 || report.FilesChecked != 0 || len(report.Diagnostics) != 1 {
			t.Fatalf("args=%v; report=%s; exit=%d", args, raw, code)
		}
	}
	cmd := NewLintCommand()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"--module", "default", "explicit.yaml"})
	if err := cmd.Execute(); err == nil {
		t.Fatal("ambiguous module/file selection was accepted")
	}
}

func TestLintCommandModulePathsAndSymlinkBoundary(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)
	writeLintFixture(t, "module/app.hyperbricks.yaml", "value: hello\n")
	writeLintFixture(t, "outside/other.hyperbricks.yaml", "value: 'outside'\n")
	if err := os.Symlink(filepath.Join(root, "outside"), "module/external"); err != nil {
		t.Fatal(err)
	}
	for _, module := range []string{"./module", filepath.Join(root, "module")} {
		report, raw, code := executeLintJSON(t, "-m", module)
		if code != 0 || report.FilesChecked != 1 || report.Warnings != 0 {
			t.Fatalf("module=%s; report=%s; exit=%d", module, raw, code)
		}
	}
	if err := os.Symlink(filepath.Join(root, "outside/other.hyperbricks.yaml"), "module/link.hyperbricks.yaml"); err != nil {
		t.Fatal(err)
	}
	report, _, code := executeLintJSON(t, "-m", "./module")
	if code != 1 || report.FilesChecked != 1 || report.Errors != 1 || report.Diagnostics[0].Code != "lint.read" {
		t.Fatalf("symlink report=%#v; exit=%d", report, code)
	}
}

func TestLintCommandHumanOutputExplainsLocationAndCorrection(t *testing.T) {
	t.Chdir(t.TempDir())
	writeLintFixture(t, "page.yaml", "value: 'hello'\n")
	previousExit, previousCode := Exit, ExitCode
	t.Cleanup(func() { Exit, ExitCode = previousExit, previousCode })
	cmd := NewLintCommand()
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetArgs([]string{"page.yaml"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if text := output.String(); !strings.Contains(text, "page.yaml:1:8: warning yaml.unnecessary_quotes:") || !strings.Contains(text, "remove the surrounding quotes") || !strings.Contains(text, "1 file(s), 1 warning(s), 0 error(s)") {
		t.Fatalf("output=%s", text)
	}
}
