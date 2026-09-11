package apiutil

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync/atomic"
	"testing"
)

func TestNewAPIHTTPClientHasNoCookieJarAndUsesSharedTransport(t *testing.T) {
	client := NewAPIHTTPClient()
	if client.Jar != nil {
		t.Fatal("API client retained a cookie jar")
	}
	if client.Transport != sharedTransport {
		t.Fatal("API client does not use the shared transport")
	}
	if client.Timeout == 0 {
		t.Fatal("API client has no timeout")
	}
	if client.CheckRedirect == nil {
		t.Fatal("API client has no redirect policy")
	}
}

func TestAPIHTTPClientForwardsConfiguredHeadersOnlyOnSameOriginRedirect(t *testing.T) {
	received := make(chan http.Header, 1)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/start":
			http.Redirect(writer, request, "/target", http.StatusFound)
		case "/target":
			received <- request.Header.Clone()
			writer.WriteHeader(http.StatusNoContent)
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()

	request, err := http.NewRequest(http.MethodGet, server.URL+"/start", nil)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	request.Header.Set("Authorization", "Bearer configured")
	request.Header.Set("X-API-Key", "configured-secret")
	response, err := NewAPIHTTPClient().Do(request)
	if err != nil {
		t.Fatalf("same-origin redirect failed: %v", err)
	}
	defer response.Body.Close()

	headers := <-received
	if headers.Get("Authorization") != "Bearer configured" || headers.Get("X-API-Key") != "configured-secret" {
		t.Fatalf("same-origin redirect lost configured headers: %#v", headers)
	}
}

func TestAPIHTTPClientBlocksCrossOriginRedirectBeforeSendingHeaders(t *testing.T) {
	var targetRequests atomic.Int32
	target := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		targetRequests.Add(1)
		writer.WriteHeader(http.StatusNoContent)
	}))
	defer target.Close()

	source := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		http.Redirect(writer, request, target.URL+"/target?token=redirect-secret", http.StatusFound)
	}))
	defer source.Close()

	request, err := http.NewRequest(http.MethodGet, source.URL+"/start", nil)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	request.Header.Set("Authorization", "Bearer configured")
	request.Header.Set("X-API-Key", "configured-secret")
	response, err := NewAPIHTTPClient().Do(request)
	if response != nil {
		response.Body.Close()
	}
	if !errors.Is(err, ErrCrossOriginRedirect) {
		t.Fatalf("cross-origin redirect error = %v, want ErrCrossOriginRedirect", err)
	}
	if got := targetRequests.Load(); got != 0 {
		t.Fatalf("cross-origin target received %d requests", got)
	}
}

func TestAPIHTTPClientDoesNotPersistResponseCookies(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/set":
			http.SetCookie(writer, &http.Cookie{Name: "session", Value: "upstream-secret", Path: "/"})
			writer.WriteHeader(http.StatusNoContent)
		case "/check":
			_, _ = io.WriteString(writer, request.Header.Get("Cookie"))
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()

	client := NewAPIHTTPClient()
	setResponse, err := client.Get(server.URL + "/set")
	if err != nil {
		t.Fatalf("set request: %v", err)
	}
	setResponse.Body.Close()

	checkResponse, err := client.Get(server.URL + "/check")
	if err != nil {
		t.Fatalf("check request: %v", err)
	}
	defer checkResponse.Body.Close()
	body, err := io.ReadAll(checkResponse.Body)
	if err != nil {
		t.Fatalf("read check response: %v", err)
	}
	if string(body) != "" {
		t.Fatalf("API client persisted upstream cookie: %q", body)
	}
}

func TestSameOriginUsesSchemeHostAndEffectivePort(t *testing.T) {
	tests := []struct {
		name   string
		first  string
		second string
		want   bool
	}{
		{name: "identical", first: "https://api.example.test/a", second: "https://api.example.test/b", want: true},
		{name: "case insensitive host", first: "https://API.EXAMPLE.TEST/a", second: "https://api.example.test/b", want: true},
		{name: "default HTTP port", first: "http://api.example.test/a", second: "http://api.example.test:80/b", want: true},
		{name: "default HTTPS port", first: "https://api.example.test/a", second: "https://api.example.test:443/b", want: true},
		{name: "different scheme", first: "http://api.example.test/a", second: "https://api.example.test/a", want: false},
		{name: "different host", first: "https://api.example.test/a", second: "https://other.example.test/a", want: false},
		{name: "different port", first: "https://api.example.test:443/a", second: "https://api.example.test:8443/a", want: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			first, err := url.Parse(test.first)
			if err != nil {
				t.Fatalf("parse first URL: %v", err)
			}
			second, err := url.Parse(test.second)
			if err != nil {
				t.Fatalf("parse second URL: %v", err)
			}
			if got := sameOrigin(first, second); got != test.want {
				t.Fatalf("sameOrigin(%q, %q) = %t, want %t", test.first, test.second, got, test.want)
			}
		})
	}
	if sameOrigin(nil, &url.URL{}) || sameOrigin(&url.URL{}, nil) {
		t.Fatal("nil URL was treated as same origin")
	}
}

func TestAPIRedirectPolicyRejectsInvalidAndExcessiveRedirects(t *testing.T) {
	original := httptest.NewRequest(http.MethodGet, "https://api.example.test/start", nil)
	validTarget := httptest.NewRequest(http.MethodGet, "https://api.example.test/target", nil)
	credentialTarget := httptest.NewRequest(http.MethodGet, "https://user:password@api.example.test/target", nil)
	opaqueTarget := &http.Request{URL: &url.URL{Scheme: "https", Opaque: "api.example.test/target"}}

	tests := []struct {
		name    string
		request *http.Request
		via     []*http.Request
		want    error
	}{
		{name: "same origin", request: validTarget, via: []*http.Request{original}},
		{name: "redirect URL userinfo", request: credentialTarget, via: []*http.Request{original}, want: ErrInvalidAPIRedirect},
		{name: "opaque redirect", request: opaqueTarget, via: []*http.Request{original}, want: ErrInvalidAPIRedirect},
		{name: "nil redirect request", request: nil, via: []*http.Request{original}, want: ErrInvalidAPIRedirect},
		{name: "missing redirect history", request: validTarget, want: ErrInvalidAPIRedirect},
		{name: "ten redirects", request: validTarget, via: repeatedRequest(original, 10), want: ErrTooManyAPIRedirects},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := checkAPIRedirect(test.request, test.via)
			if !errors.Is(err, test.want) {
				t.Fatalf("checkAPIRedirect() error = %v, want %v", err, test.want)
			}
		})
	}
}

func repeatedRequest(request *http.Request, count int) []*http.Request {
	requests := make([]*http.Request, count)
	for index := range requests {
		requests[index] = request
	}
	return requests
}
