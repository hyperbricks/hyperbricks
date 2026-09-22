package main

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/hyperbricks/hyperbricks/assets"
	"github.com/hyperbricks/hyperbricks/pkg/logging"
	"go.uber.org/zap/zapcore"
)

func TestSharedUIAssetHandlers(t *testing.T) {
	assets := []struct {
		name, contentType, contains string
		handler                     http.HandlerFunc
	}{
		{"stylesheet", "text/css", "data-theme=lofi", serveHyperbricksUIStylesheet},
		{"theme script", "text/javascript", "night", serveHyperbricksThemeScript},
		{"icons script", "text/javascript", "lucide", serveHyperbricksIconsScript},
		{"brand mark", "image/svg+xml", "<svg", serveBrandMark},
		{"favicon", "image/svg+xml", "prefers-color-scheme", serveFavicon},
	}

	for _, asset := range assets {
		t.Run(asset.name, func(t *testing.T) {
			response := httptest.NewRecorder()
			asset.handler(response, httptest.NewRequest(http.MethodGet, "/", nil))
			if response.Code != http.StatusOK || !strings.HasPrefix(response.Header().Get("Content-Type"), asset.contentType) || !strings.Contains(response.Body.String(), asset.contains) {
				t.Fatalf("unexpected response: status=%d type=%q body bytes=%d", response.Code, response.Header().Get("Content-Type"), response.Body.Len())
			}

			response = httptest.NewRecorder()
			asset.handler(response, httptest.NewRequest(http.MethodHead, "/", nil))
			if response.Code != http.StatusOK || response.Body.Len() != 0 {
				t.Fatalf("unexpected HEAD response: status=%d body bytes=%d", response.Code, response.Body.Len())
			}
			response = httptest.NewRecorder()
			asset.handler(response, httptest.NewRequest(http.MethodPost, "/", nil))
			if response.Code != http.StatusMethodNotAllowed {
				t.Fatalf("POST should be rejected, got %d", response.Code)
			}
		})
	}
}

func TestDashboardSharedUIRendering(t *testing.T) {
	data := SysData{
		Module: "example", Mode: "development", SpacesRoute: "/custom/spaces", ErrorsRoute: errorsViewPath,
		Configs: map[string]map[string]interface{}{
			"index": {"route": "index", "@type": "<HYPERMEDIA>"},
		},
		Logs: []logging.LogMessage{{Level: zapcore.ErrorLevel, Message: `<script>alert("unsafe")</script>`, Time: time.Unix(1, 0)}},
	}
	var output bytes.Buffer
	if err := tmpl.Execute(&output, data); err != nil {
		t.Fatal(err)
	}
	page := output.String()
	if got := strings.Count(page, ` hb-surface"`); got != 5 {
		t.Errorf("dashboard should frame metrics, module, routes, logs and plugins, got %d surfaces", got)
	}
	for _, expected := range []string{`class="hb-header"`, `href="/custom/spaces"`, `href="/__hyperbricks/errors"`, `data-theme-toggle`, `&lt;HYPERMEDIA&gt;`, `badge-error`, `&lt;script&gt;`, `id="logLevel"`, `id="routeFilter"`} {
		if !strings.Contains(page, expected) {
			t.Errorf("dashboard missing %s", expected)
		}
	}
	if strings.Contains(page, data.Logs[0].Message) {
		t.Error("log messages must be HTML escaped")
	}
	data.SpacesRoute = ""
	data.ErrorsRoute = ""
	output.Reset()
	if err := tmpl.Execute(&output, data); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(output.String(), `>Spaces</a>`) {
		t.Error("dashboard must not link to a disabled editor")
	}
	if strings.Contains(output.String(), `>Errors</a>`) {
		t.Error("dashboard must not link to a disabled Errors view")
	}
}

func TestDeploymentSharedUIRendering(t *testing.T) {
	local := &deployLocalServer{}
	for _, fixture := range []struct {
		name    string
		handler http.HandlerFunc
	}{
		{"remote", serveDeployDashboard}, {"local", local.serveLocalDashboard},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			response := httptest.NewRecorder()
			fixture.handler(response, httptest.NewRequest(http.MethodGet, "/", nil))
			page := response.Body.String()
			for _, expected := range []string{`data-mode="` + fixture.name + `"`, `class="hb-header"`, `data-theme-toggle`, `id="buildRows"`, `id="pluginBuild"`, `id="menuPanel"`, `/assets/hyperbricks-ui.css`, `/assets/hyperbricks-icons.js`, `class="hb-deploy-workspace hb-surface"`, `id="pluginsView" class="hb-deploy-grid hb-surface hidden"`} {
				if !strings.Contains(page, expected) {
					t.Errorf("deployment page missing %s", expected)
				}
			}
			if strings.Contains(page, "hyperbricks-controls.js") || strings.Contains(page, `class="hero"`) {
				t.Error("legacy layout or inferred controls reintroduced")
			}
			for _, control := range []string{`id="setSecret"`, `id="secretStatus"`, `id="closeConnection"`, `aria-label="Close connection settings"`, `id="deploymentContent" hidden`, `id="connectionNotice"`, `id="openConnection"`} {
				if !strings.Contains(page, control) {
					t.Errorf("deployment connection panel missing %s", control)
				}
			}
		})
	}
	if strings.Contains(assets.DashboardCSS, "@layer legacy") {
		t.Error("layout must not live below Tailwind reset")
	}
}
