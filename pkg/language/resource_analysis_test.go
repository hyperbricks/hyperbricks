package language

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hyperbricks/hyperbricks/pkg/parser"
)

func writeResourceAnalysisFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestResourceAnalysisDeniesModuleAndSymlinkEscapes(t *testing.T) {
	root := t.TempDir()
	module := filepath.Join(root, "module")
	outside := filepath.Join(root, "outside.txt")
	const sensitiveContent = "content that must not enter editor results"
	writeResourceAnalysisFile(t, outside, sensitiveContent)
	writeResourceAnalysisFile(t, filepath.Join(module, "resources", "safe.txt"), "safe")
	writeResourceAnalysisFile(t, filepath.Join(module, "templates", "safe.html"), "safe")
	analyzer := NewAnalyzer(AnalyzerOptions{WorkspaceRoot: module})
	uri := pathToURI(filepath.Join(module, "hyperbricks", "page.hyperbricks.yaml"))
	cases := []struct {
		name, source, message string
	}{
		{"absolute file", fmt.Sprintf("doc:\n  - type: text\n  - value: {file: '%s'}\n", filepath.ToSlash(outside)), "escapes selected module root"},
		{"relative file", "doc:\n  - type: text\n  - value: {file: {base: resources, path: ../../outside.txt}}\n", "escapes selected module root"},
		{"relative template", "doc:\n  - type: template\n  - template: {file: ../../outside.txt}\n", "escapes selected module root"},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			assertResourceBoundaryDiagnostic(t, analyzer.Diagnostics(uri, test.source, nil), test.message, sensitiveContent)
		})
	}
	for _, directory := range []string{"resources", "templates"} {
		if err := os.Symlink(outside, filepath.Join(module, directory, "linked.txt")); err != nil {
			t.Skipf("symlinks unavailable: %v", err)
		}
	}
	for _, source := range []string{
		"doc:\n  - type: text\n  - value: {file: {base: resources, path: linked.txt}}\n",
		"doc:\n  - type: template\n  - template: {file: linked.txt}\n",
	} {
		assertResourceBoundaryDiagnostic(t, analyzer.Diagnostics(uri, source, nil), "resolves outside selected module root", sensitiveContent)
	}
}

func assertResourceBoundaryDiagnostic(t *testing.T, diagnostics []Diagnostic, message, sensitiveContent string) {
	t.Helper()
	found := false
	for _, diagnostic := range diagnostics {
		if strings.Contains(diagnostic.Message, sensitiveContent) {
			t.Fatalf("resource content leaked: %#v", diagnostics)
		}
		if strings.Contains(diagnostic.Message, message) {
			found = true
			if diagnostic.Range.Start.Line != 2 {
				t.Fatalf("resource warning must anchor source resolver: %#v", diagnostic)
			}
		}
	}
	if !found {
		t.Fatalf("missing resource boundary diagnostic %q: %#v", message, diagnostics)
	}
}

func TestResourceAnalysisMissingFileWarningClearsAfterRepairOrSave(t *testing.T) {
	module := t.TempDir()
	writeResourceAnalysisFile(t, filepath.Join(module, "resources", "existing.md"), "existing copy")
	analyzer := NewAnalyzer(AnalyzerOptions{WorkspaceRoot: module})
	uri := pathToURI(filepath.Join(module, "hyperbricks", "page.hyperbricks.yaml"))
	source := "doc:\n  - type: text\n  - value: {file: {base: resources, path: new.md}}\n"
	diagnostics := analyzer.Diagnostics(uri, source, nil)
	warning := false
	for _, diagnostic := range diagnostics {
		if diagnostic.Severity == 2 && strings.Contains(diagnostic.Message, "new.md") {
			warning = true
		}
	}
	if !warning {
		t.Fatalf("missing file warning not exposed: %#v", diagnostics)
	}
	repaired := strings.Replace(source, "new.md", "existing.md", 1)
	if diagnostics := analyzer.Diagnostics(uri, repaired, nil); len(diagnostics) != 0 {
		t.Fatalf("source repair retained stale diagnostics: %#v", diagnostics)
	}
	writeResourceAnalysisFile(t, filepath.Join(module, "resources", "new.md"), "newly saved copy")
	if diagnostics := analyzer.Diagnostics(uri, source, nil); len(diagnostics) != 0 {
		t.Fatalf("saving missing resource retained stale diagnostics: %#v", diagnostics)
	}
}

func TestResourceAnalysisDoesNotRegisterTemplates(t *testing.T) {
	module := t.TempDir()
	name := t.Name() + ".html"
	parser.AddTemplate(name, "existing runtime content")
	writeResourceAnalysisFile(t, filepath.Join(module, "templates", name), "new editor content")
	analyzer := NewAnalyzer(AnalyzerOptions{WorkspaceRoot: module})
	uri := pathToURI(filepath.Join(module, "hyperbricks", "page.hyperbricks.yaml"))
	source := "doc:\n  - type: template\n  - template: {file: " + name + "}\n"
	if diagnostics := analyzer.Diagnostics(uri, source, nil); len(diagnostics) != 0 {
		t.Fatalf("valid template: %#v", diagnostics)
	}
	if content, _ := parser.GetTemplate(name); content != "existing runtime content" {
		t.Fatalf("source analysis replaced runtime template: %q", content)
	}
	writeResourceAnalysisFile(t, filepath.Join(module, "package.hyperbricks.yaml"), "myconf:\n  template: {file: "+name+"}\n")
	if config := analyzer.packageConfig(); config == nil {
		t.Fatal("package config failed to materialize")
	}
	if content, _ := parser.GetTemplate(name); content != "existing runtime content" {
		t.Fatalf("package analysis replaced runtime template: %q", content)
	}
}

func TestResourceAnalysisRecognizesAllComponentRuntimeVariables(t *testing.T) {
	module := t.TempDir()
	writeResourceAnalysisFile(t, filepath.Join(module, "package.hyperbricks.yaml"), "hyperbricks:\n  directories:\n    resources: private-assets\n")
	analyzer := NewAnalyzer(AnalyzerOptions{WorkspaceRoot: module})
	var source strings.Builder
	for _, name := range []string{"module_root", "root", "module", "resources", "templates", "static", "hyperbricks", "render"} {
		fmt.Fprintf(&source, "builtin_%s:\n  - type: text\n  - value: {var: %s}\n", name, name)
	}
	uri := pathToURI(filepath.Join(module, "hyperbricks", "page.hyperbricks.yaml"))
	if diagnostics := analyzer.Diagnostics(uri, source.String(), nil); len(diagnostics) != 0 {
		t.Fatalf("documented runtime variables falsely warn: %#v", diagnostics)
	}
	options := analyzer.sourceParserOptions()
	if options.Variables["resources"] != filepath.Join(module, "private-assets") || options.Variables["root"] != "." || options.Variables["module_root"] != filepath.Dir(module) {
		t.Fatalf("component variables must follow configured runtime path bases: %#v", options.Variables)
	}
}

func TestResourceAnalysisPackageMaterializationIsConfinedAndKeepsRuntimeVariables(t *testing.T) {
	root := t.TempDir()
	module := filepath.Join(root, "module")
	outside := filepath.Join(root, "outside.txt")
	writeResourceAnalysisFile(t, outside, "must not be read")
	writeResourceAnalysisFile(t, filepath.Join(module, "package.hyperbricks.yaml"), fmt.Sprintf(`myconf:
  blocked: {file: '%s'}
  module: {var: module}
  resources: {var: {name: resources, default: unavailable-in-package}}
`, filepath.ToSlash(outside)))
	analyzer := NewAnalyzer(AnalyzerOptions{WorkspaceRoot: module})
	config := analyzer.packageConfig()
	values, ok := config["myconf"].(map[string]interface{})
	if !ok || values["blocked"] != "" || values["module"] != module || values["resources"] != "unavailable-in-package" {
		t.Fatalf("package resource or runtime variable boundary changed: %#v", config)
	}
}

func TestResourceAnalysisPackageDiagnosticsCannotBypassReader(t *testing.T) {
	root := t.TempDir()
	module := filepath.Join(root, "module")
	outside := filepath.Join(root, "mode.txt")
	writeResourceAnalysisFile(t, outside, "live")
	analyzer := NewAnalyzer(AnalyzerOptions{WorkspaceRoot: module})
	uri := pathToURI(filepath.Join(module, "package.hyperbricks.yaml"))
	source := fmt.Sprintf("hyperbricks:\n  mode: {file: '%s'}\n", filepath.ToSlash(outside))
	// Reading the external file would produce a valid mode. A confined reader
	// leaves it empty and the existing strict configuration validator rejects it.
	if diagnostics := analyzer.Diagnostics(uri, source, nil); len(diagnostics) == 0 {
		t.Fatal("package diagnostics bypassed the resource reader")
	}
	link := filepath.Join(module, "resources", "mode.txt")
	if err := os.MkdirAll(filepath.Dir(link), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	symlinkSource := "hyperbricks:\n  mode: {file: {base: resources, path: mode.txt}}\n"
	if diagnostics := analyzer.Diagnostics(uri, symlinkSource, nil); len(diagnostics) == 0 {
		t.Fatal("package diagnostics followed a resource symlink outside the module")
	}
	if diagnostics := analyzer.Diagnostics(uri, "hyperbricks:\n  mode: live\n", nil); len(diagnostics) != 0 {
		t.Fatalf("repaired package diagnostics did not clear: %#v", diagnostics)
	}
}
