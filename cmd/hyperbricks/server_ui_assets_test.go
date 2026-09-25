package main

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
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
		nosniff                     bool
	}{
		{"stylesheet", "text/css", "data-theme=lofi", serveHyperbricksUIStylesheet, false},
		{"theme script", "text/javascript", "night", serveHyperbricksThemeScript, false},
		{"icons script", "text/javascript", "lucide", serveHyperbricksIconsScript, false},
		{"deployment YAML editor", "text/javascript", "createDeployYAMLEditor", serveDeployYAMLEditorScript, true},
		{"brand mark", "image/svg+xml", "<svg", serveBrandMark, false},
		{"favicon", "image/svg+xml", "prefers-color-scheme", serveFavicon, false},
	}

	for _, asset := range assets {
		t.Run(asset.name, func(t *testing.T) {
			response := httptest.NewRecorder()
			asset.handler(response, httptest.NewRequest(http.MethodGet, "/", nil))
			if response.Code != http.StatusOK || !strings.HasPrefix(response.Header().Get("Content-Type"), asset.contentType) || !strings.Contains(response.Body.String(), asset.contains) {
				t.Fatalf("unexpected response: status=%d type=%q body bytes=%d", response.Code, response.Header().Get("Content-Type"), response.Body.Len())
			}
			if response.Header().Get("Cache-Control") != "no-store" {
				t.Errorf("asset should not be cached, got %q", response.Header().Get("Cache-Control"))
			}
			if asset.nosniff && response.Header().Get("X-Content-Type-Options") != "nosniff" {
				t.Errorf("script should disable MIME sniffing")
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

func TestDeployYAMLEditorBundleBudget(t *testing.T) {
	if bytes.Contains(assets.DeployYAMLEditorScript, []byte("sourceMappingURL")) {
		t.Fatal("deployment YAML editor must not ship a source map reference")
	}
	if !bytes.Contains(assets.DeployYAMLEditorScript, []byte("Third-party software notices")) {
		t.Fatal("deployment YAML editor must retain its third-party notices")
	}

	compressed := compressedDeployYAMLEditorScript()
	const maxGzipBytes = 128 * 1024
	if len(compressed) > maxGzipBytes {
		t.Fatalf("deployment YAML editor gzip size = %d bytes, budget = %d", len(compressed), maxGzipBytes)
	}
}

func TestDeployYAMLEditorServesNegotiatedGzip(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/assets/deploy-yaml-editor.js", nil)
	request.Header.Set("Accept-Encoding", "br, gzip")
	response := httptest.NewRecorder()
	serveDeployYAMLEditorScript(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d", response.Code)
	}
	if response.Header().Get("Content-Encoding") != "gzip" {
		t.Fatalf("expected gzip content encoding, got %q", response.Header().Get("Content-Encoding"))
	}
	if response.Header().Get("Vary") != "Accept-Encoding" {
		t.Fatalf("expected Accept-Encoding variance, got %q", response.Header().Get("Vary"))
	}
	if response.Header().Get("Content-Length") != strconv.Itoa(response.Body.Len()) {
		t.Fatalf("compressed Content-Length does not match body bytes")
	}
	reader, err := gzip.NewReader(bytes.NewReader(response.Body.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}
	if err := reader.Close(); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(decoded, assets.DeployYAMLEditorScript) {
		t.Fatal("gzip response does not decode to the embedded editor bundle")
	}

	request = httptest.NewRequest(http.MethodGet, "/assets/deploy-yaml-editor.js", nil)
	request.Header.Set("Accept-Encoding", "gzip;q=0")
	response = httptest.NewRecorder()
	serveDeployYAMLEditorScript(response, request)
	if response.Header().Get("Content-Encoding") != "" {
		t.Fatalf("gzip;q=0 must return the identity representation")
	}
	if !bytes.Equal(response.Body.Bytes(), assets.DeployYAMLEditorScript) {
		t.Fatal("identity response does not match the embedded editor bundle")
	}
}

func TestDeployUIAssetRegistrationBeatsDashboardCatchAlls(t *testing.T) {
	local := &deployLocalServer{}
	for _, fixture := range []struct {
		name     string
		catchAll http.HandlerFunc
	}{
		{name: "remote", catchAll: serveDeployDashboard},
		{name: "local", catchAll: local.serveLocalDashboard},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			mux := http.NewServeMux()
			registerDeployUIAssets(mux)
			mux.HandleFunc("/", fixture.catchAll)
			response := httptest.NewRecorder()
			mux.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/assets/deploy-yaml-editor.js", nil))
			if response.Code != http.StatusOK || !strings.HasPrefix(response.Header().Get("Content-Type"), "text/javascript") || !strings.Contains(response.Body.String(), "createDeployYAMLEditor") {
				t.Fatalf("unexpected routed asset: status=%d type=%q body bytes=%d", response.Code, response.Header().Get("Content-Type"), response.Body.Len())
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
			for _, expected := range []string{`data-mode="` + fixture.name + `"`, `class="hb-header"`, `data-theme-toggle`, `id="deploymentNav"`, `id="viewModules"`, `id="viewPlugins"`, `id="moduleSidebar"`, `id="moduleDrawerToggle"`, `id="pluginSidebar"`, `id="pluginDrawerToggle"`, `id="buildRows"`, `id="pluginBuild"`, `id="menuPanel"`, `/assets/hyperbricks-ui.css`, `/assets/hyperbricks-icons.js`, `id="modulesView" class="drawer hb-deploy-modules"`, `id="pluginsView" class="drawer hb-deploy-plugins hidden"`} {
				if !strings.Contains(page, expected) {
					t.Errorf("deployment page missing %s", expected)
				}
			}
			if strings.Contains(page, "hyperbricks-controls.js") || strings.Contains(page, `class="hero"`) {
				t.Error("legacy layout or inferred controls reintroduced")
			}
			for _, control := range []string{
				`id="setSecret"`,
				`id="secretStatus"`,
				`id="closeConnection"`,
				`aria-label="Close connection settings"`,
				`id="deploymentContent" hidden`,
				`id="connectionNotice"`,
				`id="openConnection"`,
				`id="packageConfigDialog"`,
				`id="packageConfigForm"`,
				`id="packageConfigEditorShell"`,
				`id="packageConfigEditorHost"`,
				`id="packageConfigContent"`,
				`id="packageConfigEditorStatus"`,
				`id="packageConfigError"`,
				`id="packageConfigStatus"`,
				`id="savePackageConfig"`,
				`value="development"`,
				`value="live"`,
				`"/mode"`,
				`"/package-config"`,
				`import("/assets/deploy-yaml-editor.js")`,
				`"/archive"`,
			} {
				if !strings.Contains(page, control) {
					t.Errorf("deployment controls missing %s", control)
				}
			}
			if strings.Contains(page, `<script src="/assets/deploy-yaml-editor.js"`) || strings.Contains(page, `<link rel="modulepreload" href="/assets/deploy-yaml-editor.js"`) {
				t.Error("deployment YAML editor must remain lazy-loaded")
			}
		})
	}
	if strings.Contains(assets.DashboardCSS, "@layer legacy") {
		t.Error("layout must not live below Tailwind reset")
	}
}
