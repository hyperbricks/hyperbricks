package main

import (
	"html/template"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/hyperbricks/hyperbricks/pkg/component"
	"github.com/hyperbricks/hyperbricks/pkg/shared"
	"github.com/yosssi/gohtml"
)

func TestRenderContentOutputUnformattedAndRetained(t *testing.T) {
	for _, path := range []string{"compiled", "renderer"} {
		t.Run(path, func(t *testing.T) {
			fixture := setupSSRProofPipelineBenchmark(t)
			if path == "renderer" {
				configMutex.Lock()
				delete(routePlans, "index")
				configMutex.Unlock()
			}

			requestIDs := []string{`first<&"'>`, `other<&"'>`, strings.Repeat("long-", 32) + `<&"'>`, "last"}
			var retained []RenderContent
			var expected []string
			for _, rid := range requestIDs {
				request := httptest.NewRequest(http.MethodGet, "/?rid="+url.QueryEscape(rid), nil)
				want := strings.ReplaceAll(fixture.expected, "benchmark-request", template.HTMLEscapeString(rid))
				result := renderContent(httptest.NewRecorder(), "index", request, "output-regression")
				assertSSRProofRenderContent(t, result, want)
				if result.RequestID != "output-regression" || result.ContentType != "text/html; charset=utf-8" ||
					result.Headers["Cache-Control"] != "no-store" || result.Handled != nil {
					t.Fatalf("unexpected response metadata: %+v", result)
				}
				retained = append(retained, result)
				expected = append(expected, want)
			}

			for i, result := range retained {
				if result.Content != expected[i] {
					t.Fatalf("retained output for request %d changed after later renders", i)
				}
			}
		})
	}
}

func TestRenderContentOutputBeautify(t *testing.T) {
	fixture := setupSSRProofPipelineBenchmark(t)
	hbConfig := shared.GetHyperBricksConfiguration()
	formatted := gohtml.Format(fixture.expected)
	if formatted == fixture.expected {
		t.Fatal("fixture must distinguish formatted and unformatted output")
	}

	for _, tc := range []struct {
		name         string
		defaultOn    bool
		routeValue   interface{}
		wantBeautify bool
	}{
		{"default_off", false, nil, false},
		{"default_on", true, nil, true},
		{"route_on", false, true, true},
		{"route_off", true, false, false},
		{"string_on", false, "true", true},
		{"string_off", true, "false", false},
		{"invalid_default_off", false, "invalid", false},
		{"invalid_default_on", true, "invalid", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			hbConfig.Server.Beautify = tc.defaultOn
			configMutex.Lock()
			delete(configs["index"], "beautify")
			if tc.routeValue != nil {
				configs["index"]["beautify"] = tc.routeValue
			}
			configMutex.Unlock()

			want := fixture.expected
			if tc.wantBeautify {
				want = formatted
			}
			result := renderContent(httptest.NewRecorder(), "index", fixture.request, "beautify-regression")
			assertSSRProofRenderContent(t, result, want)
		})
	}
}

func TestRenderContentOutputHandledResponse(t *testing.T) {
	setupLiveModeServeContentTest(t)
	const body = "<div><span>raw</span></div>\x00"
	rm.SetPlugin("output_handled_test", handledResponseTestPlugin{
		status:      http.StatusCreated,
		contentType: "application/octet-stream",
		headers:     map[string]string{"X-Output-Test": "handled"},
		cookies:     []string{"output=1; Path=/"},
		body:        []byte(body),
	})
	setTestRouteConfig("handled-output", map[string]interface{}{
		"@type":    component.PluginRenderGetName(),
		"route":    "handled-output",
		"plugin":   "output_handled_test",
		"beautify": true,
	})

	request := httptest.NewRequest(http.MethodGet, "/handled-output", nil)
	result := renderContent(httptest.NewRecorder(), "handled-output", request, "handled-regression")
	if result.Handled == nil {
		t.Fatal("missing handled response")
	}
	if result.Content != "" || string(result.Handled.Body) != body {
		t.Fatalf("handled output changed: content=%q body=%q", result.Content, result.Handled.Body)
	}
	if result.Status != http.StatusCreated || result.ErrorCount != 0 || !result.NoCache ||
		result.RequestID != "handled-regression" || result.ContentType != "application/octet-stream" ||
		result.Headers["X-Output-Test"] != "handled" || len(result.Cookies) != 1 || result.Cookies[0] != "output=1; Path=/" {
		t.Fatalf("unexpected handled response metadata: %+v", result)
	}
}
