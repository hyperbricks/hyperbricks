package apiutil

import (
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// ErrCrossOriginRedirect reports an API redirect that would cross the exact
// origin boundary and could expose configured request headers.
var ErrCrossOriginRedirect = errors.New("cross-origin API redirect blocked")

// ErrInvalidAPIRedirect reports a malformed or credential-bearing redirect.
var ErrInvalidAPIRedirect = errors.New("invalid API redirect blocked")

// ErrTooManyAPIRedirects preserves net/http's default ten-redirect limit.
var ErrTooManyAPIRedirects = errors.New("stopped after 10 API redirects")

// NewAPIHTTPClient returns the client used for upstream component requests.
// It retains no cookies and follows redirects only within the original origin.
func NewAPIHTTPClient() *http.Client {
	return &http.Client{
		Timeout:       10 * time.Second,
		Transport:     sharedTransport,
		CheckRedirect: checkAPIRedirect,
	}
}

func checkAPIRedirect(request *http.Request, via []*http.Request) error {
	if len(via) >= 10 {
		return ErrTooManyAPIRedirects
	}
	if request == nil || request.URL == nil || request.URL.User != nil || !validOriginURL(request.URL) {
		return ErrInvalidAPIRedirect
	}
	if len(via) == 0 || via[0] == nil || !validOriginURL(via[0].URL) {
		return ErrInvalidAPIRedirect
	}
	if sameOrigin(via[0].URL, request.URL) {
		return nil
	}
	return ErrCrossOriginRedirect
}

func sameOrigin(first, second *url.URL) bool {
	if !validOriginURL(first) || !validOriginURL(second) {
		return false
	}
	return strings.EqualFold(first.Scheme, second.Scheme) &&
		strings.EqualFold(first.Hostname(), second.Hostname()) &&
		effectivePort(first) == effectivePort(second)
}

func validOriginURL(endpoint *url.URL) bool {
	if endpoint == nil || endpoint.Opaque != "" || endpoint.Host == "" || endpoint.Hostname() == "" {
		return false
	}
	return strings.EqualFold(endpoint.Scheme, "http") || strings.EqualFold(endpoint.Scheme, "https")
}

func effectivePort(endpoint *url.URL) string {
	if port := endpoint.Port(); port != "" {
		return port
	}
	switch strings.ToLower(endpoint.Scheme) {
	case "http":
		return "80"
	case "https":
		return "443"
	default:
		return ""
	}
}
