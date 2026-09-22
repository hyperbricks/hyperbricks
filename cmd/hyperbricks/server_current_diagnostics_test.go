package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/hyperbricks/hyperbricks/pkg/component"
	"github.com/hyperbricks/hyperbricks/pkg/composite"
	"github.com/hyperbricks/hyperbricks/pkg/shared"
)

type diagnosticTestRenderer func(context.Context) (string, []error)

func (renderer diagnosticTestRenderer) Render(_ interface{}, ctx context.Context) (string, []error) {
	return renderer(ctx)
}
func (diagnosticTestRenderer) Types() []string { return []string{component.TextConfigGetName()} }

func installDiagnosticTestRoute(t *testing.T, renderer diagnosticTestRenderer) {
	t.Helper()
	rm.RegisterComponent(component.TextConfigGetName(), renderer, reflect.TypeOf(component.TextConfig{}))
	updateGlobalRoutes(map[string]map[string]interface{}{"page": {
		"@type": composite.HyperMediaConfigGetName(), "route": "page", "title": "Diagnostics test",
		"hyperbricksfile": "hyperbricks/page.hyperbricks.yaml", "hyperbrickskey": "page",
		"content": map[string]interface{}{"@type": component.TextConfigGetName(), "value": "content"},
	}}, nil)
}

func diagnosticRequest(path string) *httptest.ResponseRecorder {
	response := httptest.NewRecorder()
	ServeContent(response, httptest.NewRequest(http.MethodGet, path, nil))
	return response
}

func TestLiveRenderingSkipsDeveloperDiagnosticsAndPreservesFailures(t *testing.T) {
	setupDevelopmentModeServeContentTest(t, false)
	getHyperBricksConfiguration().Mode = shared.LIVE_MODE
	request := httptest.NewRequest(http.MethodPost, "/page", strings.NewReader("application-owned body"))
	body := request.Body
	if outcome := newDiagnosticOutcome(request, nextRenderRequestID(), "page", routeGeneration, nil); outcome != nil {
		t.Fatal("live request created a developer diagnostics outcome")
	}
	if request.Body != body {
		t.Fatal("live request body was wrapped for developer diagnostics")
	}
	installDiagnosticTestRoute(t, func(ctx context.Context) (string, []error) {
		if ctx.Value(deferredDiagnosticsKey{}) != nil {
			t.Error("live renderer received developer diagnostics request context")
		}
		return "partial", []error{errors.New("live render failed")}
	})
	response := diagnosticRequest("/page")
	if response.Header().Get(renderErrorCountHeader) != "1" || response.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("live failure lost diagnostic count/cache protection: %v", response.Header())
	}
	result := renderContent(httptest.NewRecorder(), "page", httptest.NewRequest(http.MethodGet, "/page", nil), nextRenderRequestID())
	if result.outcome != nil || len(result.Diagnostics) != 1 || !result.NoCache {
		t.Fatalf("unexpected live render result: %#v", result)
	}
	if len(renderDiagnostics) != 0 || len(renderDiagnosticsOrder) != 0 {
		t.Fatal("live failures were retained in the developer store")
	}
}

func TestCurrentDiagnosticsReplaceFailuresAndClearOnlyMatchingVariant(t *testing.T) {
	setupDevelopmentModeServeContentTest(t, false)
	fail := true
	installDiagnosticTestRoute(t, func(context.Context) (string, []error) {
		if fail {
			return "partial", []error{errors.New("render failed")}
		}
		return "recovered", nil
	})
	diagnosticRequest("/page?id=a")
	first := collectRecentRenderDiagnostics(10)[0]
	diagnosticRequest("/page?id=a")
	repeated := collectRecentRenderDiagnostics(10)
	if len(repeated) != 1 || repeated[0].ContextID != first.ContextID || repeated[0].RequestID == first.RequestID {
		t.Fatalf("repeated failures = %#v", repeated)
	}
	diagnosticRequest("/page?id=b")
	if len(collectRecentRenderDiagnostics(10)) != 2 {
		t.Fatal("different variants were merged")
	}
	fail = false
	response := diagnosticRequest("/page?id=a")
	remaining := collectRecentRenderDiagnostics(10)
	if response.Header().Get(renderErrorCountHeader) != "0" || len(remaining) != 1 || remaining[0].ContextID == first.ContextID {
		t.Fatalf("recovery cleared wrong context: %#v", remaining)
	}
	diagnosticRequest("/page?id=b")
	if len(collectRecentRenderDiagnostics(10)) != 0 {
		t.Fatal("recovered errors remain")
	}
}

func TestCurrentDiagnosticsIgnoreLateOutcomes(t *testing.T) {
	for _, firstFails := range []bool{true, false} {
		t.Run(fmt.Sprintf("first-fails-%t", firstFails), func(t *testing.T) {
			setupDevelopmentModeServeContentTest(t, false)
			started, release, finished := make(chan struct{}), make(chan struct{}), make(chan struct{})
			var calls atomic.Int32
			installDiagnosticTestRoute(t, func(context.Context) (string, []error) {
				first := calls.Add(1) == 1
				if first {
					close(started)
					<-release
				}
				if first == firstFails {
					return "partial", []error{errors.New("failed")}
				}
				return "healthy", nil
			})
			go func() { diagnosticRequest("/page"); close(finished) }()
			select {
			case <-started:
			case <-time.After(5 * time.Second):
				t.Fatal("first render did not start")
			}
			diagnosticRequest("/page")
			close(release)
			select {
			case <-finished:
			case <-time.After(5 * time.Second):
				t.Fatal("first render did not finish")
			}
			want := 1
			if firstFails {
				want = 0
			}
			if records := collectRecentRenderDiagnostics(10); len(records) != want {
				t.Fatalf("late outcome replaced newer result: %#v", records)
			}
		})
	}
}

func TestCurrentDiagnosticsReloadInvalidatesPreviousGeneration(t *testing.T) {
	setupDevelopmentModeServeContentTest(t, false)
	installDiagnosticTestRoute(t, func(context.Context) (string, []error) { return "ok", nil })
	old := newDiagnosticOutcome(httptest.NewRequest("GET", "/page", nil), nextRenderRequestID(), "page", routeGeneration, configs["page"])
	old.checked, old.errors = true, []error{errors.New("old failure")}
	commitRenderDiagnostics(nil, old)
	updateGlobalRoutes(configs, routePlans)
	commitRenderDiagnostics(nil, old)
	recordConfigDiagnosticsAtGeneration([]error{errors.New("outdated source failure")}, old.generation)
	snapshot := collectCurrentRenderDiagnostics()
	if len(snapshot.Records) != 0 || snapshot.CheckedRoutes != 0 || !reflect.DeepEqual(snapshot.UncheckedRoutes, []string{"page"}) {
		t.Fatalf("reload state = %#v", snapshot)
	}
	diagnosticRequest("/page")
	if snapshot := collectCurrentRenderDiagnostics(); snapshot.CheckedRoutes != 1 || len(snapshot.UncheckedRoutes) != 0 {
		t.Fatalf("checked state = %#v", snapshot)
	}
	recordConfigDiagnostics([]error{errors.New("broken source")})
	recordConfigDiagnostics(nil)
	if len(collectRecentRenderDiagnostics(10)) != 0 {
		t.Fatal("configuration recovery did not clear")
	}
}

func TestFailedLiveRenderBypassesCacheAndRetries(t *testing.T) {
	setupDevelopmentModeServeContentTest(t, false)
	getHyperBricksConfiguration().Mode = shared.LIVE_MODE
	calls := 0
	installDiagnosticTestRoute(t, func(context.Context) (string, []error) {
		calls++
		if calls == 1 {
			return "upstream failed", []error{errors.New("upstream unavailable")}
		}
		return "upstream recovered", nil
	})
	first := diagnosticRequest("/page")
	if first.Header().Get("Cache-Control") != "no-store" || first.Header().Get("ETag") != "" || first.Header().Get(renderErrorCountHeader) != "1" {
		t.Fatalf("failure headers = %#v", first.Header())
	}
	second := diagnosticRequest("/page")
	if calls != 2 || !strings.Contains(second.Body.String(), "upstream recovered") || second.Header().Get("ETag") == "" {
		t.Fatalf("recovery calls=%d response=%s headers=%v", calls, second.Body, second.Header())
	}
	diagnosticRequest("/page")
	if calls != 2 {
		t.Fatal("healthy output no longer cached")
	}
}

func TestCurrentDiagnosticsHTTPFailureAndWrappedMetadata(t *testing.T) {
	setupDevelopmentModeServeContentTest(t, false)
	installDiagnosticTestRoute(t, func(context.Context) (string, []error) { return "ok", nil })
	configs["page"]["response"] = map[string]interface{}{"status": 42}
	response := diagnosticRequest("/page")
	records := collectRecentRenderDiagnostics(10)
	if response.Code != 500 || response.Header().Get(renderErrorCountHeader) != "1" || len(records) != 1 {
		t.Fatalf("HTTP error missing: response=%d records=%#v", response.Code, records)
	}
	if issue := records[0].Errors[0]; issue.File != "hyperbricks/page.hyperbricks.yaml" || issue.Phase != "serve" {
		t.Fatalf("HTTP context = %#v", issue)
	}
	issue := &shared.ComponentError{File: "hyperbricks/other.hyperbricks.yaml", Path: "other.content", Type: "<GOJA_RENDER>", Err: "script failed", Line: 9, Resource: "resources/calculate.js", Phase: "render"}
	collected := collectRenderDiagnostics([]error{fmt.Errorf("wrapped: %w", issue), fmt.Errorf("wrapped: %w", issue)})
	if len(collected) != 1 || collected[0].File != issue.File || collected[0].Resource != issue.Resource || collected[0].Line != 9 {
		t.Fatalf("wrapped diagnostic = %#v", collected)
	}
}

type failingDiagnosticWriter struct{ *httptest.ResponseRecorder }

func (failingDiagnosticWriter) Write([]byte) (int, error) {
	return 0, errors.New("response write failed")
}
func (failingDiagnosticWriter) WriteString(string) (int, error) {
	return 0, errors.New("response write failed")
}

func TestCurrentDiagnosticsRecordsServeFailureAndRecovery(t *testing.T) {
	setupDevelopmentModeServeContentTest(t, false)
	installDiagnosticTestRoute(t, func(context.Context) (string, []error) { return "ok", nil })
	ServeContent(failingDiagnosticWriter{httptest.NewRecorder()}, httptest.NewRequest("GET", "/page", nil))
	records := collectRecentRenderDiagnostics(10)
	if len(records) != 1 || records[0].Errors[0].Phase != "serve" {
		t.Fatalf("serve failure missing: %#v", records)
	}
	diagnosticRequest("/page")
	if len(collectRecentRenderDiagnostics(10)) != 0 {
		t.Fatal("serve recovery did not clear")
	}
}

func TestCurrentDiagnosticsDoesNotExposeRequestSecrets(t *testing.T) {
	setupDevelopmentModeServeContentTest(t, false)
	installDiagnosticTestRoute(t, func(context.Context) (string, []error) { return "partial", []error{errors.New("failed")} })
	request := httptest.NewRequest("POST", "/page?token=private-query", strings.NewReader("private-body"))
	request.Header.Set("Authorization", "Bearer private-auth")
	request.Header.Set("Cookie", "session=private-cookie")
	ServeContent(httptest.NewRecorder(), request)
	payload, err := json.Marshal(collectCurrentRenderDiagnostics())
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{"private-query", "private-body", "private-auth", "private-cookie"} {
		if strings.Contains(string(payload), secret) {
			t.Fatalf("request secret exposed: %s", secret)
		}
	}
}

func TestCurrentDiagnosticsObservesBodyWithoutReadingAhead(t *testing.T) {
	setupDevelopmentModeServeContentTest(t, false)
	commit := func(body string, fail bool) {
		request := httptest.NewRequest("POST", "/page", strings.NewReader(body))
		outcome := newDiagnosticOutcome(request, nextRenderRequestID(), "page", diagnosticsGeneration, nil)
		if outcome.body.read != 0 {
			t.Fatal("diagnostics read the body before the application")
		}
		data, err := io.ReadAll(request.Body)
		if err != nil || string(data) != body {
			t.Fatalf("application body = %q, %v", data, err)
		}
		outcome.checked = true
		if fail {
			outcome.errors = []error{errors.New("failed")}
		}
		commitRenderDiagnostics(request, outcome)
	}
	commit("first", true)
	commit("second", true)
	if len(collectCurrentRenderDiagnostics().Records) != 2 {
		t.Fatal("different consumed bodies were merged")
	}
	commit("first", false)
	if len(collectCurrentRenderDiagnostics().Records) != 1 {
		t.Fatal("body recovery cleared another context")
	}
	commit("second", false)
	if len(collectCurrentRenderDiagnostics().Records) != 0 {
		t.Fatal("body recovery retained a stale failure")
	}
}

func TestCurrentDiagnosticsReportsEvictionAndRejectsEvictedOutcomes(t *testing.T) {
	setupDevelopmentModeServeContentTest(t, false)
	var oldest *diagnosticOutcome
	for index := 0; index <= maxRenderDiagnostics; index++ {
		request := httptest.NewRequest("GET", fmt.Sprintf("/page?variant=%d", index), nil)
		outcome := newDiagnosticOutcome(request, nextRenderRequestID(), "page", diagnosticsGeneration, nil)
		outcome.checked = true
		commitRenderDiagnostics(request, outcome)
		if index == 0 {
			oldest = outcome
		}
	}
	oldest.errors = []error{errors.New("late failure after eviction")}
	commitRenderDiagnostics(nil, oldest)
	snapshot := collectCurrentRenderDiagnostics()
	if snapshot.EvictedContexts != 1 || len(snapshot.Records) != 0 || len(renderDiagnostics) != maxRenderDiagnostics {
		t.Fatalf("bounded diagnostics state = %#v", snapshot)
	}
}
