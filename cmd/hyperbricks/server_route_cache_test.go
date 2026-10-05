package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/hyperbricks/hyperbricks/pkg/composite"
	"github.com/hyperbricks/hyperbricks/pkg/shared"
)

type routeCacheBlockingPlugin struct {
	calls   atomic.Int32
	entered chan struct{}
	release chan struct{}
}

func (p *routeCacheBlockingPlugin) Render(_ interface{}, _ context.Context) (any, []error) {
	call := p.calls.Add(1)
	if call == 1 {
		close(p.entered)
		<-p.release
	}
	return fmt.Sprintf("render=%d", call), nil
}

func TestServeContent_RouteCachePurgeRejectsInflightRender(t *testing.T) {
	for _, storage := range []string{"mem", "disk"} {
		for _, scope := range []string{"route", "all"} {
			t.Run(storage+"/"+scope, func(t *testing.T) {
				setupRouteCacheIntegration(t)
				plugin := &routeCacheBlockingPlugin{entered: make(chan struct{}), release: make(chan struct{})}
				rm.SetPlugin("inflight", plugin)
				config := liveCacheMatrixRoute("inflight", "inflight")
				config["cache"] = map[string]interface{}{"storage": storage, "expire": "1m"}
				setTestRouteConfig("inflight", config)
				finished := make(chan *httptest.ResponseRecorder, 1)
				go func() { finished <- serveLiveCacheMatrix(httptest.NewRequest("GET", "/inflight", nil)) }()
				select {
				case <-plugin.entered:
				case <-time.After(5 * time.Second):
					close(plugin.release)
					t.Fatal("render did not start")
				}
				if scope == "route" {
					purgeResponseCache("inflight")
				} else {
					clearHTMLCache()
				}
				close(plugin.release)
				select {
				case first := <-finished:
					if first.Code != 200 {
						t.Fatalf("inflight response = %d", first.Code)
					}
				case <-time.After(5 * time.Second):
					t.Fatal("render did not finish")
				}
				if _, found := cachedEntry("inflight"); found {
					t.Fatal("render begun before purge repopulated cache")
				}
				second := serveLiveCacheMatrix(httptest.NewRequest("GET", "/inflight", nil))
				third := serveLiveCacheMatrix(httptest.NewRequest("GET", "/inflight", nil))
				if second.Code != 200 || second.Body.String() != "render=2" || third.Body.String() != second.Body.String() || plugin.calls.Load() != 2 {
					t.Fatal("next generation did not render and cache fresh output")
				}
			})
		}
	}
}

func TestServeContent_RouteCacheDamagedDiskEntryRefreshes(t *testing.T) {
	for _, damage := range []string{"missing", "corrupt"} {
		t.Run(damage, func(t *testing.T) {
			setupRouteCacheIntegration(t)
			plugin := installRouteCacheIntegrationPlugin("damaged", map[string]interface{}{"storage": "disk", "expire": "1m"})
			request := func() *httptest.ResponseRecorder {
				return serveLiveCacheMatrix(httptest.NewRequest("GET", "/damaged", nil))
			}
			first := request()
			entry, found := cachedEntry("damaged")
			if !found || entry.cacheDiskPath == "" {
				t.Fatal("disk entry missing")
			}
			if damage == "missing" {
				if err := os.Remove(entry.cacheDiskPath); err != nil {
					t.Fatal(err)
				}
			} else {
				if err := os.WriteFile(entry.cacheDiskPath, []byte(strings.Repeat("x", int(entry.cacheDiskSize))), 0600); err != nil {
					t.Fatal(err)
				}
			}
			fresh, reused := request(), request()
			if fresh.Code != 200 || fresh.Body.String() == first.Body.String() || reused.Body.String() != fresh.Body.String() || plugin.calls.Load() != 2 {
				t.Fatal("damaged entry was served or failed to repopulate")
			}
		})
	}
}

func TestServeContent_RouteCachePurgeFallbackRoute(t *testing.T) {
	for _, storage := range []string{"mem", "disk"} {
		t.Run(storage, func(t *testing.T) {
			setupRouteCacheIntegration(t)
			plugin := installRouteCacheIntegrationPlugin("404", map[string]interface{}{"storage": storage, "expire": "1m"})
			request := func(path string) *httptest.ResponseRecorder {
				return serveLiveCacheMatrix(httptest.NewRequest("GET", path, nil))
			}
			before := map[string]string{}
			for _, path := range []string{"/missing-one", "/missing-two"} {
				first, second := request(path), request(path)
				if first.Code != 404 || first.Body.String() != second.Body.String() {
					t.Fatalf("fallback did not cache: %d/%q -> %d/%q", first.Code, first.Body.String(), second.Code, second.Body.String())
				}
				before[path] = first.Body.String()
			}
			if plugin.calls.Load() != 2 {
				t.Fatalf("fallback renders = %d; want 2", plugin.calls.Load())
			}
			result := purgeResponseCache("404")
			if result.MemoryEntries+result.DiskEntries != 2 {
				t.Fatalf("fallback purge = %+v", result)
			}
			for _, path := range []string{"/missing-one", "/missing-two"} {
				if response := request(path); response.Code != 404 || response.Body.String() == before[path] {
					t.Fatalf("fallback route purge reused %s: %d/%q", path, response.Code, response.Body.String())
				}
			}
			if plugin.calls.Load() != 4 {
				t.Fatalf("fallback renders after purge = %d; want 4", plugin.calls.Load())
			}
		})
	}
}

func setupRouteCacheIntegration(t *testing.T) string {
	t.Helper()
	setupLiveModeServeContentTest(t)
	return setupResponseCacheStoreTest(t, 0, 0)
}

func installRouteCacheIntegrationPlugin(route string, cache interface{}) *liveCacheMatrixPlugin {
	plugin := &liveCacheMatrixPlugin{}
	rm.SetPlugin(route, plugin)
	config := liveCacheMatrixRoute(route, route)
	if cache != nil {
		config["cache"] = cache
	}
	setTestRouteConfig(route, config)
	return plugin
}

func TestServeContent_RouteCachePolicyAndExpiry(t *testing.T) {
	for _, tc := range []struct {
		name, mode, storage    string
		cache                  interface{}
		disableGlobal, noCache bool
		expire                 time.Duration
	}{
		{name: "default", storage: "mem", expire: time.Hour},
		{name: "scalar memory override", cache: "30s", storage: "mem", expire: 30 * time.Second},
		{name: "memory mapping", cache: map[string]interface{}{"storage": "mem", "expire": "30s"}, storage: "mem", expire: 30 * time.Second},
		{name: "disk override", cache: map[string]interface{}{"storage": "disk", "expire": "30s"}, storage: "disk", expire: 30 * time.Second},
		{name: "disk inherits package", cache: map[string]interface{}{"storage": "disk"}, storage: "disk", expire: time.Hour},
		{name: "scalar disabled", cache: "0s"},
		{name: "disk disabled", cache: map[string]interface{}{"storage": "disk", "expire": "0s"}},
		{name: "global disables memory", cache: "30s", disableGlobal: true},
		{name: "global disables disk", cache: map[string]interface{}{"storage": "disk", "expire": "30s"}, disableGlobal: true},
		{name: "nocache disables memory", cache: "30s", noCache: true},
		{name: "nocache disables disk", cache: map[string]interface{}{"storage": "disk", "expire": "30s"}, noCache: true},
		{name: "development disk", cache: map[string]interface{}{"storage": "disk", "expire": "30s"}, mode: shared.DEVELOPMENT_MODE},
		{name: "debug disk", cache: map[string]interface{}{"storage": "disk", "expire": "30s"}, mode: shared.DEBUG_MODE},
		{name: "development memory", cache: "30s", mode: shared.DEVELOPMENT_MODE},
		{name: "debug memory", cache: "30s", mode: shared.DEBUG_MODE},
	} {
		t.Run(tc.name, func(t *testing.T) {
			module := setupRouteCacheIntegration(t)
			plugin := installRouteCacheIntegrationPlugin("policy", tc.cache)
			config, _ := getConfig("policy")
			config["nocache"] = tc.noCache
			setTestRouteConfig("policy", config)
			if tc.mode != "" {
				getHyperBricksConfiguration().Mode = tc.mode
			}
			if tc.disableGlobal {
				getHyperBricksConfiguration().Live.CacheTime.Duration = 0
			}
			request := func() *httptest.ResponseRecorder {
				return serveLiveCacheMatrix(httptest.NewRequest(http.MethodGet, "/policy", nil))
			}
			first, second := request(), request()
			if first.Code != 200 || second.Code != 200 {
				t.Fatalf("statuses = %d, %d", first.Code, second.Code)
			}
			if tc.expire == 0 {
				if plugin.calls.Load() != 2 || first.Body.String() == second.Body.String() {
					t.Fatal("disabled route reused output")
				}
				if _, found := cachedEntry("policy"); found {
					t.Fatal("disabled route populated the response index")
				}
				if _, err := os.Stat(filepath.Join(module, ".cache")); !os.IsNotExist(err) {
					t.Fatalf("disabled cache created files: %v", err)
				}
				return
			}
			if plugin.calls.Load() != 1 || first.Body.String() != second.Body.String() {
				t.Fatal("cache hit rendered again or changed output")
			}
			entry, found := cachedEntry("policy")
			if !found || entry.cacheTTL != tc.expire {
				t.Fatalf("effective lifetime = %v, found %t; want %v", entry.cacheTTL, found, tc.expire)
			}
			if tc.storage == "disk" {
				if entry.Content != "" || entry.Body != nil || entry.cacheDiskPath == "" {
					t.Fatalf("disk body retained in memory: %+v", entry)
				}
			} else if entry.Content == "" || entry.cacheDiskPath != "" {
				t.Fatal("memory policy used disk storage")
			}
			layout := "2006-01-02 15:04:05 (-07:00)"
			renderedAt, err := time.Parse(layout, first.Header().Get(liveCacheRenderedAtHeader))
			if err != nil {
				t.Fatal(err)
			}
			expiresAt, err := time.Parse(layout, first.Header().Get(liveCacheExpiresAtHeader))
			if err != nil {
				t.Fatal(err)
			}
			if expiresAt.Sub(renderedAt) != tc.expire {
				t.Fatal("cache metadata reported package lifetime instead of route lifetime")
			}
			// Expire deterministically. A 30-second route must refresh even
			// though the package's one-hour lifetime has not elapsed.
			htmlCacheMutex.Lock()
			entry.Timestamp = time.Now().Add(-tc.expire - time.Second)
			htmlCache["policy"] = entry
			htmlCacheMutex.Unlock()
			third, fourth := request(), request()
			if plugin.calls.Load() != 2 || third.Body.String() == first.Body.String() || fourth.Body.String() != third.Body.String() {
				t.Fatal("expired response did not refresh and then reuse once")
			}
		})
	}
}

func TestServeContent_RouteCacheVariantParity(t *testing.T) {
	type input struct {
		method, target, body string
		headers              http.Header
	}
	request := func(v input) *http.Request {
		if v.method == "" {
			v.method = http.MethodGet
		}
		if v.target == "" {
			v.target = "/variants"
		}
		return liveCacheMatrixRequest(v.method, v.target, v.body, v.headers)
	}
	for _, storage := range []string{"mem", "disk"} {
		for _, tc := range []struct {
			name, vary, want string
			first, second    input
		}{
			{name: "query", first: input{target: "/variants?q=one"}, second: input{target: "/variants?q=two"}, want: `"query":"q=two"`},
			{name: "authorization", first: input{headers: http.Header{"Authorization": {"Bearer one"}}}, second: input{headers: http.Header{"Authorization": {"Bearer two"}}}, want: `"authorization":"Bearer two"`},
			{name: "cookie", first: input{headers: http.Header{"Cookie": {"session=one"}}}, second: input{headers: http.Header{"Cookie": {"session=two"}}}, want: `"cookie":"session=two"`},
			{name: "body", first: input{method: "POST", body: "one"}, second: input{method: "POST", body: "two"}, want: `"body":"two"`},
			{name: "method", first: input{method: "POST", body: "same"}, second: input{method: "PUT", body: "same"}, want: `"method":"PUT"`},
			{name: "vary language", vary: "Accept-Language", first: input{headers: http.Header{"Accept-Language": {"en"}}}, second: input{headers: http.Header{"Accept-Language": {"nl"}}}, want: `"language":"nl"`},
			{name: "vary htmx", vary: "HX-Request", second: input{headers: http.Header{"Hx-Request": {"true"}}}, want: `"hx_request":"true"`},
			{name: "vary host", vary: "Host", first: input{target: "http://one.example/variants"}, second: input{target: "http://two.example/variants"}, want: `"host":"two.example"`},
		} {
			t.Run(storage+"/"+tc.name, func(t *testing.T) {
				setupRouteCacheIntegration(t)
				plugin := installRouteCacheIntegrationPlugin("variants", map[string]interface{}{"storage": storage, "expire": "1m"})
				config, _ := getConfig("variants")
				if tc.vary != "" {
					config["response"] = map[string]interface{}{"headers": map[string]interface{}{"Vary": tc.vary}}
					setTestRouteConfig("variants", config)
				}
				first := serveLiveCacheMatrix(request(tc.first))
				second := serveLiveCacheMatrix(request(tc.second))
				repeat := serveLiveCacheMatrix(request(tc.first))
				if first.Code != 200 || second.Code != 200 || repeat.Code != 200 || !strings.Contains(second.Body.String(), tc.want) {
					t.Fatalf("variant output invalid: %d/%q %d/%q", first.Code, first.Body.String(), second.Code, second.Body.String())
				}
				if plugin.calls.Load() != 2 || first.Body.String() == second.Body.String() || first.Body.String() != repeat.Body.String() {
					t.Fatal("variant response leaked or failed to reuse")
				}
			})
		}
	}
}

func TestServeContent_RouteCacheResponseMetadataParity(t *testing.T) {
	for _, storage := range []string{"mem", "disk"} {
		for _, tc := range []struct {
			name    string
			status  int
			cookies []string
		}{
			{name: "conditional success", status: 200},
			{name: "created", status: 201},
			{name: "not found", status: 404},
			{name: "cookies", status: 200, cookies: []string{"session=one; Path=/; HttpOnly", "theme=dark; Path=/"}},
		} {
			t.Run(storage+"/"+tc.name, func(t *testing.T) {
				setupRouteCacheIntegration(t)
				config := map[string]interface{}{
					"@type": composite.HyperMediaConfigGetName(), "route": "metadata", "beautify": false, "content_type": "text/plain",
					"cache": map[string]interface{}{"storage": storage, "expire": "1m"}, "cookies": tc.cookies,
					"response": map[string]interface{}{"status": tc.status, "headers": map[string]interface{}{"HX-Retarget": "#result", "X-Response": "preserved", "Cache-Control": "no-store", "Etag": "configured-etag"}},
					"template": map[string]interface{}{"@type": composite.TemplateConfigGetName(), "inline": "stable-response", "values": map[string]interface{}{}},
				}
				setTestRouteConfig("metadata", config)
				first := serveLiveCacheMatrix(httptest.NewRequest("GET", "/metadata", nil))
				second := serveLiveCacheMatrix(httptest.NewRequest("GET", "/metadata", nil))
				entry, found := cachedEntry("metadata")
				if !found || entry.ETag == "" || entry.ETag == "configured-etag" {
					t.Fatal("response not cached with generated ETag")
				}
				for _, w := range []*httptest.ResponseRecorder{first, second} {
					if w.Code != tc.status || w.Body.String() != "stable-response" {
						t.Fatalf("response = %d/%q", w.Code, w.Body.String())
					}
					for name, want := range map[string]string{"HX-Retarget": "#result", "X-Response": "preserved", "Cache-Control": "no-store", "Content-Type": "text/plain", "Content-Length": "15", "ETag": entry.ETag} {
						if got := w.Header().Get(name); got != want {
							t.Errorf("%s = %q; want %q", name, got, want)
						}
					}
					if !reflect.DeepEqual(w.Header().Values("Set-Cookie"), tc.cookies) {
						t.Errorf("cookies = %v; want %v", w.Header().Values("Set-Cookie"), tc.cookies)
					}
				}
				conditional := httptest.NewRequest("GET", "/metadata", nil)
				conditional.Header.Set("If-None-Match", entry.ETag)
				response := serveLiveCacheMatrix(conditional)
				if tc.status == 200 && len(tc.cookies) == 0 {
					if response.Code != 304 || response.Body.Len() != 0 || response.Header().Get("Content-Length") != "" || response.Header().Get("ETag") != entry.ETag || response.Header().Get("HX-Retarget") != "#result" {
						t.Fatalf("conditional response = %d/%q headers=%v", response.Code, response.Body.String(), response.Header())
					}
				} else if response.Code != tc.status || response.Body.String() != first.Body.String() || !reflect.DeepEqual(response.Header().Values("Set-Cookie"), tc.cookies) {
					t.Fatalf("conditional request lost non-200/cookie response: %d/%q", response.Code, response.Body.String())
				}
			})
		}
	}
}

func TestServeContent_RouteCachePurgeAndReload(t *testing.T) {
	setupRouteCacheIntegration(t)
	plugins := map[string]*liveCacheMatrixPlugin{
		"memory": installRouteCacheIntegrationPlugin("memory", "1m"),
		"disk":   installRouteCacheIntegrationPlugin("disk", map[string]interface{}{"storage": "disk", "expire": "1m"}),
	}
	request := func(route, variant string) *httptest.ResponseRecorder {
		return serveLiveCacheMatrix(httptest.NewRequest("GET", "/"+route+"?q="+variant, nil))
	}
	bodies := map[string]string{}
	for _, route := range []string{"memory", "disk"} {
		for _, variant := range []string{"one", "two"} {
			bodies[route+variant] = request(route, variant).Body.String()
		}
	}
	result := purgeResponseCache("memory")
	if result.MemoryEntries != 2 || result.DiskEntries != 0 {
		t.Fatalf("route purge = %+v", result)
	}
	if got := request("memory", "one").Body.String(); got == bodies["memoryone"] {
		t.Fatal("purged memory response reused")
	}
	if got := request("disk", "one").Body.String(); got != bodies["diskone"] || plugins["disk"].calls.Load() != 2 {
		t.Fatal("memory route purge invalidated unrelated disk route")
	}
	result = purgeResponseCache("disk")
	if result.DiskEntries != 2 || result.MemoryEntries != 0 {
		t.Fatalf("disk route purge = %+v", result)
	}
	if got := request("disk", "one").Body.String(); got == bodies["diskone"] {
		t.Fatal("purged disk response reused")
	}
	// Configuration publication uses this same cache invalidation owner.
	clearHTMLCache()
	request("memory", "one")
	request("disk", "one")
	if plugins["memory"].calls.Load() != 4 || plugins["disk"].calls.Load() != 4 {
		t.Fatalf("reload did not invalidate both backends: memory=%d disk=%d", plugins["memory"].calls.Load(), plugins["disk"].calls.Load())
	}
}

func TestServeContent_RouteCacheDiskWriteFailureServesFresh(t *testing.T) {
	module := setupRouteCacheIntegration(t)
	blocked := filepath.Join(module, "blocked-cache")
	if err := os.WriteFile(blocked, []byte("not a directory"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := configureResponseCache(module, blocked, 0, 0); err != nil {
		t.Fatal(err)
	}
	plugin := installRouteCacheIntegrationPlugin("failure", map[string]interface{}{"storage": "disk", "expire": "1m"})
	request := func() *httptest.ResponseRecorder {
		return serveLiveCacheMatrix(httptest.NewRequest("GET", "/failure", nil))
	}
	first, second := request(), request()
	if first.Code != 200 || second.Code != 200 || first.Body.String() == second.Body.String() || plugin.calls.Load() != 2 {
		t.Fatal("disk failure did not continue serving fresh output")
	}
	if _, found := cachedEntry("failure"); found {
		t.Fatal("disk failure silently fell back to memory")
	}
	for _, w := range []*httptest.ResponseRecorder{first, second} {
		if w.Header().Get("ETag") != "" || w.Header().Get(liveCacheExpiresAtHeader) != "" {
			t.Fatal("failed storage advertised cached output")
		}
	}
	if err := os.Remove(blocked); err != nil {
		t.Fatal(err)
	}
	third, fourth := request(), request()
	if third.Code != 200 || third.Body.String() != fourth.Body.String() || plugin.calls.Load() != 3 {
		t.Fatal("disk storage did not recover after directory became available")
	}
	entry, found := cachedEntry("failure")
	if !found || entry.Content != "" || entry.cacheDiskPath == "" {
		t.Fatal("recovered route did not use disk storage")
	}
}
