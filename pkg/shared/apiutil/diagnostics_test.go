package apiutil

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"reflect"
	"strings"
	"testing"
)

func TestDescribeRequestContainsMetadataOnly(t *testing.T) {
	request, err := http.NewRequest(
		http.MethodPost,
		"https://url-user:url-password@api.example.test:8443/private/path-secret?access_token=query-secret",
		strings.NewReader("body-secret"),
	)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	request.Header.Set("Authorization", "Bearer header-secret")
	request.Header.Set("x-api-key", "key-secret")

	metadata := DescribeRequest(request)
	if metadata.Method != http.MethodPost || metadata.Scheme != "https" || metadata.Host != "api.example.test:8443" {
		t.Fatalf("request metadata = %#v", metadata)
	}
	if !reflect.DeepEqual(metadata.HeaderNames, []string{"Authorization", "X-Api-Key"}) {
		t.Fatalf("header names = %#v", metadata.HeaderNames)
	}
	if metadata.ContentLength != int64(len("body-secret")) {
		t.Fatalf("content length = %d", metadata.ContentLength)
	}
	assertContainsNoSecrets(t, fmt.Sprintf("%+v", metadata),
		"url-user", "url-password", "path-secret", "query-secret", "header-secret", "key-secret", "body-secret")
}

func TestDescribeResponseContainsMetadataOnly(t *testing.T) {
	response := &http.Response{
		StatusCode:    http.StatusUnauthorized,
		ContentLength: int64(len("response-body-secret")),
		Header: http.Header{
			"Set-Cookie": {"session=response-cookie-secret"},
			"X-Upstream": {"response-header-secret"},
		},
		Body: io.NopCloser(strings.NewReader("response-body-secret")),
	}
	metadata := DescribeResponse(response)
	if metadata.StatusCode != http.StatusUnauthorized || metadata.ContentLength != int64(len("response-body-secret")) {
		t.Fatalf("response metadata = %#v", metadata)
	}
	if !reflect.DeepEqual(metadata.HeaderNames, []string{"Set-Cookie", "X-Upstream"}) {
		t.Fatalf("header names = %#v", metadata.HeaderNames)
	}
	assertContainsNoSecrets(t, fmt.Sprintf("%+v", metadata),
		"response-cookie-secret", "response-header-secret", "response-body-secret")
}

func TestSafeRequestErrorRedactsCauseAndURLDetailsButPreservesUnwrap(t *testing.T) {
	request, err := http.NewRequest(
		http.MethodGet,
		"https://url-user:url-password@api.example.test/private/path-secret?access_token=query-secret",
		nil,
	)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	request.Header.Set("Authorization", "Bearer header-secret")
	cause := &url.Error{
		Op:  "Get",
		URL: "https://cause-user:cause-password@api.example.test/private/cause-path?token=cause-query",
		Err: context.Canceled,
	}

	safe := SafeRequestError("upstream request", request, cause)
	if !errors.Is(safe, context.Canceled) {
		t.Fatalf("SafeRequestError lost cancellation cause: %v", safe)
	}
	message := safe.Error()
	if !strings.Contains(message, "method=GET") || !strings.Contains(message, "scheme=https") || !strings.Contains(message, "host=api.example.test") || !strings.Contains(message, "Authorization") {
		t.Fatalf("safe request error omitted useful metadata: %q", message)
	}
	assertContainsNoSecrets(t, message,
		"url-user", "url-password", "path-secret", "query-secret", "header-secret",
		"cause-user", "cause-password", "cause-path", "cause-query")
}

func TestDiagnosticHelpersAcceptNil(t *testing.T) {
	if got := DescribeRequest(nil); !reflect.DeepEqual(got, RequestMetadata{}) {
		t.Fatalf("DescribeRequest(nil) = %#v", got)
	}
	if got := DescribeResponse(nil); !reflect.DeepEqual(got, ResponseMetadata{}) {
		t.Fatalf("DescribeResponse(nil) = %#v", got)
	}
	cause := errors.New("cause-secret")
	safe := SafeRequestError("upstream request", nil, cause)
	if !errors.Is(safe, cause) {
		t.Fatal("nil-request diagnostic lost its cause")
	}
	assertContainsNoSecrets(t, safe.Error(), "cause-secret")
}

func assertContainsNoSecrets(t *testing.T, value string, secrets ...string) {
	t.Helper()
	for _, secret := range secrets {
		if strings.Contains(value, secret) {
			t.Fatalf("diagnostic exposed %q in %q", secret, value)
		}
	}
}
