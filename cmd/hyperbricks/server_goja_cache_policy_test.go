package main

import (
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	"github.com/hyperbricks/hyperbricks/pkg/component"
	"github.com/hyperbricks/hyperbricks/pkg/composite"
	"github.com/hyperbricks/hyperbricks/pkg/logging"
)

// Internal rendered-output reuse and browser/proxy storage are separate policies.
// Exercise the public HTTP handler in live mode: a development-mode request
// would bypass the live cache even without Goja's route preparation.
func TestServeContent_GojaRenderCacheAndHTTPPolicy(t *testing.T) {
	for _, execution := range []string{"legacy", "compiled"} {
		for _, root := range []struct {
			name          string
			componentType string
			defaultHeader string
		}{
			{"hypermedia", composite.HyperMediaConfigGetName(), "no-store"},
			{"fragment", composite.FragmentConfigGetName(), ""},
		} {
			// The runtime currently compiles only HYPERMEDIA route roots.
			if execution == "compiled" && root.componentType == composite.FragmentConfigGetName() {
				continue
			}
			for _, policy := range []struct {
				name       string
				configured string
			}{
				{"default", ""},
				{"explicit-no-store", "no-store"},
				{"explicit-public", "public, max-age=60"},
			} {
				t.Run(execution+"/"+root.name+"/"+policy.name, func(t *testing.T) {
					setupLiveModeServeContentTest(t)
					const routeName = "goja-cache-policy"
					const expectedBody = "<p>request:1</p>"
					script := map[string]interface{}{
						"@type": component.GojaRenderConfigGetName(),
						"script": `var executions = 0;
function main(input) { executions++; return {id: input.query.id, executions: executions}; }`,
						"inline":    "<p>{{.Data.id}}:{{.Data.executions}}</p>",
						"querykeys": []string{"id"},
					}
					route := map[string]interface{}{
						"@type": root.componentType, "route": routeName,
						"nocache": false, "beautify": false,
						"template": map[string]interface{}{
							"inline": "{{.content}}", "values": map[string]interface{}{"content": script},
						},
					}
					expectedHeader := root.defaultHeader
					if policy.configured != "" {
						route["response"] = map[string]interface{}{
							"headers": map[string]interface{}{"Cache-Control": policy.configured},
						}
						expectedHeader = policy.configured
					}
					routes := map[string]map[string]interface{}{routeName: route}
					diagnostics := make(map[string][]error)
					prepareGojaRouteConfigs(routes, diagnostics)
					if len(diagnostics) != 0 {
						t.Fatalf("Goja preparation failed: %v", diagnostics)
					}
					if !resolveConfiguredNoCache(route) {
						t.Fatal("Goja preparation did not override nocache:false")
					}
					prepared, ok := script[component.GojaPreparedKey].(*component.PreparedGojaRender)
					if !ok || prepared.Err() != nil {
						t.Fatalf("Goja resources were not prepared: %#v", script[component.GojaPreparedKey])
					}
					if execution == "compiled" {
						plans := compileRoutePlans(routes, logging.GetLogger())
						if plans[routeName] == nil {
							t.Fatal("route did not receive a compiled plan")
						}
						updateGlobalRoutes(routes, plans)
					} else {
						setTestRouteConfig(routeName, route)
					}

					newRequest := func() *http.Request {
						return httptest.NewRequest(http.MethodGet, "/"+routeName+"?id=request", nil)
					}
					assertResponse := func() {
						t.Helper()
						response := httptest.NewRecorder()
						ServeContent(response, newRequest())
						if response.Code != http.StatusOK || response.Body.String() != expectedBody {
							t.Fatalf("status=%d body=%q; want fresh execution %q", response.Code, response.Body.String(), expectedBody)
						}
						if got := response.Header().Get("Cache-Control"); got != expectedHeader {
							t.Fatalf("Cache-Control=%q; want %q", got, expectedHeader)
						}
						for _, name := range []string{"ETag", liveCacheRenderedAtHeader, liveCacheExpiresAtHeader} {
							if got := response.Header().Get(name); got != "" {
								t.Errorf("uncached response has internal cache metadata %s=%q", name, got)
							}
						}
					}
					for attempt := 0; attempt < 3; attempt++ {
						assertResponse()
						htmlCacheMutex.RLock()
						cacheSize := len(htmlCache)
						htmlCacheMutex.RUnlock()
						if cacheSize != 0 {
							t.Fatalf("request %d populated the internal output cache (%d entries)", attempt+1, cacheSize)
						}
					}

					// A populated, unexpired entry proves lookup is bypassed too:
					// merely declining to save a newly rendered result is insufficient.
					cacheKey, cacheable := resolveLiveCacheKey(routeName, newRequest())
					if !cacheable {
						t.Fatal("request cannot exercise a valid live-cache key")
					}
					stale := CacheEntry{
						Content: "stale cached script result", ContentType: "text/html",
						Status: http.StatusOK, Timestamp: time.Now(), ETag: `"stale"`,
					}
					htmlCacheMutex.Lock()
					htmlCache[cacheKey] = stale
					htmlCacheMutex.Unlock()
					assertResponse()
					if after, found := cachedEntry(cacheKey); !found || !reflect.DeepEqual(after, stale) {
						t.Fatal("Goja response rewrote the preseeded cache entry")
					}
					if script[component.GojaPreparedKey] != prepared {
						t.Fatal("requests replaced immutable prepared Goja resources")
					}
				})
			}
		}
	}
}
