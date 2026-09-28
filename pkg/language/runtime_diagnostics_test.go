package language

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type recordingRuntimeDiagnosticsSink struct {
	mu       sync.Mutex
	updates  []RuntimeDiagnosticsUpdate
	statuses []RuntimeDiagnosticsStatus
}

type failingRuntimeDiagnosticsClient struct {
	err error
}

func (client failingRuntimeDiagnosticsClient) FetchCurrent(context.Context) (RuntimeDiagnosticsSnapshot, error) {
	return RuntimeDiagnosticsSnapshot{}, client.err
}

func (sink *recordingRuntimeDiagnosticsSink) ReplaceRuntimeDiagnostics(_ context.Context, update RuntimeDiagnosticsUpdate) error {
	sink.mu.Lock()
	defer sink.mu.Unlock()
	sink.updates = append(sink.updates, update)
	return nil
}

func (sink *recordingRuntimeDiagnosticsSink) PublishRuntimeStatus(_ context.Context, status RuntimeDiagnosticsStatus) error {
	sink.mu.Lock()
	defer sink.mu.Unlock()
	sink.statuses = append(sink.statuses, status)
	return nil
}

func (sink *recordingRuntimeDiagnosticsSink) recorded() ([]RuntimeDiagnosticsUpdate, []RuntimeDiagnosticsStatus) {
	sink.mu.Lock()
	defer sink.mu.Unlock()
	return append([]RuntimeDiagnosticsUpdate(nil), sink.updates...), append([]RuntimeDiagnosticsStatus(nil), sink.statuses...)
}

func runtimeTestSnapshot(generation uint64, records ...RuntimeDiagnosticsRecord) RuntimeDiagnosticsSnapshot {
	return RuntimeDiagnosticsSnapshot{
		Records:         records,
		Generation:      generation,
		CheckedRoutes:   1,
		TotalRoutes:     3,
		UncheckedRoutes: []string{"/contact", "/pricing"},
	}
}

func runtimeTestRecord(generation uint64, file string) RuntimeDiagnosticsRecord {
	return RuntimeDiagnosticsRecord{
		RequestID:  "hb-143",
		ContextID:  "context-one",
		Route:      "/about",
		Method:     http.MethodGet,
		Generation: generation,
		Errors: []RuntimeDiagnosticsIssue{{
			Hash:           "runtime-hash",
			Type:           "<TEMPLATE>",
			File:           file,
			Path:           "page.content.hero",
			Key:            "hero",
			Err:            "template execution failed: can't evaluate field Title",
			Line:           24,
			Column:         7,
			Resource:       "templates/hero.html",
			ResourceLine:   8,
			ResourceColumn: 12,
			Phase:          "render",
		}},
	}
}

func newRuntimeHTTPTestClient(t *testing.T, server *httptest.Server) *HTTPRuntimeDiagnosticsClient {
	t.Helper()
	client, err := NewHTTPRuntimeDiagnosticsClient(server.URL, RuntimeDiagnosticsAuth{
		Username: "developer",
		Password: "secret",
		Mode:     RuntimeCredentialsAutomatic,
	}, server.Client())
	if err != nil {
		t.Fatal(err)
	}
	return client
}

// AC-12, AC-13, and AC-16: runtime failures retain request context, resources
// become related editor locations, and route coverage stays status-only.
func TestRuntimeDiagnosticsBridgeMapsFailureContextResourceAndCoverage(t *testing.T) {
	snapshot := runtimeTestSnapshot(7, runtimeTestRecord(7, "hyperbricks/page.hyperbricks.yaml"))
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.URL.Path != runtimeDiagnosticsPath || request.URL.Query().Get("view") != "current" {
			t.Errorf("request URL = %s", request.URL.String())
		}
		username, password, ok := request.BasicAuth()
		if !ok || username != "developer" || password != "secret" {
			t.Errorf("basic auth = %q, %q, %t", username, password, ok)
		}
		response.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(response).Encode(snapshot); err != nil {
			t.Error(err)
		}
	}))
	defer server.Close()

	root := t.TempDir()
	sink := &recordingRuntimeDiagnosticsSink{}
	bridge, err := NewRuntimeDiagnosticsBridge(newRuntimeHTTPTestClient(t, server), sink, root, "", RuntimePollOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if err := bridge.Refresh(context.Background()); err != nil {
		t.Fatal(err)
	}

	updates, statuses := sink.recorded()
	if len(updates) != 1 || len(statuses) != 1 {
		t.Fatalf("updates=%d statuses=%d", len(updates), len(statuses))
	}
	sourceFile := filepath.Join(root, "hyperbricks", "page.hyperbricks.yaml")
	diagnostics := updates[0].Diagnostics[sourceFile]
	if len(diagnostics) != 1 {
		t.Fatalf("diagnostics = %#v", updates[0].Diagnostics)
	}
	diagnostic := diagnostics[0]
	if diagnostic.Source != RuntimeDiagnosticsSource || !strings.Contains(diagnostic.Message, snapshot.Records[0].Errors[0].Err) || !strings.Contains(diagnostic.Message, "GET /about") || !strings.Contains(diagnostic.Message, "request hb-143") || !strings.Contains(diagnostic.Message, "component page.content.hero") || !strings.Contains(diagnostic.Message, "phase render") || diagnostic.Severity != RuntimeSeverityError {
		t.Fatalf("diagnostic = %#v", diagnostic)
	}
	if diagnostic.Range.Start != (RuntimeEditorPosition{Line: 23, Character: 6}) {
		t.Fatalf("source position = %#v", diagnostic.Range.Start)
	}
	if len(diagnostic.Contexts) != 1 || diagnostic.Contexts[0].Route != "/about" || diagnostic.Contexts[0].RequestID != "hb-143" || diagnostic.Contexts[0].Phase != "render" {
		t.Fatalf("request context = %#v", diagnostic.Contexts)
	}
	encodedContext, err := json.Marshal(diagnostic.Contexts[0])
	if err != nil || !strings.Contains(string(encodedContext), `"requestId":"hb-143"`) || strings.Contains(string(encodedContext), "RequestID") {
		t.Fatalf("protocol context = %s, %v", encodedContext, err)
	}
	if len(diagnostic.Related) != 1 || diagnostic.Related[0].File != filepath.Join(root, "templates", "hero.html") || diagnostic.Related[0].Range.Start != (RuntimeEditorPosition{Line: 7, Character: 11}) {
		t.Fatalf("related locations = %#v", diagnostic.Related)
	}
	status := statuses[0]
	if !status.Connected || status.ErrorCount != 1 || status.CheckedRoutes != 1 || status.TotalRoutes != 3 || !reflect.DeepEqual(status.UncheckedRoutes, []string{"/contact", "/pricing"}) {
		t.Fatalf("status = %#v", status)
	}
	if len(updates[0].Diagnostics) != 1 || len(updates[0].Unmapped) != 0 {
		t.Fatal("unchecked routes were incorrectly converted to diagnostics")
	}
}

func TestRuntimeDiagnosticsMapsOnlySafeSourcePaths(t *testing.T) {
	root := t.TempDir()
	configPath := filepath.Join(root, "profiles", "editor.hyperbricks.yaml")
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(configPath, []byte("hyperbricks:\n  mode: development\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(filepath.Dir(root), "outside.hyperbricks.yaml")

	for _, test := range []struct {
		name       string
		file       string
		configPath string
		wantFile   string
	}{
		{name: "blank path", file: "", configPath: configPath},
		{name: "out of module path", file: outside, configPath: configPath},
		{name: "selected config marker", file: "__config", configPath: configPath, wantFile: configPath},
		{name: "missing config marker", file: "__config", configPath: filepath.Join(root, "missing.hyperbricks.yaml")},
	} {
		t.Run(test.name, func(t *testing.T) {
			state := NewRuntimeDiagnosticsState(root, test.configPath)
			update := state.Replace(runtimeTestSnapshot(41, RuntimeDiagnosticsRecord{
				Generation: 41,
				Errors: []RuntimeDiagnosticsIssue{{
					Hash: "mapping-test", File: test.file, Err: "runtime mapping failure",
				}},
			}))
			if test.wantFile == "" {
				if len(update.Diagnostics) != 0 || len(update.Unmapped) != 1 {
					t.Fatalf("update = %#v", update)
				}
			} else {
				if len(update.Diagnostics[test.wantFile]) != 1 || len(update.Unmapped) != 0 {
					t.Fatalf("update = %#v", update)
				}
			}
			if _, fabricated := update.Diagnostics[filepath.Join(root, "__config")]; fabricated {
				t.Fatalf("__config was fabricated as a source file: %#v", update.Diagnostics)
			}
		})
	}
}

func TestRuntimeDiagnosticsRejectsSourceAndResourceSymlinkEscapes(t *testing.T) {
	root := t.TempDir()
	externalRoot := t.TempDir()
	if err := os.WriteFile(filepath.Join(externalRoot, "page.hyperbricks.yaml"), []byte("page: []\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(externalRoot, "template.html"), []byte("secret template\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(externalRoot, filepath.Join(root, "linked")); err != nil {
		t.Fatal(err)
	}

	state := NewRuntimeDiagnosticsState(root, "")
	update := state.Replace(runtimeTestSnapshot(43, RuntimeDiagnosticsRecord{
		Generation: 43,
		Errors: []RuntimeDiagnosticsIssue{{
			Hash: "symlink-escape", File: "linked/page.hyperbricks.yaml",
			Resource: "linked/template.html", Err: "template render failed",
		}},
	}))
	if len(update.Diagnostics) != 0 || len(update.Unmapped) != 1 {
		t.Fatalf("symlink escape update = %#v", update)
	}
	if len(update.Unmapped[0].Related) != 0 {
		t.Fatalf("external resource became a related source anchor: %#v", update.Unmapped[0].Related)
	}
}

func TestRuntimeDiagnosticsStatusSanitizesUnmappedIssueDetails(t *testing.T) {
	root := t.TempDir()
	outside := filepath.Join(filepath.Dir(root), "private", "outside.hyperbricks.yaml")
	state := NewRuntimeDiagnosticsState(root, "")
	update := state.Replace(runtimeTestSnapshot(42,
		RuntimeDiagnosticsRecord{
			Generation: 42,
			Errors: []RuntimeDiagnosticsIssue{{
				Hash:  "unmapped-one",
				Type:  "<TEMPLATE>",
				Path:  "page.content.hero",
				Phase: "render",
				Err: "template lookup failed\nAuthorization: Bearer auth-secret; password=\"password-secret\"; " +
					"fetch https://developer:url-secret@example.test/debug?token=query-secret#fragment; " +
					"open /Users/alice/private/page.hyperbricks.yaml",
			}},
		},
		RuntimeDiagnosticsRecord{
			Generation: 42,
			Errors: []RuntimeDiagnosticsIssue{{
				Hash: "unmapped-two", File: outside,
				Err: "resource load failed for ../../private/resource.html and \\\\server\\share\\private\\secret.yaml; secret=second-secret",
			}},
		},
	))
	status := runtimeStatusFromSnapshot(RuntimeDiagnosticsSnapshot{
		Generation: 42, EvictedContexts: 3,
	}, update)
	if status.ErrorCount != 2 || status.EvictedContexts != 3 || len(status.UnmappedIssues) != 2 {
		t.Fatalf("status = %#v", status)
	}
	visible := strings.Join(status.UnmappedIssues, " | ")
	for _, wanted := range []string{
		"template lookup failed",
		"code unmapped-one",
		"type <TEMPLATE>",
		"component page.content.hero",
		"phase render",
		"https://example.test/debug",
	} {
		if !strings.Contains(visible, wanted) {
			t.Fatalf("sanitized status %q does not contain %q", visible, wanted)
		}
	}
	for _, forbidden := range []string{
		"auth-secret",
		"password-secret",
		"url-secret",
		"query-secret",
		"second-secret",
		"/Users/alice",
		outside,
		"../../private",
		`\\server\share\private\secret.yaml`,
		"\n",
	} {
		if strings.Contains(visible, forbidden) {
			t.Fatalf("sanitized status exposed %q: %q", forbidden, visible)
		}
	}
}

func TestSanitizeRuntimeStatusTextRedactsUNCPath(t *testing.T) {
	const input = `open \\server\share\private\secret.yaml`
	if got := sanitizeRuntimeStatusText(input, 320); got != "open [path]" {
		t.Fatalf("sanitized UNC path = %q", got)
	}
}

func TestSanitizeRuntimeDiagnosticsStatusBoundsUncheckedRoutes(t *testing.T) {
	routes := make([]string, 0, 300)
	for index := 299; index >= 0; index-- {
		routes = append(routes, fmt.Sprintf("/route/%03d?token=route-secret-%03d", index, index))
	}
	status := sanitizeRuntimeDiagnosticsStatus(RuntimeDiagnosticsStatus{
		CheckedRoutes: 7, TotalRoutes: 307, UncheckedRoutes: routes,
	})
	if status.CheckedRoutes != 7 || status.TotalRoutes != 307 || len(status.UncheckedRoutes) != maxRuntimeStatusUncheckedRoutes {
		t.Fatalf("bounded route status = %#v", status)
	}
	if status.UncheckedRoutes[0] != "/route/000" || status.UncheckedRoutes[len(status.UncheckedRoutes)-1] != "/route/255" {
		t.Fatalf("deterministic bounded routes = %#v", status.UncheckedRoutes)
	}
	if encoded, err := json.Marshal(status); err != nil || strings.Contains(string(encoded), "route-secret") {
		t.Fatalf("bounded routes exposed query data: %s, %v", encoded, err)
	}
}

func TestRuntimeDiagnosticMessageRedactsSecretsAndKeepsActionableContext(t *testing.T) {
	root := t.TempDir()
	diagnostic := runtimeEditorDiagnostic(RuntimeDiagnosticsRecord{
		RequestID:  "request-42",
		ContextID:  "context-42",
		Route:      "/checkout?token=route-secret#payment",
		Method:     http.MethodPost,
		Generation: 7,
	}, RuntimeDiagnosticsIssue{
		Hash: "safe-code", Type: "<TEMPLATE>", File: "page.hyperbricks.yaml",
		Path: "page.checkout", Phase: "render",
		Err: "template execution failed\nAuthorization: Bearer auth-secret; password=password-secret; " +
			"fetch https://developer:url-secret@example.test/template?token=query-secret#fragment",
	}, root, "")

	for _, wanted := range []string{
		"template execution failed",
		"https://example.test/template",
		"POST /checkout",
		"request request-42",
		"component page.checkout",
		"phase render",
	} {
		if !strings.Contains(diagnostic.Message, wanted) {
			t.Fatalf("diagnostic message %q does not contain %q", diagnostic.Message, wanted)
		}
	}
	encoded, err := json.Marshal(diagnostic)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{
		"auth-secret",
		"password-secret",
		"route-secret",
		"url-secret",
		"query-secret",
		"fragment",
		"\n",
	} {
		if strings.Contains(string(encoded), forbidden) {
			t.Fatalf("diagnostic exposed %q: %s", forbidden, encoded)
		}
	}
}

func TestRuntimeDiagnosticSourceRangeUsesUTF16Characters(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "page.hyperbricks.yaml")
	if err := os.WriteFile(file, []byte("title: 😀broken\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	diagnostic := runtimeEditorDiagnostic(RuntimeDiagnosticsRecord{
		Generation: 8,
	}, RuntimeDiagnosticsIssue{
		Hash: "utf16-column", File: "page.hyperbricks.yaml", Err: "render failed",
		Line: 1, Column: 9,
	}, root, "")
	if diagnostic.Range.Start != (RuntimeEditorPosition{Line: 0, Character: 9}) ||
		diagnostic.Range.End != (RuntimeEditorPosition{Line: 0, Character: 10}) {
		t.Fatalf("UTF-16 runtime source range = %#v", diagnostic.Range)
	}
}

func TestRuntimeDiagnosticRelatedResourceRangesConvertProducerByteColumnsToUTF16(t *testing.T) {
	root := t.TempDir()
	tests := []struct {
		name      string
		resource  string
		content   string
		column    int
		wantStart int
		wantEnd   int
	}{
		{
			name:      "Goja astral character",
			resource:  "script.js",
			content:   "😀broken\n",
			column:    5,
			wantStart: 2,
			wantEnd:   3,
		},
		{
			name:      "Go template multibyte character",
			resource:  "view.html",
			content:   "é{{broken\n",
			column:    3,
			wantStart: 1,
			wantEnd:   2,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := os.WriteFile(filepath.Join(root, test.resource), []byte(test.content), 0o644); err != nil {
				t.Fatal(err)
			}
			diagnostic := runtimeEditorDiagnostic(RuntimeDiagnosticsRecord{Generation: 9}, RuntimeDiagnosticsIssue{
				Hash: "resource-column", File: "page.hyperbricks.yaml", Err: "render failed",
				Resource: test.resource, ResourceLine: 1, ResourceColumn: test.column,
			}, root, "")
			if len(diagnostic.Related) != 1 {
				t.Fatalf("related runtime resource = %#v", diagnostic.Related)
			}
			got := diagnostic.Related[0].Range
			if got.Start != (RuntimeEditorPosition{Line: 0, Character: test.wantStart}) ||
				got.End != (RuntimeEditorPosition{Line: 0, Character: test.wantEnd}) {
				t.Fatalf("UTF-16 related resource range = %#v, want characters %d:%d", got, test.wantStart, test.wantEnd)
			}
		})
	}
}

func TestRuntimeEditorResourceRangeFallsBackForUnreadableOrInvalidCoordinates(t *testing.T) {
	root := t.TempDir()
	resource := filepath.Join(root, "script.js")
	if err := os.WriteFile(resource, []byte("😀broken\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	for _, test := range []struct {
		name   string
		file   string
		column int
	}{
		{name: "unreadable resource", file: filepath.Join(root, "missing.js"), column: 5},
		{name: "column inside UTF-8 rune", file: resource, column: 2},
		{name: "column beyond line", file: resource, column: 50},
	} {
		t.Run(test.name, func(t *testing.T) {
			got := runtimeEditorResourceRange(test.file, 1, test.column)
			want := runtimeEditorRange(1, test.column)
			if got != want {
				t.Fatalf("resource range = %#v, want producer fallback %#v", got, want)
			}
		})
	}
}

// AC-14: the endpoint is a complete snapshot, so a successful retry removes
// the old file from Problems rather than leaving an incremental stale error.
func TestRuntimeDiagnosticsBridgeSuccessfulRetryClearsFailure(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		snapshot := runtimeTestSnapshot(11)
		if requests.Add(1) == 1 {
			snapshot.Records = []RuntimeDiagnosticsRecord{runtimeTestRecord(11, "page.hyperbricks.yaml")}
		}
		_ = json.NewEncoder(response).Encode(snapshot)
	}))
	defer server.Close()

	root := t.TempDir()
	sink := &recordingRuntimeDiagnosticsSink{}
	bridge, err := NewRuntimeDiagnosticsBridge(newRuntimeHTTPTestClient(t, server), sink, root, "", RuntimePollOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if err := bridge.Refresh(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := bridge.Refresh(context.Background()); err != nil {
		t.Fatal(err)
	}

	updates, _ := sink.recorded()
	if len(updates) != 2 {
		t.Fatalf("updates = %d", len(updates))
	}
	file := filepath.Join(root, "page.hyperbricks.yaml")
	if len(updates[0].Diagnostics[file]) != 1 {
		t.Fatalf("initial update = %#v", updates[0])
	}
	if len(updates[1].Diagnostics) != 0 || !reflect.DeepEqual(updates[1].ClearedFiles, []string{file}) {
		t.Fatalf("recovery update = %#v", updates[1])
	}
}

// AC-15: a new runtime generation clears the previous source set and ignores
// any record whose generation does not match the enclosing snapshot.
func TestRuntimeDiagnosticsBridgeGenerationChangeClearsStaleRecords(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		if requests.Add(1) == 1 {
			_ = json.NewEncoder(response).Encode(runtimeTestSnapshot(20, runtimeTestRecord(20, "old.hyperbricks.yaml")))
			return
		}
		_ = json.NewEncoder(response).Encode(runtimeTestSnapshot(21, runtimeTestRecord(20, "stale.hyperbricks.yaml")))
	}))
	defer server.Close()

	root := t.TempDir()
	sink := &recordingRuntimeDiagnosticsSink{}
	bridge, err := NewRuntimeDiagnosticsBridge(newRuntimeHTTPTestClient(t, server), sink, root, "", RuntimePollOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if err := bridge.Refresh(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := bridge.Refresh(context.Background()); err != nil {
		t.Fatal(err)
	}

	updates, _ := sink.recorded()
	oldFile := filepath.Join(root, "old.hyperbricks.yaml")
	if !updates[1].GenerationChanged || updates[1].Generation != 21 || len(updates[1].Diagnostics) != 0 || !reflect.DeepEqual(updates[1].ClearedFiles, []string{oldFile}) {
		t.Fatalf("generation replacement = %#v", updates[1])
	}
}

func TestRuntimeDiagnosticsBridgeBacksOffAndDisconnectsCleanly(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		call := requests.Add(1)
		if call < 3 {
			http.Error(response, "temporarily unavailable", http.StatusServiceUnavailable)
			return
		}
		_ = json.NewEncoder(response).Encode(runtimeTestSnapshot(2, runtimeTestRecord(2, "page.hyperbricks.yaml")))
	}))
	defer server.Close()

	sink := &recordingRuntimeDiagnosticsSink{}
	bridge, err := NewRuntimeDiagnosticsBridge(newRuntimeHTTPTestClient(t, server), sink, t.TempDir(), "", RuntimePollOptions{
		PollInterval:   5 * time.Millisecond,
		InitialBackoff: 10 * time.Millisecond,
		MaxBackoff:     40 * time.Millisecond,
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	var delays []time.Duration
	bridge.wait = func(_ context.Context, delay time.Duration) error {
		delays = append(delays, delay)
		if len(delays) == 3 {
			cancel()
			return context.Canceled
		}
		return nil
	}
	if err := bridge.Run(ctx); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(delays, []time.Duration{10 * time.Millisecond, 20 * time.Millisecond, 5 * time.Millisecond}) {
		t.Fatalf("poll delays = %v", delays)
	}
	updates, statuses := sink.recorded()
	if len(updates) != 2 || len(updates[0].Diagnostics) != 1 || len(updates[1].ClearedFiles) != 1 {
		t.Fatalf("disconnect updates = %#v", updates)
	}
	if len(statuses) != 4 || statuses[0].Connected || statuses[0].NextRetry != 10*time.Millisecond || statuses[1].NextRetry != 20*time.Millisecond || !statuses[2].Connected || statuses[3].Connected {
		t.Fatalf("statuses = %#v", statuses)
	}
}

func TestRuntimeDiagnosticsBridgeSanitizesFetchFailureStatus(t *testing.T) {
	sink := &recordingRuntimeDiagnosticsSink{}
	bridge, err := NewRuntimeDiagnosticsBridge(failingRuntimeDiagnosticsClient{err: errors.New(
		"remote fetch failed\nAuthorization: Bearer auth-secret; token=reason-secret; " +
			"https://developer:url-secret@example.test/status?token=query-secret#fragment",
	)}, sink, t.TempDir(), "", RuntimePollOptions{InitialBackoff: time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	bridge.wait = func(context.Context, time.Duration) error {
		cancel()
		return context.Canceled
	}
	if err := bridge.Run(ctx); err != nil {
		t.Fatal(err)
	}
	_, statuses := sink.recorded()
	if len(statuses) < 1 || !strings.Contains(statuses[0].LastError, "remote fetch failed") ||
		!strings.Contains(statuses[0].LastError, "https://example.test/status") {
		t.Fatalf("sanitized bridge status = %#v", statuses)
	}
	for _, forbidden := range []string{"auth-secret", "reason-secret", "url-secret", "query-secret", "fragment", "\n"} {
		if strings.Contains(statuses[0].LastError, forbidden) {
			t.Fatalf("bridge status exposed %q: %#v", forbidden, statuses[0])
		}
	}
}
