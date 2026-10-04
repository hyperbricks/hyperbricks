package composite

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/hyperbricks/hyperbricks/pkg/shared"
)

func TestAPIFragmentRenderResponseOutcomes(t *testing.T) {
	shared.Init_configuration()
	configuration := shared.GetHyperBricksConfiguration()
	previousMode := configuration.Mode
	configuration.Mode = shared.DEVELOPMENT_MODE
	t.Cleanup(func() { configuration.Mode = previousMode })

	for _, tc := range []struct {
		name           string
		status         int
		body           string
		wantFailure    shared.APIResponseFailure
		wantDiagnostic bool
	}{
		{"success", http.StatusOK, `{"message":"ok"}`, shared.APIFailureNone, false},
		{"actual not found", http.StatusNotFound, `{"message":"missing"}`, shared.APIFailureNone, false},
		{"actual bad gateway", http.StatusBadGateway, `{"message":"unavailable"}`, shared.APIFailureNone, false},
		{"malformed success", http.StatusOK, `{"message":`, shared.APIFailureDecode, true},
		{"malformed not found", http.StatusNotFound, `{"message":`, shared.APIFailureDecode, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			}))
			t.Cleanup(upstream.Close)
			incoming := httptest.NewRequest(http.MethodGet, "https://browser.example.test/", nil)
			ctx := context.WithValue(incoming.Context(), shared.Request, incoming)
			config := ApiFragmentRenderConfig{APIConfig: APIConfig{Endpoint: upstream.URL, Method: http.MethodGet}, Route: "test-api"}
			_, outcome, diagnostic := fetchDataFromAPIOutcome(config, ctx)
			if outcome.Status != tc.status || !outcome.Received || outcome.Failure != tc.wantFailure {
				t.Fatalf("outcome = %+v, want received status=%d failure=%v", outcome, tc.status, tc.wantFailure)
			}
			if (diagnostic != nil) != tc.wantDiagnostic {
				t.Fatalf("diagnostic = %v, want error=%v", diagnostic, tc.wantDiagnostic)
			}
			if (outcome.Err != nil) != (tc.wantFailure != shared.APIFailureNone) {
				t.Fatalf("failure cause = %v; generic HTTP diagnostics must not create a failed outcome", outcome.Err)
			}
		})
	}
}

func TestAPIFragmentRenderResponseOutcomeTransportCauses(t *testing.T) {
	shared.Init_configuration()
	configuration := shared.GetHyperBricksConfiguration()
	previousMode := configuration.Mode
	configuration.Mode = shared.DEVELOPMENT_MODE
	t.Cleanup(func() { configuration.Mode = previousMode })

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.(http.Flusher).Flush()
		<-r.Context().Done()
	}))
	t.Cleanup(upstream.Close)
	config := ApiFragmentRenderConfig{APIConfig: APIConfig{Endpoint: upstream.URL, Method: http.MethodGet}, Route: "test-api"}
	incoming := httptest.NewRequest(http.MethodGet, "https://browser.example.test/", nil)
	requestContext := context.WithValue(incoming.Context(), shared.Request, incoming)

	t.Run("canceled before request", func(t *testing.T) {
		ctx, cancel := context.WithCancel(requestContext)
		cancel()
		_, outcome, _ := fetchDataFromAPIOutcome(config, ctx)
		if outcome.Received || outcome.Status != http.StatusBadGateway || outcome.Failure != shared.APIFailureTransport || !errors.Is(outcome.Err, context.Canceled) {
			t.Fatalf("cancellation outcome = %+v", outcome)
		}
	})
	t.Run("expired before request", func(t *testing.T) {
		ctx, cancel := context.WithDeadline(requestContext, time.Now().Add(-time.Second))
		defer cancel()
		_, outcome, _ := fetchDataFromAPIOutcome(config, ctx)
		if outcome.Received || outcome.Failure != shared.APIFailureTransport || !errors.Is(outcome.Err, context.DeadlineExceeded) {
			t.Fatalf("transport timeout outcome = %+v", outcome)
		}
	})
	t.Run("timeout reading received body", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(requestContext, 100*time.Millisecond)
		defer cancel()
		_, outcome, _ := fetchDataFromAPIOutcome(config, ctx)
		if !outcome.Received || outcome.Status != http.StatusOK || outcome.Failure != shared.APIFailureDecode || !errors.Is(outcome.Err, context.DeadlineExceeded) {
			t.Fatalf("body timeout outcome = %+v", outcome)
		}
	})
	t.Run("local missing context", func(t *testing.T) {
		_, outcome, _ := fetchDataFromAPIOutcome(config, nil)
		if outcome.Received || outcome.Failure != shared.APIFailureLocal || outcome.Err == nil {
			t.Fatalf("local error outcome = %+v", outcome)
		}
	})
}

func TestAPIFragmentRenderCapturesFinalResponseStatus(t *testing.T) {
	shared.Init_configuration()
	configuration := shared.GetHyperBricksConfiguration()
	previousMode := configuration.Mode
	configuration.Mode = shared.DEVELOPMENT_MODE
	t.Cleanup(func() { configuration.Mode = previousMode })
	disabled := false
	for _, tc := range []struct {
		name           string
		upstreamStatus int
		body           string
		inline         string
		policy         *shared.ResponseStatusConfig
		wantStatus     int
		wantHTML       string
		wantDiagnostic bool
	}{
		{name: "omitted", upstreamStatus: 404, body: `{"message":"missing"}`, wantHTML: "404:missing"},
		{name: "disabled", upstreamStatus: 404, body: `{"message":"missing"}`, policy: &shared.ResponseStatusConfig{Enabled: &disabled, Required: true, Map: map[string]interface{}{"404": 410}}, wantHTML: "404:missing"},
		{name: "mapped", upstreamStatus: 404, body: `{"message":"missing"}`, policy: &shared.ResponseStatusConfig{Map: map[string]interface{}{"404": 410}}, wantStatus: 410, wantHTML: "404:missing"},
		{name: "explicit ignore", upstreamStatus: 404, body: `{"message":"missing"}`, policy: &shared.ResponseStatusConfig{Required: true, Map: map[string]interface{}{"404": "ignore"}}, wantHTML: "404:missing"},
		{name: "required unmapped", upstreamStatus: 404, body: `{"message":"missing"}`, policy: &shared.ResponseStatusConfig{Required: true}, wantStatus: 502, wantHTML: "404:missing", wantDiagnostic: true},
		{name: "optional decode failure", upstreamStatus: 404, body: `{"message":`, policy: &shared.ResponseStatusConfig{Map: map[string]interface{}{"404": 410}}},
		{name: "required decode failure supersedes mapped status", upstreamStatus: 404, body: `{"message":`, policy: &shared.ResponseStatusConfig{Required: true, Map: map[string]interface{}{"404": 410}}, wantStatus: 502, wantDiagnostic: true},
		{name: "required template failure supersedes mapped status", upstreamStatus: 404, body: `{"message":"missing"}`, inline: "{{", policy: &shared.ResponseStatusConfig{Required: true, Map: map[string]interface{}{"404": 410}}, wantStatus: 500, wantDiagnostic: true},
		{name: "optional template failure cancels mapped status", upstreamStatus: 404, body: `{"message":"missing"}`, inline: "{{", policy: &shared.ResponseStatusConfig{Map: map[string]interface{}{"404": 410}}},
		{name: "decode failure precedes subsequent template failure", upstreamStatus: 404, body: `{"message":`, inline: "{{", policy: &shared.ResponseStatusConfig{Required: true}, wantStatus: 502, wantDiagnostic: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tc.upstreamStatus)
				_, _ = w.Write([]byte(tc.body))
			}))
			t.Cleanup(upstream.Close)
			incoming := httptest.NewRequest(http.MethodGet, "https://browser.example.test/", nil)
			capture := &shared.ResponseStatusCapture{}
			ctx := context.WithValue(incoming.Context(), shared.Request, incoming)
			ctx = context.WithValue(ctx, shared.ResponseStatusCaptureKey, capture)
			config := ApiFragmentRenderConfig{APIConfig: APIConfig{Endpoint: upstream.URL, Method: http.MethodGet}, Route: "test-api"}
			config.ResponseStatus = tc.policy
			config.Inline = tc.inline
			if config.Inline == "" {
				config.Inline = `{{.Status}}:{{.Data.message}}`
			}
			output, _ := (&ApiFragmentRenderer{}).Render(config, ctx)
			status, diagnostics := capture.Result()
			if status != tc.wantStatus || (len(diagnostics) > 0) != tc.wantDiagnostic {
				t.Fatalf("captured status=%d diagnostics=%v, want status=%d diagnostics=%v", status, diagnostics, tc.wantStatus, tc.wantDiagnostic)
			}
			if tc.wantHTML != "" && output != tc.wantHTML {
				t.Fatalf("HTML = %q, want %q", output, tc.wantHTML)
			}
			for _, diagnostic := range diagnostics {
				if strings.Contains(diagnostic.Error(), "more than once") {
					t.Fatalf("renderer captured more than its final outcome: %v", diagnostics)
				}
			}
		})
	}
}

func TestAPIFragmentRenderCapturesLocalFailureBeforeFetch(t *testing.T) {
	shared.Init_configuration()
	configuration := shared.GetHyperBricksConfiguration()
	previousMode := configuration.Mode
	configuration.Mode = shared.DEVELOPMENT_MODE
	t.Cleanup(func() { configuration.Mode = previousMode })
	incoming := httptest.NewRequest(http.MethodGet, "https://browser.example.test/", nil)
	for _, tc := range []struct {
		name                   string
		missingMethod, badBody bool
	}{
		{name: "missing method", missingMethod: true},
		{name: "incoming body read", badBody: true},
		{name: "missing incoming request"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			capture := &shared.ResponseStatusCapture{}
			ctx := context.WithValue(context.Background(), shared.ResponseStatusCaptureKey, capture)
			if tc.badBody {
				ctx = context.WithValue(ctx, shared.Request, incoming)
				ctx = context.WithValue(ctx, shared.RequestBody, failingAPIFragmentRequestBody{err: errors.New("private input")})
			}
			config := ApiFragmentRenderConfig{APIConfig: APIConfig{Endpoint: "https://api.example.test/", Method: http.MethodGet}, Route: "test-api"}
			config.ResponseStatus = &shared.ResponseStatusConfig{Required: true}
			config.Inline = `{{.Status}}`
			if tc.missingMethod {
				config.Method = ""
			}
			_, errs := (&ApiFragmentRenderer{}).Render(config, ctx)
			status, _ := capture.Result()
			if status != 500 || len(errs) == 0 {
				t.Fatalf("local error yielded status=%d errors=%v", status, errs)
			}
		})
	}
}

func TestAPIFragmentResponseStatusPreservesCookieSuccessGate(t *testing.T) {
	shared.Init_configuration()
	configuration := shared.GetHyperBricksConfiguration()
	previousMode := configuration.Mode
	configuration.Mode = shared.DEVELOPMENT_MODE
	t.Cleanup(func() { configuration.Mode = previousMode })
	for _, tc := range []struct {
		name           string
		upstreamStatus int
		cookie         string
		wantStatus     int
		wantCookie     bool
		wantError      bool
	}{
		{name: "success", upstreamStatus: 200, cookie: "session={{.Data.token}}; Path=/; HttpOnly; Secure; SameSite=Lax", wantCookie: true},
		{name: "mapped response", upstreamStatus: 404, cookie: "session={{.Data.token}}; Path=/; HttpOnly; Secure; SameSite=Lax", wantStatus: 410},
		{name: "cookie failure", upstreamStatus: 200, cookie: "session={{.Data.missing}}; Path=/; HttpOnly; Secure; SameSite=Lax", wantStatus: 500, wantError: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tc.upstreamStatus)
				_, _ = w.Write([]byte(`{"token":"test-token"}`))
			}))
			t.Cleanup(upstream.Close)
			incoming := httptest.NewRequest(http.MethodGet, "https://browser.example.test/", nil)
			writer := httptest.NewRecorder()
			capture := &shared.ResponseStatusCapture{}
			ctx := context.WithValue(incoming.Context(), shared.Request, incoming)
			ctx = context.WithValue(ctx, shared.ResponseStatusCaptureKey, capture)
			ctx = context.WithValue(ctx, shared.ResponseWriter, writer)
			config := ApiFragmentRenderConfig{Route: "test-api", APIConfig: APIConfig{
				Endpoint: upstream.URL, Method: http.MethodGet, Inline: `{{.Status}}`, SetCookie: tc.cookie,
				ResponseStatus: &shared.ResponseStatusConfig{Required: true, Map: map[string]interface{}{"404": 410}},
			}}
			_, errs := (&ApiFragmentRenderer{}).Render(config, ctx)
			status, _ := capture.Result()
			if status != tc.wantStatus || (len(errs) > 0) != tc.wantError {
				t.Fatalf("status=%d errors=%v, want status=%d error=%v", status, errs, tc.wantStatus, tc.wantError)
			}
			if got := len(writer.Header().Values("Set-Cookie")) > 0; got != tc.wantCookie {
				t.Fatalf("cookie emitted=%v, want %v", got, tc.wantCookie)
			}
		})
	}
}
