package render_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/hyperbricks/hyperbricks/pkg/component"
	"github.com/hyperbricks/hyperbricks/pkg/composite"
	"github.com/hyperbricks/hyperbricks/pkg/shared"
)

type apiTemplateRenderer struct {
	name   string
	render func(string, int, shared.Meta) (string, []error)
}

func apiTemplateRenderers(endpoint string) []apiTemplateRenderer {
	requestContext := func(id int) context.Context {
		request := httptest.NewRequest(http.MethodGet, "http://browser.example.test/?id="+strconv.Itoa(id), nil)
		return context.WithValue(request.Context(), shared.Request, request)
	}
	return []apiTemplateRenderer{
		{
			name: component.APIConfigGetName(),
			render: func(source string, id int, meta shared.Meta) (string, []error) {
				config := component.APIConfig{
					Component: shared.Component{Meta: meta},
					ApiRenderConfig: component.ApiRenderConfig{
						Endpoint: endpoint, Method: http.MethodGet, Inline: source,
						Values: map[string]interface{}{"Label": fmt.Sprintf("label-%d", id)},
					},
				}
				return (&component.APIRenderer{}).Render(config, requestContext(id))
			},
		},
		{
			name: composite.ApiFragmentRenderConfigGetName(),
			render: func(source string, id int, meta shared.Meta) (string, []error) {
				config := composite.ApiFragmentRenderConfig{
					Composite: shared.Composite{Meta: meta}, Route: "api-template-test",
					APIConfig: composite.APIConfig{
						Endpoint: endpoint, Method: http.MethodGet, Inline: source,
						Values: map[string]interface{}{"Label": fmt.Sprintf("label-%d", id)},
					},
				}
				return (&composite.ApiFragmentRenderer{}).Render(config, requestContext(id))
			},
		},
	}
}

func apiTemplateUpstream(t *testing.T) string {
	t.Helper()
	shared.Init_configuration()
	configuration := shared.GetHyperBricksConfiguration()
	previousMode := configuration.Mode
	configuration.Mode = shared.LIVE_MODE
	t.Cleanup(func() { configuration.Mode = previousMode })
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, _ := strconv.Atoi(r.URL.Query().Get("id"))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK + id%2)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"id": id, "title": fmt.Sprintf("request <%d>&", id), "url": "javascript:alert(1)",
		})
	}))
	t.Cleanup(upstream.Close)
	return upstream.URL
}

func TestAPITemplateReuseKeepsRequestsIsolated(t *testing.T) {
	endpoint := apiTemplateUpstream(t)
	const source = `{{define "api-row"}}<a href="{{.Data.url}}" data-id="{{.Data.id}}">{{upper .Data.title}}</a><span>{{.Status}}:{{.Label}}</span>{{end}}{{template "api-row" .}}`
	for _, renderer := range apiTemplateRenderers(endpoint) {
		t.Run(renderer.name, func(t *testing.T) {
			const requests = 32
			start := make(chan struct{})
			results := make(chan error, requests)
			for id := 0; id < requests; id++ {
				go func() {
					<-start
					output, errs := renderer.render(source, id, shared.Meta{})
					want := fmt.Sprintf(`<a href="#ZgotmplZ" data-id="%d">REQUEST &lt;%d&gt;&amp;</a><span>%d:label-%d</span>`, id, id, http.StatusOK+id%2, id)
					if len(errs) > 0 || output != want {
						results <- fmt.Errorf("request %d: output=%q errors=%v, want %q", id, output, errs, want)
						return
					}
					results <- nil
				}()
			}
			close(start)
			for range requests {
				if err := <-results; err != nil {
					t.Error(err)
				}
			}

			// A source edit must select a new template without changing the old one.
			for _, suffix := range []string{" changed", ""} {
				output, errs := renderer.render(source+suffix, 33, shared.Meta{})
				want := `<a href="#ZgotmplZ" data-id="33">REQUEST &lt;33&gt;&amp;</a><span>201:label-33</span>` + suffix
				if len(errs) > 0 || output != want {
					t.Fatalf("source suffix %q: output=%q errors=%v, want %q", suffix, output, errs, want)
				}
			}
		})
	}
}

func TestAPITemplateErrorsKeepRequestMetadata(t *testing.T) {
	endpoint := apiTemplateUpstream(t)
	for _, renderer := range apiTemplateRenderers(endpoint) {
		for _, tc := range []struct {
			name, source, output, diagnostic string
		}{
			{"parse", "{{", "[error parsing template]", "error parsing API template"},
			{"execute", "{{index .Data.title 999}}", "[error executing template]", "error executing API template"},
		} {
			t.Run(renderer.name+"/"+tc.name, func(t *testing.T) {
				var previousHash string
				for id := 0; id < 2; id++ {
					meta := shared.Meta{
						HyperBricksKey:  fmt.Sprintf("api-%d", id),
						HyperBricksPath: fmt.Sprintf("page.%d", id),
						HyperBricksFile: fmt.Sprintf("page-%d.hyperbricks.yaml", id),
					}
					output, errs := renderer.render(tc.source, id, meta)
					if output != tc.output || len(errs) != 1 {
						t.Fatalf("output=%q errors=%v, want %q and one diagnostic", output, errs, tc.output)
					}
					diagnostic, ok := errs[0].(shared.ComponentError)
					if !ok || diagnostic.Type != renderer.name || diagnostic.Err != tc.diagnostic || diagnostic.Rejected ||
						diagnostic.Key != meta.HyperBricksKey || diagnostic.Path != meta.HyperBricksPath || diagnostic.File != meta.HyperBricksFile {
						t.Fatalf("diagnostic lost current component metadata: %+v", errs[0])
					}
					if diagnostic.Hash == "" || diagnostic.Hash == previousHash {
						t.Fatalf("diagnostic reused a previous request's identity: %+v", diagnostic)
					}
					previousHash = diagnostic.Hash
				}
			})
		}
	}
}
