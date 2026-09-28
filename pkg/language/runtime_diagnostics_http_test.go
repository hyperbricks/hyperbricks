package language

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestCanonicalRuntimeDiagnosticsURLStripsNonOriginDataAndRejectsCredentials(t *testing.T) {
	canonical, err := CanonicalRuntimeDiagnosticsURL("https://runtime.example.test:8443/private?token=secret#fragment")
	if err != nil {
		t.Fatal(err)
	}
	if canonical != "https://runtime.example.test:8443" {
		t.Fatalf("canonical runtime URL = %q", canonical)
	}
	if _, err := CanonicalRuntimeDiagnosticsURL("https://developer:secret@runtime.example.test"); err == nil || !strings.Contains(err.Error(), "must not contain credentials") {
		t.Fatalf("credential URL error = %v", err)
	}
}

// AC-17: credentials discovered automatically by an editor session are never
// accepted for a non-loopback runtime URL.
func TestHTTPRuntimeDiagnosticsClientRefusesAutomaticRemoteCredentials(t *testing.T) {
	auth := RuntimeDiagnosticsAuth{Username: "developer", Password: "secret", Mode: RuntimeCredentialsAutomatic}
	for _, endpoint := range []string{"https://example.test:8443", "http://192.0.2.10:8080"} {
		if _, err := NewHTTPRuntimeDiagnosticsClient(endpoint, auth, nil); err == nil || !strings.Contains(err.Error(), "limited to loopback") {
			t.Fatalf("endpoint %q error = %v", endpoint, err)
		}
	}
	for _, endpoint := range []string{"http://127.0.0.1:8080", "http://[::1]:8080", "http://localhost:8080"} {
		if _, err := NewHTTPRuntimeDiagnosticsClient(endpoint, auth, nil); err != nil {
			t.Fatalf("loopback endpoint %q: %v", endpoint, err)
		}
	}
}

func TestHTTPRuntimeDiagnosticsClientDoesNotForwardCredentialsAcrossRedirect(t *testing.T) {
	var redirectedRequests atomic.Int32
	target := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		redirectedRequests.Add(1)
	}))
	defer target.Close()
	origin := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		response.Header().Set("Location", target.URL)
		response.WriteHeader(http.StatusTemporaryRedirect)
	}))
	defer origin.Close()

	client, err := NewHTTPRuntimeDiagnosticsClient(origin.URL, RuntimeDiagnosticsAuth{
		Username: "developer",
		Password: "secret",
		Mode:     RuntimeCredentialsAutomatic,
	}, origin.Client())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.FetchCurrent(context.Background()); err == nil || !strings.Contains(err.Error(), "307") {
		t.Fatalf("redirect error = %v", err)
	}
	if redirectedRequests.Load() != 0 {
		t.Fatal("authenticated redirect reached the target")
	}
}

func TestHTTPRuntimeDiagnosticsClientValidatesResponseContract(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(response).Encode(RuntimeDiagnosticsSnapshot{CheckedRoutes: 2, TotalRoutes: 1})
	}))
	defer server.Close()
	client, err := NewHTTPRuntimeDiagnosticsClient(server.URL, RuntimeDiagnosticsAuth{}, server.Client())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.FetchCurrent(context.Background()); err == nil || !strings.Contains(err.Error(), "invalid route coverage") {
		t.Fatalf("contract error = %v", err)
	}
}
