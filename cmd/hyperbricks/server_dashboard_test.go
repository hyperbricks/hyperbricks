package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/hyperbricks/hyperbricks/cmd/hyperbricks/commands"
	"github.com/hyperbricks/hyperbricks/pkg/shared"
)

func TestDashboardCanonicalRouteAuthenticationAndApplicationRoute(t *testing.T) {
	setupErrorsViewTest(t)
	cfg := getHyperBricksConfiguration()
	mux := http.NewServeMux()
	if !registerDashboardHandlers(mux) {
		t.Fatal("enabled development dashboard was not registered")
	}
	setTestRouteConfig("dashboard", map[string]interface{}{
		"@type": "<HYPERMEDIA>", "route": "dashboard",
		"content": map[string]interface{}{"@type": "<HTML>", "value": "<p>Application dashboard</p>"},
	})
	mux.HandleFunc("/", handler)

	for _, scenario := range []struct {
		name        string
		credentials shared.CredentialsConfig
		password    string
		want        int
	}{
		{"missing configuration", shared.CredentialsConfig{}, "", http.StatusServiceUnavailable},
		{"authentication challenge", developerTestCredentials, "", http.StatusUnauthorized},
		{"invalid login", developerTestCredentials, "wrong", http.StatusUnauthorized},
		{"authenticated", developerTestCredentials, developerTestCredentials.Password, http.StatusOK},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			cfg.Development.Dashboard.Credentials = scenario.credentials
			request := httptest.NewRequest(http.MethodGet, developerDashboardPath, nil)
			if scenario.password != "" {
				request.SetBasicAuth(developerTestCredentials.User, scenario.password)
			}
			response := httptest.NewRecorder()
			mux.ServeHTTP(response, request)
			if response.Code != scenario.want {
				t.Fatalf("GET %s = %d, want %d; body=%s", developerDashboardPath, response.Code, scenario.want, response.Body.String())
			}
			if scenario.want == http.StatusUnauthorized && response.Header().Get("WWW-Authenticate") != `Basic realm="`+shared.DeveloperInterfaceRealm+`", charset="UTF-8"` {
				t.Fatalf("wrong developer login challenge: %q", response.Header().Get("WWW-Authenticate"))
			}
			if scenario.want == http.StatusServiceUnavailable && (response.Header().Get("WWW-Authenticate") != "" || !strings.Contains(response.Body.String(), shared.DeveloperInterfaceUnavailableMessage)) {
				t.Fatal("missing credentials must stay locked without a login challenge")
			}
			if scenario.want == http.StatusOK {
				for _, expected := range []string{`href="/__hyperbricks/dashboard"`, `href="/__hyperbricks/custom-spaces"`, `href="/__hyperbricks/errors"`} {
					if !strings.Contains(response.Body.String(), expected) {
						t.Errorf("dashboard navigation missing %s", expected)
					}
				}
				if strings.Contains(response.Body.String(), `href="/dashboard" aria-current="page"`) || response.Header().Get("Cache-Control") != "no-store" {
					t.Fatal("dashboard must use its reserved route and disable response caching")
				}
			}
		})
	}

	cfg.Development.FrontendEditing.Spaces.Enabled = false
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, developerTestRequest(http.MethodGet, developerDashboardPath, nil))
	if response.Code != http.StatusOK || strings.Contains(response.Body.String(), `href="/__hyperbricks/custom-spaces"`) ||
		!strings.Contains(response.Body.String(), `href="/__hyperbricks/errors"`) {
		t.Fatalf("Dashboard should remain available without a Spaces link: %d", response.Code)
	}

	response = httptest.NewRecorder()
	mux.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/dashboard", nil))
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "Application dashboard") || response.Header().Get("WWW-Authenticate") != "" || response.Header().Get("Location") != "" {
		t.Fatalf("application /dashboard was intercepted: %d %v %s", response.Code, response.Header(), response.Body.String())
	}

	cfg.Mode = shared.LIVE_MODE
	response = httptest.NewRecorder()
	mux.ServeHTTP(response, developerTestRequest(http.MethodGet, developerDashboardPath, nil))
	if response.Code != http.StatusNotFound {
		t.Fatalf("already registered dashboard remained exposed in live mode: %d", response.Code)
	}
}

func TestDashboardRegistrationFollowsRuntimeAvailability(t *testing.T) {
	setupErrorsViewTest(t)
	cfg := getHyperBricksConfiguration()
	for _, scenario := range []struct {
		name       string
		mode       string
		enabled    bool
		production bool
		static     bool
		available  bool
	}{
		{"development", shared.DEVELOPMENT_MODE, true, false, false, true},
		{"debug", shared.DEBUG_MODE, true, false, false, true},
		{"live", shared.LIVE_MODE, true, false, false, false},
		{"disabled", shared.DEVELOPMENT_MODE, false, false, false, false},
		{"production", shared.DEVELOPMENT_MODE, true, true, false, false},
		{"static", shared.DEVELOPMENT_MODE, true, false, true, false},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			cfg.Mode, cfg.Development.Dashboard.Enabled = scenario.mode, scenario.enabled
			runtime := shared.GetRuntimeOptions()
			runtime.Production = scenario.production
			shared.SetRuntimeOptions(runtime)
			commands.RenderStatic = scenario.static
			mux := http.NewServeMux()
			if registered := registerDashboardHandlers(mux); registered != scenario.available {
				t.Fatalf("registered=%t, want %t", registered, scenario.available)
			}
			for _, path := range []string{developerDashboardPath, "/assets/dashboard.css"} {
				response := httptest.NewRecorder()
				mux.ServeHTTP(response, developerTestRequest(http.MethodGet, path, nil))
				want := http.StatusNotFound
				if scenario.available {
					want = http.StatusOK
				}
				if response.Code != want {
					t.Fatalf("GET %s = %d, want %d", path, response.Code, want)
				}
			}
			response := httptest.NewRecorder()
			mux.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/dashboard", nil))
			if response.Code != http.StatusNotFound || response.Header().Get("Location") != "" {
				t.Fatalf("legacy route should not reserve or redirect application /dashboard: %d %v", response.Code, response.Header())
			}
		})
	}
}
