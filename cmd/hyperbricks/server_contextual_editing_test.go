package main

import (
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hyperbricks/hyperbricks/cmd/hyperbricks/commands"
	"github.com/hyperbricks/hyperbricks/pkg/shared"
)

func TestContextualEditingResponseBoundaries(t *testing.T) {
	setupDevelopmentModeServeContentTest(t, false)
	cfg := shared.GetHyperBricksConfiguration()
	old, runtime, static := *cfg, shared.GetRuntimeOptions(), commands.RenderStatic
	t.Cleanup(func() { *cfg = old; shared.SetRuntimeOptions(runtime); commands.RenderStatic = static })
	module := t.TempDir()
	for _, dir := range []string{"hyperbricks", "templates", "resources", "static"} {
		if err := os.Mkdir(filepath.Join(module, dir), 0755); err != nil {
			t.Fatal(err)
		}
	}
	cfg.Directories = nil
	cfg.Plugins.Enabled = nil
	cfg.Server.Beautify = false
	cfg.Development.FrontendEditing = shared.DefaultFrontendEditingConfig()
	cfg.Development.Dashboard.Credentials = developerTestCredentials
	shared.SetRuntimeOptions(shared.RuntimeOptions{ModuleRoot: module})
	cases := []struct {
		name, kind, contentType, mode string
		status                        int
		static, spacesDisabled        bool
		noCredentials, readOnly       bool
		want                          bool
	}{
		{name: "HTML development", kind: "<HYPERMEDIA>", want: true},
		{name: "HTML debug without login", kind: "<HYPERMEDIA>", mode: shared.DEBUG_MODE, noCredentials: true, want: true},
		{name: "read-only without login", kind: "<HYPERMEDIA>", noCredentials: true, readOnly: true, want: true},
		{name: "live", kind: "<HYPERMEDIA>", mode: shared.LIVE_MODE},
		{name: "Spaces disabled", kind: "<HYPERMEDIA>", spacesDisabled: true},
		{name: "explicit HTML", kind: "<HYPERMEDIA>", contentType: "text/html; charset=utf-8", want: true},
		{name: "fragment", kind: "<FRAGMENT>"},
		{name: "JSON", kind: "<HYPERMEDIA>", contentType: "application/json"},
		{name: "error status", kind: "<HYPERMEDIA>", status: 404},
		{name: "static snapshot", kind: "<HYPERMEDIA>", static: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			config := map[string]interface{}{"@type": tc.kind, "route": "index", "content": map[string]interface{}{"@type": "<HTML>", "value": "<p>Ordinary content</p>"}}
			if tc.contentType != "" {
				config["content_type"] = tc.contentType
			}
			if tc.status != 0 {
				config["response"] = map[string]interface{}{"status": tc.status}
			}
			setTestRouteConfig("index", config)
			cfg.Mode = shared.DEVELOPMENT_MODE
			if tc.mode != "" {
				cfg.Mode = tc.mode
			}
			cfg.Development.Dashboard.Credentials = developerTestCredentials
			if tc.noCredentials {
				cfg.Development.Dashboard.Credentials = shared.CredentialsConfig{}
			}
			commands.RenderStatic = tc.static
			cfg.Development.FrontendEditing.Spaces.Enabled = !tc.spacesDisabled
			cfg.Development.FrontendEditing.Spaces.Write = !tc.readOnly
			response := httptest.NewRecorder()
			request := httptest.NewRequest("GET", "http://localhost/?edit=true", nil)
			if !tc.noCredentials {
				request.SetBasicAuth(developerTestCredentials.User, developerTestCredentials.Password)
			}
			ServeContent(response, request)
			got := strings.Contains(response.Body.String(), `id="hb-spaces-context"`)
			if got != tc.want {
				t.Fatalf("editing=%v status=%d body=%s", got, response.Code, response.Body.String())
			}
			if tc.want && response.Header().Get("Cache-Control") != "no-store" {
				t.Fatal("editing preview can be cached")
			}
			if tc.want && tc.readOnly && !strings.Contains(response.Body.String(), `"write":false`) {
				t.Fatal("read-only contextual payload did not preserve the write restriction")
			}
			if response.Header().Get(renderErrorCountHeader) != "0" {
				t.Fatalf("render errors: %s", response.Body.String())
			}
		})
	}
}
