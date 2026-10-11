package main

import (
	"context"
	"errors"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/hyperbricks/hyperbricks/cmd/hyperbricks/commands"
	"github.com/hyperbricks/hyperbricks/pkg/shared"
)

func TestStatusIndicatorResponseGates(t *testing.T) {
	setupErrorsViewTest(t)
	cfg := getHyperBricksConfiguration()
	previous, runtime, static := *cfg, shared.GetRuntimeOptions(), commands.RenderStatic
	t.Cleanup(func() { *cfg = previous; shared.SetRuntimeOptions(runtime); commands.RenderStatic = static })
	for _, tc := range []struct {
		name, mode, kind, contentType, method        string
		enabled, dashboard, production, static, want bool
	}{
		{"development", "development", "<HYPERMEDIA>", "text/html", "GET", true, true, false, false, true},
		{"debug", "debug", "<HYPERMEDIA>", "text/html; charset=utf-8", "GET", true, true, false, false, true},
		{"live", "live", "<HYPERMEDIA>", "text/html", "GET", true, true, false, false, false},
		{"disabled", "development", "<HYPERMEDIA>", "text/html", "GET", false, true, false, false, false},
		{"dashboard off", "development", "<HYPERMEDIA>", "text/html", "GET", true, false, false, false, false},
		{"production", "development", "<HYPERMEDIA>", "text/html", "GET", true, true, true, false, false},
		{"static", "development", "<HYPERMEDIA>", "text/html", "GET", true, true, false, true, false},
		{"fragment", "development", "<FRAGMENT>", "text/html", "GET", true, true, false, false, false},
		{"api fragment", "development", "<API_FRAGMENT_RENDER>", "text/html", "GET", true, true, false, false, false},
		{"json", "development", "<HYPERMEDIA>", "application/json", "GET", true, true, false, false, false},
		{"head", "development", "<HYPERMEDIA>", "text/html", "HEAD", true, true, false, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg.Mode = tc.mode
			cfg.Development.StatusIndicator = tc.enabled
			cfg.Development.Dashboard.Enabled = tc.dashboard
			opts := runtime
			opts.Production = tc.production
			shared.SetRuntimeOptions(opts)
			commands.RenderStatic = tc.static
			original := "<!doctype html><html><head><title>Test</title></head><body>Hello</body></html>"
			got := decorateStatusIndicator(original, tc.kind, tc.contentType, "hb-10", 1, httptest.NewRequest(tc.method, "/", nil))
			if strings.Contains(got, statusIndicatorPath) != tc.want {
				t.Fatalf("wrong decoration: %s", got)
			}
			if tc.want && !strings.Contains(got, `data-errors="1"`) {
				t.Fatal("missing initial failure")
			}
			if !tc.want && got != original {
				t.Fatal("changed excluded response")
			}
			response := httptest.NewRecorder()
			handleStatusIndicator(response, httptest.NewRequest("GET", statusIndicatorPath, nil))
			available := tc.enabled && tc.dashboard && !tc.production && !tc.static && tc.mode != "live"
			if available && (response.Code != 200 || !strings.Contains(response.Body.String(), "hyperbricks-devtools")) {
				t.Fatal("missing script")
			}
			if !available && response.Code != 404 {
				t.Fatal("script exposed outside development policy")
			}
			css := httptest.NewRecorder()
			handleStatusIndicator(css, httptest.NewRequest("GET", "/__hyperbricks/status-indicator.css", nil))
			if available && (css.Code != 200 || css.Header().Get("Content-Type") != "text/css; charset=utf-8" || !strings.Contains(css.Body.String(), ".badge-soft")) {
				t.Fatal("missing badge stylesheet")
			}
			if !available && css.Code != 404 {
				t.Fatal("stylesheet exposed outside development policy")
			}
		})
	}
}

func TestStatusIndicatorConfigDefault(t *testing.T) {
	for _, setting := range []string{"", "status_indicator: true", "status_indicator: false"} {
		cfg, err := shared.ValidatePackageConfigBytes([]byte("hyperbricks: {development: {"+setting+"}}"), t.TempDir())
		if err != nil {
			t.Fatal(err)
		}
		if cfg.Development.StatusIndicator != (setting != "status_indicator: false") {
			t.Fatalf("incorrect setting %q", setting)
		}
	}
}

func TestStatusIndicatorRenderedPageErrorAndRecovery(t *testing.T) {
	setupErrorsViewTest(t)
	cfg := getHyperBricksConfiguration()
	old := cfg.Development.StatusIndicator
	cfg.Development.StatusIndicator = true
	t.Cleanup(func() { cfg.Development.StatusIndicator = old })
	broken := true
	installDiagnosticTestRoute(t, func(context.Context) (string, []error) {
		if broken {
			return "unavailable", []error{errors.New("private upstream detail")}
		}
		return "recovered", nil
	})
	for _, want := range []string{"1", "0"} {
		response := diagnosticRequest("/page")
		body := response.Body.String()
		if !strings.Contains(body, `data-errors="`+want+`"`) || !strings.Contains(body, statusIndicatorPath) {
			t.Fatalf("missing page status: %s", body)
		}
		if response.Header().Get("Content-Length") != strconv.Itoa(len(body)) {
			t.Fatal("decoration broke content length")
		}
		if response.Header().Get(renderErrorCountHeader) != want {
			t.Fatal("header/status mismatch")
		}
		broken = false
	}
}

func TestStatusIndicatorUsesRealHead(t *testing.T) {
	setupErrorsViewTest(t)
	cfg := getHyperBricksConfiguration()
	old := cfg.Development.StatusIndicator
	cfg.Development.StatusIndicator = true
	t.Cleanup(func() { cfg.Development.StatusIndicator = old })
	source := `<!doctype html><!-- <head> --><html><HEAD data-example="yes"><title>Page</title></HEAD><body></body></html>`
	output := decorateStatusIndicator(source, "<HYPERMEDIA>", "text/html", "hb-1", 0, httptest.NewRequest("GET", "/", nil))
	if !strings.Contains(output, `<HEAD data-example="yes"><script src="`+statusIndicatorPath) {
		t.Fatalf("wrong insertion: %s", output)
	}
	response := httptest.NewRecorder()
	handleStatusIndicator(response, httptest.NewRequest("POST", statusIndicatorPath, nil))
	if response.Code != 405 || response.Header().Get("Allow") != "GET, HEAD" {
		t.Fatal("unsafe helper method")
	}
	response = httptest.NewRecorder()
	handleStatusIndicator(response, httptest.NewRequest("HEAD", statusIndicatorPath, nil))
	if response.Code != 200 || response.Body.Len() != 0 || response.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("wrong helper HEAD contract")
	}
}
