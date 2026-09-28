package language

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestServerAdvertisesAndHandlesDefinitionRequestsWithOverlays(t *testing.T) {
	root := t.TempDir()
	sourceDir := filepath.Join(root, "hyperbricks")
	if err := os.MkdirAll(sourceDir, 0o755); err != nil {
		t.Fatal(err)
	}
	pagePath := filepath.Join(sourceDir, "page.hyperbricks.yaml")
	basePath := filepath.Join(sourceDir, "base.hyperbricks.yaml")
	pageURI, baseURI := pathToURI(pagePath), pathToURI(basePath)
	page := "imports: [base.hyperbricks.yaml]\npage:\n  - inherit: base_component\n"
	base := "base_component:\n  - type: html\n"
	var output bytes.Buffer
	server := NewServer(bytes.NewReader(nil), &output, ServerOptions{})

	initialize, err := json.Marshal(InitializeParams{
		RootURI: pathToURI(root),
		InitializationOptions: InitializeOptions{
			ProtocolVersion: ProtocolVersion,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := server.handle(context.Background(), rpcMessage{ID: json.RawMessage("1"), Method: "initialize", Params: initialize}); err != nil {
		t.Fatal(err)
	}
	messages := readFramedRPC(t, output.Bytes())
	if len(messages) != 1 {
		t.Fatalf("initialize messages = %#v", messages)
	}
	var initialized struct {
		Capabilities map[string]interface{} `json:"capabilities"`
	}
	if err := json.Unmarshal(messages[0].Result, &initialized); err != nil {
		t.Fatal(err)
	}
	if initialized.Capabilities["definitionProvider"] != true {
		t.Fatalf("definition capability = %#v", initialized.Capabilities["definitionProvider"])
	}

	server.documents[pageURI] = documentState{Text: page, Version: 4, Dirty: true}
	server.documents[baseURI] = documentState{Text: base, Version: 2, Dirty: true}
	params, err := json.Marshal(TextDocumentPositionParams{
		TextDocument: TextDocumentIdentifier{URI: pageURI},
		Position:     definitionTestPosition(t, page, "base_component"),
	})
	if err != nil {
		t.Fatal(err)
	}
	output.Reset()
	if _, err := server.handle(context.Background(), rpcMessage{ID: json.RawMessage("2"), Method: "textDocument/definition", Params: params}); err != nil {
		t.Fatal(err)
	}
	messages = readFramedRPC(t, output.Bytes())
	if len(messages) != 1 {
		t.Fatalf("definition messages = %#v", messages)
	}
	var links []LocationLink
	if err := json.Unmarshal(messages[0].Result, &links); err != nil {
		t.Fatal(err)
	}
	assertDefinitionLink(t, links, baseURI, Position{Line: 0, Character: 0}, definitionTestRange(t, page, "base_component"))
}

func TestServerDefinitionReturnsEmptyResultForClosedDocument(t *testing.T) {
	var output bytes.Buffer
	server := NewServer(bytes.NewReader(nil), &output, ServerOptions{})
	params, err := json.Marshal(TextDocumentPositionParams{
		TextDocument: TextDocumentIdentifier{URI: pathToURI(filepath.Join(t.TempDir(), "missing.hyperbricks.yaml"))},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := server.handle(context.Background(), rpcMessage{ID: json.RawMessage("1"), Method: "textDocument/definition", Params: params}); err != nil {
		t.Fatal(err)
	}
	messages := readFramedRPC(t, output.Bytes())
	if len(messages) != 1 || string(messages[0].Result) != "[]" {
		t.Fatalf("closed-document definition response = %#v", messages)
	}
}
