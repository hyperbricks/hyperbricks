package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/hyperbricks/hyperbricks/pkg/shared"
)

func withRuntimeGatewayConfig(t *testing.T, config shared.RuntimeGatewayConfig) {
	t.Helper()
	shared.Init_configuration()
	hbConfig := shared.GetHyperBricksConfiguration()
	oldConfig := hbConfig.Server.RuntimeGateway
	hbConfig.Server.RuntimeGateway = config
	t.Cleanup(func() {
		hbConfig.Server.RuntimeGateway = oldConfig
	})
}

func TestRuntimeGatewayMatchesRuntimeHost(t *testing.T) {
	config := shared.RuntimeGatewayConfig{
		Enabled:  true,
		Domain:   "runtime.local",
		Resolver: "http://127.0.0.1:8080/resolve",
	}

	req := httptest.NewRequest(http.MethodGet, "http://test-001--current.runtime.local/about", nil)
	if !runtimeGatewayMatches(config, req) {
		t.Fatal("expected runtime host to match")
	}

	req = httptest.NewRequest(http.MethodGet, "http://test-001.runtime.local/about", nil)
	if !runtimeGatewayMatches(config, req) {
		t.Fatal("expected generic runtime subhost to match")
	}

	req = httptest.NewRequest(http.MethodGet, "http://example.local/about", nil)
	if runtimeGatewayMatches(config, req) {
		t.Fatal("expected non-runtime host to miss")
	}

	req = httptest.NewRequest(http.MethodGet, "http://runtime.local/about", nil)
	if runtimeGatewayMatches(config, req) {
		t.Fatal("expected bare runtime domain to miss")
	}
}

func TestRuntimeGatewayMatchesMultipleDomains(t *testing.T) {
	config := shared.RuntimeGatewayConfig{
		Enabled:  true,
		Domain:   "runtime.local, live.local",
		Domains:  []string{"*.staging.local"},
		Resolver: "http://127.0.0.1:8080/resolve",
	}

	for _, host := range []string{
		"project.runtime.local",
		"project.live.local",
		"project.staging.local",
	} {
		req := httptest.NewRequest(http.MethodGet, "http://"+host+"/about", nil)
		if !runtimeGatewayMatches(config, req) {
			t.Fatalf("expected %s to match", host)
		}
	}

	for _, host := range []string{
		"runtime.local",
		"live.local",
		"staging.local",
		"control.local",
		"project.other.local",
	} {
		req := httptest.NewRequest(http.MethodGet, "http://"+host+"/about", nil)
		if runtimeGatewayMatches(config, req) {
			t.Fatalf("expected %s to miss", host)
		}
	}
}

func TestRuntimeGatewayMatchesFlatHostSuffixes(t *testing.T) {
	config := shared.RuntimeGatewayConfig{
		Enabled:    true,
		HostSuffix: "-live.hyperbricks.eu, -runtime.hyperbricks.eu",
		Resolver:   "http://127.0.0.1:8080/resolve",
	}

	for _, host := range []string{
		"project-live.hyperbricks.eu",
		"b-abc123-runtime.hyperbricks.eu",
	} {
		req := httptest.NewRequest(http.MethodGet, "http://"+host+"/about", nil)
		if !runtimeGatewayMatches(config, req) {
			t.Fatalf("expected %s to match", host)
		}
	}

	for _, host := range []string{
		"control.hyperbricks.eu",
		"live.hyperbricks.eu",
		"project.live.hyperbricks.eu",
		"project-runtime.other.eu",
	} {
		req := httptest.NewRequest(http.MethodGet, "http://"+host+"/about", nil)
		if runtimeGatewayMatches(config, req) {
			t.Fatalf("expected %s to miss", host)
		}
	}
}

func TestHandleRuntimeGatewayProxiesOriginalPathAndQuery(t *testing.T) {
	var upstreamPath string
	var upstreamQuery string
	var forwardedHost string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamPath = r.URL.Path
		upstreamQuery = r.URL.RawQuery
		forwardedHost = r.Header.Get("X-Forwarded-Host")
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte("runtime asset"))
	}))
	defer upstream.Close()

	resolver := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("resolver method = %s, want POST", r.Method)
		}
		var payload runtimeGatewayResolveRequest
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("failed to decode resolver payload: %v", err)
		}
		if payload.Host != "test-001--current.runtime.local" {
			t.Fatalf("resolver host = %q", payload.Host)
		}
		if payload.Path != "/static/app.css" {
			t.Fatalf("resolver path = %q", payload.Path)
		}
		_ = json.NewEncoder(w).Encode(runtimeGatewayResolveResponse{
			Allowed: true,
			Target:  upstream.URL,
		})
	}))
	defer resolver.Close()

	withRuntimeGatewayConfig(t, shared.RuntimeGatewayConfig{
		Enabled:  true,
		Domain:   "runtime.local",
		Resolver: resolver.URL,
	})

	req := httptest.NewRequest(http.MethodGet, "http://test-001--current.runtime.local/static/app.css?v=1", nil)
	req.Host = "test-001--current.runtime.local"
	recorder := httptest.NewRecorder()

	if !handleRuntimeGateway(recorder, req) {
		t.Fatal("expected runtime gateway to handle request")
	}
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %q", recorder.Code, recorder.Body.String())
	}
	if body := recorder.Body.String(); body != "runtime asset" {
		t.Fatalf("body = %q, want runtime asset", body)
	}
	if upstreamPath != "/static/app.css" {
		t.Fatalf("upstream path = %q", upstreamPath)
	}
	if upstreamQuery != "v=1" {
		t.Fatalf("upstream query = %q", upstreamQuery)
	}
	if forwardedHost != "test-001--current.runtime.local" {
		t.Fatalf("forwarded host = %q", forwardedHost)
	}
}

func TestHandleRuntimeGatewayDeniedResolverResponse(t *testing.T) {
	resolver := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(runtimeGatewayResolveResponse{
			Allowed: false,
			Status:  http.StatusForbidden,
			Message: "forbidden",
		})
	}))
	defer resolver.Close()

	withRuntimeGatewayConfig(t, shared.RuntimeGatewayConfig{
		Enabled:  true,
		Domain:   "runtime.local",
		Resolver: resolver.URL,
	})

	req := httptest.NewRequest(http.MethodGet, "http://test-001--current.runtime.local/about", nil)
	req.Host = "test-001--current.runtime.local"
	recorder := httptest.NewRecorder()

	if !handleRuntimeGateway(recorder, req) {
		t.Fatal("expected runtime gateway to handle request")
	}
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", recorder.Code)
	}
	if cacheControl := recorder.Header().Get("Cache-Control"); cacheControl != "no-store" {
		t.Fatalf("Cache-Control = %q, want no-store", cacheControl)
	}
	if !strings.Contains(recorder.Body.String(), "forbidden") {
		t.Fatalf("body = %q, want forbidden message", recorder.Body.String())
	}
}

func TestHandleRuntimeGatewaySetsCookieAndRedirectsTokenURL(t *testing.T) {
	resolver := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(runtimeGatewayResolveResponse{
			Allowed: true,
			Target:  "http://127.0.0.1:19191",
			SetCookies: []string{
				"token=session-token; Domain=.runtime.local; Path=/; HttpOnly; SameSite=Lax",
			},
		})
	}))
	defer resolver.Close()

	withRuntimeGatewayConfig(t, shared.RuntimeGatewayConfig{
		Enabled:  true,
		Domain:   "runtime.local",
		Resolver: resolver.URL,
	})

	req := httptest.NewRequest(http.MethodGet, "http://test-001--current.runtime.local/about?runtime_token=abc&x=1", nil)
	req.Host = "test-001--current.runtime.local"
	recorder := httptest.NewRecorder()

	if !handleRuntimeGateway(recorder, req) {
		t.Fatal("expected runtime gateway to handle request")
	}
	if recorder.Code != http.StatusFound {
		t.Fatalf("status = %d, want 302", recorder.Code)
	}
	if location := recorder.Header().Get("Location"); location != "/about?x=1" {
		t.Fatalf("Location = %q, want clean URL", location)
	}
	if cookies := recorder.Header().Values("Set-Cookie"); len(cookies) != 1 || !strings.Contains(cookies[0], "Domain=.runtime.local") {
		t.Fatalf("Set-Cookie = %v", cookies)
	}
}

func TestParseAndValidateRuntimeTargetRejectsPublicHost(t *testing.T) {
	if _, err := parseAndValidateRuntimeTarget("http://8.8.8.8:8080"); err == nil {
		t.Fatal("expected public target host to be rejected")
	}
}

func TestValidateRuntimeGatewayConfigRequiresResolver(t *testing.T) {
	err := validateRuntimeGatewayConfig(shared.RuntimeGatewayConfig{
		Enabled: true,
		Domain:  "runtime.local",
	})
	if err == nil {
		t.Fatal("expected missing resolver error")
	}
}

func TestValidateRuntimeGatewayConfigAcceptsDomains(t *testing.T) {
	err := validateRuntimeGatewayConfig(shared.RuntimeGatewayConfig{
		Enabled:  true,
		Domains:  []string{"live.local", "runtime.local"},
		Resolver: "http://127.0.0.1:8080/resolve",
	})
	if err != nil {
		t.Fatalf("validateRuntimeGatewayConfig() = %v, want nil", err)
	}
}

func TestValidateRuntimeGatewayConfigAcceptsHostSuffixes(t *testing.T) {
	err := validateRuntimeGatewayConfig(shared.RuntimeGatewayConfig{
		Enabled:    true,
		HostSuffix: "-live.hyperbricks.eu,-runtime.hyperbricks.eu",
		Resolver:   "http://127.0.0.1:8080/resolve",
	})
	if err != nil {
		t.Fatalf("validateRuntimeGatewayConfig() = %v, want nil", err)
	}
}
