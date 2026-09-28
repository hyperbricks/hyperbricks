package language

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// ErrRuntimeDiagnosticsUnavailable marks a selected package profile that
// intentionally has no runtime diagnostics surface, such as live mode. Auto
// mode treats this as a quiet absence; explicit/on connections report it.
var ErrRuntimeDiagnosticsUnavailable = errors.New("runtime diagnostics unavailable")

// RuntimeConnectionRequest contains only the workspace and editor settings
// needed to resolve a runtime connection. Configuration loading remains owned
// by the HyperBricks command layer rather than the protocol transport.
type RuntimeConnectionRequest struct {
	WorkspaceRoot string
	Module        string
	Config        string
	RuntimeURL    string
}

// RuntimeDiagnosticsConnection is the resolved, secret-safe input to the
// runtime bridge. RuntimeURL must never contain user information; credentials
// are held separately and are never included in protocol status messages.
type RuntimeDiagnosticsConnection struct {
	ModuleRoot string
	ConfigPath string
	RuntimeURL string
	ErrorsURL  string
	Auth       RuntimeDiagnosticsAuth
	HTTPClient *http.Client
	Poll       RuntimePollOptions
}

// RuntimeConnectionResolver resolves the selected module, package profile,
// runtime endpoint, and any locally permitted credentials.
type RuntimeConnectionResolver func(context.Context, RuntimeConnectionRequest) (RuntimeDiagnosticsConnection, error)

// RuntimeDiagnosticsRunner is the lifecycle contract used by the LSP server.
// Disconnect must be safe to call after Run returns and more than once.
type RuntimeDiagnosticsRunner interface {
	Run(context.Context) error
	Disconnect(context.Context) error
}

// RuntimeDiagnosticsBridgeFactory is a testable seam between LSP lifecycle
// handling and the concrete HTTP polling bridge.
type RuntimeDiagnosticsBridgeFactory func(RuntimeDiagnosticsConnection, RuntimeDiagnosticsSink) (RuntimeDiagnosticsRunner, error)

func defaultRuntimeDiagnosticsBridgeFactory(connection RuntimeDiagnosticsConnection, sink RuntimeDiagnosticsSink) (RuntimeDiagnosticsRunner, error) {
	canonicalURL, err := CanonicalRuntimeDiagnosticsURL(connection.RuntimeURL)
	if err != nil {
		return nil, err
	}
	connection.RuntimeURL = canonicalURL
	client, err := NewHTTPRuntimeDiagnosticsClient(connection.RuntimeURL, connection.Auth, connection.HTTPClient)
	if err != nil {
		return nil, err
	}
	return NewRuntimeDiagnosticsBridge(client, sink, connection.ModuleRoot, connection.ConfigPath, connection.Poll)
}

type runtimeInitialization struct {
	workspaceRoot string
	module        string
	config        string
	mode          string
	runtimeURL    string
}

func normalizeRuntimeDiagnosticsMode(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "off":
		return "off"
	case "on":
		return "on"
	default:
		return "auto"
	}
}

func (server *Server) runtimeDiagnosticsMode() string {
	server.runtimeMu.Lock()
	defer server.runtimeMu.Unlock()
	return server.initialization.mode
}

func (server *Server) runtimeConnectionRequest() RuntimeConnectionRequest {
	server.runtimeMu.Lock()
	defer server.runtimeMu.Unlock()
	return RuntimeConnectionRequest{
		WorkspaceRoot: server.initialization.workspaceRoot,
		Module:        server.initialization.module,
		Config:        server.initialization.config,
		RuntimeURL:    server.initialization.runtimeURL,
	}
}

func (server *Server) connectRuntime(ctx context.Context, quietUnavailable bool) RuntimeDiagnosticsStatus {
	if server.opts.ResolveRuntimeConnection == nil {
		status := RuntimeDiagnosticsStatus{LastError: "runtime diagnostics lifecycle is not configured"}
		_ = server.PublishRuntimeStatus(ctx, status)
		return server.currentRuntimeStatus()
	}

	if err := server.disconnectRuntime(ctx); err != nil {
		status := RuntimeDiagnosticsStatus{LastError: fmt.Sprintf("disconnect previous runtime diagnostics: %v", err)}
		_ = server.PublishRuntimeStatus(ctx, status)
		return server.currentRuntimeStatus()
	}

	request := server.runtimeConnectionRequest()
	connection, err := server.opts.ResolveRuntimeConnection(ctx, request)
	if err != nil {
		if quietUnavailable && errors.Is(err, ErrRuntimeDiagnosticsUnavailable) {
			return server.currentRuntimeStatus()
		}
		status := RuntimeDiagnosticsStatus{LastError: err.Error()}
		_ = server.PublishRuntimeStatus(ctx, status)
		return server.currentRuntimeStatus()
	}
	factory := server.opts.NewRuntimeDiagnosticsBridge
	if factory == nil {
		factory = defaultRuntimeDiagnosticsBridgeFactory
	}
	bridge, err := factory(connection, server)
	if err != nil {
		status := RuntimeDiagnosticsStatus{LastError: err.Error(), RuntimeURL: connection.RuntimeURL, ErrorsURL: connection.ErrorsURL}
		_ = server.PublishRuntimeStatus(ctx, status)
		return server.currentRuntimeStatus()
	}

	runContext, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	server.runtimeMu.Lock()
	server.runtimeConnection = connection
	server.runtimeBridge = bridge
	server.runtimeCancel = cancel
	server.runtimeDone = done
	server.runtimeStatus = RuntimeDiagnosticsStatus{
		RuntimeURL: connection.RuntimeURL,
		ErrorsURL:  connection.ErrorsURL,
	}
	server.runtimeMu.Unlock()

	go func() {
		defer close(done)
		if runErr := bridge.Run(runContext); runErr != nil && runContext.Err() == nil {
			_ = server.PublishRuntimeStatus(context.Background(), RuntimeDiagnosticsStatus{
				LastError:  runErr.Error(),
				RuntimeURL: connection.RuntimeURL,
				ErrorsURL:  connection.ErrorsURL,
			})
		}
	}()
	return server.currentRuntimeStatus()
}

func (server *Server) disconnectRuntime(ctx context.Context) error {
	server.runtimeMu.Lock()
	cancel := server.runtimeCancel
	done := server.runtimeDone
	bridge := server.runtimeBridge
	server.runtimeMu.Unlock()

	if bridge == nil {
		server.runtimeMu.Lock()
		hadStatus := runtimeStatusHasPresentation(server.runtimeStatus)
		server.runtimeStatus = RuntimeDiagnosticsStatus{}
		server.runtimeConnection = RuntimeDiagnosticsConnection{}
		server.runtimeMu.Unlock()
		if hadStatus {
			return server.PublishRuntimeStatus(ctx, RuntimeDiagnosticsStatus{})
		}
		return nil
	}
	if cancel != nil {
		cancel()
	}
	if done != nil {
		select {
		case <-done:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	// The concrete bridge clears itself when Run stops. This explicit call is
	// intentional: the factory contract permits other runners, and clearing
	// runtime-owned Problems entries must be guaranteed by the LSP lifecycle.
	if err := bridge.Disconnect(ctx); err != nil {
		return err
	}
	server.runtimeMu.Lock()
	if server.runtimeBridge == bridge {
		server.runtimeBridge = nil
		server.runtimeCancel = nil
		server.runtimeDone = nil
	}
	server.runtimeMu.Unlock()
	return nil
}

func runtimeStatusHasPresentation(status RuntimeDiagnosticsStatus) bool {
	return status.Connected || status.Generation != 0 || status.ErrorCount != 0 ||
		status.CheckedRoutes != 0 || status.TotalRoutes != 0 || len(status.UncheckedRoutes) != 0 ||
		status.EvictedContexts != 0 || len(status.UnmappedIssues) != 0 || status.LastError != "" || status.NextRetry != 0 ||
		status.RuntimeURL != "" || status.ErrorsURL != ""
}

func (server *Server) currentRuntimeStatus() RuntimeDiagnosticsStatus {
	server.runtimeMu.Lock()
	defer server.runtimeMu.Unlock()
	status := sanitizeRuntimeDiagnosticsStatus(server.runtimeStatus)
	return server.enrichRuntimeStatusLocked(status)
}

func (server *Server) enrichRuntimeStatusLocked(status RuntimeDiagnosticsStatus) RuntimeDiagnosticsStatus {
	status.RuntimeURL = runtimeStatusBaseURL(status.RuntimeURL)
	if status.RuntimeURL == "" {
		status.RuntimeURL = runtimeStatusBaseURL(server.runtimeConnection.RuntimeURL)
	}
	if status.RuntimeURL == "" {
		status.RuntimeURL = runtimeStatusBaseURL(server.initialization.runtimeURL)
	}
	errorsEnabled := strings.TrimSpace(status.ErrorsURL) != "" || strings.TrimSpace(server.runtimeConnection.ErrorsURL) != ""
	status.ErrorsURL = ""
	if errorsEnabled {
		status.ErrorsURL = RuntimeDiagnosticsErrorsURL(status.RuntimeURL)
	}
	return status
}

func runtimeStatusResult(status RuntimeDiagnosticsStatus) map[string]interface{} {
	status = sanitizeRuntimeDiagnosticsStatus(status)
	return map[string]interface{}{
		"connected":       status.Connected,
		"generation":      status.Generation,
		"errorCount":      status.ErrorCount,
		"checkedRoutes":   status.CheckedRoutes,
		"totalRoutes":     status.TotalRoutes,
		"uncheckedRoutes": status.UncheckedRoutes,
		"evictedContexts": status.EvictedContexts,
		"unmappedIssues":  status.UnmappedIssues,
		"lastError":       status.LastError,
		"nextRetryMs":     status.NextRetry.Milliseconds(),
		"runtimeUrl":      status.RuntimeURL,
		"errorsUrl":       status.ErrorsURL,
	}
}

// RuntimeDiagnosticsErrorsURL returns the optional developer error-page URL
// for a runtime base. Callers must still enforce whether that surface is
// enabled in the selected package configuration.
func RuntimeDiagnosticsErrorsURL(rawURL string) string {
	baseURL := runtimeStatusBaseURL(rawURL)
	parsed, err := url.Parse(baseURL)
	if err != nil || baseURL == "" {
		return ""
	}
	parsed.Path = "/__hyperbricks/errors"
	parsed.RawPath = ""
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return parsed.String()
}

// runtimeStatusBaseURL deliberately emits only an HTTP(S) origin. The runtime
// endpoints are absolute paths, and removing user information, path, query,
// and fragment prevents settings from smuggling credentials into LSP output.
func runtimeStatusBaseURL(rawURL string) string {
	canonical, err := CanonicalRuntimeDiagnosticsURL(rawURL)
	if err != nil {
		return ""
	}
	return canonical
}

func runtimeShutdownContext() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 2*time.Second)
}
