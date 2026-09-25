package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/hyperbricks/hyperbricks/cmd/hyperbricks/commands"
	"github.com/hyperbricks/hyperbricks/pkg/shared"
)

func setupErrorsViewTest(t *testing.T) {
	t.Helper()
	setupDevelopmentModeServeContentTest(t, false)
	cfg := getHyperBricksConfiguration()
	oldDashboard, oldEditing := cfg.Development.Dashboard, cfg.Development.FrontendEditing
	oldRuntime, oldStatic := shared.GetRuntimeOptions(), commands.RenderStatic
	cfg.Development.Dashboard = shared.DevelopmentDashboardConfig{Enabled: true, Credentials: developerTestCredentials}
	cfg.Development.FrontendEditing.Enabled = true
	cfg.Development.FrontendEditing.Spaces.Enabled = true
	cfg.Development.FrontendEditing.Spaces.Route = "/__hyperbricks/custom-spaces"
	runtime := oldRuntime
	runtime.Production = false
	shared.SetRuntimeOptions(runtime)
	commands.RenderStatic = false
	t.Cleanup(func() {
		cfg.Development.Dashboard, cfg.Development.FrontendEditing = oldDashboard, oldEditing
		shared.SetRuntimeOptions(oldRuntime)
		commands.RenderStatic = oldStatic
	})
}

func TestErrorsViewReadOnlyAssetsAndNavigation(t *testing.T) {
	setupErrorsViewTest(t)
	for _, path := range []string{errorsViewPath, errorsViewPath + "/web/errors.css", errorsViewPath + "/web/errors.js", errorsViewPath + "/web/errors-model.mjs"} {
		t.Run(path, func(t *testing.T) {
			response := httptest.NewRecorder()
			handler(response, developerTestRequest(http.MethodGet, path, nil))
			if response.Code != 200 || response.Body.Len() == 0 || response.Header().Get("Cache-Control") != "no-store" || response.Header().Get("X-Content-Type-Options") != "nosniff" {
				t.Fatalf("GET %s: status=%d headers=%v", path, response.Code, response.Header())
			}
			if path == errorsViewPath {
				body := response.Body.String()
				for _, expected := range []string{`<title>Errors | HyperBricks Dashboard</title>`, `<span>Dashboard</span>`, `>Overview</a>`, `href="/__hyperbricks/custom-spaces"`, `href="` + developerDashboardPath + `"`, `aria-current="page"`, `id="issues"`, `/assets/hyperbricks-ui.css`, `data-theme-toggle`} {
					if !strings.Contains(body, expected) {
						t.Errorf("missing %s", expected)
					}
				}
				if strings.Contains(body, "Related logs") {
					t.Error("log view is outside this scope")
				}
				if !strings.Contains(response.Header().Get("Content-Security-Policy"), "script-src 'self'") {
					t.Error("script CSP missing")
				}
			}
			response = httptest.NewRecorder()
			handler(response, developerTestRequest(http.MethodHead, path, nil))
			if response.Code != 200 || response.Body.Len() != 0 {
				t.Fatalf("HEAD: %d, %d bytes", response.Code, response.Body.Len())
			}
			response = httptest.NewRecorder()
			handler(response, developerTestRequest(http.MethodPost, path, nil))
			if response.Code != 405 {
				t.Fatalf("POST: %d", response.Code)
			}
		})
	}
	for _, path := range []string{errorsViewPath + "/unknown", errorsViewPath + "/logs"} {
		response := httptest.NewRecorder()
		handler(response, developerTestRequest(http.MethodGet, path, nil))
		if response.Code != 404 {
			t.Fatalf("unexpected endpoint %s: %d", path, response.Code)
		}
	}
	cfg := getHyperBricksConfiguration()
	cfg.Development.FrontendEditing.Spaces.Enabled = false
	response := httptest.NewRecorder()
	handler(response, developerTestRequest(http.MethodGet, errorsViewPath, nil))
	if response.Code != http.StatusOK || strings.Contains(response.Body.String(), `href="/__hyperbricks/custom-spaces"`) {
		t.Fatalf("Errors view should remain available without a Spaces link: %d", response.Code)
	}
}

func TestErrorsViewDisabledOutsideDevelopmentDashboard(t *testing.T) {
	setupErrorsViewTest(t)
	cfg := getHyperBricksConfiguration()
	for _, scenario := range []string{"live", "production", "static", "dashboard disabled", "debug"} {
		t.Run(scenario, func(t *testing.T) {
			cfg.Mode = shared.DEVELOPMENT_MODE
			cfg.Development.Dashboard = shared.DevelopmentDashboardConfig{Enabled: true, Credentials: developerTestCredentials}
			runtime := shared.GetRuntimeOptions()
			runtime.Production, commands.RenderStatic = false, false
			switch scenario {
			case "live":
				cfg.Mode = shared.LIVE_MODE
			case "debug":
				cfg.Mode = shared.DEBUG_MODE
			case "production":
				runtime.Production = true
			case "static":
				commands.RenderStatic = true
			case "dashboard disabled":
				cfg.Development.Dashboard.Enabled = false
			}
			shared.SetRuntimeOptions(runtime)
			want := 404
			if scenario == "debug" {
				want = 200
			}
			for _, path := range []string{errorsViewPath, errorsViewPath + "/web/errors.js"} {
				response := httptest.NewRecorder()
				handler(response, developerTestRequest(http.MethodGet, path, nil))
				if response.Code != want {
					t.Fatalf("%s: got %d want %d", path, response.Code, want)
				}
			}
		})
	}
}

func TestErrorsViewUsesExistingDiagnosticsAndPreservesSeverity(t *testing.T) {
	setupErrorsViewTest(t)
	message := `<script>alert("not executable")</script>`
	recordRenderDiagnostics(nil, "hb-errors-test", "index", []error{
		shared.ComponentError{Level: "WARNING", Err: "Optional value", File: "page.yaml", Path: "content", Key: "label"},
		shared.ComponentError{Level: "WARNING", Rejected: true, Err: "Rejected value"},
		errors.New(message),
	})
	response := httptest.NewRecorder()
	handler(response, developerTestRequest(http.MethodGet, renderDiagnosticsPath+"?limit=200", nil))
	var records []RenderDiagnostics
	if err := json.Unmarshal(response.Body.Bytes(), &records); err != nil {
		t.Fatal(err)
	}
	if len(records) != 1 || len(records[0].Errors) != 3 || records[0].RequestID != "hb-errors-test" {
		t.Fatalf("unexpected records: %#v", records)
	}
	if records[0].Errors[0].Level != "WARNING" || !records[0].Errors[1].Rejected || records[0].Errors[2].Err != message {
		t.Fatalf("metadata lost: %#v", records[0].Errors)
	}
	if strings.Contains(response.Body.String(), "<script>") {
		t.Error("JSON should escape HTML")
	}
	response = httptest.NewRecorder()
	handler(response, developerTestRequest(http.MethodGet, renderDiagnosticsPath+"?request_id=expired", nil))
	if response.Code != 404 {
		t.Errorf("expired diagnostic: %d", response.Code)
	}
}
