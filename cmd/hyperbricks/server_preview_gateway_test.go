package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/hyperbricks/hyperbricks/pkg/shared"
)

func withPreviewGatewayConfig(t *testing.T, config shared.PreviewGatewayConfig) {
	t.Helper()
	shared.Init_configuration()
	hbConfig := shared.GetHyperBricksConfiguration()
	oldConfig := hbConfig.Server.PreviewGateway
	hbConfig.Server.PreviewGateway = config
	t.Cleanup(func() {
		hbConfig.Server.PreviewGateway = oldConfig
	})
}

func TestPreviewGatewayMatchesPreviewHost(t *testing.T) {
	config := shared.PreviewGatewayConfig{
		Enabled:  true,
		Domain:   "preview.local",
		Resolver: "http://127.0.0.1:8080/resolve",
	}

	req := httptest.NewRequest(http.MethodGet, "http://test-001--current.preview.local/about", nil)
	if !previewGatewayMatches(config, req) {
		t.Fatal("expected preview host to match")
	}

	req = httptest.NewRequest(http.MethodGet, "http://example.local/about", nil)
	if previewGatewayMatches(config, req) {
		t.Fatal("expected non-preview host to miss")
	}

	req = httptest.NewRequest(http.MethodGet, "http://composer.preview.local/about", nil)
	if previewGatewayMatches(config, req) {
		t.Fatal("expected non-preview app host below preview domain to miss")
	}
}

func TestHandlePreviewGatewayProxiesOriginalPathAndQuery(t *testing.T) {
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
		var payload previewGatewayResolveRequest
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("failed to decode resolver payload: %v", err)
		}
		if payload.Host != "test-001--current.preview.local" {
			t.Fatalf("resolver host = %q", payload.Host)
		}
		if payload.Path != "/static/app.css" {
			t.Fatalf("resolver path = %q", payload.Path)
		}
		_ = json.NewEncoder(w).Encode(previewGatewayResolveResponse{
			Allowed: true,
			Target:  upstream.URL,
		})
	}))
	defer resolver.Close()

	withPreviewGatewayConfig(t, shared.PreviewGatewayConfig{
		Enabled:  true,
		Domain:   "preview.local",
		Resolver: resolver.URL,
	})

	req := httptest.NewRequest(http.MethodGet, "http://test-001--current.preview.local/static/app.css?v=1", nil)
	req.Host = "test-001--current.preview.local"
	recorder := httptest.NewRecorder()

	if !handlePreviewGateway(recorder, req) {
		t.Fatal("expected preview gateway to handle request")
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
	if forwardedHost != "test-001--current.preview.local" {
		t.Fatalf("forwarded host = %q", forwardedHost)
	}
}

func TestHandlePreviewGatewayDeniedResolverResponse(t *testing.T) {
	resolver := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(previewGatewayResolveResponse{
			Allowed: false,
			Status:  http.StatusForbidden,
			Message: "forbidden",
		})
	}))
	defer resolver.Close()

	withPreviewGatewayConfig(t, shared.PreviewGatewayConfig{
		Enabled:  true,
		Domain:   "preview.local",
		Resolver: resolver.URL,
	})

	req := httptest.NewRequest(http.MethodGet, "http://test-001--current.preview.local/about", nil)
	req.Host = "test-001--current.preview.local"
	recorder := httptest.NewRecorder()

	if !handlePreviewGateway(recorder, req) {
		t.Fatal("expected preview gateway to handle request")
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

func TestHandlePreviewGatewaySetsCookieAndRedirectsTokenURL(t *testing.T) {
	resolver := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(previewGatewayResolveResponse{
			Allowed: true,
			Target:  "http://127.0.0.1:19191",
			SetCookies: []string{
				"token=session-token; Domain=.preview.local; Path=/; HttpOnly; SameSite=Lax",
			},
		})
	}))
	defer resolver.Close()

	withPreviewGatewayConfig(t, shared.PreviewGatewayConfig{
		Enabled:  true,
		Domain:   "preview.local",
		Resolver: resolver.URL,
	})

	req := httptest.NewRequest(http.MethodGet, "http://test-001--current.preview.local/about?preview_token=abc&x=1", nil)
	req.Host = "test-001--current.preview.local"
	recorder := httptest.NewRecorder()

	if !handlePreviewGateway(recorder, req) {
		t.Fatal("expected preview gateway to handle request")
	}
	if recorder.Code != http.StatusFound {
		t.Fatalf("status = %d, want 302", recorder.Code)
	}
	if location := recorder.Header().Get("Location"); location != "/about?x=1" {
		t.Fatalf("Location = %q, want clean URL", location)
	}
	if cookies := recorder.Header().Values("Set-Cookie"); len(cookies) != 1 || !strings.Contains(cookies[0], "Domain=.preview.local") {
		t.Fatalf("Set-Cookie = %v", cookies)
	}
}

func TestParseAndValidatePreviewTargetRejectsPublicHost(t *testing.T) {
	if _, err := parseAndValidatePreviewTarget("http://8.8.8.8:8080"); err == nil {
		t.Fatal("expected public target host to be rejected")
	}
}

func TestValidatePreviewGatewayConfigRequiresResolver(t *testing.T) {
	err := validatePreviewGatewayConfig(shared.PreviewGatewayConfig{
		Enabled: true,
		Domain:  "preview.local",
	})
	if err == nil {
		t.Fatal("expected missing resolver error")
	}
}
