package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/hyperbricks/hyperbricks/cmd/hyperbricks/commands"
	"github.com/hyperbricks/hyperbricks/pkg/logging"
	"github.com/hyperbricks/hyperbricks/pkg/shared"
)

func loggedRenderDiagnosticsURL(t *testing.T, requestID string) string {
	t.Helper()
	const prefix = "Render diagnostics recorded: "
	entries := logging.GetLogs()
	for index := len(entries) - 1; index >= 0; index-- {
		entry := entries[index]
		if !strings.HasPrefix(entry.Message, prefix) {
			continue
		}
		link := strings.TrimPrefix(entry.Message, prefix)
		parsed, err := url.Parse(link)
		if err == nil && parsed.Query().Get("request_id") == requestID {
			return link
		}
	}
	t.Fatalf("no diagnostics URL logged for request %q", requestID)
	return ""
}

func TestRenderDiagnosticsURL(t *testing.T) {
	setupDevelopmentModeServeContentTest(t, false)
	hbConfig := getHyperBricksConfiguration()
	oldPort, oldStatic := hbConfig.Server.Port, commands.RenderStatic
	hbConfig.Server.Port = 9099
	t.Cleanup(func() {
		hbConfig.Server.Port = oldPort
		commands.RenderStatic = oldStatic
	})

	tests := []struct {
		name, requestURL, forwardedProto, mode, wantOrigin string
		static                                             bool
	}{
		{name: "startup uses configured port", wantOrigin: "http://localhost:9099"},
		{name: "request uses actual port", requestURL: "http://localhost:8097/broken?token=private", wantOrigin: "http://localhost:8097"},
		{name: "HTTPS", requestURL: "https://example.com/broken", wantOrigin: "https://example.com"},
		{name: "proxied HTTPS", requestURL: "http://example.com/broken", forwardedProto: "https, http", wantOrigin: "https://example.com"},
		{name: "invalid forwarded scheme", requestURL: "http://example.com/broken", forwardedProto: "javascript", wantOrigin: "http://example.com"},
		{name: "IPv6", requestURL: "http://[::1]:8097/broken", wantOrigin: "http://[::1]:8097"},
		{name: "debug", mode: shared.DEBUG_MODE, wantOrigin: "http://localhost:9099"},
		{name: "live has no link", mode: shared.LIVE_MODE},
		{name: "static has no link", static: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			hbConfig.Mode = shared.DEVELOPMENT_MODE
			if test.mode != "" {
				hbConfig.Mode = test.mode
			}
			commands.RenderStatic = test.static
			var request *http.Request
			if test.requestURL != "" {
				request = httptest.NewRequest(http.MethodGet, test.requestURL, nil)
				request.Header.Set("X-Forwarded-Proto", test.forwardedProto)
			}
			want := test.wantOrigin
			if want != "" {
				want += "/__hyperbricks/render-diagnostics?request_id=hb-12"
			}
			if got := renderDiagnosticsURL(request, "hb-12"); got != want {
				t.Fatalf("diagnostics URL = %q, want %q", got, want)
			}
		})
	}
}

func TestRecordConfigDiagnosticsLogsWorkingURL(t *testing.T) {
	setupDevelopmentModeServeContentTest(t, false)
	recordConfigDiagnostics([]error{errors.New("invalid YAML source")})
	records := collectRecentRenderDiagnostics(1)
	if len(records) != 1 {
		t.Fatalf("record count = %d, want 1", len(records))
	}
	link := loggedRenderDiagnosticsURL(t, records[0].RequestID)
	response := httptest.NewRecorder()
	handler(response, httptest.NewRequest(http.MethodGet, link, nil))
	if response.Code != http.StatusOK {
		t.Fatalf("diagnostics status = %d, want 200", response.Code)
	}
	var payload RenderDiagnostics
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.RequestID != records[0].RequestID || payload.Route != "__config" || len(payload.Errors) != 1 || payload.Errors[0].Err != "invalid YAML source" {
		t.Fatalf("unexpected startup diagnostics: %#v", payload)
	}

	getHyperBricksConfiguration().Mode = shared.LIVE_MODE
	response = httptest.NewRecorder()
	handler(response, httptest.NewRequest(http.MethodGet, link, nil))
	if response.Code != http.StatusNotFound {
		t.Fatalf("live diagnostics status = %d, want 404", response.Code)
	}
}
