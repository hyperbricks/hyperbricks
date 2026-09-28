package language

import "encoding/json"

// ProtocolVersion is the compatibility contract between the HyperBricks
// executable and editor clients. It is intentionally independent from the LSP
// version, which is currently 3.17.
const ProtocolVersion = 1

const (
	MethodRuntimeConnect    = "hyperbricks/runtime/connect"
	MethodRuntimeDisconnect = "hyperbricks/runtime/disconnect"
	MethodRuntimeStatus     = "hyperbricks/runtime/status"
)

type Position struct {
	Line      int `json:"line"`
	Character int `json:"character"`
}

type Range struct {
	Start Position `json:"start"`
	End   Position `json:"end"`
}

type Location struct {
	URI   string `json:"uri"`
	Range Range  `json:"range"`
}

// LocationLink carries both the complete source relation under the cursor and
// its destination. Definition clients use OriginSelectionRange for the
// Cmd/Ctrl-click underline instead of guessing a punctuation-delimited word.
type LocationLink struct {
	OriginSelectionRange *Range `json:"originSelectionRange,omitempty"`
	TargetURI            string `json:"targetUri"`
	TargetRange          Range  `json:"targetRange"`
	TargetSelectionRange Range  `json:"targetSelectionRange"`
}

type DiagnosticRelatedInformation struct {
	Location Location `json:"location"`
	Message  string   `json:"message"`
}

type Diagnostic struct {
	Range              Range                          `json:"range"`
	Severity           int                            `json:"severity,omitempty"`
	Code               string                         `json:"code,omitempty"`
	Source             string                         `json:"source,omitempty"`
	Message            string                         `json:"message"`
	RelatedInformation []DiagnosticRelatedInformation `json:"relatedInformation,omitempty"`
	Data               interface{}                    `json:"data,omitempty"`
}

const (
	DiagnosticSeverityError       = 1
	DiagnosticSeverityWarning     = 2
	DiagnosticSeverityInformation = 3
)

type CompletionItem struct {
	Label            string      `json:"label"`
	Kind             int         `json:"kind,omitempty"`
	Detail           string      `json:"detail,omitempty"`
	Documentation    interface{} `json:"documentation,omitempty"`
	InsertText       string      `json:"insertText,omitempty"`
	InsertTextFormat int         `json:"insertTextFormat,omitempty"`
	SortText         string      `json:"sortText,omitempty"`
}

const (
	CompletionItemKindField     = 5
	CompletionItemKindFile      = 17
	CompletionItemKindReference = 18
	CompletionItemKindClass     = 7
	InsertTextFormatSnippet     = 2
)

type MarkupContent struct {
	Kind  string `json:"kind"`
	Value string `json:"value"`
}

type Hover struct {
	Contents MarkupContent `json:"contents"`
	Range    *Range        `json:"range,omitempty"`
}

type TextEdit struct {
	Range   Range  `json:"range"`
	NewText string `json:"newText"`
}

type InitializeOptions struct {
	ProtocolVersion    int      `json:"protocolVersion"`
	Module             string   `json:"module,omitempty"`
	Config             string   `json:"config,omitempty"`
	RuntimeDiagnostics string   `json:"runtimeDiagnostics,omitempty"`
	RuntimeURL         string   `json:"runtimeUrl,omitempty"`
	DirtyDocuments     []string `json:"dirtyDocuments,omitempty"`
}

type InitializeParams struct {
	ProcessID             *int              `json:"processId,omitempty"`
	RootPath              string            `json:"rootPath,omitempty"`
	RootURI               string            `json:"rootUri,omitempty"`
	WorkspaceFolders      []WorkspaceFolder `json:"workspaceFolders,omitempty"`
	InitializationOptions InitializeOptions `json:"initializationOptions,omitempty"`
}

type WorkspaceFolder struct {
	URI  string `json:"uri"`
	Name string `json:"name"`
}

type TextDocumentIdentifier struct {
	URI string `json:"uri"`
}

type VersionedTextDocumentIdentifier struct {
	URI     string `json:"uri"`
	Version int    `json:"version"`
}

type TextDocumentItem struct {
	URI        string `json:"uri"`
	LanguageID string `json:"languageId,omitempty"`
	Version    int    `json:"version"`
	Text       string `json:"text"`
}

type TextDocumentPositionParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
	Position     Position               `json:"position"`
}

type FormattingParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
	Options      struct {
		TabSize      int  `json:"tabSize"`
		InsertSpaces bool `json:"insertSpaces"`
	} `json:"options"`
}

type didOpenParams struct {
	TextDocument TextDocumentItem `json:"textDocument"`
}

type contentChange struct {
	Text string `json:"text"`
}

type didChangeParams struct {
	TextDocument   VersionedTextDocumentIdentifier `json:"textDocument"`
	ContentChanges []contentChange                 `json:"contentChanges"`
}

type didCloseParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
}

type didSaveParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
	Text         *string                `json:"text,omitempty"`
}

type rpcMessage struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}
