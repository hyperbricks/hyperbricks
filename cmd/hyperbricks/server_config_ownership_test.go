package main

import (
	"context"
	"encoding/json"
	"html/template"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"testing"

	"github.com/hyperbricks/hyperbricks/pkg/component"
	"github.com/hyperbricks/hyperbricks/pkg/composite"
	"github.com/hyperbricks/hyperbricks/pkg/renderplan"
	"github.com/hyperbricks/hyperbricks/pkg/shared"
)

func TestRenderContentCompiledConfigOwnership(t *testing.T) {
	for _, fallback := range []bool{false, true} {
		name := "dynamic"
		if fallback {
			name = "404_fallback"
		}
		t.Run(name, func(t *testing.T) {
			fixture := setupSSRProofPipelineBenchmark(t)
			source, plan, found := getConfigAndPlan("index")
			if !found || plan != fixture.plan {
				t.Fatal("missing compiled fixture route")
			}
			route, wantStatus := "index", http.StatusOK
			if fallback {
				setTestRouteConfig("404", source)
				configMutex.Lock()
				routePlans["404"] = plan
				configMutex.Unlock()
				route, wantStatus = "missing", http.StatusNotFound
			}
			before := configOwnershipSnapshot(t, source)
			rm.RegisterComponent(composite.HyperMediaConfigGetName(), configOwnershipRenderer(func(interface{}, context.Context) (string, []error) {
				t.Fatal("compiled route reached the legacy renderer")
				return "", nil
			}), reflect.TypeOf(composite.HyperMediaConfig{}))

			for _, rid := range []string{"first", `second<&"'>`, "last"} {
				request := httptest.NewRequest(http.MethodGet, "/"+route+"?rid="+url.QueryEscape(rid), nil)
				result := renderContent(httptest.NewRecorder(), route, request, rid)
				want := strings.ReplaceAll(fixture.expected, "benchmark-request", template.HTMLEscapeString(rid))
				if result.Status != wantStatus || result.ErrorCount != 0 || !result.NoCache || result.Content != want {
					t.Fatalf("request %q: status=%d errors=%d nocache=%t output matches=%t", rid, result.Status, result.ErrorCount, result.NoCache, result.Content == want)
				}
				if got := configOwnershipSnapshot(t, source); got != before {
					t.Fatalf("request %q mutated the source config", rid)
				}
			}
		})
	}
}

func TestRenderContentLegacyConfigOwnership(t *testing.T) {
	for _, configType := range []string{composite.HyperMediaConfigGetName(), composite.FragmentConfigGetName(), composite.ApiFragmentRenderConfigGetName()} {
		t.Run(configType, func(t *testing.T) {
			setupLiveModeServeContentTest(t)
			source := map[string]interface{}{"@type": configType, "route": "ownership", "beautify": false}
			setTestRouteConfig("ownership", source)
			before := configOwnershipSnapshot(t, source)
			var retained []map[string]interface{}
			var writers []*httptest.ResponseRecorder
			var retainedWriters []http.ResponseWriter
			renderer := configOwnershipRenderer(func(instance interface{}, ctx context.Context) (string, []error) {
				requestConfig, ok := instance.(map[string]interface{})
				if !ok {
					t.Fatalf("renderer received %T, want request map", instance)
				}
				if requestConfig["route"] != "ownership" || requestConfig["request_marker"] != nil {
					t.Fatal("renderer received mutations from an earlier request")
				}
				if _, hasWriter := requestConfig["hx_response"]; hasWriter {
					t.Fatal("legacy renderer received the removed hx_response field")
				}
				writer, ok := ctx.Value(shared.ResponseWriter).(http.ResponseWriter)
				if !ok || writer != writers[len(writers)-1] {
					t.Fatal("response writer context does not match the current request")
				}
				retainedWriters = append(retainedWriters, writer)
				request := ctx.Value(shared.Request).(*http.Request)
				marker := request.URL.Query().Get("rid")
				requestConfig["request_marker"] = marker
				delete(requestConfig, "route")
				retained = append(retained, requestConfig)
				return marker, nil
			})
			// Decoding into an empty interface preserves the incoming map identity.
			rm.RegisterComponent(configType, renderer, reflect.TypeOf((*interface{})(nil)).Elem())

			for _, rid := range []string{"first", "second"} {
				writer := httptest.NewRecorder()
				writers = append(writers, writer)
				request := httptest.NewRequest(http.MethodGet, "/ownership?rid="+rid, nil)
				result := renderContent(writer, "ownership", request, rid)
				if result.Status != http.StatusOK || result.ErrorCount != 0 || result.Content != rid {
					t.Fatalf("request %q: unexpected result %+v", rid, result)
				}
				if got := configOwnershipSnapshot(t, source); got != before {
					t.Fatalf("request %q mutated the source config", rid)
				}
			}
			if len(retained) != 2 || retained[0]["request_marker"] != "first" || retained[1]["request_marker"] != "second" {
				t.Fatal("legacy requests shared a mutable config map")
			}
			for i, writer := range retainedWriters {
				if writer != writers[i] {
					t.Fatalf("request %d retained another request's response writer", i)
				}
			}
		})
	}
}

func TestRenderContentMetadataReadsPreserveConfig(t *testing.T) {
	source := map[string]interface{}{
		"@type":    composite.HyperMediaConfigGetName(),
		"nocache":  " false ",
		"beautify": "false",
		"headers":  map[string]interface{}{" Content-Type ": " text/plain ", " X-Ownership ": " source "},
		"cookies":  []interface{}{" session=source; Path=/ ", nil, " "},
		"guard": map[string]interface{}{
			"enabled": true,
			"require": map[string]interface{}{
				"authenticated": true,
				"query":         map[string]interface{}{"rid": "true"},
			},
		},
		"children": []interface{}{map[string]interface{}{"@type": component.APIConfigGetName()}},
	}
	before := configOwnershipSnapshot(t, source)
	request := httptest.NewRequest(http.MethodGet, "/?rid=metadata", nil)
	request.Header.Set("Authorization", "Bearer request-token")
	if response, token := evaluateRouteGuard(source, request); response != nil || token != "request-token" {
		t.Fatalf("guard result = %+v, token = %q", response, token)
	}
	if !resolveConfiguredNoCache(source) || resolveBeautify(source, true) || !renderplan.NeedsAPIRequestContext(source) {
		t.Fatal("unexpected route metadata decisions")
	}
	headers := extractResponseHeaders(source)
	cookies := extractResponseCookies(source)
	if headerContentType(headers) != "text/plain" || headers["X-Ownership"] != "source" || !reflect.DeepEqual(cookies, []string{"session=source; Path=/"}) {
		t.Fatalf("unexpected metadata: headers=%v cookies=%v", headers, cookies)
	}
	headers["X-Ownership"] = "request"
	cookies[0] = "session=request; Path=/"
	if got := configOwnershipSnapshot(t, source); got != before {
		t.Fatal("metadata reads or returned collections mutated the source config")
	}
}

type configOwnershipRenderer func(interface{}, context.Context) (string, []error)

func (r configOwnershipRenderer) Render(instance interface{}, ctx context.Context) (string, []error) {
	return r(instance, ctx)
}

func (configOwnershipRenderer) Types() []string { return nil }

func configOwnershipSnapshot(t *testing.T, config map[string]interface{}) string {
	t.Helper()
	data, err := json.Marshal(config)
	if err != nil {
		t.Fatalf("snapshot source config: %v", err)
	}
	return string(data)
}
