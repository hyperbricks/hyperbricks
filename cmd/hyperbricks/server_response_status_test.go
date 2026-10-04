package main

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/hyperbricks/hyperbricks/pkg/component"
	"github.com/hyperbricks/hyperbricks/pkg/composite"
)

// The three mounts exercise both renderers and both HTTP ownership contexts of
// API_FRAGMENT_RENDER. Each call returns fresh maps for request isolation.
func responseStatusTestRoute(mount, endpoint string, policy interface{}) (map[string]interface{}, map[string]interface{}) {
	api := map[string]interface{}{
		"@type": component.APIConfigGetName(), "endpoint": endpoint,
		"method": http.MethodGet, "inline": `status={{.Status}};message={{.Data.message}}`,
	}
	if policy != nil {
		api["response_status"] = policy
	}
	if mount != "api_render" {
		api["@type"] = composite.ApiFragmentRenderConfigGetName()
		api["route"] = "response-status-fragment"
	}
	if mount == "direct_api_fragment" {
		api["route"] = "response-status"
		api["beautify"] = false
		return api, api
	}
	return map[string]interface{}{
		"@type": composite.FragmentConfigGetName(), "route": "response-status", "beautify": false, "10": api,
	}, api
}

func serveResponseStatusTest(method string, headers http.Header) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, "/response-status", nil)
	if headers != nil {
		r.Header = headers.Clone()
	}
	w := httptest.NewRecorder()
	ServeContent(w, r)
	return w
}

func TestServeContent_ResponseStatusOptionalAndFallback(t *testing.T) {
	for _, mount := range []string{"api_render", "direct_api_fragment", "nested_api_fragment"} {
		for _, tc := range []struct {
			name   string
			policy interface{}
			status int
		}{
			{name: "omitted default", status: 200},
			{name: "omitted literal fallback", status: 202},
			{name: "disabled required map", status: 202, policy: map[string]interface{}{"enabled": false, "required": true, "map": map[string]interface{}{"404": 410}}},
			{name: "enabled unmapped optional", status: 202, policy: map[string]interface{}{}},
			{name: "explicit ignore required", status: 202, policy: map[string]interface{}{"required": true, "map": map[string]interface{}{"404": "ignore"}}},
		} {
			t.Run(mount+"/"+tc.name, func(t *testing.T) {
				setupLiveModeServeContentTest(t)
				upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusNotFound)
					_, _ = io.WriteString(w, `{"message":"missing"}`)
				}))
				t.Cleanup(upstream.Close)
				config, _ := responseStatusTestRoute(mount, upstream.URL, tc.policy)
				if tc.status != 200 {
					config["response"] = map[string]interface{}{"status": tc.status}
				}
				setTestRouteConfig("response-status", config)
				response := serveResponseStatusTest(http.MethodGet, nil)
				if response.Code != tc.status || response.Body.String() != "status=404;message=missing" {
					t.Fatalf("status=%d body=%q, want fallback %d with upstream content", response.Code, response.Body.String(), tc.status)
				}
			})
		}
	}
}

func TestServeContent_ResponseStatusMappedPreservesBodyAndBypassesCache(t *testing.T) {
	for _, mount := range []string{"api_render", "direct_api_fragment", "nested_api_fragment"} {
		t.Run(mount, func(t *testing.T) {
			setupLiveModeServeContentTest(t)
			var calls atomic.Int32
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusNotFound)
				_, _ = io.WriteString(w, `{"message":"missing"}`)
			}))
			t.Cleanup(upstream.Close)
			config, _ := responseStatusTestRoute(mount, upstream.URL, map[string]interface{}{"map": map[string]interface{}{"404": 410}})
			config["response"] = map[string]interface{}{"status": 202, "headers": map[string]interface{}{
				"Cache-Control": "public, max-age=3600", "ETag": `"obsolete"`, "X-Route": "retained",
			}}
			setTestRouteConfig("response-status", config)
			for _, method := range []string{http.MethodGet, http.MethodGet} {
				response := serveResponseStatusTest(method, http.Header{"If-None-Match": {`"obsolete"`}})
				if response.Code != http.StatusGone || response.Header().Get("Cache-Control") != "no-store" || response.Header().Get("ETag") != "" || response.Header().Get("X-Route") != "retained" {
					t.Fatalf("%s status=%d headers=%v", method, response.Code, response.Header())
				}
				wantBody := "status=404;message=missing"
				if response.Body.String() != wantBody {
					t.Fatalf("%s body=%q, want %q", method, response.Body.String(), wantBody)
				}
				if response.Header().Get(liveCacheRenderedAtHeader) != "" || response.Header().Get(liveCacheExpiresAtHeader) != "" {
					t.Fatalf("mapped response advertises cached output: %v", response.Header())
				}
			}
			if calls.Load() != 2 {
				t.Fatalf("upstream calls=%d, want fresh execution for both requests", calls.Load())
			}
			if _, found := cachedEntry("response-status"); found {
				t.Fatal("mapped response entered rendered output cache")
			}
		})
	}
}

func TestServeContent_ResponseStatusRequiredFailures(t *testing.T) {
	for _, mount := range []string{"api_render", "direct_api_fragment", "nested_api_fragment"} {
		for _, failure := range []string{"unmapped upstream", "decode", "template", "local config", "transport"} {
			t.Run(mount+"/"+failure, func(t *testing.T) {
				setupLiveModeServeContentTest(t)
				upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Content-Type", "application/json")
					if failure == "unmapped upstream" {
						w.WriteHeader(http.StatusUnauthorized)
					}
					if failure == "decode" {
						_, _ = io.WriteString(w, `{"message":`)
					} else {
						_, _ = io.WriteString(w, `{"message":"ok"}`)
					}
				}))
				t.Cleanup(upstream.Close)
				if failure == "transport" {
					upstream.Close()
				}
				config, api := responseStatusTestRoute(mount, upstream.URL, map[string]interface{}{
					"required": true, "map": map[string]interface{}{"502": 410},
				})
				wantStatus := http.StatusBadGateway
				if failure == "template" {
					api["inline"], wantStatus = "{{", http.StatusInternalServerError
				}
				if failure == "local config" {
					delete(api, "method")
					wantStatus = http.StatusInternalServerError
				}
				setTestRouteConfig("response-status", config)
				response := serveResponseStatusTest(http.MethodGet, nil)
				if response.Code != wantStatus || response.Header().Get("Cache-Control") != "no-store" || response.Header().Get("ETag") != "" {
					t.Fatalf("status=%d headers=%v body=%q, want %d uncached", response.Code, response.Header(), response.Body.String(), wantStatus)
				}
				if response.Body.Len() == 0 {
					t.Fatal("required failure unexpectedly discarded rendered body")
				}
			})
		}
	}
}

func TestServeContent_ResponseStatusPrioritiesAndTies(t *testing.T) {
	for _, tc := range []struct {
		name           string
		secondPriority int
		secondStatus   int
		wantStatus     int
	}{
		{name: "higher priority", secondPriority: 2, secondStatus: 503, wantStatus: 503},
		{name: "different tied statuses", secondPriority: 1, secondStatus: 503, wantStatus: 500},
		{name: "matching tied statuses", secondPriority: 1, secondStatus: 404, wantStatus: 404},
	} {
		t.Run(tc.name, func(t *testing.T) {
			setupLiveModeServeContentTest(t)
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusNotFound)
				_, _ = io.WriteString(w, `{"message":"missing"}`)
			}))
			t.Cleanup(upstream.Close)
			config, _ := responseStatusTestRoute("api_render", upstream.URL, map[string]interface{}{"priority": 1, "map": map[string]interface{}{"404": 404}})
			_, second := responseStatusTestRoute("nested_api_fragment", upstream.URL, map[string]interface{}{"priority": tc.secondPriority, "map": map[string]interface{}{"404": tc.secondStatus}})
			config["20"] = second
			setTestRouteConfig("response-status", config)
			response := serveResponseStatusTest(http.MethodGet, nil)
			if response.Code != tc.wantStatus || response.Header().Get("Cache-Control") != "no-store" || strings.Count(response.Body.String(), "status=404;message=missing") != 2 {
				t.Fatalf("status=%d headers=%v body=%q, want status=%d with both bodies", response.Code, response.Header(), response.Body.String(), tc.wantStatus)
			}
		})
	}
}

func TestServeContent_ResponseStatusInvalidPolicyBeforeFetch(t *testing.T) {
	for _, mount := range []string{"api_render", "direct_api_fragment", "nested_api_fragment"} {
		for _, policy := range []interface{}{
			"true",
			map[string]interface{}{"unknown": true},
			map[string]interface{}{"enabled": "false"},
			map[string]interface{}{"priority": 1.5},
			map[string]interface{}{"map": map[string]interface{}{"404": 302}},
		} {
			t.Run(fmt.Sprintf("%s/%v", mount, policy), func(t *testing.T) {
				setupLiveModeServeContentTest(t)
				var calls atomic.Int32
				upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					calls.Add(1)
					_, _ = io.WriteString(w, `{"message":"ok"}`)
				}))
				t.Cleanup(upstream.Close)
				config, _ := responseStatusTestRoute(mount, upstream.URL, policy)
				setTestRouteConfig("response-status", config)
				response := serveResponseStatusTest(http.MethodGet, nil)
				if response.Code != http.StatusInternalServerError || response.Header().Get("Cache-Control") != "no-store" || calls.Load() != 0 {
					t.Fatalf("status=%d headers=%v upstream calls=%d; invalid policy must fail before fetching", response.Code, response.Header(), calls.Load())
				}
			})
		}
	}
}

func TestServeContent_ResponseStatusGuardAndPluginOwnership(t *testing.T) {
	t.Run("guard prevents API execution", func(t *testing.T) {
		setupLiveModeServeContentTest(t)
		var calls atomic.Int32
		upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls.Add(1) }))
		t.Cleanup(upstream.Close)
		config, _ := responseStatusTestRoute("direct_api_fragment", upstream.URL, map[string]interface{}{"required": true})
		config["guard"] = map[string]interface{}{
			"enabled": true, "require": map[string]interface{}{"authenticated": true},
			"on_unauthenticated": map[string]interface{}{"default": map[string]interface{}{"status": 401, "headers": map[string]interface{}{"X-Owner": "guard"}}},
		}
		setTestRouteConfig("response-status", config)
		response := serveResponseStatusTest(http.MethodGet, nil)
		if response.Code != http.StatusUnauthorized || response.Header().Get("X-Owner") != "guard" || calls.Load() != 0 {
			t.Fatalf("status=%d headers=%v API calls=%d", response.Code, response.Header(), calls.Load())
		}
	})
	t.Run("handled plugin keeps response ownership", func(t *testing.T) {
		setupLiveModeServeContentTest(t)
		upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotFound)
			_, _ = io.WriteString(w, `{"message":"missing"}`)
		}))
		t.Cleanup(upstream.Close)
		config, _ := responseStatusTestRoute("api_render", upstream.URL, map[string]interface{}{"map": map[string]interface{}{"404": 410}})
		config["20"] = map[string]interface{}{"@type": component.PluginRenderGetName(), "plugin": "response_status_owner"}
		config["response"] = map[string]interface{}{"status": 202}
		rm.SetPlugin("response_status_owner", handledResponseTestPlugin{status: 201, body: []byte("plugin body"), headers: map[string]string{"X-Owner": "plugin"}})
		setTestRouteConfig("response-status", config)
		response := serveResponseStatusTest(http.MethodGet, nil)
		if response.Code != http.StatusCreated || response.Body.String() != "plugin body" || response.Header().Get("X-Owner") != "plugin" {
			t.Fatalf("status=%d headers=%v body=%q", response.Code, response.Header(), response.Body.String())
		}
	})
}

func TestServeContent_ResponseStatusDiscardsSuccessfulSiblingCookies(t *testing.T) {
	setupLiveModeServeContentTest(t)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/missing" {
			w.WriteHeader(http.StatusNotFound)
		}
		_, _ = io.WriteString(w, `{"message":"ok","token":"secret-token"}`)
	}))
	t.Cleanup(upstream.Close)
	config, _ := responseStatusTestRoute("nested_api_fragment", upstream.URL+"/missing", map[string]interface{}{"map": map[string]interface{}{"404": 404}})
	_, successful := responseStatusTestRoute("nested_api_fragment", upstream.URL+"/success", nil)
	successful["setcookie"] = "session={{.Data.token}}; Path=/; HttpOnly; SameSite=Lax"
	config["20"] = successful
	setTestRouteConfig("response-status", config)
	response := serveResponseStatusTest(http.MethodGet, nil)
	if response.Code != http.StatusNotFound || len(response.Header().Values("Set-Cookie")) != 0 || strings.Count(response.Body.String(), "message=ok") != 2 {
		t.Fatalf("status=%d headers=%v body=%q; dynamic error must discard staged sibling cookies", response.Code, response.Header(), response.Body.String())
	}
}

func TestServeContent_ResponseStatusPreflightImplicitHeadAndFieldCasing(t *testing.T) {
	for _, shape := range []string{"implicit head", "case-confusable field"} {
		t.Run(shape, func(t *testing.T) {
			setupLiveModeServeContentTest(t)
			var calls atomic.Int32
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				w.Header().Set("Content-Type", "application/json")
				_, _ = io.WriteString(w, `{"message":"ok"}`)
			}))
			t.Cleanup(upstream.Close)
			config, api := responseStatusTestRoute("api_render", upstream.URL, map[string]interface{}{"enabled": "false"})
			if shape == "implicit head" {
				delete(config, "10")
				config["@type"] = composite.HyperMediaConfigGetName()
				config["head"] = map[string]interface{}{"10": api}
				_, validSibling := responseStatusTestRoute("api_render", upstream.URL, nil)
				config["20"] = validSibling
			} else {
				delete(api, "response_status")
				api["Response_Status"] = map[string]interface{}{"enabled": true, "map": map[string]interface{}{"404": 404}}
			}
			setTestRouteConfig("response-status", config)
			response := serveResponseStatusTest(http.MethodGet, nil)
			if response.Code != http.StatusInternalServerError || calls.Load() != 0 {
				t.Fatalf("status=%d body=%q upstream calls=%d; malformed policy must fail before every fetch", response.Code, response.Body.String(), calls.Load())
			}
		})
	}
}
