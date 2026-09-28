package language

import (
	"bytes"
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

func TestRequiredValueDiagnosticsDistinguishAbsentAndEmpty(t *testing.T) {
	analyzer := NewAnalyzer(AnalyzerOptions{WorkspaceRoot: t.TempDir()})
	for _, test := range []struct {
		name, source, code string
		line               int
	}{
		{"absent", "doc:\n  - type: text\n", "component.missing_required_field", 1},
		{"empty string", "doc:\n  - type: text\n  - value: ''\n", "component.empty_required_field", 2},
		{"normalized field key", "doc:\n  - type: text\n  - ' value ': ''\n", "component.empty_required_field", 2},
		{"null", "doc:\n  - type: text\n  - value:\n", "component.empty_required_field", 2},
	} {
		t.Run(test.name, func(t *testing.T) {
			diagnostics := analyzer.Diagnostics("untitled:required-value", test.source, nil)
			if len(diagnostics) != 1 || diagnostics[0].Code != test.code {
				t.Fatalf("diagnostics = %#v, want one %s", diagnostics, test.code)
			}
			diagnostic := diagnostics[0]
			if diagnostic.Range.Start.Line != test.line || diagnostic.Range.Start.Character < 4 {
				t.Fatalf("diagnostic must identify the owning source field: %#v", diagnostic)
			}
			if test.code == "component.empty_required_field" && (!strings.Contains(diagnostic.Message, "text.value") || !strings.Contains(diagnostic.Message, "empty")) {
				t.Fatalf("empty value message must explain the field's value, not its absence: %#v", diagnostic)
			}
		})
	}
}

func TestRequiredValueDiagnosticsExplainEmptyFileAndClearOnRepair(t *testing.T) {
	module := t.TempDir()
	emptyPath := filepath.Join(module, "resources", "docs", "test.md")
	writeResourceAnalysisFile(t, emptyPath, "")
	writeResourceAnalysisFile(t, filepath.Join(module, "resources", "docs", "populated.md"), "Hello from the file")
	analyzer := NewAnalyzer(AnalyzerOptions{WorkspaceRoot: module})
	uri := pathToURI(filepath.Join(module, "hyperbricks", "page.hyperbricks.yaml"))
	for _, test := range []struct {
		name, expression string
	}{
		{"flow", "{file: {base: resources, path: docs/test.md}}"},
		{"quoted path", "{file: {base: resources, path: \"docs/test.md\"}}"},
		{"block", "\n      file:\n        base: resources\n        path: docs/test.md"},
	} {
		t.Run(test.name, func(t *testing.T) {
			source := "doc:\n  - type: text\n  - value: " + test.expression + "\n"
			diagnostics := analyzer.Diagnostics(uri, source, nil)
			if len(diagnostics) != 1 || diagnostics[0].Code != "component.empty_required_field" {
				t.Fatalf("existing empty resource diagnostics = %#v", diagnostics)
			}
			diagnostic := diagnostics[0]
			for _, want := range []string{"text.value", "empty", "resources/docs/test.md"} {
				if !strings.Contains(diagnostic.Message, want) {
					t.Errorf("message %q must include %q", diagnostic.Message, want)
				}
			}
			if diagnostic.Range.Start.Line < 2 {
				t.Fatalf("empty resource diagnostic must anchor value expression, not type: %#v", diagnostic)
			}
			repaired := strings.Replace(source, "docs/test.md", "docs/populated.md", 1)
			if diagnostics := analyzer.Diagnostics(uri, repaired, nil); len(diagnostics) != 0 {
				t.Fatalf("source repair retained empty value error: %#v", diagnostics)
			}
		})
	}
	writeResourceAnalysisFile(t, emptyPath, "Saved content")
	source := "doc:\n  - type: text\n  - value: {file: {base: resources, path: docs/test.md}}\n"
	if diagnostics := analyzer.Diagnostics(uri, source, nil); len(diagnostics) != 0 {
		t.Fatalf("saved resource content retained an empty value error after reanalysis: %#v", diagnostics)
	}
}

func TestRequiredValueDiagnosticsKeepRuntimeEmptyValueContracts(t *testing.T) {
	module := t.TempDir()
	writeResourceAnalysisFile(t, filepath.Join(module, "resources", "empty.md"), "")
	writeResourceAnalysisFile(t, filepath.Join(module, "resources", "space.txt"), " \t\n")
	analyzer := NewAnalyzer(AnalyzerOptions{WorkspaceRoot: module})
	uri := pathToURI(filepath.Join(module, "hyperbricks", "page.hyperbricks.yaml"))
	for _, source := range []string{
		"doc:\n  - type: markdown\n  - content: {file: {base: resources, path: empty.md}}\n",
		"doc:\n  - type: text\n  - value: ' '\n",
		"doc:\n  - type: text\n  - value: {file: {base: resources, path: space.txt}}\n",
		"base:\n  - type: text\n  - value: available\ndoc:\n  - inherit: base\n",
	} {
		if diagnostics := analyzer.Diagnostics(uri, source, nil); len(diagnostics) != 0 {
			t.Errorf("valid runtime value rejected:\n%s\n%#v", source, diagnostics)
		}
	}

	// The source field is present in the base. Inheritance must not turn an
	// empty effective value into an absent-field diagnostic on the inheritor.
	source := "base:\n  - type: text\n  - value: ''\ndoc:\n  - inherit: base\n"
	diagnostics := analyzer.Diagnostics(uri, source, nil)
	if len(diagnostics) == 0 {
		t.Fatal("empty inherited text value was accepted")
	}
	for _, diagnostic := range diagnostics {
		if diagnostic.Code != "component.empty_required_field" {
			t.Errorf("inherited empty field misdiagnosed: %#v", diagnostic)
		}
	}
}

func TestServerPublishesEmptyRequiredValueDiagnosticAndClearsCorrection(t *testing.T) {
	module := t.TempDir()
	writeResourceAnalysisFile(t, filepath.Join(module, "resources", "empty.md"), "")
	uri := pathToURI(filepath.Join(module, "hyperbricks", "page.hyperbricks.yaml"))
	input := bytes.Join([][]byte{
		framedRPC(t, map[string]interface{}{
			"jsonrpc": "2.0", "id": 1, "method": "initialize",
			"params": map[string]interface{}{
				"rootUri": pathToURI(module),
				"initializationOptions": map[string]interface{}{
					"protocolVersion": ProtocolVersion, "runtimeDiagnostics": "off",
				},
			},
		}),
		framedRPC(t, map[string]interface{}{
			"jsonrpc": "2.0", "method": "textDocument/didOpen",
			"params": map[string]interface{}{"textDocument": map[string]interface{}{
				"uri": uri, "languageId": "hyperbricks", "version": 1,
				"text": "doc:\n  - type: text\n  - value: {file: {base: resources, path: empty.md}}\n",
			}},
		}),
		framedRPC(t, map[string]interface{}{
			"jsonrpc": "2.0", "method": "textDocument/didChange",
			"params": map[string]interface{}{
				"textDocument":   map[string]interface{}{"uri": uri, "version": 2},
				"contentChanges": []map[string]interface{}{{"text": "doc:\n  - type: text\n  - value: Corrected\n"}},
			},
		}),
	}, nil)
	var output bytes.Buffer
	if err := NewServer(bytes.NewReader(input), &output, ServerOptions{}).Serve(context.Background()); err != nil {
		t.Fatal(err)
	}
	var publications []struct {
		Version     int          `json:"version"`
		Diagnostics []Diagnostic `json:"diagnostics"`
	}
	for _, message := range readFramedRPC(t, output.Bytes()) {
		if message.Method != "textDocument/publishDiagnostics" {
			continue
		}
		var publication struct {
			Version     int          `json:"version"`
			Diagnostics []Diagnostic `json:"diagnostics"`
		}
		if err := json.Unmarshal(message.Params, &publication); err != nil {
			t.Fatal(err)
		}
		publications = append(publications, publication)
	}
	if len(publications) != 2 || publications[0].Version != 1 || len(publications[0].Diagnostics) != 1 || publications[0].Diagnostics[0].Code != "component.empty_required_field" {
		t.Fatalf("initial resolver error publication = %#v", publications)
	}
	if publications[0].Diagnostics[0].Range.Start.Line != 2 {
		t.Fatalf("resolver error published on wrong line: %#v", publications[0])
	}
	if publications[1].Version != 2 || publications[1].Diagnostics == nil || len(publications[1].Diagnostics) != 0 {
		t.Fatalf("correction must publish a non-null empty diagnostic array: %#v", publications[1])
	}
}
