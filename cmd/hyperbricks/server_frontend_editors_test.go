package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/hyperbricks/hyperbricks/pkg/component"
	"github.com/hyperbricks/hyperbricks/pkg/shared"
	"github.com/hyperbricks/hyperbricks/pkg/spaces"
)

type editorTestPlugin struct {
	calls int
	route string
}

func TestBuiltinSpacesMountDefaultsAndSafety(t *testing.T) {
	setupDevelopmentModeServeContentTest(t, false)
	cfg := shared.GetHyperBricksConfiguration()
	old := *cfg
	runtime := shared.GetRuntimeOptions()
	t.Cleanup(func() { *cfg = old; shared.SetRuntimeOptions(runtime) })
	module := t.TempDir()
	cfg.Mode = shared.DEVELOPMENT_MODE
	cfg.Directories = nil
	cfg.Plugins.Enabled = nil
	cfg.Development.FrontendEditing = shared.DefaultFrontendEditingConfig()
	cfg.Development.Dashboard.Credentials = developerTestCredentials
	shared.SetRuntimeOptions(shared.RuntimeOptions{ModuleRoot: module})
	request := func(method, path, body string) *httptest.ResponseRecorder {
		t.Helper()
		w := httptest.NewRecorder()
		r := developerTestRequest(method, "http://localhost"+path, strings.NewReader(body))
		r.Header.Set("Origin", "http://localhost")
		r.Header.Set("X-Spaces-Request", "1")
		r.Header.Set("Content-Type", "application/json")
		if !handleFrontendEditor(w, r) {
			t.Fatal("built-in Spaces was not mounted")
		}
		return w
	}
	for _, path := range []string{"", "/web/app.js", "/web/recovery.mjs", "/web/http.mjs", "/web/navigation.mjs", "/web/contextual.js", "/web/contextual.css", "/web/document.css", "/web/style.css", "/web/hyperbricks.css", "/web/theme.js", "/web/brandmark.svg", "/web/favicon.svg", "/web/lucide.js"} {
		w := request("GET", shared.DefaultSpacesRoute+path, "")
		if w.Code != 200 || w.Body.Len() == 0 || w.Header().Get("Cache-Control") != "no-store" {
			t.Fatalf("asset %s: %d %s", path, w.Code, w.Body.String())
		}
	}
	if w := request("GET", shared.DefaultSpacesRoute+"/web/licenses.txt", ""); w.Code != http.StatusNotFound {
		t.Fatalf("removed licenses endpoint: %d %s", w.Code, w.Body.String())
	}
	for _, dashboard := range []bool{true, false} {
		cfg.Development.Dashboard.Enabled = dashboard
		body := request("GET", shared.DefaultSpacesRoute, "").Body.String()
		if !strings.Contains(body, `<title>Spaces | HyperBricks Dashboard</title>`) || !strings.Contains(body, `<span>Dashboard</span>`) || strings.Contains(body, `>Overview</a>`) != dashboard {
			t.Fatalf("Spaces branding or Overview navigation does not match Dashboard availability: %v", dashboard)
		}
		if strings.Contains(body, `href="`+developerDashboardPath+`"`) != dashboard || strings.Contains(body, `href="/dashboard"`) || strings.Contains(body, "__DASHBOARD_NAV__") {
			t.Fatalf("Dashboard navigation does not follow its reserved route and availability: %v", dashboard)
		}
		if strings.Contains(body, `href="/__hyperbricks/errors"`) != dashboard || strings.Contains(body, "__ERRORS_NAV__") {
			t.Fatalf("Errors navigation does not follow Dashboard availability: %v", dashboard)
		}
		if dashboard && strings.Index(body, ">Errors</a>") < strings.Index(body, ">Spaces</a>") {
			t.Error("Errors should follow Spaces in navigation")
		}
	}
	w := request("GET", shared.DefaultSpacesRoute+"/api", "")
	var snapshot spaces.Snapshot
	if err := json.Unmarshal(w.Body.Bytes(), &snapshot); err != nil {
		t.Fatal(err)
	}
	if w.Code != 200 || snapshot.Write || len(snapshot.Spaces) != 0 || len(snapshot.Sources) != 0 {
		t.Fatalf("default snapshot: %d %+v", w.Code, snapshot)
	}
	for _, action := range []string{"create", "save", "trash", "restore"} {
		if w := request("POST", shared.DefaultSpacesRoute+"/api", `{"action":"`+action+`"}`); w.Code != 403 {
			t.Fatalf("read-only %s: %d", action, w.Code)
		}
	}
	for _, path := range []string{"/api/upload", "/api/document"} {
		if w := request("POST", shared.DefaultSpacesRoute+path, `{"action":"copy"}`); w.Code != 403 {
			t.Fatalf("read-only %s: %d", path, w.Code)
		}
	}
	files, err := os.ReadDir(module)
	if err != nil || len(files) != 0 {
		t.Fatalf("read-only requests created files: %v %v", files, err)
	}
	cfg.Development.FrontendEditing.Spaces.Write = true
	if w := request("POST", shared.DefaultSpacesRoute+"/api", `{"action":"create"}`); w.Code != 409 {
		t.Fatalf("write should reach revision validation: %d %s", w.Code, w.Body.String())
	}
	for _, mode := range []string{shared.LIVE_MODE, shared.DEBUG_MODE, shared.DEVELOPMENT_MODE} {
		for _, disabled := range []bool{false, true} {
			for _, production := range []bool{false, true} {
				cfg.Mode = mode
				cfg.Development.FrontendEditing.Enabled = !disabled
				shared.SetRuntimeOptions(shared.RuntimeOptions{ModuleRoot: module, Production: production})
				if mode == shared.DEVELOPMENT_MODE && !disabled && !production {
					continue
				}
				for _, path := range []string{"", "/web/app.js", "/web/navigation.mjs", "/web/contextual.js", "/web/contextual.css", "/api", "/api/assets", "/api/document", "/api/upload"} {
					for _, method := range []string{"GET", "POST"} {
						r := httptest.NewRequest(method, "http://localhost"+shared.DefaultSpacesRoute+path, nil)
						if handleFrontendEditor(httptest.NewRecorder(), r) {
							t.Fatalf("editor exposed in %s, disabled=%v production=%v", mode, disabled, production)
						}
						// The package also enforces the guard when called without the server mount.
						var handler spaces.Handler
						w := httptest.NewRecorder()
						handler.ServeHTTP(w, r)
						if w.Code != 404 {
							t.Fatalf("package guard: %d", w.Code)
						}
					}
				}
			}
		}
	}
	cfg.Mode = shared.DEVELOPMENT_MODE
	cfg.Development.FrontendEditing.Enabled = true
	cfg.Development.FrontendEditing.Spaces.Enabled = false
	shared.SetRuntimeOptions(shared.RuntimeOptions{ModuleRoot: module})
	for _, path := range []string{shared.DefaultSpacesRoute, shared.DefaultSpacesRoute + "/web/app.js", shared.DefaultSpacesRoute + "/api"} {
		r := httptest.NewRequest(http.MethodGet, "http://localhost"+path, nil)
		if handleFrontendEditor(httptest.NewRecorder(), r) {
			t.Fatalf("disabled Spaces route mounted: %s", path)
		}
		var handler spaces.Handler
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		if w.Code != http.StatusNotFound {
			t.Fatalf("disabled Spaces direct handler = %d for %s", w.Code, path)
		}
	}
}

func (p *editorTestPlugin) Render(instance interface{}, ctx context.Context) (any, []error) {
	p.calls++
	cfg := instance.(component.PluginConfig)
	p.route = cfg.Data["route"].(string)
	if _, ok := ctx.Value(shared.Request).(*http.Request); !ok {
		panic("missing request")
	}
	return shared.HandledResponse{Status: 201, ContentType: "application/json", Headers: map[string]string{"X-Editor": "yes"}, Body: []byte(`{"ok":true}`)}, nil
}
func TestFrontendEditorGenericMountDevelopmentAndEnablement(t *testing.T) {
	setupDevelopmentModeServeContentTest(t, false)
	cfg := shared.GetHyperBricksConfiguration()
	oldDashboard, oldEditing, oldPlugins, oldRuntime := cfg.Development.Dashboard, cfg.Development.FrontendEditing, cfg.Plugins, shared.GetRuntimeOptions()
	t.Cleanup(func() {
		cfg.Development.Dashboard = oldDashboard
		cfg.Development.FrontendEditing = oldEditing
		cfg.Plugins = oldPlugins
		shared.SetRuntimeOptions(oldRuntime)
	})
	cfg.Development.FrontendEditing = shared.DefaultFrontendEditingConfig()
	cfg.Development.Dashboard.Credentials = developerTestCredentials
	cfg.Development.FrontendEditing.Editors = map[string]shared.FrontendEditorConfig{"test": {Plugin: "Editor@1.0.0", Route: "/__hyperbricks/test", Data: map[string]interface{}{"route": "/cannot-override"}}}
	cfg.Plugins.Enabled = []string{"Editor@1.0.0"}
	shared.SetRuntimeOptions(shared.RuntimeOptions{})
	p := &editorTestPlugin{}
	rm.SetPlugin("Editor@1.0.0", p)
	for _, tc := range []struct {
		name, mode, path    string
		enabled, production bool
		handled             bool
		status              int
	}{
		{"development", shared.DEVELOPMENT_MODE, "/__hyperbricks/test/api", true, false, true, 201},
		{"prefix-boundary", shared.DEVELOPMENT_MODE, "/__hyperbricks/testing", true, false, false, 200},
		{"debug", shared.DEBUG_MODE, "/__hyperbricks/test", true, false, false, 200},
		{"live", shared.LIVE_MODE, "/__hyperbricks/test", true, false, false, 200},
		{"production", shared.DEVELOPMENT_MODE, "/__hyperbricks/test", true, true, false, 200},
		{"disabled", shared.DEVELOPMENT_MODE, "/__hyperbricks/test", false, false, false, 200},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg.Mode = tc.mode
			cfg.Development.FrontendEditing.Enabled = tc.enabled
			shared.SetRuntimeOptions(shared.RuntimeOptions{Production: tc.production})
			w := httptest.NewRecorder()
			handled := handleFrontendEditor(w, developerTestRequest("GET", tc.path, nil))
			if handled != tc.handled || w.Code != tc.status {
				t.Fatalf("handled %t status %d", handled, w.Code)
			}
			if handled && (w.Header().Get("Cache-Control") != "no-store" || w.Header().Get("X-Editor") != "yes") {
				t.Fatal(w.Header())
			}
		})
	}
	if p.calls != 1 || p.route != "/__hyperbricks/test" {
		t.Fatalf("calls %d route %s", p.calls, p.route)
	}
	cfg.Mode = shared.DEVELOPMENT_MODE
	cfg.Development.FrontendEditing.Enabled = true
	cfg.Development.FrontendEditing.Spaces.Enabled = false
	shared.SetRuntimeOptions(shared.RuntimeOptions{})
	w := httptest.NewRecorder()
	if !handleFrontendEditor(w, developerTestRequest("GET", "/__hyperbricks/test/api", nil)) || w.Code != 201 || p.calls != 2 {
		t.Fatalf("external editor should remain available with Spaces disabled: %d calls=%d", w.Code, p.calls)
	}
	cfg.Plugins.Enabled = nil
	w = httptest.NewRecorder()
	if !handleFrontendEditor(w, developerTestRequest("POST", "/__hyperbricks/test/api", nil)) || w.Code != 503 {
		t.Fatal("unlisted plugin was called")
	}
}
