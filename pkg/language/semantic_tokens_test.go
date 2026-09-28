package language

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"unicode/utf16"
)

func TestSemanticHighlightsDistinguishComponentsSchemaFieldsAndDataKeys(t *testing.T) {
	source := `page:
  - type: hypermedia
  - route: index
  - response:
      status: 201
      headers:
        Cache-Control: no-store
  - body:
      - type: template
      - template:
          base: templates
          path: shell.html
      - values:
          navigation:
            - type: html
            - value: Navigation
`
	analyzer := NewAnalyzer(AnalyzerOptions{WorkspaceRoot: t.TempDir()})
	highlights := analyzer.SemanticHighlights("untitled:page", source, nil)

	for _, expected := range []semanticExpectation{
		{text: "page", tokenType: SemanticTokenTypeClass, declaration: true},
		{text: "type", tokenType: SemanticTokenTypeKeyword},
		{text: "hypermedia", tokenType: SemanticTokenTypeType},
		{text: "route", tokenType: SemanticTokenTypeProperty},
		{text: "response", tokenType: SemanticTokenTypeProperty},
		{text: "status", tokenType: SemanticTokenTypeProperty},
		{text: "headers", tokenType: SemanticTokenTypeProperty},
		{text: "body", tokenType: SemanticTokenTypeClass, declaration: true},
		{text: "template", tokenType: SemanticTokenTypeType},
		{text: "template", tokenType: SemanticTokenTypeProperty},
		{text: "values", tokenType: SemanticTokenTypeProperty},
		{text: "html", tokenType: SemanticTokenTypeType},
		{text: "value", tokenType: SemanticTokenTypeProperty},
	} {
		if !hasSemanticExpectation(t, source, highlights, expected) {
			t.Fatalf("missing semantic highlight %#v in %s", expected, describeSemanticHighlights(t, source, highlights))
		}
	}
	for _, ordinary := range []string{"Cache-Control", "base", "path", "navigation"} {
		if hasSemanticText(t, source, highlights, ordinary) {
			t.Fatalf("ordinary YAML key %q was semantically reclassified: %s", ordinary, describeSemanticHighlights(t, source, highlights))
		}
	}
}

func TestSemanticHighlightsFollowImportedInheritanceAndInheritedChildOverlays(t *testing.T) {
	root := t.TempDir()
	sourceDir := filepath.Join(root, "hyperbricks")
	if err := os.MkdirAll(sourceDir, 0o755); err != nil {
		t.Fatal(err)
	}
	pageURI := pathToURI(filepath.Join(sourceDir, "app.hyperbricks.yaml"))
	baseURI := pathToURI(filepath.Join(sourceDir, "base.hyperbricks.yaml"))
	viewsURI := pathToURI(filepath.Join(sourceDir, "views.hyperbricks.yaml"))
	page := `imports:
  - base.hyperbricks.yaml
  - views.hyperbricks.yaml
about_page:
  - inherit: todo_page
  - body:
      - values:
          content:
            - inherit: todo_about
`
	documents := map[string]string{
		pageURI: page,
		baseURI: `todo_page:
  - type: hypermedia
  - body:
      - type: template
      - values: {}
`,
		viewsURI: `todo_about:
  - type: html
  - value: About
`,
	}
	analyzer := NewAnalyzer(AnalyzerOptions{WorkspaceRoot: root})
	highlights := analyzer.SemanticHighlights(pageURI, page, documents)

	for _, expected := range []semanticExpectation{
		{text: "imports", tokenType: SemanticTokenTypeKeyword},
		{text: "about_page", tokenType: SemanticTokenTypeClass, declaration: true},
		{text: "inherit", tokenType: SemanticTokenTypeKeyword},
		{text: "todo_page", tokenType: SemanticTokenTypeClass},
		{text: "body", tokenType: SemanticTokenTypeClass, declaration: true},
		{text: "values", tokenType: SemanticTokenTypeProperty},
		{text: "todo_about", tokenType: SemanticTokenTypeClass},
	} {
		if !hasSemanticExpectation(t, page, highlights, expected) {
			t.Fatalf("missing imported/inherited highlight %#v in %s", expected, describeSemanticHighlights(t, page, highlights))
		}
	}
	if hasSemanticText(t, page, highlights, "content") {
		t.Fatalf("dynamic template value key was semantically reclassified: %s", describeSemanticHighlights(t, page, highlights))
	}
}

func TestSemanticHighlightsLeaveUnknownPluginFieldsUnclassified(t *testing.T) {
	source := `widget:
  - type: acme_card
  - heading: Hello
`
	analyzer := NewAnalyzer(AnalyzerOptions{WorkspaceRoot: t.TempDir()})
	highlights := analyzer.SemanticHighlights("untitled:widget", source, nil)
	for _, expected := range []semanticExpectation{
		{text: "widget", tokenType: SemanticTokenTypeClass, declaration: true},
		{text: "type", tokenType: SemanticTokenTypeKeyword},
	} {
		if !hasSemanticExpectation(t, source, highlights, expected) {
			t.Fatalf("missing unknown-plugin structural highlight %#v in %s", expected, describeSemanticHighlights(t, source, highlights))
		}
	}
	for _, lexical := range []string{"acme_card", "heading"} {
		if hasSemanticText(t, source, highlights, lexical) {
			t.Fatalf("unknown plugin syntax %q was treated as native schema: %s", lexical, describeSemanticHighlights(t, source, highlights))
		}
	}
}

func TestSemanticHighlightsUseQuotedSourceUTF16RangesAndStayOrdered(t *testing.T) {
	source := "\"😀page\":\n  - type: \"\\u0068tml\"\n  - \"value\": ok\n"
	analyzer := NewAnalyzer(AnalyzerOptions{WorkspaceRoot: t.TempDir()})
	highlights := analyzer.SemanticHighlights("untitled:utf16", source, nil)
	if len(highlights) != 4 {
		t.Fatalf("UTF-16 highlights = %s", describeSemanticHighlights(t, source, highlights))
	}
	if got := highlights[0].Range; got != (Range{Start: Position{Line: 0, Character: 1}, End: Position{Line: 0, Character: 7}}) {
		t.Fatalf("quoted astral root range = %#v, want quote-free UTF-16 width 6", got)
	}
	if !hasSemanticExpectation(t, source, highlights, semanticExpectation{text: "\\u0068tml", tokenType: SemanticTokenTypeType}) ||
		!hasSemanticExpectation(t, source, highlights, semanticExpectation{text: "value", tokenType: SemanticTokenTypeProperty}) {
		t.Fatalf("quoted scalar source ranges = %s", describeSemanticHighlights(t, source, highlights))
	}
	for index := 1; index < len(highlights); index++ {
		previous, current := highlights[index-1].Range, highlights[index].Range
		if current.Start.Line < previous.Start.Line ||
			(current.Start.Line == previous.Start.Line && current.Start.Character < previous.End.Character) {
			t.Fatalf("semantic ranges overlap or are unsorted: %#v then %#v", previous, current)
		}
	}
}

func TestSemanticHighlightsReturnEmptyForPackageConfigAndMalformedYAML(t *testing.T) {
	root := t.TempDir()
	analyzer := NewAnalyzer(AnalyzerOptions{WorkspaceRoot: root})
	configURI := pathToURI(filepath.Join(root, "package.hyperbricks.yaml"))
	if got := analyzer.SemanticHighlights(configURI, "hyperbricks:\n  mode: live\n", nil); len(got) != 0 {
		t.Fatalf("package config semantic highlights = %#v", got)
	}
	if got := analyzer.SemanticHighlights("untitled:broken", "page:\n  - type: [\n", nil); len(got) != 0 {
		t.Fatalf("malformed YAML semantic highlights = %#v", got)
	}
}

func TestServerAdvertisesAndEncodesFullSemanticTokens(t *testing.T) {
	root := t.TempDir()
	uri := pathToURI(filepath.Join(root, "hyperbricks", "page.hyperbricks.yaml"))
	source := "page:\n  - type: html\n  - value: hello\n"
	var output bytes.Buffer
	server := NewServer(bytes.NewReader(nil), &output, ServerOptions{})
	initialize, err := json.Marshal(InitializeParams{
		RootURI:               pathToURI(root),
		InitializationOptions: InitializeOptions{ProtocolVersion: ProtocolVersion},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := server.handle(context.Background(), rpcMessage{ID: json.RawMessage("1"), Method: "initialize", Params: initialize}); err != nil {
		t.Fatal(err)
	}
	messages := readFramedRPC(t, output.Bytes())
	var initialized struct {
		Capabilities struct {
			SemanticTokensProvider struct {
				Legend struct {
					TokenTypes     []string `json:"tokenTypes"`
					TokenModifiers []string `json:"tokenModifiers"`
				} `json:"legend"`
				Full  bool `json:"full"`
				Range bool `json:"range"`
			} `json:"semanticTokensProvider"`
		} `json:"capabilities"`
	}
	if len(messages) != 1 || json.Unmarshal(messages[0].Result, &initialized) != nil {
		t.Fatalf("initialize semantic capability messages = %#v", messages)
	}
	provider := initialized.Capabilities.SemanticTokensProvider
	if !provider.Full || provider.Range || !reflect.DeepEqual(provider.Legend.TokenTypes, SemanticTokenTypes) ||
		!reflect.DeepEqual(provider.Legend.TokenModifiers, SemanticTokenModifiers) {
		t.Fatalf("semantic token provider = %#v", provider)
	}

	server.documents[uri] = documentState{Text: source, Version: 1}
	params, err := json.Marshal(SemanticTokensParams{TextDocument: TextDocumentIdentifier{URI: uri}})
	if err != nil {
		t.Fatal(err)
	}
	output.Reset()
	if _, err := server.handle(context.Background(), rpcMessage{ID: json.RawMessage("2"), Method: "textDocument/semanticTokens/full", Params: params}); err != nil {
		t.Fatal(err)
	}
	messages = readFramedRPC(t, output.Bytes())
	var tokens SemanticTokens
	if len(messages) != 1 || json.Unmarshal(messages[0].Result, &tokens) != nil {
		t.Fatalf("semantic token messages = %#v", messages)
	}
	want := []uint32{
		0, 0, 4, 1, 1,
		1, 4, 4, 0, 0,
		0, 6, 4, 3, 0,
		1, 4, 5, 2, 0,
	}
	if !reflect.DeepEqual(tokens.Data, want) {
		t.Fatalf("encoded semantic tokens = %#v, want %#v", tokens.Data, want)
	}
}

func TestServerRequestsSemanticRefreshWhenClientSupportsIt(t *testing.T) {
	root := t.TempDir()
	uri := pathToURI(filepath.Join(root, "hyperbricks", "page.hyperbricks.yaml"))
	var output bytes.Buffer
	server := NewServer(bytes.NewReader(nil), &output, ServerOptions{})
	params := InitializeParams{
		RootURI:               pathToURI(root),
		InitializationOptions: InitializeOptions{ProtocolVersion: ProtocolVersion},
	}
	params.Capabilities.Workspace.SemanticTokens.RefreshSupport = true
	initialize, err := json.Marshal(params)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := server.handle(context.Background(), rpcMessage{ID: json.RawMessage("1"), Method: "initialize", Params: initialize}); err != nil {
		t.Fatal(err)
	}
	server.documents[uri] = documentState{Text: "page:\n  - type: html\n  - value: hello\n", Version: 1}
	output.Reset()
	if err := server.reanalyzeOpenDocuments(context.Background()); err != nil {
		t.Fatal(err)
	}
	messages := readFramedRPC(t, output.Bytes())
	if len(messages) != 2 || messages[0].Method != "textDocument/publishDiagnostics" ||
		messages[1].Method != "workspace/semanticTokens/refresh" || len(messages[1].ID) == 0 {
		t.Fatalf("semantic refresh messages = %#v", messages)
	}

	output.Reset()
	if _, err := server.handle(context.Background(), rpcMessage{ID: messages[1].ID, Result: json.RawMessage("null")}); err != nil {
		t.Fatal(err)
	}
	if output.Len() != 0 {
		t.Fatalf("server responded to semantic refresh response: %q", output.String())
	}
}

type semanticExpectation struct {
	text        string
	tokenType   string
	declaration bool
}

func hasSemanticExpectation(t *testing.T, source string, highlights []SemanticHighlight, expected semanticExpectation) bool {
	t.Helper()
	for _, highlight := range highlights {
		if semanticRangeText(t, source, highlight.Range) != expected.text || highlight.TokenType != expected.tokenType {
			continue
		}
		declaration := false
		for _, modifier := range highlight.Modifiers {
			declaration = declaration || modifier == SemanticTokenModifierDeclaration
		}
		if declaration == expected.declaration {
			return true
		}
	}
	return false
}

func hasSemanticText(t *testing.T, source string, highlights []SemanticHighlight, text string) bool {
	t.Helper()
	for _, highlight := range highlights {
		if semanticRangeText(t, source, highlight.Range) == text {
			return true
		}
	}
	return false
}

func describeSemanticHighlights(t *testing.T, source string, highlights []SemanticHighlight) string {
	t.Helper()
	parts := make([]string, 0, len(highlights))
	for _, highlight := range highlights {
		parts = append(parts, semanticRangeText(t, source, highlight.Range)+":"+highlight.TokenType+":"+strings.Join(highlight.Modifiers, ","))
	}
	return strings.Join(parts, " ")
}

func semanticRangeText(t *testing.T, source string, rangeValue Range) string {
	t.Helper()
	lines := splitLines(source)
	if rangeValue.Start.Line < 0 || rangeValue.Start.Line >= len(lines) || rangeValue.End.Line != rangeValue.Start.Line {
		t.Fatalf("invalid semantic source range %#v", rangeValue)
	}
	line := []rune(lines[rangeValue.Start.Line])
	start, end := utf16RuneIndex(line, rangeValue.Start.Character), utf16RuneIndex(line, rangeValue.End.Character)
	if start < 0 || end < start || end > len(line) {
		t.Fatalf("semantic source range %#v does not fit %q", rangeValue, string(line))
	}
	return string(line[start:end])
}

func utf16RuneIndex(runes []rune, column int) int {
	units := 0
	for index, value := range runes {
		if units == column {
			return index
		}
		units += len(utf16.Encode([]rune{value}))
		if units > column {
			return -1
		}
	}
	if units == column {
		return len(runes)
	}
	return -1
}
