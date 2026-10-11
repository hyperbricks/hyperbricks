package render_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/hyperbricks/hyperbricks/pkg/shared"
)

// Real HTTP connection reuse checks pool availability after fully consumed
// responses, including decode/render failures. It does not prove Close itself.
func TestAPIResponseConnectionsReusableAfterErrors(t *testing.T) {
	shared.Init_configuration()
	configuration := shared.GetHyperBricksConfiguration()
	previousMode := configuration.Mode
	configuration.Mode = shared.LIVE_MODE
	t.Cleanup(func() { configuration.Mode = previousMode })
	for _, kind := range []string{"success", "decode", "status", "template"} {
		t.Run(kind, func(t *testing.T) {
			var mu sync.Mutex
			var addresses []string
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				mu.Lock()
				addresses = append(addresses, r.RemoteAddr)
				mu.Unlock()
				w.Header().Set("Content-Type", "application/json")
				if r.URL.Query().Get("id") == "0" {
					if kind == "status" {
						w.WriteHeader(http.StatusServiceUnavailable)
					}
					if kind == "decode" {
						_, _ = w.Write([]byte(`{"message":`))
						return
					}
				}
				_, _ = w.Write([]byte(`{"message":"ok"}`))
			}))
			defer upstream.Close()
			for _, renderer := range apiTemplateRenderers(upstream.URL) {
				t.Run(renderer.name, func(t *testing.T) {
					mu.Lock()
					addresses = nil
					mu.Unlock()
					source := `{{.Status}}:{{.Data.message}}`
					if kind == "template" {
						source = "{{"
					}
					first, errs := renderer.render(apiTemplateInput{inline: source}, 0, shared.Meta{})
					if kind == "status" && first != "503:ok" {
						t.Fatalf("non-2xx response not rendered: %q errors=%v", first, errs)
					}
					if kind == "success" && (first != "200:ok" || len(errs) != 0) {
						t.Fatalf("success response: %q errors=%v", first, errs)
					}
					if (kind == "decode" || kind == "template") && len(errs) == 0 {
						t.Fatal("expected diagnostic from failing first response")
					}
					output, errs := renderer.render(apiTemplateInput{inline: `{{.Data.message}}`}, 1, shared.Meta{})
					if output != "ok" || len(errs) != 0 {
						t.Fatalf("healthy follow-up: output=%q errors=%v", output, errs)
					}
					mu.Lock()
					defer mu.Unlock()
					if len(addresses) != 2 || addresses[0] != addresses[1] {
						t.Fatalf("response connection was not reused: %v", addresses)
					}
				})
			}
		})
	}
}

func TestAPITruncatedResponseFailsWithoutLeakingPayload(t *testing.T) {
	shared.Init_configuration()
	configuration := shared.GetHyperBricksConfiguration()
	previousMode := configuration.Mode
	configuration.Mode = shared.LIVE_MODE
	t.Cleanup(func() { configuration.Mode = previousMode })
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Query().Get("id") == "0" {
			// The declared body never arrives in full. This exercises a real
			// transport read failure, rather than only malformed complete JSON.
			w.Header().Set("Content-Length", "1000")
			_, _ = w.Write([]byte(`{"secret":"private-upstream-payload"}`))
			return
		}
		_, _ = w.Write([]byte(`{"message":"recovered"}`))
	}))
	defer upstream.Close()
	for _, renderer := range apiTemplateRenderers(upstream.URL) {
		t.Run(renderer.name, func(t *testing.T) {
			output, errs := renderer.render(apiTemplateInput{inline: `{{.Data.secret}}`}, 0, shared.Meta{})
			if len(errs) == 0 {
				t.Fatal("truncated response was accepted")
			}
			if strings.Contains(output, "private-upstream-payload") {
				t.Fatalf("partial upstream payload leaked into output: %q", output)
			}
			for _, err := range errs {
				if strings.Contains(err.Error(), "private-upstream-payload") {
					t.Fatal("partial upstream payload leaked into diagnostic")
				}
			}
			output, errs = renderer.render(apiTemplateInput{inline: `{{.Data.message}}`}, 1, shared.Meta{})
			if output != "recovered" || len(errs) != 0 {
				t.Fatalf("healthy follow-up: output=%q errors=%v", output, errs)
			}
		})
	}
}
