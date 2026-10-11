package language

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"go.yaml.in/yaml/v4"
)

func TestLintDocumentUnnecessaryQuotes(t *testing.T) {
	for _, source := range []string{
		"page:\n  - type: 'hypermedia'\n  - route: \"events\"\n  - title: 'Event calendar' # keep\n",
		"\"page\":\n  - type: html\n  - value: 'Welcome'\n  - extra: {source: \"css/site.css\"}\n",
		"values: ['hello', \"two words\", 'café']\n",
	} {
		diagnostics := LintDocument(source)
		if len(diagnostics) != 3 {
			t.Fatalf("diagnostics for %q = %#v", source, diagnostics)
		}
		for _, diagnostic := range diagnostics {
			if diagnostic.Code != "yaml.unnecessary_quotes" || diagnostic.Severity != DiagnosticSeverityWarning {
				t.Fatalf("diagnostic = %#v", diagnostic)
			}
			assertLintRemovalPreservesYAML(t, source, []Diagnostic{diagnostic})
		}
		assertLintRemovalPreservesYAML(t, source, diagnostics)
		if again := LintDocument(removeLintQuotes(t, source, diagnostics)); len(again) != 0 {
			t.Fatalf("corrected document still has diagnostics: %#v", again)
		}
	}
}

func TestLintDocumentPreservesNecessaryAndExpressiveQuotes(t *testing.T) {
	values := []string{
		`""`, `'true'`, `'false'`, `'null'`, `'~'`, `'001'`, `'1'`, `'1.0'`, `'1e3'`, `'0xFF'`,
		`'.nan'`, `'.inf'`, `'2026-10-10'`, `'2026-10-10T12:00:00Z'`, `'yes'`, `'NO'`, `'off'`,
		`'#status'`, `'Status: ready'`, `'hello # comment'`, `' leading'`, `'trailing '`,
		`'- value'`, `'---'`, `'...'`, `'[one, two]'`, `'{var: title}'`, `'*alias'`, `'&anchor'`,
		`"Line one\nLine two"`, `"hello\u0020world"`, `"hello\tworld"`, `'C:\assets\main.js'`,
		`'<p>Hello</p>'`, `'{{.title}}'`, `'document.querySelector("#target")'`, `'It''s ready'`,
		"'multi\n  line'", `!!str "hello"`, `&name "hello"`, "|\n  \"hello\"\n", ">-\n  \"hello\"\n",
	}
	for _, value := range values {
		t.Run(value, func(t *testing.T) {
			source := "value: " + value + "\n"
			if diagnostics := LintDocument(source); len(diagnostics) != 0 {
				t.Fatalf("quotes must be preserved: %#v", diagnostics)
			}
		})
	}
}

func TestLintDocumentUsesOrdinaryQuoteRulesForModuleVersion(t *testing.T) {
	for _, source := range []string{
		"hyperbricks:\n  metadata:\n    moduleversion: 1.0.0\n",
		"hyperbricks: {metadata: {moduleversion: '1.0'}}\n",
	} {
		if diagnostics := LintDocument(source); len(diagnostics) != 0 {
			t.Fatalf("valid plain value or necessary quotes reported: %#v", diagnostics)
		}
	}
	for _, source := range []string{
		"hyperbricks:\n  metadata:\n    moduleversion: \"1.0.0\"\n",
		"hyperbricks: {metadata: {moduleversion: '1.2.3-beta'}}\n",
		"moduleversion: '1.0.0'\n",
		"other: {metadata: {moduleversion: '1.0.0'}}\n",
	} {
		diagnostics := LintDocument(source)
		if len(diagnostics) != 1 || diagnostics[0].Code != "yaml.unnecessary_quotes" {
			t.Fatalf("unnecessary version quotes must be reported: %#v", diagnostics)
		}
		assertLintRemovalPreservesYAML(t, source, diagnostics)
	}
}

func TestLintDocumentModuleVersionTypes(t *testing.T) {
	for _, test := range []struct {
		value string
		code  string
	}{
		{"1.0.0", ""},
		{`"1.0.0"`, "yaml.unnecessary_quotes"},
		{`'1.0.0'`, "yaml.unnecessary_quotes"},
		{`"1.0"`, ""},
		{`'1.0'`, ""},
		{"1.2.3-beta.2+build.4", ""},
		{`"1.2.3-beta.2+build.4"`, ""},
		{"!!str 1.0", ""},
		{`""`, ""}, // Only the type is checked; doctor/build validate the version.
		{"1.0", "metadata.moduleversion_type"},
		{"1", "metadata.moduleversion_type"},
		{"true", "metadata.moduleversion_type"},
		{"null", "metadata.moduleversion_type"},
		{"~", "metadata.moduleversion_type"},
		{"", "metadata.moduleversion_type"},
		{"2026-10-10", "metadata.moduleversion_type"},
		{"[1.0.0]", "metadata.moduleversion_type"},
		{"{major: 1}", "metadata.moduleversion_type"},
		{`!!float "1.0"`, "metadata.moduleversion_type"},
	} {
		t.Run(test.value, func(t *testing.T) {
			source := "hyperbricks:\n  metadata:\n    moduleversion: " + test.value + "\n"
			diagnostics := LintDocument(source)
			if test.code == "" {
				if len(diagnostics) != 0 {
					t.Fatalf("string value must not produce a type error: %#v", diagnostics)
				}
				return
			}
			if len(diagnostics) != 1 || diagnostics[0].Code != test.code || diagnostics[0].Source != lintDiagnosticSource {
				t.Fatalf("diagnostics = %#v, want %s", diagnostics, test.code)
			}
			if test.code == "metadata.moduleversion_type" {
				diagnostic := diagnostics[0]
				wantRange := Range{Start: Position{Line: 2, Character: 4}, End: Position{Line: 2, Character: 17}}
				if diagnostic.Severity != DiagnosticSeverityError || diagnostic.Range != wantRange ||
					!strings.Contains(diagnostic.Message, "1.0.0") || !strings.Contains(diagnostic.Message, `"1.0"`) {
					t.Fatalf("type diagnostic must identify the field and explain the correction: %#v", diagnostic)
				}
			} else if diagnostics[0].Severity != DiagnosticSeverityWarning {
				t.Fatalf("unnecessary quotes should remain a warning: %#v", diagnostics)
			}
		})
	}
}

func TestLintDocumentModuleVersionScopeAndRanges(t *testing.T) {
	for _, source := range []string{
		"moduleversion: 1.0\n",
		"other: {metadata: {moduleversion: 1.0}}\n",
		"metadata: {moduleversion: 1.0}\n",
		"hyperbricks: {metadata: {module: demo}}\n",
		"hyperbricks: {metadata: {moduleversion: {env: MODULE_VERSION}}}\n",
		"hyperbricks: {metadata: {moduleversion: {var: version}}}\n",
		"hyperbricks: {metadata: {moduleversion: {format: '%s.0.0', args: [1]}}}\n",
	} {
		if diagnostics := LintDocument(source); len(diagnostics) != 0 {
			t.Fatalf("type check must stay within literal package metadata: %q: %#v", source, diagnostics)
		}
	}
	source := "# package profile\r\nhyperbricks: {metadata: {owner: 😀, moduleversion: 1.0}}\r\n"
	diagnostics := LintDocument(source)
	key := strings.Index(source, "moduleversion")
	line := strings.LastIndex(source[:key], "\n") + 1
	column := utf16Length(source[line:key])
	want := Range{Start: Position{Line: 1, Character: column}, End: Position{Line: 1, Character: column + len("moduleversion")}}
	if len(diagnostics) != 1 || diagnostics[0].Range != want {
		t.Fatalf("flow/CRLF/UTF-16 range = %#v, want %#v", diagnostics, want)
	}
	source = "title: 'Demo'\nhyperbricks:\n  metadata:\n    moduleversion: 1.0\n    module: 'demo'\n"
	diagnostics = LintDocument(source)
	if len(diagnostics) != 3 || diagnostics[0].Code != "yaml.unnecessary_quotes" ||
		diagnostics[1].Code != "metadata.moduleversion_type" || diagnostics[2].Code != "yaml.unnecessary_quotes" {
		t.Fatalf("mixed diagnostics must stay in source order: %#v", diagnostics)
	}
}

func TestLintDocumentFlowContextAndUTF16Ranges(t *testing.T) {
	for _, source := range []string{
		`{"key":"hello",'other':'world'}`,
		"{\"key\": 'hello', other: [\"first\", 'second']}\n",
		"# comment\r\n'𐐀': \"café\" # unchanged\r\n",
		"value: &name 'hello'\ncopy: *name\nother: 'world'\n",
	} {
		diagnostics := LintDocument(source)
		if len(diagnostics) == 0 {
			t.Fatalf("expected at least one safe quote removal in %q", source)
		}
		for _, diagnostic := range diagnostics {
			assertLintRemovalPreservesYAML(t, source, []Diagnostic{diagnostic})
		}
		assertLintRemovalPreservesYAML(t, source, diagnostics)
	}
	source := "# comment\r\n'𐐀': \"café\" # unchanged\r\n"
	diagnostics := LintDocument(source)
	if len(diagnostics) != 2 || diagnostics[1].Range != (Range{Start: Position{Line: 1, Character: 6}, End: Position{Line: 1, Character: 12}}) {
		t.Fatalf("UTF-16 ranges = %#v", diagnostics)
	}
}

func TestLintDocumentSyntaxAndEmptyInput(t *testing.T) {
	for _, source := range []string{"", " \n", "# comment\n", "value: plain\n"} {
		if diagnostics := LintDocument(source); len(diagnostics) != 0 {
			t.Fatalf("unexpected diagnostics for %q: %#v", source, diagnostics)
		}
	}
	for _, source := range []string{"value: 'unterminated\n", "value: [broken\n", "value: 'hello'\n---\nvalue: world\n"} {
		diagnostics := LintDocument(source)
		if len(diagnostics) != 1 || diagnostics[0].Severity != DiagnosticSeverityError || diagnostics[0].Code != "yaml.syntax" {
			t.Fatalf("syntax diagnostics for %q = %#v", source, diagnostics)
		}
	}
}

func TestAnalyzerSharesLintForSourceAndPackageBuffers(t *testing.T) {
	root := t.TempDir()
	analyzer := NewAnalyzer(AnalyzerOptions{WorkspaceRoot: root, Module: "."})
	for name, source := range map[string]string{
		"page.hyperbricks.yaml":    "page:\n  - type: 'html'\n  - value: 'Welcome'\n",
		"package.hyperbricks.yaml": "hyperbricks:\n  mode: 'development'\n",
	} {
		uri := pathToURI(filepath.Join(root, name))
		var lint []Diagnostic
		for _, diagnostic := range analyzer.Diagnostics(uri, source, map[string]string{uri: source}) {
			if diagnostic.Source == lintDiagnosticSource {
				lint = append(lint, diagnostic)
			}
		}
		if !reflect.DeepEqual(lint, LintDocument(source)) {
			t.Fatalf("analyzer/shared lint mismatch for %s: %#v", name, lint)
		}
	}
	diagnostics := analyzer.Diagnostics("untitled:broken", "page: [\n", nil)
	if len(diagnostics) != 1 || diagnostics[0].Severity != DiagnosticSeverityError {
		t.Fatalf("parse error should not be duplicated: %#v", diagnostics)
	}
}

func TestServerPublishesAndClearsSharedLintForUnsavedBuffer(t *testing.T) {
	root := t.TempDir()
	uri := pathToURI(filepath.Join(root, "page.hyperbricks.yaml"))
	source := "page:\n  - type: 'html'\n  - value: Hello\n"
	input := bytes.Join([][]byte{
		framedRPC(t, map[string]interface{}{"jsonrpc": "2.0", "id": 1, "method": "initialize", "params": map[string]interface{}{
			"rootUri": pathToURI(root), "initializationOptions": map[string]interface{}{"protocolVersion": ProtocolVersion, "runtimeDiagnostics": "off"},
		}}),
		framedRPC(t, map[string]interface{}{"jsonrpc": "2.0", "method": "textDocument/didOpen", "params": map[string]interface{}{
			"textDocument": map[string]interface{}{"uri": uri, "languageId": "hyperbricks", "version": 1, "text": source},
		}}),
		framedRPC(t, map[string]interface{}{"jsonrpc": "2.0", "method": "textDocument/didChange", "params": map[string]interface{}{
			"textDocument":   map[string]interface{}{"uri": uri, "version": 2},
			"contentChanges": []map[string]interface{}{{"text": strings.ReplaceAll(source, "'html'", "html")}},
		}}),
		framedRPC(t, map[string]interface{}{"jsonrpc": "2.0", "id": 2, "method": "shutdown"}),
		framedRPC(t, map[string]interface{}{"jsonrpc": "2.0", "method": "exit"}),
	}, nil)
	var output bytes.Buffer
	if err := NewServer(bytes.NewReader(input), &output, ServerOptions{Version: "test"}).Serve(context.Background()); err != nil {
		t.Fatal(err)
	}
	var published [][]Diagnostic
	for _, message := range readFramedRPC(t, output.Bytes()) {
		if message.Method == "textDocument/publishDiagnostics" {
			var params struct {
				Diagnostics []Diagnostic `json:"diagnostics"`
			}
			if err := json.Unmarshal(message.Params, &params); err != nil {
				t.Fatal(err)
			}
			published = append(published, params.Diagnostics)
		}
	}
	if len(published) != 2 || !reflect.DeepEqual(published[0], LintDocument(source)) || published[1] == nil || len(published[1]) != 0 {
		t.Fatalf("published diagnostics = %#v", published)
	}
}

func TestServerPublishesModuleVersionTypesAndClearsAfterCorrection(t *testing.T) {
	root := t.TempDir()
	uri := pathToURI(filepath.Join(root, "package.hyperbricks.yaml"))
	values := []string{"1.0", `"1.0"`, "1.0.0", `"1.0.0"`, "true", "1.2.3-beta.2+build.4"}
	input := framedRPC(t, map[string]interface{}{"jsonrpc": "2.0", "id": 1, "method": "initialize", "params": map[string]interface{}{
		"rootUri": pathToURI(root), "initializationOptions": map[string]interface{}{"protocolVersion": ProtocolVersion, "runtimeDiagnostics": "off"},
	}})
	for i, value := range values {
		source := "hyperbricks:\n  metadata:\n    moduleversion: " + value + "\n"
		method := "textDocument/didChange"
		params := map[string]interface{}{
			"textDocument":   map[string]interface{}{"uri": uri, "version": i + 1},
			"contentChanges": []map[string]interface{}{{"text": source}},
		}
		if i == 0 {
			method = "textDocument/didOpen"
			params = map[string]interface{}{"textDocument": map[string]interface{}{"uri": uri, "languageId": "hyperbricks", "version": 1, "text": source}}
		}
		input = append(input, framedRPC(t, map[string]interface{}{"jsonrpc": "2.0", "method": method, "params": params})...)
	}
	input = append(input, framedRPC(t, map[string]interface{}{"jsonrpc": "2.0", "id": 2, "method": "shutdown"})...)
	input = append(input, framedRPC(t, map[string]interface{}{"jsonrpc": "2.0", "method": "exit"})...)
	var output bytes.Buffer
	if err := NewServer(bytes.NewReader(input), &output, ServerOptions{Version: "test"}).Serve(context.Background()); err != nil {
		t.Fatal(err)
	}
	published := 0
	for _, message := range readFramedRPC(t, output.Bytes()) {
		if message.Method != "textDocument/publishDiagnostics" {
			continue
		}
		var params struct {
			Diagnostics []Diagnostic `json:"diagnostics"`
		}
		if err := json.Unmarshal(message.Params, &params); err != nil {
			t.Fatal(err)
		}
		if published >= len(values) {
			t.Fatal("unexpected extra publication")
		}
		want := LintDocument("hyperbricks:\n  metadata:\n    moduleversion: " + values[published] + "\n")
		if params.Diagnostics == nil || (len(want) != 0 && !reflect.DeepEqual(params.Diagnostics, want)) || len(params.Diagnostics) != len(want) {
			t.Fatalf("publication %d: got %#v, want %#v", published, params.Diagnostics, want)
		}
		published++
	}
	if published != len(values) {
		t.Fatalf("published %d updates, want %d", published, len(values))
	}
}

// Tests remove only the quoted source spans, just as an agent following a
// diagnostic would. Every individual recommendation must preserve semantics.
func removeLintQuotes(t testing.TB, source string, diagnostics []Diagnostic) string {
	t.Helper()
	for i := len(diagnostics) - 1; i >= 0; i-- {
		r := diagnostics[i].Range
		lines := strings.SplitAfter(source, "\n")
		offset := 0
		for _, line := range lines[:r.Start.Line] {
			offset += len(line)
		}
		byteAt := func(character int) int {
			units := 0
			for index, value := range lines[r.Start.Line] {
				if units == character {
					return offset + index
				}
				units += utf16Length(string(value))
			}
			return offset + len(lines[r.Start.Line])
		}
		start, end := byteAt(r.Start.Character), byteAt(r.End.Character)
		if r.Start.Line != r.End.Line || end-start < 2 || (source[start] != '\'' && source[start] != '"') || source[start] != source[end-1] {
			t.Fatalf("invalid quote range %#v for %q", r, source)
		}
		source = source[:start] + source[start+1:end-1] + source[end:]
	}
	return source
}

func assertLintRemovalPreservesYAML(t testing.TB, source string, diagnostics []Diagnostic) {
	t.Helper()
	before, err := decodeYAMLDocument(source)
	if err != nil {
		t.Fatal(err)
	}
	after, err := decodeYAMLDocument(removeLintQuotes(t, source, diagnostics))
	if err != nil || !sameYAMLSemantics(before, after) || !equalStrings(yamlComments(before, nil), yamlComments(after, nil)) {
		t.Fatalf("lint recommendation changes YAML: %v; source=%q; diagnostics=%#v", err, source, diagnostics)
	}
}

func FuzzLintQuoteRemovalPreservesMeaning(f *testing.F) {
	for _, value := range []string{"hello", "001", "true", "a b", "- x", "...", "foo/bar", "𐐀 café", "yes", "2026-10-10", "<p>Hi</p>", "a\nb"} {
		f.Add(value)
	}
	f.Fuzz(func(t *testing.T, value string) {
		if len(value) > 2048 {
			t.Skip()
		}
		// Go/JSON quoting is sufficient for seeds that are also valid YAML.
		for _, source := range []string{"value: " + strconv.Quote(value) + "\n", "{" + strconv.Quote(value) + ":\"hello\"}\n"} {
			var root yaml.Node
			if err := yaml.Unmarshal([]byte(source), &root); err != nil {
				continue
			}
			diagnostics := LintDocument(source)
			for _, diagnostic := range diagnostics {
				if diagnostic.Severity == DiagnosticSeverityWarning {
					assertLintRemovalPreservesYAML(t, source, []Diagnostic{diagnostic})
				}
			}
		}
	})
}

func BenchmarkLintDocumentQuotes(b *testing.B) {
	var source strings.Builder
	for i := 0; i < 250; i++ {
		fmt.Fprintf(&source, "page%d:\n  - type: 'hypermedia'\n  - route: 'page-%d'\n  - title: 'Event calendar'\n", i, i)
	}
	text := source.String()
	b.ReportAllocs()
	b.SetBytes(int64(len(text)))
	for b.Loop() {
		if got := len(LintDocument(text)); got != 750 {
			b.Fatalf("diagnostics = %d", got)
		}
	}
}
