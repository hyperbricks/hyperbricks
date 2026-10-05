package main

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
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

	open := assertStatus("no account", func() *http.Request {
		return httptest.NewRequest(http.MethodGet, errorsViewPath, nil)
	}, http.StatusOK)
	if open.Header().Get("WWW-Authenticate") != "" {
		t.Fatal("unconfigured dashboard must not challenge for login")
	}
	cfg.Development.Dashboard.Credentials = shared.CredentialsConfig{User: "developer"}
	locked := assertStatus("partial account", func() *http.Request {
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
	openResponse := httptest.NewRecorder()
	handler(openResponse, httptest.NewRequest(http.MethodGet, "http://localhost/?edit=true", nil))
	if openResponse.Code != http.StatusOK || openResponse.Header().Get("WWW-Authenticate") != "" {
		t.Fatalf("open edit response = %d", openResponse.Code)
	}
	cfg.Development.Dashboard.Credentials = shared.CredentialsConfig{User: "developer"}
	partialResponse := httptest.NewRecorder()
	handler(partialResponse, httptest.NewRequest(http.MethodGet, "http://localhost/?edit=true", nil))
	if partialResponse.Code != http.StatusServiceUnavailable {
		t.Fatalf("partial account edit response = %d", partialResponse.Code)
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

func TestOpenSpacesDoesNotOpenExternalEditorsOrFrontendPanels(t *testing.T) {
	setupErrorsViewTest(t)
	cfg := getHyperBricksConfiguration()
	cfg.Development.Dashboard.Credentials = shared.CredentialsConfig{}
	cfg.Development.FrontendEditing = shared.DefaultFrontendEditingConfig()
	cfg.Development.FrontendEditing.Editors = map[string]shared.FrontendEditorConfig{
		"external": {Plugin: "Editor@1.0.0", Route: "/__hyperbricks/external"},
	}
	for _, path := range []string{"/__hyperbricks/external"} {
		response := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, path, nil)
		if !handleFrontendEditor(response, request) {
			handler(response, request)
		}
		if response.Code != http.StatusServiceUnavailable {
			t.Errorf("GET %s = %d, want locked editor", path, response.Code)
		}
	}
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	ctx := context.WithValue(context.Background(), shared.Request, request)
	if shared.DeveloperInterfaceAuthorized(ctx) {
		t.Fatal("open dashboard authorized frontend error panels")
	}
}

func TestSpacesMountOptionalLoginInDevelopmentAndDebug(t *testing.T) {
	setupDevelopmentModeServeContentTest(t, false)
	cfg := getHyperBricksConfiguration()
	old, runtime, static := *cfg, shared.GetRuntimeOptions(), commands.RenderStatic
	t.Cleanup(func() { *cfg = old; shared.SetRuntimeOptions(runtime); commands.RenderStatic = static })
	cfg.Directories = nil
	cfg.Plugins.Enabled = nil
	cfg.Development.FrontendEditing = shared.DefaultFrontendEditingConfig()
	cfg.Development.FrontendEditing.Spaces.AllowedHosts = []string{"editor.example.test"}
	shared.SetRuntimeOptions(shared.RuntimeOptions{ModuleRoot: t.TempDir()})
	commands.RenderStatic = false
	for _, mode := range []string{shared.DEVELOPMENT_MODE, shared.DEBUG_MODE} {
		for _, dashboard := range []bool{false, true} {
			for _, tc := range []struct {
				name, host, password string
				credentials          shared.CredentialsConfig
				want                 int
			}{
				{name: "open localhost", host: "localhost", want: http.StatusOK},
				{name: "open allowed host", host: "editor.example.test:8125", want: http.StatusOK},
				{name: "untrusted host", host: "untrusted.example.test", want: http.StatusForbidden},
				{name: "user only", host: "localhost", credentials: shared.CredentialsConfig{User: "developer"}, want: http.StatusServiceUnavailable},
				{name: "password only", host: "localhost", credentials: shared.CredentialsConfig{Password: "secret"}, want: http.StatusServiceUnavailable},
				{name: "login required", host: "localhost", credentials: developerTestCredentials, want: http.StatusUnauthorized},
				{name: "wrong login", host: "localhost", credentials: developerTestCredentials, password: "wrong", want: http.StatusUnauthorized},
				{name: "valid login", host: "localhost", credentials: developerTestCredentials, password: developerTestCredentials.Password, want: http.StatusOK},
			} {
				t.Run(mode+"/"+tc.name+"/dashboard="+strconv.FormatBool(dashboard), func(t *testing.T) {
					cfg.Mode, cfg.Development.Dashboard.Enabled = mode, dashboard
					cfg.Development.Dashboard.Credentials = tc.credentials
					request := httptest.NewRequest(http.MethodGet, "http://"+tc.host+shared.DefaultSpacesRoute, nil)
					if tc.password != "" {
						request.SetBasicAuth(developerTestCredentials.User, tc.password)
					}
					response := httptest.NewRecorder()
					if !handleFrontendEditor(response, request) || response.Code != tc.want {
						t.Fatalf("Spaces status=%d, want %d; body=%s", response.Code, tc.want, response.Body.String())
					}
					if (response.Header().Get("WWW-Authenticate") != "") != (tc.want == http.StatusUnauthorized) {
						t.Fatalf("unexpected login challenge: %v", response.Header())
					}
					if response.Header().Get("Cache-Control") != "no-store" {
						t.Fatal("Spaces response may be cached")
					}
				})
			}
		}
	}
	commands.RenderStatic = true
	if handleFrontendEditor(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "http://localhost"+shared.DefaultSpacesRoute, nil)) {
		t.Fatal("Spaces mounted during static output")
	}
}
