package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/hyperbricks/hyperbricks/pkg/component"
	"github.com/hyperbricks/hyperbricks/pkg/composite"
)

// Use both renderer types in one route: their request state and cookie capture
// must remain isolated even when sibling channels complete in a different order.
func multichannelLifecycleRoute(endpoint string) map[string]interface{} {
	channel := func(kind, name string) map[string]interface{} {
		return map[string]interface{}{"@type": kind, "endpoint": endpoint + "/" + name,
			"method": "GET", "querykeys": []string{"id", "fail"}, "forwardtoken": "session",
			"inline": name + "={{.Data.identity}};",
		}
	}
	slow := channel(composite.ApiFragmentRenderConfigGetName(), "slow")
	slow["route"] = "multi-slow"
	slow["setcookies"] = []interface{}{map[string]interface{}{"name": "issued", "value": "{{.Data.identity}}", "path": "/", "http_only": true}}
	return map[string]interface{}{"@type": composite.FragmentConfigGetName(), "route": "multi-lifecycle", "nocache": true,
		"content": map[string]interface{}{"@type": composite.TreeRendererConfigGetName(),
			"10": channel(component.APIConfigGetName(), "fast"), "20": slow,
			"30": channel(component.APIConfigGetName(), "optional")}}
}

func TestAPIMultichannelConcurrentFailureIsolation(t *testing.T) {
	setupDevelopmentModeServeContentTest(t, false)
	const users = 18
	// A per-user barrier makes slow channels wait until their sibling has run;
	// without relying on a wall-clock delay. Cancellation below proves fanout overlap.
	gates := make([]chan struct{}, users)
	for i := range gates {
		gates[i] = make(chan struct{})
	}
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.Atoi(r.URL.Query().Get("id"))
		if err != nil || id < 0 || id >= users {
			t.Errorf("unexpected identity %q", r.URL.RawQuery)
			w.WriteHeader(400)
			return
		}
		identity := fmt.Sprintf("user-%02d", id)
		if got := r.Header.Get("Authorization"); got != "Bearer "+identity {
			t.Errorf("user %d got credential %q", id, got)
		}
		switch r.URL.Path {
		case "/fast":
			close(gates[id])
		case "/slow":
			select {
			case <-gates[id]:
			case <-r.Context().Done():
				return
			}
		case "/optional":
			if r.URL.Query().Get("fail") == "true" {
				w.Header().Set("Content-Type", "application/json")
				fmt.Fprint(w, `{"invalid":`)
				return
			}
		default:
			t.Errorf("unexpected upstream path %q", r.URL.Path)
		}
		writeAPISecurityJSON(w, 200, map[string]interface{}{"identity": identity})
	}))
	t.Cleanup(upstream.Close)
	config := multichannelLifecycleRoute(upstream.URL)
	setTestRouteConfig("multi-lifecycle", config)
	before := configOwnershipSnapshot(t, config)
	var workers sync.WaitGroup
	for i := 0; i < users; i++ {
		workers.Add(1)
		go func(id int) {
			defer workers.Done()
			fail := id%3 == 0
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			request := httptest.NewRequest("GET", fmt.Sprintf("/multi-lifecycle?id=%d&fail=%t", id, fail), nil).WithContext(ctx)
			identity := fmt.Sprintf("user-%02d", id)
			request.AddCookie(&http.Cookie{Name: "session", Value: identity})
			response := httptest.NewRecorder()
			ServeContent(response, request)
			if response.Code != 200 {
				t.Errorf("user %d: status %d", id, response.Code)
			}
			body := response.Body.String()
			for _, name := range []string{"fast", "slow"} {
				if !strings.Contains(body, name+"="+identity+";") {
					t.Errorf("user %d missing %s output: %q", id, name, body)
				}
			}
			for other := 0; other < users; other++ {
				if other != id && strings.Contains(body, fmt.Sprintf("user-%02d", other)) {
					t.Errorf("user %d leaked user %d: %q", id, other, body)
				}
			}
			cookies := response.Result().Cookies()
			count, _ := strconv.Atoi(response.Header().Get(renderErrorCountHeader))
			if fail {
				if count == 0 || len(cookies) != 0 || strings.Contains(body, "optional="+identity+";") {
					t.Errorf("failed user %d: errors=%d cookies=%v body=%q", id, count, cookies, body)
				}
			} else {
				if count != 0 || len(cookies) != 1 || cookies[0].Name != "issued" || cookies[0].Value != identity || !strings.Contains(body, "optional="+identity+";") {
					t.Errorf("healthy user %d: errors=%d cookies=%v body=%q", id, count, cookies, body)
				}
			}
		}(i)
	}
	workers.Wait()
	if after := configOwnershipSnapshot(t, config); after != before {
		t.Fatal("concurrent requests mutated shared configuration")
	}
}

func TestAPIMultichannelRepeatedCancellationReleasesUpstream(t *testing.T) {
	setupDevelopmentModeServeContentTest(t, false)
	started := make(chan string, 3)
	exited := make(chan string, 3)
	release := make(chan struct{})
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started <- r.URL.Path
		defer func() { exited <- r.URL.Path }()
		select {
		case <-r.Context().Done():
		case <-release:
		}
	}))
	t.Cleanup(upstream.Close)
	t.Cleanup(func() { close(release) })
	setTestRouteConfig("multi-lifecycle", multichannelLifecycleRoute(upstream.URL))
	for round := 0; round < 12; round++ {
		ctx, cancel := context.WithCancel(context.Background())
		response := httptest.NewRecorder()
		done := make(chan struct{})
		go func() {
			defer close(done)
			ServeContent(response, httptest.NewRequest("GET", "/multi-lifecycle", nil).WithContext(ctx))
		}()
		for i := 0; i < 3; i++ {
			select {
			case <-started:
			case <-time.After(5 * time.Second):
				cancel()
				t.Fatal("channels did not start concurrently")
			}
		}
		cancel()
		seen := map[string]bool{}
		for i := 0; i < 3; i++ {
			select {
			case path := <-exited:
				seen[path] = true
			case <-time.After(5 * time.Second):
				t.Fatal("cancelled upstream handler did not exit")
			}
		}
		if len(seen) != 3 {
			t.Fatalf("round %d: duplicate/missing channels: %v", round, seen)
		}
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Fatal("cancelled renderer did not return")
		}
		if cookies := response.Header().Values("Set-Cookie"); len(cookies) != 0 {
			t.Fatalf("cancelled render committed cookies: %v", cookies)
		}
	}
}
