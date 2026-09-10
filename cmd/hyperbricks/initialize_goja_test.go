package main

import (
	"fmt"
	"net/http/httptest"
	"reflect"
	"sync"
	"testing"

	"github.com/hyperbricks/hyperbricks/pkg/component"
	"github.com/hyperbricks/hyperbricks/pkg/composite"
	"github.com/hyperbricks/hyperbricks/pkg/logging"
	"github.com/hyperbricks/hyperbricks/pkg/shared"
)

func TestGojaLoadedRouteIsUncachedAndIsolated(t *testing.T) {
	setupLiveModeServeContentTest(t)
	script := map[string]interface{}{
		"@type":  component.GojaRenderConfigGetName(),
		"script": `var seen = 0; function main(input) { seen++; return {rid: input.query.rid, seen: seen}; }`,
		"inline": "<p>{{.Data.rid}}:{{.Data.seen}}</p>", "querykeys": []string{"rid"},
		component.GojaPreparedKey: "source-authored metadata must be replaced",
	}
	route := map[string]interface{}{
		"@type": composite.HyperMediaConfigGetName(), "route": "index", "nocache": false, "beautify": false,
		"headers":  map[string]interface{}{"cache-control": "public, max-age=3600", "X-Example": "kept"},
		"template": map[string]interface{}{"inline": "{{.content}}", "values": map[string]interface{}{"content": script}},
	}
	ordinary := map[string]interface{}{"@type": composite.FragmentConfigGetName(), "route": "ordinary", "nocache": false}
	before := shared.CloneMapDeep(ordinary)
	routes := map[string]map[string]interface{}{"index": route, "ordinary": ordinary}
	diagnostics := make(map[string][]error)
	prepareGojaRouteConfigs(routes, diagnostics)
	if len(diagnostics) != 0 || !reflect.DeepEqual(ordinary, before) {
		t.Fatalf("diagnostics=%v ordinary route changed=%t", diagnostics, !reflect.DeepEqual(ordinary, before))
	}
	prepared, ok := script[component.GojaPreparedKey].(*component.PreparedGojaRender)
	if !ok || prepared.Err() != nil {
		t.Fatalf("invalid prepared resource: %#v", script[component.GojaPreparedKey])
	}
	plans := compileRoutePlans(routes, logging.GetLogger())
	if plans["index"] == nil {
		t.Fatal("goja page did not receive a compiled plan")
	}
	updateGlobalRoutes(routes, plans)
	var wg sync.WaitGroup
	for i := 0; i < 64; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			rid := fmt.Sprintf("client-%d", i)
			response := httptest.NewRecorder()
			ServeContent(response, httptest.NewRequest("GET", "/?rid="+rid, nil))
			if response.Code != 200 || response.Body.String() != "<p>"+rid+":1</p>" || response.Header().Get("Cache-Control") != "no-store" || response.Header().Get("X-Example") != "kept" {
				t.Errorf("rid=%s status=%d body=%q headers=%v", rid, response.Code, response.Body.String(), response.Header())
			}
		}(i)
	}
	wg.Wait()
	if script[component.GojaPreparedKey] != prepared {
		t.Fatal("requests replaced prepared resource")
	}
	htmlCacheMutex.RLock()
	defer htmlCacheMutex.RUnlock()
	if len(htmlCache) != 0 {
		t.Fatalf("script responses entered cache: %d entries", len(htmlCache))
	}
}

func TestGojaInvalidPreparationKeepsDiagnosticsAndDisablesCache(t *testing.T) {
	setupLiveModeServeContentTest(t)
	route := map[string]interface{}{
		"@type": composite.FragmentConfigGetName(), "route": "broken",
		"child": map[string]interface{}{"@type": component.GojaRenderConfigGetName(), "inline": "ok", "script": "function main("},
	}
	diagnostics := make(map[string][]error)
	prepareGojaRouteConfigs(map[string]map[string]interface{}{"broken": route}, diagnostics)
	if len(diagnostics["broken"]) == 0 || !resolveConfiguredNoCache(route) {
		t.Fatalf("invalid script: diagnostics=%v nocache=%v", diagnostics, route["nocache"])
	}
	child := route["child"].(map[string]interface{})
	output, errs := rm.Render(component.GojaRenderConfigGetName(), child, nil)
	if output != "" || len(errs) == 0 {
		t.Fatalf("invalid script rendered: %q %v", output, errs)
	}
}
