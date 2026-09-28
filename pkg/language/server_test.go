package language

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestServerInitializePublishesProtocolCapabilitiesAndStaticDiagnostics(t *testing.T) {
	root := t.TempDir()
	uri := pathToURI(filepath.Join(root, "page.hyperbricks.yaml"))
	input := bytes.Join([][]byte{
		framedRPC(t, map[string]interface{}{
			"jsonrpc": "2.0", "id": 1, "method": "initialize",
			"params": map[string]interface{}{
				"rootUri":               pathToURI(root),
				"initializationOptions": map[string]interface{}{"protocolVersion": ProtocolVersion},
			},
		}),
		framedRPC(t, map[string]interface{}{
			"jsonrpc": "2.0", "method": "textDocument/didOpen",
			"params": map[string]interface{}{"textDocument": map[string]interface{}{
				"uri": uri, "languageId": "hyperbricks", "version": 1,
				"text": "page:\n  - type: html\n",
			}},
		}),
		framedRPC(t, map[string]interface{}{"jsonrpc": "2.0", "id": 2, "method": "shutdown"}),
		framedRPC(t, map[string]interface{}{"jsonrpc": "2.0", "method": "exit"}),
	}, nil)
	var output bytes.Buffer
	server := NewServer(bytes.NewReader(input), &output, ServerOptions{Version: "test"})
	if err := server.Serve(context.Background()); err != nil {
		t.Fatal(err)
	}
	messages := readFramedRPC(t, output.Bytes())
	if len(messages) != 3 {
		t.Fatalf("messages = %d: %#v", len(messages), messages)
	}
	var initialize map[string]interface{}
	if err := json.Unmarshal(messages[0].Result, &initialize); err != nil {
		t.Fatal(err)
	}
	capabilities := initialize["capabilities"].(map[string]interface{})
	experimental := capabilities["experimental"].(map[string]interface{})
	if experimental["hyperbricksProtocolVersion"] != float64(ProtocolVersion) || capabilities["hoverProvider"] != true || capabilities["documentFormattingProvider"] != true {
		t.Fatalf("initialize result = %#v", initialize)
	}
	if messages[1].Method != "textDocument/publishDiagnostics" {
		t.Fatalf("second message = %#v", messages[1])
	}
	var published struct {
		URI         string       `json:"uri"`
		Diagnostics []Diagnostic `json:"diagnostics"`
	}
	if err := json.Unmarshal(messages[1].Params, &published); err != nil {
		t.Fatal(err)
	}
	if published.URI != uri || len(published.Diagnostics) != 1 || published.Diagnostics[0].Code != "component.missing_required_field" {
		t.Fatalf("published diagnostics = %#v", published)
	}
}

func TestServerRejectsIncompatibleProtocolVersion(t *testing.T) {
	for _, version := range []int{0, ProtocolVersion + 1} {
		t.Run(jsonNumber(version), func(t *testing.T) {
			input := framedRPC(t, map[string]interface{}{
				"jsonrpc": "2.0", "id": 1, "method": "initialize",
				"params": map[string]interface{}{"initializationOptions": map[string]interface{}{"protocolVersion": version}},
			})
			var output bytes.Buffer
			server := NewServer(bytes.NewReader(input), &output, ServerOptions{})
			if err := server.Serve(context.Background()); err != nil {
				t.Fatal(err)
			}
			messages := readFramedRPC(t, output.Bytes())
			if len(messages) != 1 || messages[0].Error == nil || messages[0].Error.Code != -32602 {
				t.Fatalf("protocol mismatch response = %#v", messages)
			}
		})
	}
}

func TestServerMergesClearsAndSuppressesRuntimeDiagnosticsForDirtyDocuments(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "page.hyperbricks.yaml")
	uri := pathToURI(file)
	var output bytes.Buffer
	server := NewServer(bytes.NewReader(nil), &output, ServerOptions{})
	server.staticDiagnostics[uri] = []Diagnostic{{Code: "static", Message: "static", Severity: DiagnosticSeverityError}}
	server.documents[uri] = documentState{Text: "page: {}\n"}
	runtimeDiagnostic := RuntimeEditorDiagnostic{
		Source: RuntimeDiagnosticsSource, Code: "runtime", Message: "runtime", Severity: RuntimeSeverityError, File: file,
		Range:    RuntimeEditorRange{Start: RuntimeEditorPosition{Line: 1, Character: 2}, End: RuntimeEditorPosition{Line: 1, Character: 3}},
		Contexts: []RuntimeRequestContext{{RequestID: "request-one", Route: "/"}},
	}
	update := RuntimeDiagnosticsUpdate{Diagnostics: map[string][]RuntimeEditorDiagnostic{file: {runtimeDiagnostic}}}
	if err := server.ReplaceRuntimeDiagnostics(context.Background(), update); err != nil {
		t.Fatal(err)
	}
	if got := publishedDiagnosticCount(t, output.Bytes()); got != 2 {
		t.Fatalf("merged diagnostic count = %d", got)
	}

	output.Reset()
	server.documents[uri] = documentState{Text: "changed", Dirty: true}
	if err := server.ReplaceRuntimeDiagnostics(context.Background(), update); err != nil {
		t.Fatal(err)
	}
	if got := publishedDiagnosticCount(t, output.Bytes()); got != 1 {
		t.Fatalf("dirty-document diagnostic count = %d", got)
	}

	output.Reset()
	server.documents[uri] = documentState{Text: "saved", Dirty: false}
	if err := server.publishMergedDiagnostics(uri); err != nil {
		t.Fatal(err)
	}
	if got := publishedDiagnosticCount(t, output.Bytes()); got != 2 {
		t.Fatalf("saved-document diagnostic count = %d", got)
	}

	output.Reset()
	if err := server.ReplaceRuntimeDiagnostics(context.Background(), RuntimeDiagnosticsUpdate{ClearedFiles: []string{file}}); err != nil {
		t.Fatal(err)
	}
	if got := publishedDiagnosticCount(t, output.Bytes()); got != 1 {
		t.Fatalf("static-only diagnostic count = %d", got)
	}
}

func TestServerPreservesDirtySuppressionAcrossClientRestart(t *testing.T) {
	root := t.TempDir()
	uri := pathToURI(filepath.Join(root, "hyperbricks", "page.hyperbricks.yaml"))
	server := NewServer(bytes.NewReader(nil), io.Discard, ServerOptions{})
	initializeParams, err := json.Marshal(InitializeParams{
		RootURI: pathToURI(root),
		InitializationOptions: InitializeOptions{
			ProtocolVersion: ProtocolVersion,
			DirtyDocuments:  []string{uri},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := server.handle(context.Background(), rpcMessage{ID: json.RawMessage("1"), Method: "initialize", Params: initializeParams}); err != nil {
		t.Fatal(err)
	}
	openParams, err := json.Marshal(didOpenParams{TextDocument: TextDocumentItem{
		URI: uri, LanguageID: "hyperbricks", Version: 7,
		Text: "page:\n  - type: html\n  - value: unsaved\n",
	}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := server.handle(context.Background(), rpcMessage{Method: "textDocument/didOpen", Params: openParams}); err != nil {
		t.Fatal(err)
	}
	if document := server.documents[uri]; !document.Dirty {
		t.Fatalf("reopened document state = %#v", document)
	}

	var output bytes.Buffer
	server.output = &output
	file, err := uriToPath(uri)
	if err != nil {
		t.Fatal(err)
	}
	runtimeDiagnostic := RuntimeEditorDiagnostic{
		Source: RuntimeDiagnosticsSource, Code: "runtime", Message: "saved runtime failure",
		Severity: RuntimeSeverityError, File: file,
	}
	if err := server.ReplaceRuntimeDiagnostics(context.Background(), RuntimeDiagnosticsUpdate{
		Diagnostics: map[string][]RuntimeEditorDiagnostic{file: {runtimeDiagnostic}},
	}); err != nil {
		t.Fatal(err)
	}
	messages := readFramedRPC(t, output.Bytes())
	if len(messages) != 1 {
		t.Fatalf("dirty runtime publications = %#v", messages)
	}
	var published struct {
		Version     *int         `json:"version"`
		Diagnostics []Diagnostic `json:"diagnostics"`
	}
	if err := json.Unmarshal(messages[0].Params, &published); err != nil {
		t.Fatal(err)
	}
	if published.Version == nil || *published.Version != 7 || len(published.Diagnostics) != 0 {
		t.Fatalf("dirty runtime publication = %#v", published)
	}
}

func TestPublishDiagnosticsSamplesDirtyStateAfterAcquiringWriteOrder(t *testing.T) {
	uri := pathToURI(filepath.Join(t.TempDir(), "page.hyperbricks.yaml"))
	var output bytes.Buffer
	server := NewServer(bytes.NewReader(nil), &output, ServerOptions{})
	server.documents[uri] = documentState{Text: "saved", Version: 1}
	server.runtimeDiagnostics[uri] = []Diagnostic{{
		Code: "runtime", Source: RuntimeDiagnosticsSource, Message: "stale runtime failure",
	}}

	server.writeMu.Lock()
	started := make(chan struct{})
	done := make(chan error, 1)
	go func() {
		close(started)
		done <- server.publishMergedDiagnostics(uri)
	}()
	<-started
	// Give the publisher time to reach the held write-order lock. The protected
	// state must remain unsampled until that lock becomes available.
	time.Sleep(10 * time.Millisecond)
	server.stateMu.Lock()
	server.documents[uri] = documentState{Text: "unsaved", Version: 2, Dirty: true}
	server.stateMu.Unlock()
	server.writeMu.Unlock()
	if err := <-done; err != nil {
		t.Fatal(err)
	}

	messages := readFramedRPC(t, output.Bytes())
	if len(messages) != 1 {
		t.Fatalf("diagnostic publications = %#v", messages)
	}
	var published struct {
		Version     *int         `json:"version"`
		Diagnostics []Diagnostic `json:"diagnostics"`
	}
	if err := json.Unmarshal(messages[0].Params, &published); err != nil {
		t.Fatal(err)
	}
	if published.Version == nil || *published.Version != 2 || len(published.Diagnostics) != 0 {
		t.Fatalf("serialized dirty publication = %#v", published)
	}
}

func TestServerPublishesLowerCamelRuntimeStatus(t *testing.T) {
	var output bytes.Buffer
	server := NewServer(bytes.NewReader(nil), &output, ServerOptions{})
	if err := server.PublishRuntimeStatus(context.Background(), RuntimeDiagnosticsStatus{
		Connected: true, ErrorCount: 3, CheckedRoutes: 2, TotalRoutes: 4,
		EvictedContexts: 2, UnmappedIssues: []string{"configuration failed (code config-one)"},
		UncheckedRoutes: []string{
			"/ordinary",
			"/products?token=route-secret#fragment",
			"https://developer:url-secret@example.test/orders?token=query-secret#fragment",
		},
		LastError: "remote fetch failed\nAuthorization: Bearer auth-secret; token=reason-secret; " +
			"https://developer:url-secret@example.test/status?token=query-secret#fragment",
	}); err != nil {
		t.Fatal(err)
	}
	messages := readFramedRPC(t, output.Bytes())
	var params map[string]interface{}
	if len(messages) != 1 || messages[0].Method != MethodRuntimeStatus || json.Unmarshal(messages[0].Params, &params) != nil {
		t.Fatalf("status messages = %#v", messages)
	}
	issues, _ := params["unmappedIssues"].([]interface{})
	routes, _ := params["uncheckedRoutes"].([]interface{})
	lastError, _ := params["lastError"].(string)
	if params["errorCount"] != float64(3) || params["checkedRoutes"] != float64(2) || params["evictedContexts"] != float64(2) || len(issues) != 1 || issues[0] != "configuration failed (code config-one)" ||
		!reflect.DeepEqual(routes, []interface{}{"/orders", "/ordinary", "/products"}) ||
		!strings.Contains(lastError, "remote fetch failed") || !strings.Contains(lastError, "https://example.test/status") || strings.Contains(lastError, "\n") || params["ErrorCount"] != nil {
		t.Fatalf("runtime status = %#v", params)
	}
	for _, forbidden := range []string{"auth-secret", "reason-secret", "route-secret", "url-secret", "query-secret", "fragment"} {
		if strings.Contains(output.String(), forbidden) {
			t.Fatalf("runtime status protocol exposed %q: %s", forbidden, output.String())
		}
	}
}

func TestServerReanalyzesOpenDocumentsWhenWatchedFilesChange(t *testing.T) {
	root := t.TempDir()
	uri := pathToURI(filepath.Join(root, "page.hyperbricks.yaml"))
	input := bytes.Join([][]byte{
		framedRPC(t, map[string]interface{}{
			"jsonrpc": "2.0", "id": 1, "method": "initialize",
			"params": map[string]interface{}{
				"rootUri":               pathToURI(root),
				"initializationOptions": map[string]interface{}{"protocolVersion": ProtocolVersion, "runtimeDiagnostics": "off"},
			},
		}),
		framedRPC(t, map[string]interface{}{
			"jsonrpc": "2.0", "method": "textDocument/didOpen",
			"params": map[string]interface{}{"textDocument": map[string]interface{}{
				"uri": uri, "languageId": "hyperbricks", "version": 1, "text": "page:\n  - type: html\n",
			}},
		}),
		framedRPC(t, map[string]interface{}{
			"jsonrpc": "2.0", "method": "workspace/didChangeWatchedFiles",
			"params": map[string]interface{}{"changes": []interface{}{}},
		}),
	}, nil)
	var output bytes.Buffer
	if err := NewServer(bytes.NewReader(input), &output, ServerOptions{}).Serve(context.Background()); err != nil {
		t.Fatal(err)
	}
	publications := 0
	for _, message := range readFramedRPC(t, output.Bytes()) {
		if message.Method == "textDocument/publishDiagnostics" {
			publications++
		}
	}
	if publications != 2 {
		t.Fatalf("diagnostic publications = %d, want open + watched-file refresh", publications)
	}
}

func TestServerReanalyzesImportersWhenUnsavedImportedDocumentCloses(t *testing.T) {
	root := t.TempDir()
	sourceDir := filepath.Join(root, "hyperbricks")
	if err := os.MkdirAll(sourceDir, 0o755); err != nil {
		t.Fatal(err)
	}
	basePath := filepath.Join(sourceDir, "base.hyperbricks.yaml")
	if err := os.WriteFile(basePath, []byte("base:\n  - type: not_native\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	baseURI := pathToURI(basePath)
	pageURI := pathToURI(filepath.Join(sourceDir, "page.hyperbricks.yaml"))
	server := NewServer(bytes.NewReader(nil), io.Discard, ServerOptions{})
	server.analyzer = NewAnalyzer(AnalyzerOptions{WorkspaceRoot: root})
	server.documents[baseURI] = documentState{Text: "base:\n  - type: html\n  - value: valid overlay\n"}
	server.documents[pageURI] = documentState{Text: "imports: [base.hyperbricks.yaml]\npage:\n  - inherit: base\n"}
	if err := server.reanalyzeOpenDocuments(context.Background()); err != nil {
		t.Fatal(err)
	}
	if diagnostics := server.staticDiagnostics[pageURI]; len(diagnostics) != 0 {
		t.Fatalf("importer diagnostics with open overlay = %#v", diagnostics)
	}

	params, err := json.Marshal(didCloseParams{TextDocument: TextDocumentIdentifier{URI: baseURI}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := server.handle(context.Background(), rpcMessage{Method: "textDocument/didClose", Params: params}); err != nil {
		t.Fatal(err)
	}
	diagnostics := server.staticDiagnostics[pageURI]
	if len(diagnostics) != 1 || diagnostics[0].Code != "component.unknown_type" || !bytes.Contains([]byte(diagnostics[0].Message), []byte("base.hyperbricks.yaml")) {
		t.Fatalf("importer diagnostics after overlay close = %#v", diagnostics)
	}
}

func TestServerHoverUsesUnsavedImportedDocumentOverlay(t *testing.T) {
	root := t.TempDir()
	sourceDir := filepath.Join(root, "hyperbricks")
	if err := os.MkdirAll(sourceDir, 0o755); err != nil {
		t.Fatal(err)
	}
	pageURI := pathToURI(filepath.Join(sourceDir, "page.hyperbricks.yaml"))
	baseURI := pathToURI(filepath.Join(sourceDir, "base.hyperbricks.yaml"))
	page := "imports: [base.hyperbricks.yaml]\npage:\n  - inherit: base\n  - route: inherited\n"
	var output bytes.Buffer
	server := NewServer(bytes.NewReader(nil), &output, ServerOptions{})
	server.analyzer = NewAnalyzer(AnalyzerOptions{WorkspaceRoot: root})
	server.documents[pageURI] = documentState{Text: page}
	server.documents[baseURI] = documentState{Text: "base:\n  - type: hypermedia\n"}
	params, err := json.Marshal(TextDocumentPositionParams{
		TextDocument: TextDocumentIdentifier{URI: pageURI},
		Position:     Position{Line: 3, Character: 5},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := server.handle(context.Background(), rpcMessage{ID: json.RawMessage("1"), Method: "textDocument/hover", Params: params}); err != nil {
		t.Fatal(err)
	}
	messages := readFramedRPC(t, output.Bytes())
	if len(messages) != 1 {
		t.Fatalf("hover responses = %#v", messages)
	}
	var hover Hover
	if err := json.Unmarshal(messages[0].Result, &hover); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(hover.Contents.Value, "**route**") {
		t.Fatalf("server inherited overlay hover = %#v", hover)
	}
}

func framedRPC(t *testing.T, value interface{}) []byte {
	t.Helper()
	payload, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return append([]byte("Content-Length: "+jsonNumber(len(payload))+"\r\n\r\n"), payload...)
}

func jsonNumber(value int) string {
	payload, _ := json.Marshal(value)
	return string(payload)
}

func readFramedRPC(t *testing.T, raw []byte) []rpcMessage {
	t.Helper()
	reader := bufio.NewReader(bytes.NewReader(raw))
	var messages []rpcMessage
	for {
		message, err := readRPCMessage(reader)
		if err == io.EOF {
			return messages
		}
		if err != nil {
			t.Fatal(err)
		}
		messages = append(messages, message)
	}
}

func publishedDiagnosticCount(t *testing.T, raw []byte) int {
	t.Helper()
	messages := readFramedRPC(t, raw)
	if len(messages) != 1 || messages[0].Method != "textDocument/publishDiagnostics" {
		t.Fatalf("published messages = %#v", messages)
	}
	var params struct {
		Diagnostics []Diagnostic `json:"diagnostics"`
	}
	if err := json.Unmarshal(messages[0].Params, &params); err != nil {
		t.Fatal(err)
	}
	return len(params.Diagnostics)
}
