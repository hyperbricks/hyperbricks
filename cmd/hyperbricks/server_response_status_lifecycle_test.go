package main

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/hyperbricks/hyperbricks/pkg/component"
	"github.com/hyperbricks/hyperbricks/pkg/composite"
)

func responseStatusLifecycleRoute(kind, endpoint string) map[string]interface{} {
	api := map[string]interface{}{
		"@type": kind, "endpoint": endpoint, "method": "GET",
		"querykeys": []string{"status"}, "inline": "upstream={{.Status}}",
		"response_status": map[string]interface{}{"map": map[string]interface{}{"404": 404, "503": 503}},
	}
	if kind == composite.ApiFragmentRenderConfigGetName() {
		api["route"] = "status-lifecycle"
		return api
	}
	return map[string]interface{}{
		"@type": composite.FragmentConfigGetName(), "route": "status-lifecycle", "nocache": true,
		"content": api,
	}
}

func TestResponseStatusRequestIsolation(t *testing.T) {
	for _, kind := range []string{component.APIConfigGetName(), composite.ApiFragmentRenderConfigGetName()} {
		t.Run(kind, func(t *testing.T) {
			setupLiveModeServeContentTest(t)
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				status, _ := strconv.Atoi(r.URL.Query().Get("status"))
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(status)
				fmt.Fprint(w, `{}`)
			}))
			defer upstream.Close()
			config := responseStatusLifecycleRoute(kind, upstream.URL)
			setTestRouteConfig("status-lifecycle", config)
			before := configOwnershipSnapshot(t, config)
			var workers sync.WaitGroup
			for i := 0; i < 24; i++ {
				workers.Add(1)
				go func(i int) {
					defer workers.Done()
					want := []int{200, 404, 503}[i%3]
					recorder := httptest.NewRecorder()
					ServeContent(recorder, httptest.NewRequest("GET", fmt.Sprintf("/status-lifecycle?status=%d", want), nil))
					if recorder.Code != want || !strings.Contains(recorder.Body.String(), fmt.Sprintf("upstream=%d", want)) {
						t.Errorf("request %d mixed response state: HTTP %d body %q", i, recorder.Code, recorder.Body.String())
					}
				}(i)
			}
			workers.Wait()
			if after := configOwnershipSnapshot(t, config); after != before {
				t.Fatal("requests mutated shared API policy configuration")
			}
		})
	}
}

func TestResponseStatusHEADAndStaticSnapshot(t *testing.T) {
	for _, kind := range []string{component.APIConfigGetName(), composite.ApiFragmentRenderConfigGetName()} {
		t.Run(kind, func(t *testing.T) {
			setupLiveModeServeContentTest(t)
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusNotFound)
				fmt.Fprint(w, `{}`)
			}))
			defer upstream.Close()
			setTestRouteConfig("status-lifecycle", responseStatusLifecycleRoute(kind, upstream.URL))
			server := httptest.NewServer(http.HandlerFunc(ServeContent))
			defer server.Close()
			for _, method := range []string{http.MethodGet, http.MethodHead} {
				request, _ := http.NewRequest(method, server.URL+"/status-lifecycle", nil)
				response, err := server.Client().Do(request)
				if err != nil {
					t.Fatal(err)
				}
				body, err := io.ReadAll(response.Body)
				response.Body.Close()
				if err != nil {
					t.Fatal(err)
				}
				if response.StatusCode != 404 || response.Header.Get("Cache-Control") != "no-store" {
					t.Fatalf("%s: HTTP %d headers %v", method, response.StatusCode, response.Header)
				}
				if method == http.MethodHead && len(body) != 0 {
					t.Fatalf("HEAD body = %q", body)
				}
				if method == http.MethodGet && !strings.Contains(string(body), "upstream=404") {
					t.Fatalf("GET lost rendered body: %q", body)
				}
			}
			_, err := fetchStaticSnapshotTarget(server.Client(), server.URL, staticSnapshotTarget{RequestPath: "/status-lifecycle"})
			if err == nil || !strings.Contains(err.Error(), "returned 404") {
				t.Fatalf("static snapshot must reject dynamically selected 404: %v", err)
			}
		})
	}
}
