package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/hyperbricks/hyperbricks/cmd/hyperbricks/commands"
	"github.com/hyperbricks/hyperbricks/pkg/shared"
)

var developerTestCredentials = shared.CredentialsConfig{User: "developer", Password: "correct horse battery staple"}

func developerTestRequest(method, target string, body io.Reader) *http.Request {
	request := httptest.NewRequest(method, target, body)
	request.SetBasicAuth(developerTestCredentials.User, developerTestCredentials.Password)
	return request
}

func TestDeveloperRouteAuthenticationStates(t *testing.T) {
	setupDevelopmentModeServeContentTest(t, false)
	cfg := getHyperBricksConfiguration()
	old, runtime, static := *cfg, shared.GetRuntimeOptions(), commands.RenderStatic
	t.Cleanup(func() { *cfg = old; shared.SetRuntimeOptions(runtime); commands.RenderStatic = static })
	cfg.Mode = shared.DEVELOPMENT_MODE
	cfg.Development.Dashboard.Enabled = true
	cfg.Development.FrontendEditing = shared.DefaultFrontendEditingConfig()
	activeRuntime := runtime
	activeRuntime.Production = false
	shared.SetRuntimeOptions(activeRuntime)
	commands.RenderStatic = false

	assertStatus := func(name string, request func() *http.Request, want int) *httptest.ResponseRecorder {
		t.Helper()
		response := httptest.NewRecorder()
		handler(response, request())
		if response.Code != want {
			t.Fatalf("%s: status = %d, want %d; body=%q", name, response.Code, want, response.Body.String())
		}
		return response
	}

	locked := assertStatus("locked", func() *http.Request {
		return httptest.NewRequest(http.MethodGet, errorsViewPath, nil)
	}, http.StatusServiceUnavailable)
	if locked.Header().Get("WWW-Authenticate") != "" || !strings.Contains(locked.Body.String(), shared.DeveloperInterfaceUnavailableMessage) {
		t.Fatalf("locked response = %#v %q", locked.Header(), locked.Body.String())
	}

	cfg.Development.Dashboard.Credentials = developerTestCredentials
	challenged := assertStatus("challenge", func() *http.Request {
		return httptest.NewRequest(http.MethodGet, errorsViewPath, nil)
	}, http.StatusUnauthorized)
	if challenged.Header().Get("WWW-Authenticate") == "" {
		t.Fatal("developer route did not issue a Basic challenge")
	}
	assertStatus("wrong password", func() *http.Request {
		request := httptest.NewRequest(http.MethodGet, errorsViewPath, nil)
		request.SetBasicAuth(developerTestCredentials.User, "wrong")
		return request
	}, http.StatusUnauthorized)
	assertStatus("authorized", func() *http.Request {
		return developerTestRequest(http.MethodGet, errorsViewPath, nil)
	}, http.StatusOK)

	cfg.Development.Dashboard.Enabled = false
	assertStatus("disabled feature", func() *http.Request {
		return httptest.NewRequest(http.MethodGet, errorsViewPath, nil)
	}, http.StatusNotFound)
}

func TestContextualEditingRequiresLoginButPublicRouteDoesNot(t *testing.T) {
	setupDevelopmentModeServeContentTest(t, false)
	cfg := getHyperBricksConfiguration()
	old, runtime := *cfg, shared.GetRuntimeOptions()
	t.Cleanup(func() { *cfg = old; shared.SetRuntimeOptions(runtime) })
	cfg.Mode = shared.DEVELOPMENT_MODE
	cfg.Development.FrontendEditing = shared.DefaultFrontendEditingConfig()
	cfg.Development.Dashboard.Credentials = developerTestCredentials
	setTestRouteConfig("index", map[string]interface{}{
		"@type": "<HYPERMEDIA>", "route": "index",
		"content": map[string]interface{}{"@type": "<HTML>", "value": "<p>Public</p>"},
	})

	publicResponse := httptest.NewRecorder()
	handler(publicResponse, httptest.NewRequest(http.MethodGet, "http://localhost/", nil))
	if publicResponse.Code != http.StatusOK || !strings.Contains(publicResponse.Body.String(), "Public") {
		t.Fatalf("public route = %d %q", publicResponse.Code, publicResponse.Body.String())
	}

	editResponse := httptest.NewRecorder()
	handler(editResponse, httptest.NewRequest(http.MethodGet, "http://localhost/?edit=true", nil))
	if editResponse.Code != http.StatusUnauthorized || editResponse.Header().Get("WWW-Authenticate") == "" {
		t.Fatalf("edit response = %d %#v", editResponse.Code, editResponse.Header())
	}

	cfg.Development.Dashboard.Credentials = shared.CredentialsConfig{}
	lockedResponse := httptest.NewRecorder()
	handler(lockedResponse, httptest.NewRequest(http.MethodGet, "http://localhost/?edit=true", nil))
	if lockedResponse.Code != http.StatusServiceUnavailable {
		t.Fatalf("locked edit response = %d", lockedResponse.Code)
	}

	cfg.Development.FrontendEditing.Spaces.Enabled = false
	spacesDisabled := httptest.NewRecorder()
	handler(spacesDisabled, httptest.NewRequest(http.MethodGet, "http://localhost/?edit=true", nil))
	if spacesDisabled.Code != http.StatusOK || !strings.Contains(spacesDisabled.Body.String(), "Public") || spacesDisabled.Header().Get("WWW-Authenticate") != "" {
		t.Fatalf("disabled Spaces should leave public route accessible: %d", spacesDisabled.Code)
	}
}

func TestFrontendErrorPanelRequiresDeveloperLogin(t *testing.T) {
	setupDevelopmentModeServeContentTest(t, false)
	cfg := getHyperBricksConfiguration()
	old := *cfg
	t.Cleanup(func() { *cfg = old })
	cfg.Mode = shared.DEVELOPMENT_MODE
	cfg.Development.FrontendErrors = true
	cfg.Development.Dashboard.Credentials = developerTestCredentials
	setTestRouteConfig("index", map[string]interface{}{
		"@type": "<HYPERMEDIA>", "route": "index",
		"content": map[string]interface{}{"@type": "<HTML>", "value": "<p>Public</p>"},
	})

	publicResponse := httptest.NewRecorder()
	handler(publicResponse, httptest.NewRequest(http.MethodGet, "http://localhost/", nil))
	if strings.Contains(publicResponse.Body.String(), `id="error_panel"`) {
		t.Fatal("developer error panel leaked into an unauthenticated public response")
	}

	authenticatedResponse := httptest.NewRecorder()
	handler(authenticatedResponse, developerTestRequest(http.MethodGet, "http://localhost/", nil))
	if !strings.Contains(authenticatedResponse.Body.String(), `id="error_panel"`) {
		t.Fatal("authenticated developer response omitted the configured error panel")
	}
}
