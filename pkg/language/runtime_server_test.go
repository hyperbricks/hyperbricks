package language

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

type runtimeLifecycleTestRunner struct {
	sink RuntimeDiagnosticsSink

	mu          sync.Mutex
	runs        int
	disconnects int
}

func (runner *runtimeLifecycleTestRunner) Run(ctx context.Context) error {
	runner.mu.Lock()
	runner.runs++
	runner.mu.Unlock()
	_ = runner.sink.PublishRuntimeStatus(ctx, RuntimeDiagnosticsStatus{
		Connected:     true,
		Generation:    9,
		ErrorCount:    2,
		CheckedRoutes: 1,
		TotalRoutes:   2,
	})
	<-ctx.Done()
	return nil
}

func (runner *runtimeLifecycleTestRunner) Disconnect(ctx context.Context) error {
	runner.mu.Lock()
	runner.disconnects++
	runner.mu.Unlock()
	if err := runner.sink.ReplaceRuntimeDiagnostics(ctx, RuntimeDiagnosticsUpdate{}); err != nil {
		return err
	}
	return runner.sink.PublishRuntimeStatus(ctx, RuntimeDiagnosticsStatus{})
}

func (runner *runtimeLifecycleTestRunner) counts() (int, int) {
	runner.mu.Lock()
	defer runner.mu.Unlock()
	return runner.runs, runner.disconnects
}

func TestServerRuntimeConnectDisconnectLifecycleAndStatusContract(t *testing.T) {
	root := t.TempDir()
	moduleRoot := filepath.Join(root, "modules", "demo")
	var resolved RuntimeConnectionRequest
	var factoryConnection RuntimeDiagnosticsConnection
	var runner *runtimeLifecycleTestRunner
	options := ServerOptions{
		ResolveRuntimeConnection: func(_ context.Context, request RuntimeConnectionRequest) (RuntimeDiagnosticsConnection, error) {
			resolved = request
			return RuntimeDiagnosticsConnection{ModuleRoot: moduleRoot, RuntimeURL: "http://localhost:9090", ErrorsURL: "http://localhost:9090/__hyperbricks/errors"}, nil
		},
		NewRuntimeDiagnosticsBridge: func(connection RuntimeDiagnosticsConnection, sink RuntimeDiagnosticsSink) (RuntimeDiagnosticsRunner, error) {
			factoryConnection = connection
			runner = &runtimeLifecycleTestRunner{sink: sink}
			return runner, nil
		},
	}
	input := bytes.Join([][]byte{
		framedRPC(t, map[string]interface{}{
			"jsonrpc": "2.0", "id": 1, "method": "initialize",
			"params": map[string]interface{}{
				"rootUri": pathToURI(root),
				"initializationOptions": map[string]interface{}{
					"protocolVersion": ProtocolVersion,
					"module":          "demo", "config": "profiles/dev.hyperbricks.yaml",
					"runtimeDiagnostics": "off", "runtimeUrl": "http://localhost:9090",
				},
			},
		}),
		framedRPC(t, map[string]interface{}{"jsonrpc": "2.0", "method": "initialized"}),
		framedRPC(t, map[string]interface{}{"jsonrpc": "2.0", "id": 2, "method": MethodRuntimeConnect}),
		framedRPC(t, map[string]interface{}{"jsonrpc": "2.0", "id": 3, "method": MethodRuntimeDisconnect}),
		framedRPC(t, map[string]interface{}{"jsonrpc": "2.0", "id": 4, "method": "shutdown"}),
		framedRPC(t, map[string]interface{}{"jsonrpc": "2.0", "method": "exit"}),
	}, nil)
	var output bytes.Buffer
	server := NewServer(bytes.NewReader(input), &output, options)
	if err := server.Serve(context.Background()); err != nil {
		t.Fatal(err)
	}

	if resolved.WorkspaceRoot != root || resolved.Module != "demo" || resolved.Config != "profiles/dev.hyperbricks.yaml" || resolved.RuntimeURL != "http://localhost:9090" {
		t.Fatalf("runtime resolver request = %#v", resolved)
	}
	if factoryConnection.ModuleRoot != moduleRoot || factoryConnection.RuntimeURL != "http://localhost:9090" {
		t.Fatalf("factory connection = %#v", factoryConnection)
	}
	if runs, disconnects := runner.counts(); runs != 1 || disconnects != 1 {
		t.Fatalf("runtime lifecycle runs=%d disconnects=%d", runs, disconnects)
	}

	messages := readFramedRPC(t, output.Bytes())
	connect := responseResultByID(t, messages, "2")
	if connect["runtimeUrl"] != "http://localhost:9090" || connect["errorsUrl"] != "http://localhost:9090/__hyperbricks/errors" || connect["errorCount"] == nil || connect["RuntimeURL"] != nil {
		t.Fatalf("connect status contract = %#v", connect)
	}
	disconnect := responseResultByID(t, messages, "3")
	if disconnect["connected"] != false || disconnect["runtimeUrl"] != "http://localhost:9090" {
		t.Fatalf("disconnect status contract = %#v", disconnect)
	}
	encoded, err := json.Marshal(messages)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), "password") || strings.Contains(string(encoded), "secret") {
		t.Fatalf("runtime status exposed credentials: %s", encoded)
	}
}

func TestServerRuntimeAutoStartsAndAllTerminationsCancelAndClear(t *testing.T) {
	for _, test := range []struct {
		name        string
		termination []byte
	}{
		{name: "EOF"},
		{name: "exit", termination: framedRPC(t, map[string]interface{}{"jsonrpc": "2.0", "method": "exit"})},
		{name: "shutdown", termination: framedRPC(t, map[string]interface{}{"jsonrpc": "2.0", "id": 2, "method": "shutdown"})},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			var runner *runtimeLifecycleTestRunner
			input := bytes.Join([][]byte{
				framedRPC(t, map[string]interface{}{
					"jsonrpc": "2.0", "id": 1, "method": "initialize",
					"params": map[string]interface{}{
						"rootUri": pathToURI(root),
						"initializationOptions": map[string]interface{}{
							"protocolVersion": ProtocolVersion, "module": "demo", "runtimeDiagnostics": "auto",
						},
					},
				}),
				framedRPC(t, map[string]interface{}{"jsonrpc": "2.0", "method": "initialized"}),
				test.termination,
			}, nil)
			server := NewServer(bytes.NewReader(input), &bytes.Buffer{}, ServerOptions{
				ResolveRuntimeConnection: func(_ context.Context, _ RuntimeConnectionRequest) (RuntimeDiagnosticsConnection, error) {
					return RuntimeDiagnosticsConnection{ModuleRoot: filepath.Join(root, "modules", "demo"), RuntimeURL: "http://localhost:8080"}, nil
				},
				NewRuntimeDiagnosticsBridge: func(_ RuntimeDiagnosticsConnection, sink RuntimeDiagnosticsSink) (RuntimeDiagnosticsRunner, error) {
					runner = &runtimeLifecycleTestRunner{sink: sink}
					return runner, nil
				},
			})
			if err := server.Serve(context.Background()); err != nil {
				t.Fatal(err)
			}
			if runner == nil {
				t.Fatal("auto mode did not construct the runtime bridge")
			}
			if runs, disconnects := runner.counts(); runs != 1 || disconnects != 1 {
				t.Fatalf("%s lifecycle runs=%d disconnects=%d", test.name, runs, disconnects)
			}
		})
	}
}

func TestServerRuntimeResolverFailureIsStatusOnly(t *testing.T) {
	var output bytes.Buffer
	server := NewServer(bytes.NewReader(nil), &output, ServerOptions{
		ResolveRuntimeConnection: func(context.Context, RuntimeConnectionRequest) (RuntimeDiagnosticsConnection, error) {
			return RuntimeDiagnosticsConnection{}, fmt.Errorf(
				"runtime resolution failed\nAuthorization: Bearer resolver-secret; token=status-secret; " +
					"https://developer:url-secret@example.test/status?token=query-secret#fragment",
			)
		},
	})
	server.initialization = runtimeInitialization{runtimeURL: "https://runtime.example.test"}
	status := server.connectRuntime(context.Background(), false)
	if status.Connected || !strings.Contains(status.LastError, "runtime resolution failed") ||
		!strings.Contains(status.LastError, "https://example.test/status") || status.RuntimeURL != "https://runtime.example.test" {
		t.Fatalf("failure status = %#v", status)
	}
	if strings.Contains(server.runtimeStatus.LastError, "\n") || strings.Contains(status.LastError, "\n") {
		t.Fatalf("resolver failure retained control characters: stored=%#v returned=%#v", server.runtimeStatus, status)
	}
	for _, forbidden := range []string{"resolver-secret", "status-secret", "url-secret", "query-secret", "fragment"} {
		if strings.Contains(server.runtimeStatus.LastError, forbidden) || strings.Contains(status.LastError, forbidden) || strings.Contains(output.String(), forbidden) {
			t.Fatalf("resolver failure status exposed %q: stored=%#v returned=%#v output=%s", forbidden, server.runtimeStatus, status, output.String())
		}
	}
	for _, message := range readFramedRPC(t, output.Bytes()) {
		if message.Method == "textDocument/publishDiagnostics" {
			t.Fatalf("transport failure became a source diagnostic: %#v", message)
		}
	}
}

func TestServerRuntimeDisconnectClearsResolverFailureStatus(t *testing.T) {
	var output bytes.Buffer
	server := NewServer(bytes.NewReader(nil), &output, ServerOptions{
		ResolveRuntimeConnection: func(context.Context, RuntimeConnectionRequest) (RuntimeDiagnosticsConnection, error) {
			return RuntimeDiagnosticsConnection{}, context.DeadlineExceeded
		},
	})
	server.initialization = runtimeInitialization{runtimeURL: "https://runtime.example.test"}
	if status := server.connectRuntime(context.Background(), false); status.LastError == "" {
		t.Fatalf("resolver failure status = %#v", status)
	}
	output.Reset()
	if err := server.disconnectRuntime(context.Background()); err != nil {
		t.Fatal(err)
	}
	status := server.currentRuntimeStatus()
	if status.Connected || status.LastError != "" || status.ErrorsURL != "" {
		t.Fatalf("disconnect retained failed runtime status = %#v", status)
	}
	messages := readFramedRPC(t, output.Bytes())
	if len(messages) != 1 || messages[0].Method != MethodRuntimeStatus {
		t.Fatalf("disconnect status notifications = %#v", messages)
	}
}

func TestServerRuntimeResolverFailureNeverEchoesURLCredentials(t *testing.T) {
	var output bytes.Buffer
	server := NewServer(bytes.NewReader(nil), &output, ServerOptions{
		ResolveRuntimeConnection: func(context.Context, RuntimeConnectionRequest) (RuntimeDiagnosticsConnection, error) {
			return RuntimeDiagnosticsConnection{}, context.DeadlineExceeded
		},
	})
	server.initialization = runtimeInitialization{runtimeURL: "https://developer:private-secret@runtime.example.test/token?key=query-secret"}
	status := server.connectRuntime(context.Background(), false)
	if status.RuntimeURL != "" || status.ErrorsURL != "" {
		t.Fatalf("credential-bearing URL reached status: %#v", status)
	}
	if encoded := output.String(); strings.Contains(encoded, "private-secret") || strings.Contains(encoded, "query-secret") || strings.Contains(encoded, "developer") {
		t.Fatalf("credential-bearing URL reached LSP output: %s", encoded)
	}
}

func TestServerAutoModeSilentlySkipsUnavailableLiveRuntimeButOnReportsIt(t *testing.T) {
	for _, test := range []struct {
		mode       string
		wantStatus bool
	}{
		{mode: "auto", wantStatus: false},
		{mode: "on", wantStatus: true},
	} {
		t.Run(test.mode, func(t *testing.T) {
			input := bytes.Join([][]byte{
				framedRPC(t, map[string]interface{}{
					"jsonrpc": "2.0", "id": 1, "method": "initialize",
					"params": map[string]interface{}{
						"rootUri": pathToURI(t.TempDir()),
						"initializationOptions": map[string]interface{}{
							"protocolVersion": ProtocolVersion, "runtimeDiagnostics": test.mode,
						},
					},
				}),
				framedRPC(t, map[string]interface{}{"jsonrpc": "2.0", "method": "initialized"}),
			}, nil)
			var output bytes.Buffer
			server := NewServer(bytes.NewReader(input), &output, ServerOptions{
				ResolveRuntimeConnection: func(context.Context, RuntimeConnectionRequest) (RuntimeDiagnosticsConnection, error) {
					return RuntimeDiagnosticsConnection{}, fmt.Errorf("%w: selected profile mode is live", ErrRuntimeDiagnosticsUnavailable)
				},
			})
			if err := server.Serve(context.Background()); err != nil {
				t.Fatal(err)
			}
			gotStatus := false
			for _, message := range readFramedRPC(t, output.Bytes()) {
				if message.Method == MethodRuntimeStatus {
					gotStatus = true
				}
			}
			if gotStatus != test.wantStatus {
				t.Fatalf("runtime status notification = %t, want %t", gotStatus, test.wantStatus)
			}
		})
	}
}

func TestProtocolRuntimeDiagnosticKeepsVisibleAndStructuredRequestContext(t *testing.T) {
	input := runtimeEditorDiagnostic(RuntimeDiagnosticsRecord{
		RequestID: "request-42",
		Route:     "/about",
		Method:    "GET",
	}, RuntimeDiagnosticsIssue{
		Err:  "template execution failed",
		File: "page.hyperbricks.yaml",
	}, "", "")
	diagnostic := protocolRuntimeDiagnostic(input)
	if !strings.Contains(diagnostic.Message, "GET /about") || !strings.Contains(diagnostic.Message, "request request-42") {
		t.Fatalf("visible runtime context = %q", diagnostic.Message)
	}
	data, ok := diagnostic.Data.(map[string]interface{})
	if !ok {
		t.Fatalf("diagnostic data = %#v", diagnostic.Data)
	}
	contexts, ok := data["contexts"].([]RuntimeRequestContext)
	if !ok || len(contexts) != 1 || contexts[0].RequestID != "request-42" || contexts[0].Route != "/about" {
		t.Fatalf("structured runtime context = %#v", data["contexts"])
	}
}

func responseResultByID(t *testing.T, messages []rpcMessage, id string) map[string]interface{} {
	t.Helper()
	for _, message := range messages {
		if string(message.ID) != id {
			continue
		}
		var result map[string]interface{}
		if err := json.Unmarshal(message.Result, &result); err != nil {
			t.Fatal(err)
		}
		return result
	}
	t.Fatalf("response id %s not found in %#v", id, messages)
	return nil
}
