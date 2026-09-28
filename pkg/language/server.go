package language

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"sync"
)

type ServerOptions struct {
	Name                        string
	Version                     string
	ResolveRuntimeConnection    RuntimeConnectionResolver
	NewRuntimeDiagnosticsBridge RuntimeDiagnosticsBridgeFactory
}

type documentState struct {
	Text    string
	Version int
	Dirty   bool
}

// Server is a small LSP 3.17 JSON-RPC server. It deliberately uses full text
// synchronization so the HyperBricks parser always receives one coherent
// in-memory source document.
type Server struct {
	input  *bufio.Reader
	output io.Writer
	opts   ServerOptions

	writeMu   sync.Mutex
	stateMu   sync.RWMutex
	runtimeMu sync.Mutex

	analyzer           *Analyzer
	documents          map[string]documentState
	staticDiagnostics  map[string][]Diagnostic
	runtimeDiagnostics map[string][]Diagnostic
	initialDirty       map[string]bool
	shutdown           bool

	initialization    runtimeInitialization
	runtimeConnection RuntimeDiagnosticsConnection
	runtimeBridge     RuntimeDiagnosticsRunner
	runtimeCancel     context.CancelFunc
	runtimeDone       chan struct{}
	runtimeStatus     RuntimeDiagnosticsStatus
}

func NewServer(input io.Reader, output io.Writer, opts ServerOptions) *Server {
	if opts.Name == "" {
		opts.Name = "hyperbricks-language-server"
	}
	return &Server{
		input:              bufio.NewReader(input),
		output:             output,
		opts:               opts,
		documents:          make(map[string]documentState),
		staticDiagnostics:  make(map[string][]Diagnostic),
		runtimeDiagnostics: make(map[string][]Diagnostic),
		initialDirty:       make(map[string]bool),
	}
}

func (server *Server) Serve(ctx context.Context) error {
	defer func() {
		shutdownContext, cancel := runtimeShutdownContext()
		defer cancel()
		_ = server.disconnectRuntime(shutdownContext)
	}()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		message, err := readRPCMessage(server.input)
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return err
		}
		exit, handleErr := server.handle(ctx, message)
		if handleErr != nil {
			return handleErr
		}
		if exit {
			return nil
		}
	}
}

func (server *Server) handle(ctx context.Context, message rpcMessage) (bool, error) {
	isRequest := len(message.ID) > 0
	respond := func(result interface{}, rpcErr *rpcError) error {
		if !isRequest {
			return nil
		}
		return server.writeResponse(message.ID, result, rpcErr)
	}

	switch message.Method {
	case "initialize":
		var params InitializeParams
		if err := json.Unmarshal(message.Params, &params); err != nil {
			return false, respond(nil, &rpcError{Code: -32602, Message: "invalid initialize parameters", Data: err.Error()})
		}
		if version := params.InitializationOptions.ProtocolVersion; version != ProtocolVersion {
			return false, respond(nil, &rpcError{Code: -32602, Message: fmt.Sprintf("unsupported HyperBricks language protocol version %d; expected %d", version, ProtocolVersion)})
		}
		rootURI := params.RootURI
		if len(params.WorkspaceFolders) > 0 && params.WorkspaceFolders[0].URI != "" {
			rootURI = params.WorkspaceFolders[0].URI
		}
		rootPath := params.RootPath
		if rootURI != "" {
			if parsed, err := uriToPath(rootURI); err == nil {
				rootPath = parsed
			}
		}
		server.stateMu.Lock()
		server.analyzer = NewAnalyzer(AnalyzerOptions{
			WorkspaceRoot: rootPath,
			Module:        params.InitializationOptions.Module,
			Config:        params.InitializationOptions.Config,
		})
		server.initialDirty = make(map[string]bool, len(params.InitializationOptions.DirtyDocuments))
		for _, documentURI := range params.InitializationOptions.DirtyDocuments {
			if strings.TrimSpace(documentURI) != "" {
				server.initialDirty[documentURI] = true
			}
		}
		server.stateMu.Unlock()
		server.runtimeMu.Lock()
		server.initialization = runtimeInitialization{
			workspaceRoot: rootPath,
			module:        params.InitializationOptions.Module,
			config:        params.InitializationOptions.Config,
			mode:          normalizeRuntimeDiagnosticsMode(params.InitializationOptions.RuntimeDiagnostics),
			runtimeURL:    strings.TrimSpace(params.InitializationOptions.RuntimeURL),
		}
		server.runtimeMu.Unlock()
		result := map[string]interface{}{
			"capabilities": map[string]interface{}{
				"textDocumentSync": map[string]interface{}{
					"openClose": true,
					"change":    1,
					"save":      map[string]interface{}{"includeText": true},
				},
				"completionProvider": map[string]interface{}{
					"resolveProvider":   false,
					"triggerCharacters": []string{":", ".", "/"},
				},
				"hoverProvider":              true,
				"documentFormattingProvider": true,
				"experimental": map[string]interface{}{
					"hyperbricksProtocolVersion": ProtocolVersion,
				},
			},
			"serverInfo": map[string]interface{}{"name": server.opts.Name, "version": server.opts.Version},
		}
		return false, respond(result, nil)
	case "initialized":
		mode := server.runtimeDiagnosticsMode()
		if mode == "auto" || mode == "on" {
			server.connectRuntime(ctx, mode == "auto")
		}
		return false, nil
	case "workspace/didChangeWatchedFiles":
		return false, server.reanalyzeOpenDocuments(ctx)
	case "workspace/didChangeConfiguration", "$/setTrace", "$/cancelRequest":
		return false, nil
	case "shutdown":
		server.stateMu.Lock()
		server.shutdown = true
		server.stateMu.Unlock()
		shutdownContext, cancel := runtimeShutdownContext()
		defer cancel()
		_ = server.disconnectRuntime(shutdownContext)
		return false, respond(nil, nil)
	case "exit":
		return true, nil
	case "textDocument/didOpen":
		var params didOpenParams
		if err := json.Unmarshal(message.Params, &params); err != nil {
			return false, err
		}
		server.stateMu.Lock()
		dirty := server.initialDirty[params.TextDocument.URI] || strings.HasPrefix(params.TextDocument.URI, "untitled:")
		delete(server.initialDirty, params.TextDocument.URI)
		server.documents[params.TextDocument.URI] = documentState{Text: params.TextDocument.Text, Version: params.TextDocument.Version, Dirty: dirty}
		server.stateMu.Unlock()
		return false, server.reanalyzeOpenDocuments(ctx)
	case "textDocument/didChange":
		var params didChangeParams
		if err := json.Unmarshal(message.Params, &params); err != nil {
			return false, err
		}
		if len(params.ContentChanges) == 0 {
			return false, nil
		}
		server.stateMu.Lock()
		server.documents[params.TextDocument.URI] = documentState{Text: params.ContentChanges[len(params.ContentChanges)-1].Text, Version: params.TextDocument.Version, Dirty: true}
		server.stateMu.Unlock()
		return false, server.reanalyzeOpenDocuments(ctx)
	case "textDocument/didSave":
		var params didSaveParams
		if err := json.Unmarshal(message.Params, &params); err != nil {
			return false, err
		}
		if params.Text != nil {
			server.stateMu.Lock()
			state := server.documents[params.TextDocument.URI]
			state.Text = *params.Text
			state.Dirty = false
			server.documents[params.TextDocument.URI] = state
			server.stateMu.Unlock()
		} else {
			server.stateMu.Lock()
			state := server.documents[params.TextDocument.URI]
			state.Dirty = false
			server.documents[params.TextDocument.URI] = state
			server.stateMu.Unlock()
		}
		return false, server.reanalyzeOpenDocuments(ctx)
	case "textDocument/didClose":
		var params didCloseParams
		if err := json.Unmarshal(message.Params, &params); err != nil {
			return false, err
		}
		server.stateMu.Lock()
		delete(server.documents, params.TextDocument.URI)
		delete(server.staticDiagnostics, params.TextDocument.URI)
		server.stateMu.Unlock()
		if err := server.publishMergedDiagnostics(params.TextDocument.URI); err != nil {
			return false, err
		}
		// Closing an imported document removes its unsaved overlay. Reanalyze
		// every remaining open document so importers immediately observe the
		// saved file again instead of keeping diagnostics from stale text.
		return false, server.reanalyzeOpenDocuments(ctx)
	case "textDocument/completion":
		var params TextDocumentPositionParams
		if err := json.Unmarshal(message.Params, &params); err != nil {
			return false, respond(nil, &rpcError{Code: -32602, Message: err.Error()})
		}
		analyzer, text, documents, ok := server.snapshotFor(params.TextDocument.URI)
		if !ok {
			return false, respond([]CompletionItem{}, nil)
		}
		return false, respond(analyzer.Completions(params.TextDocument.URI, text, params.Position, documents), nil)
	case "textDocument/hover":
		var params TextDocumentPositionParams
		if err := json.Unmarshal(message.Params, &params); err != nil {
			return false, respond(nil, &rpcError{Code: -32602, Message: err.Error()})
		}
		analyzer, text, documents, ok := server.snapshotFor(params.TextDocument.URI)
		if !ok {
			return false, respond(nil, nil)
		}
		return false, respond(analyzer.Hover(params.TextDocument.URI, text, params.Position, documents), nil)
	case "textDocument/formatting":
		var params FormattingParams
		if err := json.Unmarshal(message.Params, &params); err != nil {
			return false, respond(nil, &rpcError{Code: -32602, Message: err.Error()})
		}
		_, text, _, ok := server.snapshotFor(params.TextDocument.URI)
		if !ok {
			return false, respond([]TextEdit{}, nil)
		}
		formatted, err := FormatDocument(text)
		if err != nil {
			return false, respond(nil, &rpcError{Code: -32603, Message: "HyperBricks formatter refused the document", Data: err.Error()})
		}
		if formatted == text {
			return false, respond([]TextEdit{}, nil)
		}
		return false, respond([]TextEdit{{Range: Range{Start: Position{}, End: documentEndPosition(text)}, NewText: formatted}}, nil)
	case MethodRuntimeConnect:
		status := server.connectRuntime(ctx, false)
		return false, respond(runtimeStatusResult(status), nil)
	case MethodRuntimeDisconnect:
		shutdownContext, cancel := runtimeShutdownContext()
		defer cancel()
		if err := server.disconnectRuntime(shutdownContext); err != nil {
			status := RuntimeDiagnosticsStatus{LastError: err.Error()}
			_ = server.PublishRuntimeStatus(ctx, status)
		}
		return false, respond(runtimeStatusResult(server.currentRuntimeStatus()), nil)
	default:
		if isRequest {
			return false, respond(nil, &rpcError{Code: -32601, Message: "method not found: " + message.Method})
		}
		return false, nil
	}
}

func (server *Server) snapshotFor(uri string) (*Analyzer, string, map[string]string, bool) {
	server.stateMu.RLock()
	defer server.stateMu.RUnlock()
	state, ok := server.documents[uri]
	if !ok || server.analyzer == nil {
		return nil, "", nil, false
	}
	documents := make(map[string]string, len(server.documents))
	for documentURI, document := range server.documents {
		documents[documentURI] = document.Text
	}
	return server.analyzer, state.Text, documents, true
}

func (server *Server) reanalyzeOpenDocuments(_ context.Context) error {
	server.stateMu.RLock()
	analyzer := server.analyzer
	documents := make(map[string]string, len(server.documents))
	for uri, document := range server.documents {
		documents[uri] = document.Text
	}
	server.stateMu.RUnlock()
	if analyzer == nil {
		return nil
	}
	URIs := make([]string, 0, len(documents))
	for uri := range documents {
		URIs = append(URIs, uri)
	}
	sort.Strings(URIs)
	for _, uri := range URIs {
		diagnostics := analyzer.Diagnostics(uri, documents[uri], documents)
		server.stateMu.Lock()
		server.staticDiagnostics[uri] = append([]Diagnostic(nil), diagnostics...)
		server.stateMu.Unlock()
		if err := server.publishMergedDiagnostics(uri); err != nil {
			return err
		}
	}
	return nil
}

func (server *Server) publishMergedDiagnostics(uri string) error {
	// Serialize before sampling document state. A concurrent didChange can then
	// either mark the document dirty before this snapshot, or publish its newer
	// version after this complete notification; stale runtime feedback cannot be
	// written last over an unsaved buffer.
	server.writeMu.Lock()
	defer server.writeMu.Unlock()
	server.stateMu.RLock()
	static := append([]Diagnostic(nil), server.staticDiagnostics[uri]...)
	runtime := append([]Diagnostic(nil), server.runtimeDiagnostics[uri]...)
	document, open := server.documents[uri]
	if open && document.Dirty {
		runtime = nil
	}
	server.stateMu.RUnlock()
	diagnostics := append(static, runtime...)
	params := map[string]interface{}{"uri": uri, "diagnostics": diagnostics}
	if open {
		params["version"] = document.Version
	}
	return server.writeMessageLocked(map[string]interface{}{
		"jsonrpc": "2.0", "method": "textDocument/publishDiagnostics", "params": params,
	})
}

// ReplaceRuntimeDiagnostics implements RuntimeDiagnosticsSink. Static and
// runtime diagnostics remain separately owned so clearing a runtime failure
// republishes any still-valid source findings.
func (server *Server) ReplaceRuntimeDiagnostics(_ context.Context, update RuntimeDiagnosticsUpdate) error {
	affected := make(map[string]bool)
	server.stateMu.Lock()
	if update.GenerationChanged {
		for uri := range server.runtimeDiagnostics {
			affected[uri] = true
		}
		server.runtimeDiagnostics = make(map[string][]Diagnostic)
	}
	for file, diagnostics := range update.Diagnostics {
		uri := pathToURI(file)
		mapped := make([]Diagnostic, 0, len(diagnostics))
		for _, diagnostic := range diagnostics {
			mapped = append(mapped, protocolRuntimeDiagnostic(diagnostic))
		}
		server.runtimeDiagnostics[uri] = mapped
		affected[uri] = true
	}
	for _, file := range update.ClearedFiles {
		uri := pathToURI(file)
		delete(server.runtimeDiagnostics, uri)
		affected[uri] = true
	}
	server.stateMu.Unlock()
	URIs := make([]string, 0, len(affected))
	for uri := range affected {
		URIs = append(URIs, uri)
	}
	sort.Strings(URIs)
	for _, uri := range URIs {
		if err := server.publishMergedDiagnostics(uri); err != nil {
			return err
		}
	}
	return nil
}

// PublishRuntimeStatus implements RuntimeDiagnosticsSink without representing
// unchecked routes as source warnings.
func (server *Server) PublishRuntimeStatus(_ context.Context, status RuntimeDiagnosticsStatus) error {
	status = sanitizeRuntimeDiagnosticsStatus(status)
	server.runtimeMu.Lock()
	status = server.enrichRuntimeStatusLocked(status)
	server.runtimeStatus = status
	server.runtimeMu.Unlock()
	return server.writeNotification(MethodRuntimeStatus, runtimeStatusResult(status))
}

func protocolRuntimeDiagnostic(input RuntimeEditorDiagnostic) Diagnostic {
	related := make([]DiagnosticRelatedInformation, 0, len(input.Related))
	for _, item := range input.Related {
		related = append(related, DiagnosticRelatedInformation{
			Location: Location{URI: pathToURI(item.File), Range: Range{
				Start: Position{Line: item.Range.Start.Line, Character: item.Range.Start.Character},
				End:   Position{Line: item.Range.End.Line, Character: item.Range.End.Character},
			}},
			Message: item.Message,
		})
	}
	return Diagnostic{
		Range: Range{
			Start: Position{Line: input.Range.Start.Line, Character: input.Range.Start.Character},
			End:   Position{Line: input.Range.End.Line, Character: input.Range.End.Character},
		},
		Severity:           int(input.Severity),
		Code:               input.Code,
		Source:             input.Source,
		Message:            input.Message,
		RelatedInformation: related,
		Data: map[string]interface{}{
			"componentType": input.ComponentType,
			"componentPath": input.ComponentPath,
			"key":           input.Key,
			"phase":         input.Phase,
			"contexts":      input.Contexts,
		},
	}
}

func (server *Server) writeResponse(id json.RawMessage, result interface{}, rpcErr *rpcError) error {
	message := map[string]interface{}{"jsonrpc": "2.0", "id": json.RawMessage(id)}
	if rpcErr != nil {
		message["error"] = rpcErr
	} else {
		message["result"] = result
	}
	return server.writeMessage(message)
}

func (server *Server) writeNotification(method string, params interface{}) error {
	return server.writeMessage(map[string]interface{}{"jsonrpc": "2.0", "method": method, "params": params})
}

func (server *Server) writeMessage(message interface{}) error {
	server.writeMu.Lock()
	defer server.writeMu.Unlock()
	return server.writeMessageLocked(message)
}

func (server *Server) writeMessageLocked(message interface{}) error {
	payload, err := json.Marshal(message)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintf(server.output, "Content-Length: %d\r\n\r\n", len(payload)); err != nil {
		return err
	}
	_, err = server.output.Write(payload)
	return err
}

func readRPCMessage(reader *bufio.Reader) (rpcMessage, error) {
	contentLength := -1
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return rpcMessage{}, err
		}
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			break
		}
		name, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(name), "Content-Length") {
			parsed, parseErr := strconv.Atoi(strings.TrimSpace(value))
			if parseErr != nil || parsed < 0 {
				return rpcMessage{}, fmt.Errorf("invalid Content-Length header %q", value)
			}
			contentLength = parsed
		}
	}
	if contentLength < 0 {
		return rpcMessage{}, fmt.Errorf("missing Content-Length header")
	}
	payload := make([]byte, contentLength)
	if _, err := io.ReadFull(reader, payload); err != nil {
		return rpcMessage{}, err
	}
	var message rpcMessage
	if err := json.Unmarshal(payload, &message); err != nil {
		return rpcMessage{}, err
	}
	return message, nil
}
