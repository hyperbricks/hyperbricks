package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/hyperbricks/hyperbricks/pkg/component"
	"github.com/hyperbricks/hyperbricks/pkg/composite"
	"github.com/hyperbricks/hyperbricks/pkg/shared"
)

// The sequence makes a reused response observable without relying on cache keys.
type liveCacheMatrixPlugin struct{ calls atomic.Int32 }

func (p *liveCacheMatrixPlugin) Render(_ interface{}, ctx context.Context) (any, []error) {
	r := ctx.Value(shared.Request).(*http.Request)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return "", []error{err}
	}
	content, err := json.Marshal(map[string]interface{}{
		"render": p.calls.Add(1), "method": r.Method, "query": r.URL.Query().Encode(),
		"authorization": r.Header.Get("Authorization"), "cookie": r.Header.Get("Cookie"),
		"body": string(body), "language": r.Header.Get("Accept-Language"),
		"hx_request": r.Header.Get("HX-Request"), "host": r.Host,
	})
	if err != nil {
		return "", []error{err}
	}
	return string(content), nil
}

func liveCacheMatrixRoute(route, plugin string) map[string]interface{} {
	return map[string]interface{}{
		"@type": composite.FragmentConfigGetName(), "route": route,
		"content_type": "application/json", "beautify": false,
		"10": map[string]interface{}{"@type": component.PluginRenderGetName(), "plugin": plugin},
	}
}

func liveCacheMatrixRequest(method, path, body string, headers http.Header) *http.Request {
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	if headers != nil {
		r.Header = headers.Clone()
	}
	return r
}

func serveLiveCacheMatrix(r *http.Request) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	ServeContent(w, r)
	return w
}

func TestServeContent_LiveCacheMixedPolicies(t *testing.T) {
	setupLiveModeServeContentTest(t)
	policies := []struct {
		route        string
		noCache      bool
		cacheControl string
		vary         string
		wantCached   bool
	}{
		{route: "public", wantCached: true},
		{route: "dynamic", noCache: true},
		{route: "browser-no-store", cacheControl: "no-store", wantCached: true},
		{route: "private", noCache: true, cacheControl: "private, no-store"},
		{route: "vary-star", vary: "*"},
	}
	plugins := make(map[string]*liveCacheMatrixPlugin)
	firstBodies := make(map[string]string)
	for _, policy := range policies {
		plugin := &liveCacheMatrixPlugin{}
		plugins[policy.route] = plugin
		rm.SetPlugin(policy.route, plugin)
		config := liveCacheMatrixRoute(policy.route, policy.route)
		config["nocache"] = policy.noCache
		headers := map[string]interface{}{}
		if policy.cacheControl != "" {
			headers["Cache-Control"] = policy.cacheControl
		}
		if policy.vary != "" {
			headers["Vary"] = policy.vary
		}
		config["response"] = map[string]interface{}{"headers": headers}
		setTestRouteConfig(policy.route, config)
	}

	// Interleave routes to verify their policies coexist in one live application.
	for attempt := 0; attempt < 2; attempt++ {
		for _, policy := range policies {
			w := serveLiveCacheMatrix(httptest.NewRequest(http.MethodGet, "/"+policy.route, nil))
			if w.Code != http.StatusOK || !json.Valid(w.Body.Bytes()) {
				t.Fatalf("%s: status=%d body=%q", policy.route, w.Code, w.Body.String())
			}
			if got := w.Header().Get("Cache-Control"); got != policy.cacheControl {
				t.Errorf("%s: Cache-Control=%q, want %q", policy.route, got, policy.cacheControl)
			}
			if got := w.Header().Get(liveCacheRenderedAtHeader) != ""; got != policy.wantCached {
				t.Errorf("%s: cache metadata present=%t, want %t", policy.route, got, policy.wantCached)
			}
			if attempt == 0 {
				firstBodies[policy.route] = w.Body.String()
			} else if same := w.Body.String() == firstBodies[policy.route]; same != policy.wantCached {
				t.Errorf("%s: response reused=%t, want %t", policy.route, same, policy.wantCached)
			}
		}
	}
	for _, policy := range policies {
		wantCalls := int32(2)
		if policy.wantCached {
			wantCalls = 1
		}
		if calls := plugins[policy.route].calls.Load(); calls != wantCalls {
			t.Errorf("%s: renderer calls=%d, want %d", policy.route, calls, wantCalls)
		}
	}

	// Age the entry rather than sleep, then verify expiry triggers fresh output.
	htmlCacheMutex.Lock()
	entry := htmlCache["public"]
	entry.Timestamp = time.Now().Add(-2 * time.Hour)
	htmlCache["public"] = entry
	htmlCacheMutex.Unlock()
	fresh := serveLiveCacheMatrix(httptest.NewRequest(http.MethodGet, "/public", nil))
	reused := serveLiveCacheMatrix(httptest.NewRequest(http.MethodGet, "/public", nil))
	if fresh.Code != http.StatusOK || fresh.Body.String() == firstBodies["public"] || reused.Body.String() != fresh.Body.String() || plugins["public"].calls.Load() != 2 {
		t.Fatalf("expired public response was not refreshed once: fresh=%q reused=%q calls=%d", fresh.Body.String(), reused.Body.String(), plugins["public"].calls.Load())
	}
}

func TestServeContent_LiveCacheRequestVariantMatrix(t *testing.T) {
	type requestInput struct {
		method, target, body string
		headers              http.Header
	}
	request := func(input requestInput) *http.Request {
		if input.method == "" {
			input.method = http.MethodGet
		}
		if input.target == "" {
			input.target = "/variants"
		}
		return liveCacheMatrixRequest(input.method, input.target, input.body, input.headers)
	}
	for _, tc := range []struct {
		name, vary string
		first      requestInput
		second     requestInput
		wantValue  string
	}{
		{
			name: "query", first: requestInput{target: "/variants?viewer=alice"},
			second: requestInput{target: "/variants?viewer=bob"}, wantValue: `"query":"viewer=bob"`,
		},
		{
			name: "authorization", first: requestInput{headers: http.Header{"Authorization": {"Bearer alice"}}},
			second: requestInput{headers: http.Header{"Authorization": {"Bearer bob"}}}, wantValue: `"authorization":"Bearer bob"`,
		},
		{
			name: "cookie", first: requestInput{headers: http.Header{"Cookie": {"session=alice"}}},
			second: requestInput{headers: http.Header{"Cookie": {"session=bob"}}}, wantValue: `"cookie":"session=bob"`,
		},
		{
			name: "body", first: requestInput{method: http.MethodPost, body: "alice"},
			second: requestInput{method: http.MethodPost, body: "bob"}, wantValue: `"body":"bob"`,
		},
		{
			name: "method", first: requestInput{method: http.MethodPost, body: "same"},
			second: requestInput{method: http.MethodPut, body: "same"}, wantValue: `"method":"PUT"`,
		},
		{
			name: "language", vary: "Accept-Language", first: requestInput{headers: http.Header{"Accept-Language": {"en"}}},
			second: requestInput{headers: http.Header{"Accept-Language": {"nl"}}}, wantValue: `"language":"nl"`,
		},
		{
			name: "htmx", vary: "HX-Request", first: requestInput{},
			second: requestInput{headers: http.Header{"Hx-Request": {"true"}}}, wantValue: `"hx_request":"true"`,
		},
		{
			name: "host", vary: "Host", first: requestInput{target: "http://one.example/variants"},
			second: requestInput{target: "http://two.example/variants"}, wantValue: `"host":"two.example"`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			setupLiveModeServeContentTest(t)
			plugin := &liveCacheMatrixPlugin{}
			rm.SetPlugin("variants", plugin)
			config := liveCacheMatrixRoute("variants", "variants")
			if tc.vary != "" {
				config["response"] = map[string]interface{}{"headers": map[string]interface{}{"Vary": tc.vary}}
			}
			setTestRouteConfig("variants", config)
			first := serveLiveCacheMatrix(request(tc.first))
			second := serveLiveCacheMatrix(request(tc.second))
			repeated := serveLiveCacheMatrix(request(tc.first))
			if first.Code != http.StatusOK || second.Code != http.StatusOK || repeated.Code != http.StatusOK || !strings.Contains(second.Body.String(), tc.wantValue) {
				t.Fatalf("wrong variant response: first=%d/%q second=%d/%q repeated=%d/%q", first.Code, first.Body.String(), second.Code, second.Body.String(), repeated.Code, repeated.Body.String())
			}
			if first.Body.String() == second.Body.String() || first.Body.String() != repeated.Body.String() || plugin.calls.Load() != 2 {
				t.Fatalf("variant isolation/reuse failed: first=%q second=%q repeated=%q calls=%d", first.Body.String(), second.Body.String(), repeated.Body.String(), plugin.calls.Load())
			}
		})
	}
}

func TestServeContent_LiveCacheDynamicOwnersAndRecovery(t *testing.T) {
	setupLiveModeServeContentTest(t)
	var authorized atomic.Bool
	authorized.Store(true)
	var authCalls, apiCalls, streamCalls atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/authorize" {
			authCalls.Add(1)
			if !authorized.Load() {
				w.WriteHeader(http.StatusForbidden)
				return
			}
			w.WriteHeader(http.StatusNoContent)
			return
		}
		call := apiCalls.Add(1)
		if call == 1 {
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"call":%d}`, call)
	}))
	defer upstream.Close()

	public := &liveCacheMatrixPlugin{}
	rm.SetPlugin("public-mixed", public)
	setTestRouteConfig("public-mixed", liveCacheMatrixRoute("public-mixed", "public-mixed"))
	guarded := &liveCacheMatrixPlugin{}
	rm.SetPlugin("guarded-mixed", guarded)
	guardConfig := liveCacheMatrixRoute("guarded-mixed", "guarded-mixed")
	guardConfig["guard"] = map[string]interface{}{
		"enabled": true, "require": map[string]interface{}{"authenticated": true},
		"authorize": map[string]interface{}{"endpoint": upstream.URL + "/authorize", "method": "GET"},
	}
	setTestRouteConfig("guarded-mixed", guardConfig)
	setTestRouteConfig("api-mixed", map[string]interface{}{
		"@type": composite.ApiFragmentRenderConfigGetName(), "route": "api-mixed",
		"endpoint": upstream.URL + "/api", "method": "GET", "inline": `call={{.Data.call}}`,
	})
	stream := &nativeStreamTestPlugin{response: func(context.Context) shared.HandledResponse {
		return shared.HandledResponse{ContentType: "text/plain", Stream: func(_ context.Context, w io.Writer, flush func() error) error {
			fmt.Fprintf(w, "stream=%d", streamCalls.Add(1))
			return flush()
		}}
	}}
	rm.SetPlugin("stream-mixed", stream)
	setTestRouteConfig("stream-mixed", liveCacheMatrixRoute("stream-mixed", "stream-mixed"))

	for attempt := 1; attempt <= 3; attempt++ {
		if attempt == 3 {
			authorized.Store(false)
		}
		publicResponse := serveLiveCacheMatrix(httptest.NewRequest(http.MethodGet, "/public-mixed", nil))
		if publicResponse.Code != http.StatusOK || public.calls.Load() != 1 {
			t.Fatalf("public route stopped reusing its response: %d/%q calls=%d", publicResponse.Code, publicResponse.Body.String(), public.calls.Load())
		}
		guardResponse := serveLiveCacheMatrix(liveCacheMatrixRequest("GET", "/guarded-mixed", "", http.Header{"Authorization": {"Bearer same-token"}}))
		wantStatus := http.StatusOK
		if attempt == 3 {
			wantStatus = http.StatusForbidden
		}
		if guardResponse.Code != wantStatus || guardResponse.Header().Get(liveCacheRenderedAtHeader) != "" {
			t.Fatalf("guard did not recheck authorization: attempt=%d status=%d body=%q", attempt, guardResponse.Code, guardResponse.Body.String())
		}
		apiResponse := serveLiveCacheMatrix(httptest.NewRequest(http.MethodGet, "/api-mixed", nil))
		if apiResponse.Header().Get(liveCacheRenderedAtHeader) != "" || (attempt > 1 && (apiResponse.Code != http.StatusOK || !strings.Contains(apiResponse.Body.String(), fmt.Sprintf("call=%d", attempt)))) {
			t.Fatalf("API failure or earlier result was reused: attempt=%d status=%d body=%q", attempt, apiResponse.Code, apiResponse.Body.String())
		}
		if attempt == 1 && parseRenderErrorCount(apiResponse.Header().Get(renderErrorCountHeader)) <= 0 {
			t.Fatalf("upstream failure was not reported: status=%d body=%q", apiResponse.Code, apiResponse.Body.String())
		}
		streamResponse := serveLiveCacheMatrix(httptest.NewRequest(http.MethodGet, "/stream-mixed", nil))
		if streamResponse.Code != http.StatusOK || streamResponse.Body.String() != fmt.Sprintf("stream=%d", attempt) || streamResponse.Header().Get("Cache-Control") != "no-store" || streamResponse.Header().Get(liveCacheRenderedAtHeader) != "" {
			t.Fatalf("stream reused a producer/result: status=%d headers=%v body=%q", streamResponse.Code, streamResponse.Header(), streamResponse.Body.String())
		}
	}
	if authCalls.Load() != 3 || guarded.calls.Load() != 2 || apiCalls.Load() != 3 || stream.calls.Load() != 3 || streamCalls.Load() != 3 {
		t.Fatalf("fresh work counts: authorization=%d guarded=%d API=%d stream plugin=%d producer=%d", authCalls.Load(), guarded.calls.Load(), apiCalls.Load(), stream.calls.Load(), streamCalls.Load())
	}
	htmlCacheMutex.RLock()
	defer htmlCacheMutex.RUnlock()
	if len(htmlCache) != 1 {
		t.Fatalf("mixed routes populated %d entries, want only the public response", len(htmlCache))
	}
}

func TestServeContent_LiveCacheInvalidConfigurationRecovery(t *testing.T) {
	setupLiveModeServeContentTest(t)
	plugin := &liveCacheMatrixPlugin{}
	rm.SetPlugin("recovery", plugin)
	config := liveCacheMatrixRoute("recovery", "recovery")
	config["response"] = map[string]interface{}{"status": 600}
	setTestRouteConfig("recovery", config)
	failure := serveLiveCacheMatrix(httptest.NewRequest(http.MethodGet, "/recovery", nil))
	if failure.Code != http.StatusInternalServerError || failure.Header().Get("Cache-Control") != "no-store" || plugin.calls.Load() != 0 {
		t.Fatalf("invalid configuration did not stop rendering: status=%d headers=%v calls=%d", failure.Code, failure.Header(), plugin.calls.Load())
	}
	// Fix the route without flushing the cache: the failed response must not stick.
	setTestRouteConfig("recovery", liveCacheMatrixRoute("recovery", "recovery"))
	first := serveLiveCacheMatrix(httptest.NewRequest(http.MethodGet, "/recovery", nil))
	repeated := serveLiveCacheMatrix(httptest.NewRequest(http.MethodGet, "/recovery", nil))
	if first.Code != http.StatusOK || repeated.Code != http.StatusOK || first.Body.String() != repeated.Body.String() || plugin.calls.Load() != 1 {
		t.Fatalf("corrected route did not recover and cache: first=%d/%q repeated=%d/%q calls=%d", first.Code, first.Body.String(), repeated.Code, repeated.Body.String(), plugin.calls.Load())
	}
}
