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
	shared.SetRuntimeOptions(shared.RuntimeOptions{ModuleRoot: module})
	cases := []struct {
		name, kind, contentType string
		status                  int
		static                  bool
		want                    bool
	}{
		{name: "HTML development", kind: "<HYPERMEDIA>", want: true},
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
			commands.RenderStatic = tc.static
			response := httptest.NewRecorder()
			ServeContent(response, httptest.NewRequest("GET", "http://localhost/?edit=true", nil))
			got := strings.Contains(response.Body.String(), `id="hb-spaces-context"`)
			if got != tc.want {
				t.Fatalf("editing=%v status=%d body=%s", got, response.Code, response.Body.String())
			}
			if tc.want && response.Header().Get("Cache-Control") != "no-store" {
				t.Fatal("editing preview can be cached")
			}
			if response.Header().Get(renderErrorCountHeader) != "0" {
				t.Fatalf("render errors: %s", response.Body.String())
			}
		})
	}
}
