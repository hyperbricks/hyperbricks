package main

import (
	"fmt"
	"html/template"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/hyperbricks/hyperbricks/pkg/component"
	"github.com/hyperbricks/hyperbricks/pkg/composite"
	"github.com/hyperbricks/hyperbricks/pkg/parser"
	"github.com/hyperbricks/hyperbricks/pkg/renderplan"
	"github.com/hyperbricks/hyperbricks/pkg/shared"
	yamlparser "github.com/hyperbricks/hyperbricks/pkg/yaml-parser"
)

func TestServeContent_FragmentResponseHeadersSurviveCache(t *testing.T) {
	for _, status := range []int{http.StatusOK, http.StatusCreated} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			setupLiveModeServeContentTest(t)
			setTestRouteConfig("cached-fragment", map[string]interface{}{
				"@type": composite.FragmentConfigGetName(), "route": "cached-fragment", "beautify": false,
				"response": map[string]interface{}{
					"status": status,
					"headers": map[string]interface{}{
						"HX-Retarget": "#result", "X-Fragment": "cached", "Vary": "Accept-Language",
						"Etag": "configured-etag",
					},
				},
				"template": map[string]interface{}{
					"@type": composite.TemplateConfigGetName(), "inline": "stable-content", "values": map[string]interface{}{},
				},
			})
			first := httptest.NewRecorder()
			firstRequest := httptest.NewRequest(http.MethodGet, "/cached-fragment", nil)
			ServeContent(first, firstRequest)
			cacheKey, cacheable := resolveLiveCacheKey("cached-fragment", firstRequest)
			if !cacheable {
				t.Fatal("configured fragment was not cacheable")
			}
			before, found := cachedEntry(cacheKey)
			if !found || before.ETag == "" || before.ETag == "configured-etag" {
				t.Fatal("first response did not populate the live cache with an ETag")
			}
			second := httptest.NewRecorder()
			ServeContent(second, httptest.NewRequest(http.MethodGet, "/cached-fragment", nil))
			after, found := cachedEntry(cacheKey)
			if !found || !before.Timestamp.Equal(after.Timestamp) {
				t.Fatal("second request did not reuse the original cache entry")
			}
			for i, response := range []*httptest.ResponseRecorder{first, second} {
				if response.Code != status || response.Body.String() != "stable-content" {
					t.Fatalf("response %d: status=%d body=%q", i+1, response.Code, response.Body.String())
				}
				if response.Header().Get("HX-Retarget") != "#result" || response.Header().Get("X-Fragment") != "cached" || response.Header().Get("Vary") != "Accept-Language" {
					t.Fatalf("response %d lost configured headers: %v", i+1, response.Header())
				}
				if got := response.Header().Values("ETag"); len(got) != 1 || got[0] != before.ETag {
					t.Fatalf("response %d retained conflicting configured/generated ETags: %v", i+1, got)
				}
			}
			if status == http.StatusOK {
				conditional := httptest.NewRequest(http.MethodGet, "/cached-fragment", nil)
				conditional.Header.Set("If-None-Match", before.ETag)
				response := httptest.NewRecorder()
				ServeContent(response, conditional)
				if response.Code != http.StatusNotModified || response.Body.Len() != 0 {
					t.Fatalf("conditional response: status=%d body=%q", response.Code, response.Body.String())
				}
				if response.Header().Get("HX-Retarget") != "#result" || response.Header().Get("X-Fragment") != "cached" || response.Header().Get("Vary") != "Accept-Language" || response.Header().Get("ETag") != before.ETag {
					t.Fatalf("conditional response lost cached metadata: %v", response.Header())
				}
			}
		})
	}
}

func TestServeContent_GuardVariantsUseConfiguredHeaders(t *testing.T) {
	setupLiveModeServeContentTest(t)
	var renders int32
	rm.SetPlugin("guard_protected", handledResponseTestPlugin{calls: &renders, body: []byte("protected")})
	setTestRouteConfig("guard-variants", map[string]interface{}{
		"@type": composite.FragmentConfigGetName(), "route": "guard-variants", "nocache": false,
		"response": map[string]interface{}{"status": http.StatusOK, "headers": map[string]interface{}{"Cache-Control": "public", "X-Route": "allowed"}},
		"guard": map[string]interface{}{
			"enabled": true,
			"auth":    map[string]interface{}{"header": "X-Session", "cookie": "session"},
			"require": map[string]interface{}{"authenticated": true},
			"on_unauthenticated": map[string]interface{}{
				"default": map[string]interface{}{"status": http.StatusSeeOther, "headers": map[string]interface{}{"Location": "/login", "X-Default": "yes", "Cache-Control": "public"}},
				"variants": []interface{}{
					map[string]interface{}{
						"when":     map[string]interface{}{"request_headers": map[string]interface{}{"x-ui": "enhanced", "X-Version": "2"}},
						"response": map[string]interface{}{"status": http.StatusUnauthorized, "headers": map[string]interface{}{"X-Choice": "first", "Vary": "Accept-Language", "cache-control": "public"}},
					},
					map[string]interface{}{
						"when":     map[string]interface{}{"request_headers": map[string]interface{}{"X-UI": "enhanced"}},
						"response": map[string]interface{}{"status": http.StatusForbidden, "headers": map[string]interface{}{"X-Choice": "second"}},
					},
				},
			},
			"on_forbidden": map[string]interface{}{
				"variants": []interface{}{map[string]interface{}{
					"when":     map[string]interface{}{"request_headers": map[string]interface{}{"X-Forbidden-Format": "json"}},
					"response": map[string]interface{}{"status": http.StatusForbidden},
				}},
			},
		},
		"10": map[string]interface{}{"@type": component.PluginRenderGetName(), "plugin": "guard_protected"},
	})
	for _, tc := range []struct {
		name    string
		headers map[string]string
		status  int
		choice  string
	}{
		{"ordinary browser", nil, http.StatusSeeOther, ""},
		{"HTMX has no implicit behavior", map[string]string{"HX-Request": "true"}, http.StatusSeeOther, ""},
		{"first matching variant wins", map[string]string{"X-UI": "enhanced", "x-version": "2"}, http.StatusUnauthorized, "first"},
		{"all conditions must match", map[string]string{"X-UI": "enhanced", "X-Version": "3"}, http.StatusForbidden, "second"},
		{"missing condition does not match", map[string]string{"X-UI": "enhanced"}, http.StatusForbidden, "second"},
		{"values are case sensitive", map[string]string{"X-UI": "Enhanced", "X-Version": "2"}, http.StatusSeeOther, ""},
		{"values are not trimmed", map[string]string{"X-UI": " enhanced ", "X-Version": "2"}, http.StatusSeeOther, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, "/guard-variants", nil)
			for name, value := range tc.headers {
				request.Header.Set(name, value)
			}
			response := httptest.NewRecorder()
			ServeContent(response, request)
			if response.Code != tc.status || response.Header().Get("X-Choice") != tc.choice {
				t.Fatalf("status=%d headers=%v", response.Code, response.Header())
			}
			if tc.choice == "" {
				if response.Header().Get("Location") != "/login" || response.Header().Get("X-Default") != "yes" {
					t.Fatalf("default response was not selected: %v", response.Header())
				}
			} else if response.Header().Get("Location") != "" || response.Header().Get("X-Default") != "" {
				t.Fatalf("variant inherited default response headers: %v", response.Header())
			}
			if response.Header().Get("Cache-Control") != "no-store" || response.Header().Get("X-Route") != "" || response.Header().Get("HX-Redirect") != "" {
				t.Fatalf("denial headers were overridden or implicit HTMX behavior leaked: %v", response.Header())
			}
			wantVary := []string{"Cookie", "Authorization", "X-Session", "X-UI", "X-Version", "X-Forbidden-Format"}
			if tc.choice == "first" {
				wantVary = append(wantVary, "Accept-Language")
			}
			assertHTTPVary(t, response.Header(), wantVary...)
			if _, found := cachedEntry("guard-variants"); found {
				t.Fatal("guard denial populated the live cache")
			}
		})
	}
	if got := atomic.LoadInt32(&renders); got != 0 {
		t.Fatalf("response variants bypassed authentication and rendered %d times", got)
	}
}

func TestServeContent_GuardVariantsNeverBypassAuthorization(t *testing.T) {
	setupLiveModeServeContentTest(t)
	var authCalls, upstreamCalls int32
	authz := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&authCalls, 1)
		switch r.Header.Get("Authorization") {
		case "Bearer allowed":
			w.WriteHeader(http.StatusNoContent)
		case "Bearer forbidden":
			w.WriteHeader(http.StatusForbidden)
		default:
			w.WriteHeader(http.StatusUnauthorized)
		}
	}))
	defer authz.Close()
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&upstreamCalls, 1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"message":"protected"}`)
	}))
	defer upstream.Close()
	action := map[string]interface{}{
		"default": map[string]interface{}{"status": http.StatusSeeOther, "headers": map[string]interface{}{"Location": "/login"}},
		"variants": []interface{}{map[string]interface{}{
			"when":     map[string]interface{}{"request_headers": map[string]interface{}{"X-UI": "enhanced"}},
			"response": map[string]interface{}{"headers": map[string]interface{}{"X-Denied": "yes"}},
		}},
	}
	setTestRouteConfig("authorized-api", map[string]interface{}{
		"@type": composite.ApiFragmentRenderConfigGetName(), "route": "authorized-api", "endpoint": upstream.URL, "method": "GET", "inline": `{{ .Data.message }}`,
		"guard": map[string]interface{}{
			"enabled": true, "require": map[string]interface{}{"authenticated": true},
			"authorize":          map[string]interface{}{"endpoint": authz.URL},
			"on_unauthenticated": action, "on_forbidden": action,
		},
		"response": map[string]interface{}{"status": http.StatusAccepted, "headers": map[string]interface{}{"X-Allowed": "yes"}},
	})
	for _, tc := range []struct {
		token  string
		status int
	}{
		{"", http.StatusUnauthorized}, {"invalid", http.StatusUnauthorized}, {"forbidden", http.StatusForbidden}, {"allowed", http.StatusAccepted},
	} {
		response := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, "/authorized-api", nil)
		request.Header.Set("X-UI", "enhanced")
		if tc.token != "" {
			request.Header.Set("Authorization", "Bearer "+tc.token)
		}
		ServeContent(response, request)
		if response.Code != tc.status {
			t.Fatalf("token %q: status=%d body=%q", tc.token, response.Code, response.Body.String())
		}
		if tc.token == "allowed" {
			if !strings.Contains(response.Body.String(), "protected") || response.Header().Get("X-Allowed") != "yes" || response.Header().Get("X-Denied") != "" {
				t.Fatalf("allowed response retained a prior denial: headers=%v body=%q", response.Header(), response.Body.String())
			}
		} else if response.Header().Get("X-Denied") != "yes" || response.Header().Get("X-Allowed") != "" || strings.Contains(response.Body.String(), "protected") {
			t.Fatalf("denied request reached the route: headers=%v body=%q", response.Header(), response.Body.String())
		}
		assertHTTPVary(t, response.Header(), "Cookie", "Authorization", "X-UI")
	}
	if got := atomic.LoadInt32(&authCalls); got != 3 {
		t.Fatalf("authorization calls=%d, want 3 after authentication", got)
	}
	if got := atomic.LoadInt32(&upstreamCalls); got != 1 {
		t.Fatalf("protected upstream calls=%d, want 1 for the authorized request", got)
	}
	htmlCacheMutex.RLock()
	defer htmlCacheMutex.RUnlock()
	if len(htmlCache) != 0 {
		t.Fatalf("guarded requests populated the live cache: %v", htmlCache)
	}
}

func TestServeContent_InvalidHTTPConfigurationFailsBeforeRender(t *testing.T) {
	for _, configType := range []string{composite.HyperMediaConfigGetName(), composite.FragmentConfigGetName(), composite.ApiFragmentRenderConfigGetName()} {
		t.Run(configType, func(t *testing.T) {
			setupLiveModeServeContentTest(t)
			var calls int32
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				atomic.AddInt32(&calls, 1)
				w.Header().Set("Content-Type", "application/json")
				_, _ = io.WriteString(w, `{"message":"protected"}`)
			}))
			defer upstream.Close()
			rm.SetPlugin("invalid_config_child", handledResponseTestPlugin{calls: &calls, body: []byte("protected")})
			for _, tc := range []struct {
				name  string
				field string
				value interface{}
			}{
				{"status below final range", "response", map[string]interface{}{"status": 199}},
				{"status above range", "response", map[string]interface{}{"status": 600}},
				{"fractional status", "response", map[string]interface{}{"status": 200.5}},
				{"non-numeric status", "response", map[string]interface{}{"status": "accepted"}},
				{"removed response field", "response", map[string]interface{}{"hx_retarget": "#result"}},
				{"misspelled response field", "response", map[string]interface{}{"header": map[string]interface{}{"X-Test": "yes"}}},
				{"invalid header name", "response", map[string]interface{}{"headers": map[string]interface{}{"X Bad": "value"}}},
				{"metadata name cannot hide invalid header", "response", map[string]interface{}{"headers": map[string]interface{}{"@order": "invalid"}}},
				{"header newline", "response", map[string]interface{}{"headers": map[string]interface{}{"X-Test": "value\r\nX-Injected: yes"}}},
				{"duplicate header names", "response", map[string]interface{}{"headers": map[string]interface{}{"X-Test": "one", "x-test": "two"}}},
				{"malformed response", "response", "invalid"},
				{"unknown guard field", "guard", map[string]interface{}{"enabled": true, "requirements": map[string]interface{}{"authenticated": true}}},
				{"removed guard action", "guard", map[string]interface{}{"enabled": true, "on_unauthenticated": map[string]interface{}{"redirect": "/login"}}},
				{"invalid unselected guard response", "guard", map[string]interface{}{"enabled": true, "require": map[string]interface{}{"authenticated": true}, "on_forbidden": map[string]interface{}{"default": map[string]interface{}{"status": 600}}}},
				{"fractional guard status", "guard", map[string]interface{}{"enabled": true, "require": map[string]interface{}{"authenticated": true}, "on_unauthenticated": map[string]interface{}{"default": map[string]interface{}{"status": 401.5}}}},
				{"empty variant condition", "guard", map[string]interface{}{"enabled": true, "on_unauthenticated": map[string]interface{}{"variants": []interface{}{map[string]interface{}{"when": map[string]interface{}{"request_headers": map[string]interface{}{}}, "response": map[string]interface{}{"status": http.StatusUnauthorized}}}}}},
				{"unknown variant condition", "guard", map[string]interface{}{"enabled": true, "on_unauthenticated": map[string]interface{}{"variants": []interface{}{map[string]interface{}{"when": map[string]interface{}{"header": map[string]interface{}{"X-UI": "enhanced"}}, "response": map[string]interface{}{"status": http.StatusUnauthorized}}}}}},
				{"metadata name cannot hide invalid match header", "guard", map[string]interface{}{"enabled": true, "on_unauthenticated": map[string]interface{}{"variants": []interface{}{map[string]interface{}{"when": map[string]interface{}{"request_headers": map[string]interface{}{"@order": "invalid", "X-UI": "enhanced"}}, "response": map[string]interface{}{"status": http.StatusUnauthorized}}}}}},
			} {
				t.Run(tc.name, func(t *testing.T) {
					atomic.StoreInt32(&calls, 0)
					config := map[string]interface{}{
						"@type": configType, "route": "invalid-http", "endpoint": upstream.URL, "method": "GET", "inline": "protected",
						"10": map[string]interface{}{"@type": component.PluginRenderGetName(), "plugin": "invalid_config_child"},
					}
					config[tc.field] = tc.value
					setTestRouteConfig("invalid-http", config)
					for attempt := 0; attempt < 2; attempt++ {
						response := httptest.NewRecorder()
						ServeContent(response, httptest.NewRequest(http.MethodGet, "/invalid-http", nil))
						if response.Code != http.StatusInternalServerError || response.Header().Get("Cache-Control") != "no-store" {
							t.Fatalf("attempt %d: status=%d headers=%v body=%q", attempt+1, response.Code, response.Header(), response.Body.String())
						}
						if atomic.LoadInt32(&calls) != 0 || strings.Contains(response.Body.String(), "protected") {
							t.Fatalf("invalid configuration rendered a child or called upstream: calls=%d body=%q", atomic.LoadInt32(&calls), response.Body.String())
						}
						if _, found := cachedEntry("invalid-http"); found {
							t.Fatal("invalid configuration populated the live cache")
						}
					}
				})
			}
		})
	}
}

func TestServeContent_APIFragmentSeparatesUpstreamAndBrowserHeaders(t *testing.T) {
	setupLiveModeServeContentTest(t)
	// This test uses literal loopback HTTP; production upstream secrets require HTTPS.
	shared.GetHyperBricksConfiguration().Mode = shared.DEVELOPMENT_MODE
	var upstreamHeaders http.Header
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamHeaders = r.Header.Clone()
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"message":"created"}`)
	}))
	defer upstream.Close()
	setTestRouteConfig("api-response", map[string]interface{}{
		"@type": composite.ApiFragmentRenderConfigGetName(), "route": "api-response", "endpoint": upstream.URL, "method": "POST", "inline": `{{ .Data.message }}`,
		"headers":   map[string]interface{}{"X-Direction": "upstream", "X-Upstream-Only": "secret"},
		"response":  map[string]interface{}{"status": http.StatusCreated, "headers": map[string]interface{}{"X-Direction": "browser", "X-Browser-Only": "public", "Set-Cookie": "response=1; Path=/"}},
		"setcookie": "session=1; Path=/; HttpOnly", "setcookies": []interface{}{"theme=light; Path=/"},
	})
	response := httptest.NewRecorder()
	ServeContent(response, httptest.NewRequest(http.MethodPost, "/api-response", nil))
	if response.Code != http.StatusCreated || !strings.Contains(response.Body.String(), "created") {
		t.Fatalf("browser status=%d body=%q", response.Code, response.Body.String())
	}
	if upstreamHeaders.Get("X-Direction") != "upstream" || upstreamHeaders.Get("X-Upstream-Only") != "secret" || upstreamHeaders.Get("X-Browser-Only") != "" || upstreamHeaders.Get("Set-Cookie") != "" {
		t.Fatalf("browser response metadata leaked upstream: %v", upstreamHeaders)
	}
	if response.Header().Get("X-Direction") != "browser" || response.Header().Get("X-Browser-Only") != "public" || response.Header().Get("X-Upstream-Only") != "" {
		t.Fatalf("upstream request headers leaked to browser: %v", response.Header())
	}
	assertHTTPCookies(t, response.Header(), "response=1; Path=/", "session=1; Path=/; HttpOnly", "theme=light; Path=/")
}

func TestServeContent_ResponseMetadataPrecedence(t *testing.T) {
	for _, explicitContentType := range []string{"", "application/xhtml+xml"} {
		t.Run("content_type="+explicitContentType, func(t *testing.T) {
			setupLiveModeServeContentTest(t)
			setTestRouteConfig("response-priority", map[string]interface{}{
				"@type": composite.HyperMediaConfigGetName(), "route": "response-priority", "content_type": explicitContentType,
				"headers":  map[string]interface{}{"X-Priority": "legacy", "Content-Type": "text/html", "Set-Cookie": "legacy=1; Path=/", "X-Legacy": "retained"},
				"response": map[string]interface{}{"status": http.StatusAccepted, "headers": map[string]interface{}{"x-priority": "response", "content-type": "text/plain", "set-cookie": "response=1; Path=/"}},
				"cookies":  []interface{}{"first=1; Path=/", "second=2; Path=/"},
				"template": map[string]interface{}{"@type": composite.TemplateConfigGetName(), "inline": "priority", "values": map[string]interface{}{}},
			})
			for attempt := 0; attempt < 2; attempt++ {
				response := httptest.NewRecorder()
				ServeContent(response, httptest.NewRequest(http.MethodGet, "/response-priority", nil))
				wantContentType := explicitContentType
				if wantContentType == "" {
					wantContentType = "text/plain"
				}
				if response.Code != http.StatusAccepted || response.Header().Get("X-Priority") != "response" || response.Header().Get("X-Legacy") != "retained" || response.Header().Get("Content-Type") != wantContentType {
					t.Fatalf("attempt %d: status=%d headers=%v", attempt+1, response.Code, response.Header())
				}
				assertHTTPCookies(t, response.Header(), "response=1; Path=/", "first=1; Path=/", "second=2; Path=/")
			}
		})
	}
}

func TestServeContent_HandledPluginOverridesRouteResponseMetadata(t *testing.T) {
	setupLiveModeServeContentTest(t)
	setTestRouteConfig("handled-response-priority", map[string]interface{}{
		"@type": composite.HyperMediaConfigGetName(), "route": "handled-response-priority", "content_type": "text/plain",
		"headers":  map[string]interface{}{"X-Priority": "legacy"},
		"response": map[string]interface{}{"status": http.StatusAccepted, "headers": map[string]interface{}{"x-priority": "route", "Content-Type": "text/html", "Set-Cookie": "response=1; Path=/", "X-Route": "retained"}},
		"cookies":  []interface{}{"route=1; Path=/"},
		"10":       map[string]interface{}{"@type": component.PluginRenderGetName(), "plugin": "handled_response_priority"},
	})
	var calls int32
	rm.SetPlugin("handled_response_priority", handledResponseTestPlugin{
		calls: &calls, status: http.StatusCreated, contentType: "application/octet-stream",
		headers: map[string]string{"X-PRIORITY": "plugin", "set-cookie": "plugin-header=1; Path=/"},
		cookies: []string{"plugin=1; Path=/"}, body: []byte{0, 1, 2},
	})
	for attempt := 0; attempt < 2; attempt++ {
		response := httptest.NewRecorder()
		ServeContent(response, httptest.NewRequest(http.MethodGet, "/handled-response-priority", nil))
		if response.Code != http.StatusCreated || response.Header().Get("X-Priority") != "plugin" || response.Header().Get("Content-Type") != "application/octet-stream" || response.Header().Get("X-Route") != "retained" || response.Body.String() != string([]byte{0, 1, 2}) {
			t.Fatalf("plugin response metadata lost precedence: status=%d headers=%v body=%v", response.Code, response.Header(), response.Body.Bytes())
		}
		assertHTTPCookies(t, response.Header(), "plugin-header=1; Path=/", "route=1; Path=/", "plugin=1; Path=/")
	}
	if got := atomic.LoadInt32(&calls); got != 2 {
		t.Fatalf("plugin calls=%d, want 2", got)
	}
	if _, found := cachedEntry("handled-response-priority"); found {
		t.Fatal("handled plugin response populated live cache")
	}
}

func TestServeContent_ConfiguredBodylessResponses(t *testing.T) {
	for _, live := range []bool{false, true} {
		for _, guarded := range []bool{false, true} {
			for _, status := range []int{http.StatusNoContent, http.StatusResetContent, http.StatusNotModified} {
				t.Run(fmt.Sprintf("live=%t/guard=%t/status=%d", live, guarded, status), func(t *testing.T) {
					if live {
						setupLiveModeServeContentTest(t)
					} else {
						setupDevelopmentModeServeContentTest(t, true)
					}
					responseConfig := map[string]interface{}{"status": status, "headers": map[string]interface{}{"X-Bodyless": "configured"}}
					config := map[string]interface{}{
						"@type": composite.FragmentConfigGetName(), "route": "bodyless", "response": responseConfig,
						"template": map[string]interface{}{"@type": composite.TemplateConfigGetName(), "inline": "body must not be sent", "values": map[string]interface{}{}},
					}
					if guarded {
						delete(config, "response")
						config["guard"] = map[string]interface{}{"enabled": true, "require": map[string]interface{}{"authenticated": true}, "on_unauthenticated": map[string]interface{}{"default": responseConfig}}
					}
					setTestRouteConfig("bodyless", config)
					for attempt := 0; attempt < 2; attempt++ {
						response := httptest.NewRecorder()
						ServeContent(response, httptest.NewRequest(http.MethodGet, "/bodyless", nil))
						if response.Code != status || response.Body.Len() != 0 || response.Header().Get("X-Bodyless") != "configured" {
							t.Fatalf("attempt %d: status=%d headers=%v body=%q", attempt+1, response.Code, response.Header(), response.Body.String())
						}
					}
				})
			}
		}
	}
}

func TestServeContent_HTTPMetadataIsolatedAcrossRenderPipelines(t *testing.T) {
	for _, compiled := range []bool{false, true} {
		t.Run(fmt.Sprintf("compiled=%t", compiled), func(t *testing.T) {
			fixture := setupSSRProofPipelineBenchmark(t)
			original, _, _ := getConfigAndPlan("index")
			source := shared.CloneMapDeep(original)
			source["response"] = map[string]interface{}{"status": http.StatusAccepted, "headers": map[string]interface{}{"X-Metadata": "source"}}
			source["guard"] = map[string]interface{}{
				"enabled": true, "require": map[string]interface{}{"authenticated": true},
				"on_unauthenticated": map[string]interface{}{
					"default": map[string]interface{}{"status": http.StatusSeeOther, "headers": map[string]interface{}{"Location": "/login"}},
					"variants": []interface{}{map[string]interface{}{
						"when":     map[string]interface{}{"request_headers": map[string]interface{}{"X-UI": "enhanced"}},
						"response": map[string]interface{}{"status": http.StatusUnauthorized, "headers": map[string]interface{}{"X-Denied": "enhanced"}},
					}},
				},
			}
			plan, err := renderplan.Compile(rm, source, parser.GetTemplate)
			if err != nil {
				t.Fatalf("compile route with HTTP response and guard: %v", err)
			}
			setTestRouteConfig("index", source)
			if compiled {
				configMutex.Lock()
				routePlans["index"] = plan
				configMutex.Unlock()
			}
			before := configOwnershipSnapshot(t, source)
			var wg sync.WaitGroup
			for i := 0; i < 24; i++ {
				wg.Add(1)
				go func(i int) {
					defer wg.Done()
					rid := fmt.Sprintf("request-%d", i)
					request := httptest.NewRequest(http.MethodGet, "/?rid="+rid, nil)
					response := httptest.NewRecorder()
					switch i % 3 {
					case 0:
						request.Header.Set("Authorization", "Bearer "+rid)
					case 1:
						request.Header.Set("X-UI", "enhanced")
					}
					ServeContent(response, request)
					switch i % 3 {
					case 0:
						want := strings.ReplaceAll(fixture.expected, "benchmark-request", template.HTMLEscapeString(rid))
						if response.Code != http.StatusAccepted || response.Body.String() != want || response.Header().Get("X-Metadata") != "source" || response.Header().Get("X-Denied") != "" || response.Header().Get("Location") != "" {
							t.Errorf("authorized request %d mixed metadata or body: status=%d headers=%v", i, response.Code, response.Header())
						}
					case 1:
						if response.Code != http.StatusUnauthorized || response.Header().Get("X-Denied") != "enhanced" || response.Header().Get("Location") != "" || response.Header().Get("X-Metadata") != "" {
							t.Errorf("enhanced denial %d mixed metadata: status=%d headers=%v", i, response.Code, response.Header())
						}
					case 2:
						if response.Code != http.StatusSeeOther || response.Header().Get("Location") != "/login" || response.Header().Get("X-Denied") != "" || response.Header().Get("X-Metadata") != "" {
							t.Errorf("default denial %d mixed metadata: status=%d headers=%v", i, response.Code, response.Header())
						}
					}
				}(i)
			}
			wg.Wait()
			if got := configOwnershipSnapshot(t, source); got != before {
				t.Fatal("concurrent requests mutated the shared response or guard configuration")
			}
			htmlCacheMutex.RLock()
			defer htmlCacheMutex.RUnlock()
			if len(htmlCache) != 0 {
				t.Fatal("guarded compiled or legacy requests populated the live cache")
			}
		})
	}
}

func TestServeContent_ParsedYAMLHTTPResponseAndGuard(t *testing.T) {
	setupLiveModeServeContentTest(t)
	doc, err := yamlparser.ParseBytes([]byte(`
page:
  - type: hypermedia
  - route: parsed-response
  - beautify: false
  - response:
      - status: 202
      - headers:
          X-Route: parsed
          HX-Retarget: "#result"
  - guard:
      - enabled: true
      - require:
          - authenticated: true
      - on_unauthenticated:
          - default:
              - status: 303
              - headers:
                  Location: /login
          - variants:
              - when:
                  request_headers:
                    HX-Request: "true"
                response:
                  status: 401
                  headers:
                    HX-Redirect: /login
  - template:
      - type: template
      - inline: <p>parsed configuration</p>
      - values: {}
`))
	if err != nil {
		t.Fatalf("parse structured YAML: %v", err)
	}
	materialized, err := doc.Materialize()
	if err != nil {
		t.Fatalf("materialize structured YAML: %v", err)
	}
	source := materialized["page"].(map[string]interface{})
	before := configOwnershipSnapshot(t, source)
	plan, err := renderplan.Compile(rm, source, parser.GetTemplate)
	if err != nil {
		t.Fatalf("compile parsed HTTP configuration: %v", err)
	}
	for _, compiled := range []bool{false, true} {
		setTestRouteConfig("parsed-response", source)
		if compiled {
			configMutex.Lock()
			routePlans["parsed-response"] = plan
			configMutex.Unlock()
		}
		for _, tc := range []struct {
			auth, enhanced bool
			status         int
		}{
			{false, false, http.StatusSeeOther}, {false, true, http.StatusUnauthorized}, {true, true, http.StatusAccepted},
		} {
			response := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodGet, "/parsed-response", nil)
			if tc.auth {
				request.Header.Set("Authorization", "Bearer parsed-token")
			}
			if tc.enhanced {
				request.Header.Set("HX-Request", "true")
			}
			ServeContent(response, request)
			if response.Code != tc.status {
				t.Fatalf("compiled=%t auth=%t enhanced=%t: status=%d body=%q", compiled, tc.auth, tc.enhanced, response.Code, response.Body.String())
			}
			switch tc.status {
			case http.StatusSeeOther:
				if response.Header().Get("Location") != "/login" || response.Header().Get("HX-Redirect") != "" {
					t.Fatalf("parsed default headers=%v", response.Header())
				}
			case http.StatusUnauthorized:
				if response.Header().Get("HX-Redirect") != "/login" || response.Header().Get("Location") != "" {
					t.Fatalf("parsed variant headers=%v", response.Header())
				}
			case http.StatusAccepted:
				if response.Header().Get("X-Route") != "parsed" || response.Header().Get("HX-Retarget") != "#result" || response.Body.String() != "<p>parsed configuration</p>" {
					t.Fatalf("parsed response headers=%v body=%q", response.Header(), response.Body.String())
				}
			}
			assertHTTPVary(t, response.Header(), "Cookie", "Authorization", "HX-Request")
		}
	}
	if got := configOwnershipSnapshot(t, source); got != before {
		t.Fatal("serving parsed HTTP configuration mutated the materialized YAML")
	}
}

func TestServeContent_ConfiguredVarySeparatesLiveCache(t *testing.T) {
	for _, legacy := range []bool{false, true} {
		t.Run(fmt.Sprintf("legacy_headers=%t", legacy), func(t *testing.T) {
			setupLiveModeServeContentTest(t)
			rm.SetPlugin("vary_echo", requestEchoTestPlugin{})
			source := map[string]interface{}{
				"@type": composite.HyperMediaConfigGetName(), "route": "vary-cache", "beautify": false,
				"10": map[string]interface{}{"@type": component.PluginRenderGetName(), "plugin": "vary_echo"},
			}
			headers := map[string]interface{}{"vary": "x-snapshot"}
			if legacy {
				source["headers"] = headers
			} else {
				source["response"] = map[string]interface{}{"headers": headers}
			}
			setTestRouteConfig("vary-cache", source)
			var firstEntry CacheEntry
			for i, tc := range []struct{ name, value string }{{"X-Snapshot", "en"}, {"x-snapshot", "nl"}, {"X-SNAPSHOT", "EN"}, {"x-snapshot", "en"}} {
				request := httptest.NewRequest(http.MethodGet, "/vary-cache", nil)
				request.Header.Set(tc.name, tc.value)
				response := httptest.NewRecorder()
				ServeContent(response, request)
				if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "header="+tc.value) {
					t.Fatalf("request %d reused a different Vary value: status=%d body=%q", i, response.Code, response.Body.String())
				}
				assertHTTPVary(t, response.Header(), "X-Snapshot")
				key, cacheable := resolveLiveCacheKey("vary-cache", request)
				entry, found := cachedEntry(key)
				if !cacheable || !found {
					t.Fatalf("request %d did not populate a cache variant", i)
				}
				if i == 0 {
					firstEntry = entry
				} else if i == 3 && !entry.Timestamp.Equal(firstEntry.Timestamp) {
					t.Fatal("HTTP header name casing prevented reuse of the matching cache variant")
				}
			}
			htmlCacheMutex.RLock()
			defer htmlCacheMutex.RUnlock()
			if len(htmlCache) != 3 {
				t.Fatalf("cache variants=%d, want 3 distinct case-sensitive header values", len(htmlCache))
			}
		})
	}
}

func TestServeContent_VaryStarBypassesLiveCache(t *testing.T) {
	setupLiveModeServeContentTest(t)
	rm.SetPlugin("vary_star_echo", requestEchoTestPlugin{})
	setTestRouteConfig("vary-star", map[string]interface{}{
		"@type": composite.FragmentConfigGetName(), "route": "vary-star", "nocache": false,
		"response": map[string]interface{}{"headers": map[string]interface{}{"Vary": "*"}},
		"10":       map[string]interface{}{"@type": component.PluginRenderGetName(), "plugin": "vary_star_echo"},
	})
	for _, value := range []string{"first", "second"} {
		response := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, "/vary-star", nil)
		request.Header.Set("X-Snapshot", value)
		ServeContent(response, request)
		if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "header="+value) || response.Header().Get("Vary") != "*" {
			t.Fatalf("Vary:* response used a previous request: status=%d headers=%v body=%q", response.Code, response.Header(), response.Body.String())
		}
		if response.Header().Get("ETag") != "" || response.Header().Get(liveCacheRenderedAtHeader) != "" {
			t.Fatalf("Vary:* response received cache metadata: %v", response.Header())
		}
	}
	htmlCacheMutex.RLock()
	defer htmlCacheMutex.RUnlock()
	if len(htmlCache) != 0 {
		t.Fatalf("Vary:* populated %d cache entries", len(htmlCache))
	}
}

func TestServeContent_GuardQueryOrderKeyRemainsARequirement(t *testing.T) {
	setupLiveModeServeContentTest(t)
	setTestRouteConfig("guard-query-key", map[string]interface{}{
		"@type": composite.FragmentConfigGetName(), "route": "guard-query-key",
		"guard":    map[string]interface{}{"enabled": true, "require": map[string]interface{}{"query": map[string]interface{}{"@order": true}}},
		"template": map[string]interface{}{"@type": composite.TemplateConfigGetName(), "inline": "protected", "values": map[string]interface{}{}},
	})
	for _, tc := range []struct {
		path   string
		status int
	}{
		{"/guard-query-key", http.StatusForbidden},
		{"/guard-query-key?%40order=present", http.StatusOK},
	} {
		response := httptest.NewRecorder()
		ServeContent(response, httptest.NewRequest(http.MethodGet, tc.path, nil))
		if response.Code != tc.status {
			t.Fatalf("path=%q status=%d body=%q", tc.path, response.Code, response.Body.String())
		}
		if tc.status == http.StatusForbidden && strings.Contains(response.Body.String(), "protected") {
			t.Fatal("removing internal ordering metadata bypassed the user-defined @order query requirement")
		}
	}
}

func TestServeContent_HostSelectsGuardResponseAndCacheVariant(t *testing.T) {
	setupLiveModeServeContentTest(t)
	rm.SetPlugin("host_echo", requestEchoTestPlugin{})
	setTestRouteConfig("guard-host", map[string]interface{}{
		"@type": composite.FragmentConfigGetName(), "route": "guard-host",
		"guard": map[string]interface{}{
			"enabled": true, "require": map[string]interface{}{"authenticated": true},
			"on_unauthenticated": map[string]interface{}{
				"default": map[string]interface{}{"status": http.StatusSeeOther, "headers": map[string]interface{}{"Location": "/login"}},
				"variants": []interface{}{map[string]interface{}{
					"when":     map[string]interface{}{"request_headers": map[string]interface{}{"host": "tenant.example"}},
					"response": map[string]interface{}{"status": http.StatusUnauthorized, "headers": map[string]interface{}{"X-Tenant": "selected"}},
				}},
			},
		},
	})
	for _, host := range []string{"tenant.example", "other.example"} {
		request := httptest.NewRequest(http.MethodGet, "http://"+host+"/guard-host", nil)
		response := httptest.NewRecorder()
		ServeContent(response, request)
		want := http.StatusSeeOther
		if host == "tenant.example" {
			want = http.StatusUnauthorized
			if response.Header().Get("X-Tenant") != "selected" {
				t.Fatalf("Host variant not selected: %v", response.Header())
			}
		}
		if response.Code != want {
			t.Fatalf("Host=%q status=%d, want %d", host, response.Code, want)
		}
		assertHTTPVary(t, response.Header(), "Cookie", "Authorization", "Host")
	}
	setTestRouteConfig("host-cache", map[string]interface{}{
		"@type": composite.FragmentConfigGetName(), "route": "host-cache", "beautify": false,
		"response": map[string]interface{}{"headers": map[string]interface{}{"Vary": "Host"}},
		"10":       map[string]interface{}{"@type": component.PluginRenderGetName(), "plugin": "host_echo"},
	})
	for _, host := range []string{"tenant.example", "other.example", "tenant.example"} {
		request := httptest.NewRequest(http.MethodGet, "http://"+host+"/host-cache", nil)
		response := httptest.NewRecorder()
		ServeContent(response, request)
		if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "host="+host+" ") {
			t.Fatalf("Host=%q reused the wrong host cache: status=%d body=%q", host, response.Code, response.Body.String())
		}
		assertHTTPVary(t, response.Header(), "Host")
	}
	htmlCacheMutex.RLock()
	defer htmlCacheMutex.RUnlock()
	if len(htmlCache) != 2 {
		t.Fatalf("Host variants=%d, want 2", len(htmlCache))
	}
}

func assertHTTPCookies(t *testing.T, headers http.Header, want ...string) {
	t.Helper()
	got := headers.Values("Set-Cookie")
	if len(got) != len(want) {
		t.Fatalf("Set-Cookie=%v, want %v", got, want)
	}
	for _, cookie := range want {
		found := false
		for _, actual := range got {
			if actual == cookie {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("Set-Cookie=%v, missing %q", got, cookie)
		}
	}
}

func assertHTTPVary(t *testing.T, headers http.Header, want ...string) {
	t.Helper()
	got := map[string]bool{}
	for _, line := range headers.Values("Vary") {
		for _, name := range strings.Split(line, ",") {
			name = http.CanonicalHeaderKey(strings.TrimSpace(name))
			if name != "" {
				got[name] = true
			}
		}
	}
	expected := map[string]bool{}
	for _, name := range want {
		expected[http.CanonicalHeaderKey(name)] = true
	}
	if !reflect.DeepEqual(got, expected) {
		t.Fatalf("Vary=%v, want exactly %v", headers.Values("Vary"), want)
	}
}
