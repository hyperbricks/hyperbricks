package apiutil

import (
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestDecodeAPIResponseKeepsFailures(t *testing.T) {
	for _, tc := range []struct {
		name, contentType, body string
		status                  int
		wantError               bool
	}{
		{"valid JSON", "application/json", `{"token":"value"}`, 200, false},
		{"unlabelled JSON", "text/plain", `{"token":"value"}`, 200, false},
		{"malformed JSON", "application/json", `{"token":"SECRET"`, 200, true},
		{"malformed JSON media type", "Application/JSON; charset=utf-8", `{"token":"SECRET"`, 200, true},
		{"malformed vendor JSON", "application/problem+json", `"SECRET`, 200, true},
		{"malformed unlabelled JSON", "text/plain", `{"token":"SECRET"`, 200, true},
		{"trailing data", "application/json", `{"token":"value"} garbage`, 200, true},
		{"multiple objects", "application/json", `{"token":"value"}{}`, 200, true},
		{"empty JSON", "application/json", "", 200, true},
		{"bodyless deletion", "application/json", "", 204, false},
		{"bodyless plain response", "", "", 200, false},
		{"plain text", "text/plain", "ok", 200, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			response := &http.Response{StatusCode: tc.status, Header: http.Header{"Content-Type": {tc.contentType}}, Body: io.NopCloser(strings.NewReader(tc.body))}
			_, err := DecodeAPIResponse(response)
			if (err != nil) != tc.wantError {
				t.Fatalf("unexpected result: %v", err)
			}
			if err != nil && strings.Contains(err.Error(), "SECRET") {
				t.Fatal("decode error exposed payload")
			}
		})
	}
}
