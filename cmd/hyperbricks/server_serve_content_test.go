package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/hyperbricks/hyperbricks/pkg/component"
	"github.com/hyperbricks/hyperbricks/pkg/composite"
	"github.com/hyperbricks/hyperbricks/pkg/shared"
)

type handledResponseTestPlugin struct {
	calls       *int32
	status      int
	contentType string
	headers     map[string]string
	cookies     []string
	body        []byte
}

func (p handledResponseTestPlugin) Render(_ interface{}, _ context.Context) (any, []error) {
	if p.calls != nil {
		atomic.AddInt32(p.calls, 1)
	}

	headers := map[string]string(nil)
	if len(p.headers) > 0 {
		headers = make(map[string]string, len(p.headers))
		for key, value := range p.headers {
			headers[key] = value
		}
	}

	return shared.HandledResponse{
		Status:      p.status,
		ContentType: p.contentType,
		Headers:     headers,
		Cookies:     append([]string(nil), p.cookies...),
		Body:        append([]byte(nil), p.body...),
	}, nil
}

type staticContextTestPlugin struct {
	ctx context.Context
}

func (p *staticContextTestPlugin) Render(_ interface{}, ctx context.Context) (any, []error) {
	p.ctx = ctx
	return "static plugin output", nil
}

func setupLiveModeServeContentTest(t testing.TB) {
	t.Helper()

	shared.Init_configuration()
	hbConfig := shared.GetHyperBricksConfiguration()

	oldMode := hbConfig.Mode
	oldCacheDuration := hbConfig.Live.CacheTime.Duration
	oldConfigs := configs
	oldRM := rm

	htmlCacheMutex.Lock()
	oldHTMLCache := htmlCache
	htmlCache = make(map[string]CacheEntry)
	htmlCacheMutex.Unlock()

	configMutex.Lock()
	configs = make(map[string]map[string]interface{})
	configMutex.Unlock()

	hbConfig.Mode = shared.LIVE_MODE
	hbConfig.Live.CacheTime.Duration = time.Hour

	initializeComponents()

	t.Cleanup(func() {
		hbConfig.Mode = oldMode
		hbConfig.Live.CacheTime.Duration = oldCacheDuration
		rm = oldRM

		configMutex.Lock()
		configs = oldConfigs
		configMutex.Unlock()

		htmlCacheMutex.Lock()
		htmlCache = oldHTMLCache
		htmlCacheMutex.Unlock()
	})
}

func setupDevelopmentModeServeContentTest(t testing.TB, frontendErrors bool) {
	t.Helper()

	shared.Init_configuration()
	hbConfig := shared.GetHyperBricksConfiguration()

	oldMode := hbConfig.Mode
	oldFrontendErrors := hbConfig.Development.FrontendErrors
	oldConfigs := configs
	oldRM := rm

	htmlCacheMutex.Lock()
	oldHTMLCache := htmlCache
	htmlCache = make(map[string]CacheEntry)
	htmlCacheMutex.Unlock()

	renderDiagnosticsMutex.Lock()
	oldRenderDiagnostics := renderDiagnostics
	oldRenderDiagnosticsOrder := renderDiagnosticsOrder
	renderDiagnostics = make(map[string]RenderDiagnostics)
	renderDiagnosticsOrder = nil
	renderDiagnosticsMutex.Unlock()

	configMutex.Lock()
	configs = make(map[string]map[string]interface{})
	configMutex.Unlock()

	renderDiagnosticsSeq = 0

	hbConfig.Mode = shared.DEVELOPMENT_MODE
	hbConfig.Development.FrontendErrors = frontendErrors

	initializeComponents()

	t.Cleanup(func() {
		hbConfig.Mode = oldMode
		hbConfig.Development.FrontendErrors = oldFrontendErrors
		rm = oldRM

		configMutex.Lock()
		configs = oldConfigs
		configMutex.Unlock()

		htmlCacheMutex.Lock()
		htmlCache = oldHTMLCache
		htmlCacheMutex.Unlock()

		renderDiagnosticsMutex.Lock()
		renderDiagnostics = oldRenderDiagnostics
		renderDiagnosticsOrder = oldRenderDiagnosticsOrder
		renderDiagnosticsMutex.Unlock()
	})
}

func setTestRouteConfig(route string, config map[string]interface{}) {
	configMutex.Lock()
	defer configMutex.Unlock()
	configs[route] = config
}

func cachedEntry(route string) (CacheEntry, bool) {
	htmlCacheMutex.RLock()
	defer htmlCacheMutex.RUnlock()
	entry, ok := htmlCache[route]
	return entry, ok
}

func TestMakeStaticProvidesContextToPluginRender(t *testing.T) {
	setupDevelopmentModeServeContentTest(t, false)

	plugin := &staticContextTestPlugin{}
	rm.SetPlugin("static_context_test", plugin)

	renderDir := t.TempDir()
	err := makeStatic(map[string]map[string]interface{}{
		"plugin": {
			"@type":  component.PluginRenderGetName(),
			"route":  "plugin.html",
			"plugin": "static_context_test",
		},
	}, renderDir)
	if err != nil {
		t.Fatalf("makeStatic returned error: %v", err)
	}

	if plugin.ctx == nil {
		t.Fatal("plugin context is nil, want static render context")
	}
	if route, _ := plugin.ctx.Value(shared.CurrentRoute).(string); route != "plugin.html" {
		t.Fatalf("current route = %q, want plugin.html", route)
	}

	body, err := os.ReadFile(filepath.Join(renderDir, "plugin.html"))
	if err != nil {
		t.Fatalf("failed to read rendered static file: %v", err)
	}
	if string(body) != "static plugin output" {
		t.Fatalf("static file body = %q, want plugin output", string(body))
	}
}

type trackingReadCloser struct {
	readCalls int
}

func (r *trackingReadCloser) Read(p []byte) (int, error) {
	r.readCalls++
	return 0, io.EOF
}

func (r *trackingReadCloser) Close() error {
	return nil
}

func TestResolveNoCacheValue(t *testing.T) {
	tests := []struct {
		name  string
		input interface{}
		want  bool
	}{
		{name: "bool true", input: true, want: true},
		{name: "bool false", input: false, want: false},
		{name: "string true", input: "true", want: true},
		{name: "string false", input: "false", want: false},
		{name: "string uppercase false", input: "FALSE", want: false},
		{name: "string invalid", input: "not-a-bool", want: false},
		{name: "missing value", input: nil, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := resolveNoCacheValue(tt.input); got != tt.want {
				t.Fatalf("resolveNoCacheValue(%v) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestResolveConfiguredNoCache(t *testing.T) {
	tests := []struct {
		name   string
		config map[string]interface{}
		want   bool
	}{
		{
			name: "explicit nocache true",
			config: map[string]interface{}{
				"nocache": true,
			},
			want: true,
		},
		{
			name: "explicit nocache false",
			config: map[string]interface{}{
				"nocache": false,
			},
			want: false,
		},
		{
			name: "api fragment render is forced nocache",
			config: map[string]interface{}{
				"@type": composite.ApiFragmentRenderConfigGetName(),
			},
			want: true,
		},
		{
			name: "plain fragment remains cacheable by default",
			config: map[string]interface{}{
				"@type": composite.FragmentConfigGetName(),
			},
			want: false,
		},
		{
			name: "guarded hypermedia is forced nocache",
			config: map[string]interface{}{
				"@type": composite.HyperMediaConfigGetName(),
				"guard": map[string]interface{}{
					"enabled": true,
				},
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := resolveConfiguredNoCache(tt.config); got != tt.want {
				t.Fatalf("resolveConfiguredNoCache(%#v) = %v, want %v", tt.config, got, tt.want)
			}
		})
	}
}

func TestServeContent_LiveMode_DoesNotLeakRequestSensitiveContent(t *testing.T) {
	setupLiveModeServeContentTest(t)

	setTestRouteConfig("leak", map[string]interface{}{
		"@type": composite.FragmentConfigGetName(),
		"route": "leak",
		"template": map[string]interface{}{
			"@type":     composite.TemplateConfigGetName(),
			"inline":    `viewer={{index .Params "viewer"}}`,
			"querykeys": "viewer",
			"values": map[string]interface{}{
				"seed": "x",
			},
		},
	})

	firstWriter := httptest.NewRecorder()
	firstRequest := httptest.NewRequest(http.MethodGet, "/leak?viewer=alice", nil)
	ServeContent(firstWriter, firstRequest)

	secondWriter := httptest.NewRecorder()
	secondRequest := httptest.NewRequest(http.MethodGet, "/leak?viewer=bob", nil)
	ServeContent(secondWriter, secondRequest)

	firstBody := firstWriter.Body.String()
	secondBody := secondWriter.Body.String()

	if !strings.Contains(firstBody, "viewer=alice") {
		t.Fatalf("expected first response to contain alice, got %q", firstBody)
	}
	if !strings.Contains(secondBody, "viewer=bob") {
		t.Fatalf("expected second response to contain bob, got %q", secondBody)
	}
	if strings.Contains(secondBody, "viewer=alice") {
		t.Fatalf("expected second response not to leak alice, got %q", secondBody)
	}
}

func TestServeContent_LiveMode_StillCachesRequestInsensitiveRoute(t *testing.T) {
	setupLiveModeServeContentTest(t)

	setTestRouteConfig("static", map[string]interface{}{
		"@type": composite.FragmentConfigGetName(),
		"route": "static",
		"template": map[string]interface{}{
			"@type":  composite.TemplateConfigGetName(),
			"inline": `stable-content`,
			"values": map[string]interface{}{
				"seed": "x",
			},
		},
	})

	firstWriter := httptest.NewRecorder()
	firstRequest := httptest.NewRequest(http.MethodGet, "/static", nil)
	ServeContent(firstWriter, firstRequest)

	firstEntry, found := cachedEntry("static")
	if !found {
		t.Fatalf("expected static route to be cached after first request")
	}
	firstRenderedAt := firstWriter.Header().Get(liveCacheRenderedAtHeader)
	firstExpiresAt := firstWriter.Header().Get(liveCacheExpiresAtHeader)
	if firstRenderedAt == "" || firstExpiresAt == "" {
		t.Fatalf("expected cache metadata headers to be present on first response, got rendered=%q expires=%q", firstRenderedAt, firstExpiresAt)
	}

	secondWriter := httptest.NewRecorder()
	secondRequest := httptest.NewRequest(http.MethodGet, "/static", nil)
	ServeContent(secondWriter, secondRequest)

	secondEntry, found := cachedEntry("static")
	if !found {
		t.Fatalf("expected static route cache entry to remain present")
	}

	if !firstEntry.Timestamp.Equal(secondEntry.Timestamp) {
		t.Fatalf("expected second request to reuse cached entry, timestamps differ: %v vs %v", firstEntry.Timestamp, secondEntry.Timestamp)
	}
	if secondWriter.Header().Get(liveCacheRenderedAtHeader) != firstRenderedAt {
		t.Fatalf("expected cached response to reuse rendered-at header, got %q want %q", secondWriter.Header().Get(liveCacheRenderedAtHeader), firstRenderedAt)
	}
	if secondWriter.Header().Get(liveCacheExpiresAtHeader) != firstExpiresAt {
		t.Fatalf("expected cached response to reuse cache-expires header, got %q want %q", secondWriter.Header().Get(liveCacheExpiresAtHeader), firstExpiresAt)
	}

	if !strings.Contains(firstWriter.Body.String(), "stable-content") {
		t.Fatalf("expected first response to contain stable content, got %q", firstWriter.Body.String())
	}
	if !strings.Contains(secondWriter.Body.String(), "stable-content") {
		t.Fatalf("expected second response to contain stable content, got %q", secondWriter.Body.String())
	}
	if strings.Contains(firstWriter.Body.String(), "Rendered at") || strings.Contains(firstWriter.Body.String(), "Cache expires at") {
		t.Fatalf("expected first HTML response body not to include cache comments, got %q", firstWriter.Body.String())
	}
	if strings.Contains(secondWriter.Body.String(), "Rendered at") || strings.Contains(secondWriter.Body.String(), "Cache expires at") {
		t.Fatalf("expected cached HTML response body not to include cache comments, got %q", secondWriter.Body.String())
	}
}

func TestServeContent_LiveMode_UsesETagForNotModified(t *testing.T) {
	setupLiveModeServeContentTest(t)

	setTestRouteConfig("etag-static", map[string]interface{}{
		"@type": composite.FragmentConfigGetName(),
		"route": "etag-static",
		"template": map[string]interface{}{
			"@type":  composite.TemplateConfigGetName(),
			"inline": `etag-content`,
			"values": map[string]interface{}{
				"seed": "x",
			},
		},
	})

	firstWriter := httptest.NewRecorder()
	firstRequest := httptest.NewRequest(http.MethodGet, "/etag-static", nil)
	ServeContent(firstWriter, firstRequest)

	etag := firstWriter.Header().Get("ETag")
	if etag == "" {
		t.Fatalf("expected first cacheable response to include ETag")
	}
	if firstWriter.Body.String() != "etag-content" {
		t.Fatalf("first body = %q, want etag-content", firstWriter.Body.String())
	}

	notModifiedWriter := httptest.NewRecorder()
	notModifiedRequest := httptest.NewRequest(http.MethodGet, "/etag-static", nil)
	notModifiedRequest.Header.Set("If-None-Match", `"different", W/`+etag)
	ServeContent(notModifiedWriter, notModifiedRequest)

	if notModifiedWriter.Code != http.StatusNotModified {
		t.Fatalf("expected 304 for matching ETag, got %d", notModifiedWriter.Code)
	}
	if body := notModifiedWriter.Body.String(); body != "" {
		t.Fatalf("expected empty 304 body, got %q", body)
	}
	if got := notModifiedWriter.Header().Get("ETag"); got != etag {
		t.Fatalf("expected 304 to keep ETag %q, got %q", etag, got)
	}
	if got := notModifiedWriter.Header().Get(renderErrorCountHeader); got != "0" {
		t.Fatalf("expected render error count 0 on 304, got %q", got)
	}

	changedWriter := httptest.NewRecorder()
	changedRequest := httptest.NewRequest(http.MethodGet, "/etag-static", nil)
	changedRequest.Header.Set("If-None-Match", `"different"`)
	ServeContent(changedWriter, changedRequest)

	if changedWriter.Code != http.StatusOK {
		t.Fatalf("expected 200 for non-matching ETag, got %d", changedWriter.Code)
	}
	if changedWriter.Body.String() != "etag-content" {
		t.Fatalf("expected body on non-matching ETag, got %q", changedWriter.Body.String())
	}
}

func TestServeContent_LiveMode_HonorsExplicitNoCacheTrue(t *testing.T) {
	setupLiveModeServeContentTest(t)

	setTestRouteConfig("nocache-true", map[string]interface{}{
		"@type":   composite.FragmentConfigGetName(),
		"route":   "nocache-true",
		"nocache": "true",
		"template": map[string]interface{}{
			"@type":  composite.TemplateConfigGetName(),
			"inline": `uncached-content`,
			"values": map[string]interface{}{
				"seed": "x",
			},
		},
	})

	writer := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/nocache-true", nil)
	ServeContent(writer, request)

	if _, found := cachedEntry("nocache-true"); found {
		t.Fatalf("expected nocache=true route not to be stored in live cache")
	}
	if writer.Header().Get(liveCacheRenderedAtHeader) != "" || writer.Header().Get(liveCacheExpiresAtHeader) != "" {
		t.Fatalf("expected nocache=true response not to include cache metadata headers, got rendered=%q expires=%q", writer.Header().Get(liveCacheRenderedAtHeader), writer.Header().Get(liveCacheExpiresAtHeader))
	}
}

func TestServeContent_LiveMode_SkipsRequestSignatureForNoCacheRoute(t *testing.T) {
	setupLiveModeServeContentTest(t)

	setTestRouteConfig("nocache-body", map[string]interface{}{
		"@type":   composite.FragmentConfigGetName(),
		"route":   "nocache-body",
		"nocache": true,
		"template": map[string]interface{}{
			"@type":  composite.TemplateConfigGetName(),
			"inline": `uncached-body`,
			"values": map[string]interface{}{
				"seed": "x",
			},
		},
	})

	body := &trackingReadCloser{}
	request := httptest.NewRequest(http.MethodGet, "/nocache-body", nil)
	request.Body = body

	writer := httptest.NewRecorder()
	ServeContent(writer, request)

	if body.readCalls != 0 {
		t.Fatalf("expected nocache route to skip request-body cache signature reads, got %d reads", body.readCalls)
	}
	if _, found := cachedEntry("nocache-body"); found {
		t.Fatalf("expected nocache route to remain out of live cache")
	}
	if !strings.Contains(writer.Body.String(), "uncached-body") {
		t.Fatalf("expected nocache route to render content, got %q", writer.Body.String())
	}
}

func TestServeContent_LiveMode_HonorsExplicitNoCacheFalse(t *testing.T) {
	setupLiveModeServeContentTest(t)

	setTestRouteConfig("nocache-false", map[string]interface{}{
		"@type":   composite.FragmentConfigGetName(),
		"route":   "nocache-false",
		"nocache": "false",
		"template": map[string]interface{}{
			"@type":  composite.TemplateConfigGetName(),
			"inline": `cached-content`,
			"values": map[string]interface{}{
				"seed": "x",
			},
		},
	})

	writer := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/nocache-false", nil)
	ServeContent(writer, request)

	if _, found := cachedEntry("nocache-false"); !found {
		t.Fatalf("expected nocache=false route to remain cacheable")
	}
	if writer.Header().Get(liveCacheRenderedAtHeader) == "" || writer.Header().Get(liveCacheExpiresAtHeader) == "" {
		t.Fatalf("expected nocache=false response to include cache metadata headers, got rendered=%q expires=%q", writer.Header().Get(liveCacheRenderedAtHeader), writer.Header().Get(liveCacheExpiresAtHeader))
	}
}

func TestServeContent_LiveMode_DoesNotAppendHTMLCacheCommentsToJSON(t *testing.T) {
	setupLiveModeServeContentTest(t)

	setTestRouteConfig("json", map[string]interface{}{
		"@type":        composite.FragmentConfigGetName(),
		"route":        "json",
		"content_type": "application/json",
		"template": map[string]interface{}{
			"@type":  composite.TemplateConfigGetName(),
			"inline": `{"status":"ok"}`,
			"values": map[string]interface{}{
				"seed": "x",
			},
		},
	})

	writer := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/json", nil)
	ServeContent(writer, request)

	body := writer.Body.Bytes()
	if !json.Valid(body) {
		t.Fatalf("expected live-mode JSON response to remain valid JSON, got %q", string(body))
	}
	if writer.Header().Get(liveCacheRenderedAtHeader) == "" || writer.Header().Get(liveCacheExpiresAtHeader) == "" {
		t.Fatalf("expected cache metadata headers on JSON response, got rendered=%q expires=%q", writer.Header().Get(liveCacheRenderedAtHeader), writer.Header().Get(liveCacheExpiresAtHeader))
	}
	if strings.Contains(string(body), "Rendered at") || strings.Contains(string(body), "Cache expires at") {
		t.Fatalf("expected live-mode JSON response not to include HTML cache comments, got %q", string(body))
	}

	entry, found := cachedEntry("json")
	if !found {
		t.Fatalf("expected json route to be cached")
	}
	if !json.Valid([]byte(entry.Content)) {
		t.Fatalf("expected cached JSON body to remain valid JSON, got %q", entry.Content)
	}
	if entry.Headers[liveCacheRenderedAtHeader] == "" || entry.Headers[liveCacheExpiresAtHeader] == "" {
		t.Fatalf("expected cached JSON entry to include cache metadata headers, got %#v", entry.Headers)
	}
	if strings.Contains(entry.Content, "Rendered at") || strings.Contains(entry.Content, "Cache expires at") {
		t.Fatalf("expected cached JSON body not to include HTML cache comments, got %q", entry.Content)
	}
}

func TestServeContent_HyperMediaGuardRedirectsUnauthenticated(t *testing.T) {
	setupLiveModeServeContentTest(t)

	setTestRouteConfig("guarded", map[string]interface{}{
		"@type": composite.HyperMediaConfigGetName(),
		"route": "guarded",
		"guard": map[string]interface{}{
			"enabled": true,
			"auth": map[string]interface{}{
				"cookie": "token",
			},
			"require": map[string]interface{}{
				"authenticated": true,
			},
			"on_unauthenticated": map[string]interface{}{
				"redirect": "/login",
			},
		},
		"template": map[string]interface{}{
			"@type":  composite.TemplateConfigGetName(),
			"inline": `protected`,
		},
	})

	writer := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/guarded", nil)
	ServeContent(writer, request)

	if writer.Code != http.StatusSeeOther {
		t.Fatalf("expected 303 redirect, got %d", writer.Code)
	}
	if got := writer.Header().Get("Location"); got != "/login" {
		t.Fatalf("expected Location /login, got %q", got)
	}
	if strings.Contains(writer.Body.String(), "protected") {
		t.Fatalf("expected protected content not to render, got %q", writer.Body.String())
	}
	if _, found := cachedEntry("guarded"); found {
		t.Fatalf("expected guarded route to bypass live cache")
	}
}

func TestServeContent_HyperMediaGuardUsesHxRedirectForHTMX(t *testing.T) {
	setupLiveModeServeContentTest(t)

	setTestRouteConfig("guarded-hx", map[string]interface{}{
		"@type": composite.HyperMediaConfigGetName(),
		"route": "guarded-hx",
		"guard": map[string]interface{}{
			"enabled": true,
			"auth": map[string]interface{}{
				"cookie": "token",
			},
			"require": map[string]interface{}{
				"authenticated": true,
			},
			"on_unauthenticated": map[string]interface{}{
				"redirect": "/login",
			},
		},
		"template": map[string]interface{}{
			"@type":  composite.TemplateConfigGetName(),
			"inline": `protected`,
		},
	})

	writer := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/guarded-hx", nil)
	request.Header.Set("HX-Request", "true")
	ServeContent(writer, request)

	if writer.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for HTMX guard response, got %d", writer.Code)
	}
	if got := writer.Header().Get("HX-Redirect"); got != "/login" {
		t.Fatalf("expected HX-Redirect /login, got %q", got)
	}
	if got := writer.Header().Get("Location"); got != "" {
		t.Fatalf("expected no Location header for HTMX redirect, got %q", got)
	}
}

func TestServeContent_FragmentGuardDeniesBeforeRender(t *testing.T) {
	setupLiveModeServeContentTest(t)

	setTestRouteConfig("guarded-fragment", map[string]interface{}{
		"@type": composite.FragmentConfigGetName(),
		"route": "guarded-fragment",
		"guard": map[string]interface{}{
			"enabled": true,
			"auth": map[string]interface{}{
				"cookie": "token",
			},
			"require": map[string]interface{}{
				"authenticated": true,
			},
			"on_unauthenticated": map[string]interface{}{
				"redirect": "/login",
			},
		},
		"10": map[string]interface{}{
			"@type": component.HTMLConfigGetName(),
			"value": "protected-fragment",
		},
	})

	writer := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/guarded-fragment", nil)
	ServeContent(writer, request)

	if writer.Code != http.StatusSeeOther {
		t.Fatalf("expected 303 redirect, got %d", writer.Code)
	}
	if got := writer.Header().Get("Location"); got != "/login" {
		t.Fatalf("expected Location /login, got %q", got)
	}
	if strings.Contains(writer.Body.String(), "protected-fragment") {
		t.Fatalf("expected guarded fragment content not to render, got %q", writer.Body.String())
	}
	if _, found := cachedEntry("guarded-fragment"); found {
		t.Fatalf("expected guarded fragment route to bypass live cache")
	}
}

func TestServeContent_FragmentGuardUsesHxRedirectForHTMX(t *testing.T) {
	setupLiveModeServeContentTest(t)

	setTestRouteConfig("guarded-fragment-hx", map[string]interface{}{
		"@type": composite.FragmentConfigGetName(),
		"route": "guarded-fragment-hx",
		"guard": map[string]interface{}{
			"enabled": true,
			"auth": map[string]interface{}{
				"cookie": "token",
			},
			"require": map[string]interface{}{
				"authenticated": true,
			},
			"on_unauthenticated": map[string]interface{}{
				"redirect": "/login",
			},
		},
		"10": map[string]interface{}{
			"@type": component.HTMLConfigGetName(),
			"value": "protected-fragment",
		},
	})

	writer := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/guarded-fragment-hx", nil)
	request.Header.Set("HX-Request", "true")
	ServeContent(writer, request)

	if writer.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for HTMX fragment guard response, got %d", writer.Code)
	}
	if got := writer.Header().Get("HX-Redirect"); got != "/login" {
		t.Fatalf("expected HX-Redirect /login, got %q", got)
	}
	if strings.Contains(writer.Body.String(), "protected-fragment") {
		t.Fatalf("expected guarded fragment content not to render, got %q", writer.Body.String())
	}
}

func TestServeContent_APIFragmentGuardDeniesBeforeUpstreamCall(t *testing.T) {
	setupLiveModeServeContentTest(t)

	called := 0
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called++
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer upstream.Close()

	setTestRouteConfig("guarded-api-fragment", map[string]interface{}{
		"@type":    composite.ApiFragmentRenderConfigGetName(),
		"route":    "guarded-api-fragment",
		"method":   "GET",
		"endpoint": upstream.URL,
		"inline":   `denied`,
		"guard": map[string]interface{}{
			"enabled": true,
			"auth": map[string]interface{}{
				"cookie": "token",
			},
			"require": map[string]interface{}{
				"authenticated": true,
			},
			"on_unauthenticated": map[string]interface{}{
				"redirect": "/login",
			},
		},
	})

	writer := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/guarded-api-fragment", nil)
	ServeContent(writer, request)

	if writer.Code != http.StatusSeeOther {
		t.Fatalf("expected 303 redirect, got %d", writer.Code)
	}
	if called != 0 {
		t.Fatalf("expected denied API fragment guard not to call upstream, got %d calls", called)
	}
}

func TestServeContent_APIFragmentGuardAllowsAuthorizedRequest(t *testing.T) {
	setupLiveModeServeContentTest(t)

	called := 0
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called++
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"message":"allowed"}`))
	}))
	defer upstream.Close()

	setTestRouteConfig("guarded-api-fragment-allowed", map[string]interface{}{
		"@type":    composite.ApiFragmentRenderConfigGetName(),
		"route":    "guarded-api-fragment-allowed",
		"method":   "GET",
		"endpoint": upstream.URL,
		"inline":   `{{ index .Data "message" }}`,
		"guard": map[string]interface{}{
			"enabled": true,
			"auth": map[string]interface{}{
				"cookie": "token",
			},
			"require": map[string]interface{}{
				"authenticated": true,
			},
		},
	})

	writer := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/guarded-api-fragment-allowed", nil)
	request.AddCookie(&http.Cookie{Name: "token", Value: "test-token"})
	ServeContent(writer, request)

	if writer.Code != http.StatusOK {
		t.Fatalf("expected 200 for authorized API fragment guard request, got %d", writer.Code)
	}
	if called != 1 {
		t.Fatalf("expected authorized API fragment guard to call upstream once, got %d calls", called)
	}
	if !strings.Contains(writer.Body.String(), "allowed") {
		t.Fatalf("expected authorized API fragment content to render, got %q", writer.Body.String())
	}
}

func TestServeContent_APIFragmentRenderSetCookieOnNoContentResponse(t *testing.T) {
	setupLiveModeServeContentTest(t)

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer upstream.Close()

	setTestRouteConfig("api-fragment-logout", map[string]interface{}{
		"@type":     composite.ApiFragmentRenderConfigGetName(),
		"route":     "api-fragment-logout",
		"method":    "POST",
		"endpoint":  upstream.URL,
		"inline":    `logged-out`,
		"setcookie": `token=; Path=/; HttpOnly; Max-Age=0`,
	})

	writer := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api-fragment-logout", nil)
	ServeContent(writer, request)

	if writer.Code != http.StatusOK {
		t.Fatalf("expected served fragment response to remain 200, got %d", writer.Code)
	}
	if got := writer.Header().Values("Set-Cookie"); len(got) != 1 || got[0] != "token=; Path=/; HttpOnly; Max-Age=0" {
		t.Fatalf("expected single logout cookie on 204 upstream response, got %v", got)
	}
}

func TestServeContent_APIFragmentRenderSetCookiesAddsMultipleHeaders(t *testing.T) {
	setupLiveModeServeContentTest(t)

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer upstream.Close()

	setTestRouteConfig("api-fragment-logout-all", map[string]interface{}{
		"@type":      composite.ApiFragmentRenderConfigGetName(),
		"route":      "api-fragment-logout-all",
		"method":     "POST",
		"endpoint":   upstream.URL,
		"inline":     `logged-out`,
		"setcookie":  `token=; Path=/; HttpOnly; Max-Age=0`,
		"setcookies": []interface{}{`runtime_session=; Path=/; HttpOnly; Max-Age=0`, `theme=light; Path=/; Max-Age=300`},
	})

	writer := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api-fragment-logout-all", nil)
	ServeContent(writer, request)

	if writer.Code != http.StatusOK {
		t.Fatalf("expected served fragment response to remain 200, got %d", writer.Code)
	}
	got := writer.Header().Values("Set-Cookie")
	want := []string{
		"token=; Path=/; HttpOnly; Max-Age=0",
		"runtime_session=; Path=/; HttpOnly; Max-Age=0",
		"theme=light; Path=/; Max-Age=300",
	}
	if len(got) != len(want) {
		t.Fatalf("expected %d Set-Cookie headers, got %d: %v", len(want), len(got), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("expected Set-Cookie[%d] = %q, got %q", i, want[i], got[i])
		}
	}
}

func TestServeContent_HyperMediaGuardAuthorizesBeforeRender(t *testing.T) {
	setupLiveModeServeContentTest(t)

	authz := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer test-token" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		body, _ := io.ReadAll(r.Body)
		if !strings.Contains(string(body), `"project_slug":"alpha"`) {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer authz.Close()

	setTestRouteConfig("guarded-authz", map[string]interface{}{
		"@type": composite.HyperMediaConfigGetName(),
		"route": "guarded-authz",
		"guard": map[string]interface{}{
			"enabled": true,
			"auth": map[string]interface{}{
				"cookie": "token",
			},
			"require": map[string]interface{}{
				"authenticated": true,
				"query": map[string]interface{}{
					"project": true,
				},
			},
			"authorize": map[string]interface{}{
				"endpoint": authz.URL,
				"method":   "POST",
				"body":     `{"project_slug":"$project"}`,
			},
			"on_unauthenticated": map[string]interface{}{
				"redirect": "/login",
			},
			"on_forbidden": map[string]interface{}{
				"redirect": "/forbidden",
			},
		},
		"template": map[string]interface{}{
			"@type":  composite.TemplateConfigGetName(),
			"inline": `authorized`,
		},
	})

	request := httptest.NewRequest(http.MethodGet, "/guarded-authz?project=alpha", nil)
	request.AddCookie(&http.Cookie{Name: "token", Value: "test-token"})
	writer := httptest.NewRecorder()
	ServeContent(writer, request)

	if writer.Code != http.StatusOK {
		t.Fatalf("expected 200 for authorized request, got %d", writer.Code)
	}
	if !strings.Contains(writer.Body.String(), "authorized") {
		t.Fatalf("expected protected content to render, got %q", writer.Body.String())
	}

	forbiddenWriter := httptest.NewRecorder()
	forbiddenRequest := httptest.NewRequest(http.MethodGet, "/guarded-authz", nil)
	forbiddenRequest.AddCookie(&http.Cookie{Name: "token", Value: "test-token"})
	ServeContent(forbiddenWriter, forbiddenRequest)

	if forbiddenWriter.Code != http.StatusSeeOther {
		t.Fatalf("expected 303 for missing required query key, got %d", forbiddenWriter.Code)
	}
	if got := forbiddenWriter.Header().Get("Location"); got != "/forbidden" {
		t.Fatalf("expected Location /forbidden, got %q", got)
	}
}

func TestServeContent_DevelopmentLeavesBodyCleanWhenNoRenderErrors(t *testing.T) {
	setupDevelopmentModeServeContentTest(t, false)

	setTestRouteConfig("clean-dev", map[string]interface{}{
		"@type": composite.HyperMediaConfigGetName(),
		"route": "clean-dev",
		"10": map[string]interface{}{
			"@type": component.HTMLConfigGetName(),
			"value": "<main>clean output</main>",
		},
	})

	writer := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/clean-dev", nil)
	ServeContent(writer, request)

	if writer.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", writer.Code)
	}
	if got := writer.Body.String(); !strings.Contains(got, "<main>clean output</main>") {
		t.Fatalf("expected clean rendered content in body, got %q", got)
	}
	if got, want := writer.Header().Get("Content-Length"), strconv.Itoa(writer.Body.Len()); got != want {
		t.Fatalf("expected Content-Length %q, got %q", want, got)
	}
	if got := writer.Header().Get(renderErrorCountHeader); got != "0" {
		t.Fatalf("expected render error count 0, got %q", got)
	}
	if got := writer.Header().Get(requestIDHeader); got == "" {
		t.Fatalf("expected request id header to be set")
	}
}

func TestServeContent_DevelopmentHandledPluginResponseWritesRawBody(t *testing.T) {
	setupDevelopmentModeServeContentTest(t, false)

	setTestRouteConfig("handled-plugin", map[string]interface{}{
		"@type":  component.PluginRenderGetName(),
		"route":  "handled-plugin",
		"plugin": "handled_test",
	})

	var calls int32
	rm.SetPlugin("handled_test", handledResponseTestPlugin{
		calls:       &calls,
		status:      http.StatusCreated,
		contentType: "application/octet-stream",
		headers: map[string]string{
			"X-Handled-Test": "1",
		},
		cookies: []string{"runtime_gateway=1; Path=/; HttpOnly"},
		body:    []byte{0x00, 0x41, 0x42, 0x43},
	})

	writer := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/handled-plugin", nil)
	ServeContent(writer, request)

	response := writer.Result()
	if response.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, want %d", response.StatusCode, http.StatusCreated)
	}
	if got := response.Header.Get("Content-Type"); got != "application/octet-stream" {
		t.Fatalf("content type = %q, want application/octet-stream", got)
	}
	if got := response.Header.Get("Content-Length"); got != "4" {
		t.Fatalf("content length = %q, want 4", got)
	}
	if got := response.Header.Get("X-Handled-Test"); got != "1" {
		t.Fatalf("X-Handled-Test = %q, want 1", got)
	}
	if cookies := response.Header.Values("Set-Cookie"); len(cookies) != 1 || cookies[0] != "runtime_gateway=1; Path=/; HttpOnly" {
		t.Fatalf("Set-Cookie = %v, want [runtime_gateway=1; Path=/; HttpOnly]", cookies)
	}
	if got := response.Header.Get(requestIDHeader); got == "" {
		t.Fatal("missing request id header")
	}
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatalf("ReadAll body: %v", err)
	}
	if want := []byte{0x00, 0x41, 0x42, 0x43}; string(body) != string(want) {
		t.Fatalf("body = %v, want %v", body, want)
	}
	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Fatalf("plugin calls = %d, want 1", got)
	}
}

func TestServeContent_DevelopmentHandledNestedPluginResponseWritesRawBody(t *testing.T) {
	setupDevelopmentModeServeContentTest(t, false)

	setTestRouteConfig("handled-nested-plugin", map[string]interface{}{
		"@type": composite.FragmentConfigGetName(),
		"route": "handled-nested-plugin",
		"10": map[string]interface{}{
			"@type":  component.PluginRenderGetName(),
			"plugin": "handled_nested_test",
		},
	})

	var calls int32
	rm.SetPlugin("handled_nested_test", handledResponseTestPlugin{
		calls:       &calls,
		status:      http.StatusAccepted,
		contentType: "application/octet-stream",
		body:        []byte{0x52, 0x54},
	})

	writer := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/handled-nested-plugin", nil)
	handler(writer, request)

	response := writer.Result()
	if response.StatusCode != http.StatusAccepted {
		t.Fatalf("status = %d, want %d", response.StatusCode, http.StatusAccepted)
	}
	if got := response.Header.Get(renderErrorCountHeader); got != "0" {
		t.Fatalf("render error count = %q, want 0", got)
	}
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatalf("ReadAll body: %v", err)
	}
	if want := []byte{0x52, 0x54}; string(body) != string(want) {
		t.Fatalf("body = %v, want %v", body, want)
	}
	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Fatalf("plugin calls = %d, want 1", got)
	}
}

func TestServeContent_LiveModeHandledPluginResponseBypassesCache(t *testing.T) {
	setupLiveModeServeContentTest(t)

	setTestRouteConfig("handled-live", map[string]interface{}{
		"@type":  component.PluginRenderGetName(),
		"route":  "handled-live",
		"plugin": "handled_live_test",
	})

	var calls int32
	rm.SetPlugin("handled_live_test", handledResponseTestPlugin{
		calls:       &calls,
		status:      http.StatusOK,
		contentType: "text/plain; charset=utf-8",
		body:        []byte("runtime-proxy"),
	})

	firstWriter := httptest.NewRecorder()
	firstRequest := httptest.NewRequest(http.MethodGet, "/handled-live", nil)
	ServeContent(firstWriter, firstRequest)

	secondWriter := httptest.NewRecorder()
	secondRequest := httptest.NewRequest(http.MethodGet, "/handled-live", nil)
	ServeContent(secondWriter, secondRequest)

	if got := atomic.LoadInt32(&calls); got != 2 {
		t.Fatalf("plugin calls = %d, want 2", got)
	}
	if _, ok := cachedEntry("handled-live"); ok {
		t.Fatal("handled live response should not be cached")
	}
	if body := secondWriter.Body.String(); body != "runtime-proxy" {
		t.Fatalf("second body = %q, want runtime-proxy", body)
	}
}

func TestServeContent_DevelopmentRecordsDiagnosticsSeparately(t *testing.T) {
	setupDevelopmentModeServeContentTest(t, false)

	setTestRouteConfig("missing-plugin", map[string]interface{}{
		"@type":  component.PluginRenderGetName(),
		"route":  "missing-plugin",
		"plugin": "DoesNotExist@1.0.0",
	})

	writer := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/missing-plugin", nil)
	ServeContent(writer, request)

	if writer.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", writer.Code)
	}
	requestID := writer.Header().Get(requestIDHeader)
	if requestID == "" {
		t.Fatalf("expected request id header to be set")
	}
	if got := writer.Header().Get(renderErrorCountHeader); got == "0" || got == "" {
		t.Fatalf("expected non-zero render error count, got %q", got)
	}
	if strings.Contains(writer.Body.String(), "<!-- Error:") {
		t.Fatalf("expected body not to contain appended render diagnostics, got %q", writer.Body.String())
	}

	renderDiagnosticsMutex.RLock()
	diagnostics, ok := renderDiagnostics[requestID]
	renderDiagnosticsMutex.RUnlock()
	if !ok {
		t.Fatalf("expected diagnostics to be recorded for request %q", requestID)
	}
	if diagnostics.Route != "missing-plugin" {
		t.Fatalf("expected diagnostics route missing-plugin, got %q", diagnostics.Route)
	}
	if len(diagnostics.Errors) == 0 {
		t.Fatalf("expected recorded diagnostics, got %d", len(diagnostics.Errors))
	}
}

func TestRenderDiagnosticsEndpointReturnsRecordedRequest(t *testing.T) {
	setupDevelopmentModeServeContentTest(t, false)

	setTestRouteConfig("missing-plugin", map[string]interface{}{
		"@type":  component.PluginRenderGetName(),
		"route":  "missing-plugin",
		"plugin": "DoesNotExist@1.0.0",
	})

	sourceWriter := httptest.NewRecorder()
	sourceRequest := httptest.NewRequest(http.MethodGet, "/missing-plugin", nil)
	handler(sourceWriter, sourceRequest)

	requestID := sourceWriter.Header().Get(requestIDHeader)
	if requestID == "" {
		t.Fatal("expected request id header to be set")
	}

	diagnosticsWriter := httptest.NewRecorder()
	diagnosticsRequest := httptest.NewRequest(http.MethodGet, "/__hyperbricks/render-diagnostics?request_id="+requestID, nil)
	handler(diagnosticsWriter, diagnosticsRequest)

	if diagnosticsWriter.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", diagnosticsWriter.Code)
	}
	if got := diagnosticsWriter.Header().Get("Content-Type"); !strings.HasPrefix(got, "application/json") {
		t.Fatalf("content type = %q, want application/json", got)
	}

	var payload RenderDiagnostics
	if err := json.Unmarshal(diagnosticsWriter.Body.Bytes(), &payload); err != nil {
		t.Fatalf("Unmarshal diagnostics payload: %v", err)
	}
	if payload.RequestID != requestID {
		t.Fatalf("request id = %q, want %q", payload.RequestID, requestID)
	}
	if payload.Route != "missing-plugin" {
		t.Fatalf("route = %q, want missing-plugin", payload.Route)
	}
	if len(payload.Errors) == 0 {
		t.Fatal("expected at least one recorded error")
	}
}
