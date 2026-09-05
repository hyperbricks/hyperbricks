package renderplan_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/hyperbricks/hyperbricks/pkg/component"
	"github.com/hyperbricks/hyperbricks/pkg/composite"
	"github.com/hyperbricks/hyperbricks/pkg/render"
	"github.com/hyperbricks/hyperbricks/pkg/renderer"
	"github.com/hyperbricks/hyperbricks/pkg/renderplan"
	"github.com/hyperbricks/hyperbricks/pkg/shared"
	yamlparser "github.com/hyperbricks/hyperbricks/pkg/yaml-parser"
)

var benchmarkOutput string

func BenchmarkSSRProofProductionPlan(b *testing.B) {
	manager := newTestRenderManager()
	raw := loadSSRProofRouteBenchmark(b)
	plan, err := renderplan.Compile(manager, raw, nil)
	if err != nil {
		b.Fatalf("compile route plan: %v", err)
	}
	ctx := requestContext("benchmark-request")
	want, wantErrors := manager.Render(composite.HyperMediaConfigGetName(), raw, ctx)
	got, gotErrors := plan.Render(ctx)
	if len(wantErrors) != 0 || len(gotErrors) != 0 || got != want {
		b.Fatalf("benchmark contract failed: legacy errors=%v compiled errors=%v", wantErrors, gotErrors)
	}

	b.Run("legacy-map-pipeline", func(b *testing.B) {
		b.ReportAllocs()
		b.SetBytes(int64(len(want)))
		for b.Loop() {
			output, renderErrors := manager.Render(composite.HyperMediaConfigGetName(), raw, ctx)
			if len(renderErrors) != 0 {
				b.Fatalf("legacy render errors: %v", renderErrors)
			}
			benchmarkOutput = output
		}
	})

	b.Run("compiled-route-plan", func(b *testing.B) {
		b.ReportAllocs()
		b.SetBytes(int64(len(want)))
		for b.Loop() {
			output, renderErrors := plan.Render(ctx)
			if len(renderErrors) != 0 {
				b.Fatalf("compiled render errors: %v", renderErrors)
			}
			benchmarkOutput = output
		}
	})
}

func TestCompiledPlanMatchesSSRProofOutput(t *testing.T) {
	manager := newTestRenderManager()
	raw := loadSSRProofRoute(t)
	plan, err := renderplan.Compile(manager, raw, nil)
	if err != nil {
		t.Fatalf("compile route plan: %v", err)
	}

	ctx := requestContext(`special <&> "request"`)
	want, wantErrors := manager.Render(composite.HyperMediaConfigGetName(), raw, ctx)
	got, gotErrors := plan.Render(ctx)
	if got != want {
		t.Fatalf("compiled output differs\ngot:  %q\nwant: %q", got, want)
	}
	if len(gotErrors) != len(wantErrors) {
		t.Fatalf("compiled errors = %v, want %v", gotErrors, wantErrors)
	}
}

func TestCompiledPlanKeepsRequestStateIsolated(t *testing.T) {
	manager := newTestRenderManager()
	plan, err := renderplan.Compile(manager, loadSSRProofRoute(t), nil)
	if err != nil {
		t.Fatalf("compile route plan: %v", err)
	}

	const requests = 256
	var wait sync.WaitGroup
	errors := make(chan error, requests)
	for index := 0; index < requests; index++ {
		requestID := fmt.Sprintf("request-%03d", index)
		wait.Add(1)
		go func() {
			defer wait.Done()
			output, renderErrors := plan.Render(requestContext(requestID))
			if len(renderErrors) != 0 {
				errors <- fmt.Errorf("%s render errors: %v", requestID, renderErrors)
				return
			}
			if count := strings.Count(output, requestID); count != 3 {
				errors <- fmt.Errorf("%s appears %d times, want 3", requestID, count)
			}
		}()
	}
	wait.Wait()
	close(errors)
	for err := range errors {
		t.Error(err)
	}
}

func TestCompiledPlanKeepsMutatingParamsPrivatePerTemplate(t *testing.T) {
	manager := newTestRenderManager()
	raw := headlessTemplateRoute(map[string]interface{}{
		"content": map[string]interface{}{
			"@type":  composite.TreeRendererConfigGetName(),
			"@order": []string{"mutates", "observes"},
			"mutates": map[string]interface{}{
				"@type":     composite.TemplateConfigGetName(),
				"inline":    `{{$_ := set .Params "rid" "changed"}}{{.Params.rid}}|`,
				"querykeys": []string{"rid"},
			},
			"observes": map[string]interface{}{
				"@type":     composite.TemplateConfigGetName(),
				"inline":    `{{.Params.rid}}`,
				"querykeys": []string{"rid"},
			},
		},
	})

	assertPlanParity(t, manager, raw, requestContext("original"), "changed|original")
}

func TestCompiledPlanPreservesExplicitTreeOrder(t *testing.T) {
	manager := newTestRenderManager()
	raw := headlessTemplateRoute(map[string]interface{}{
		"content": map[string]interface{}{
			"@type":  composite.TreeRendererConfigGetName(),
			"@order": []string{"second", "first"},
			"first": map[string]interface{}{
				"@type": component.TextConfigGetName(),
				"value": "A",
			},
			"second": map[string]interface{}{
				"@type": component.TextConfigGetName(),
				"value": "B",
			},
			"not_in_order": map[string]interface{}{
				"@type": component.TextConfigGetName(),
				"value": "C",
			},
		},
	})

	assertPlanParity(t, manager, raw, requestContext("order"), "BA")
}

func TestCompiledPlanExposesTemplateQueryParamsIndependentOfValues(t *testing.T) {
	manager := newTestRenderManager()
	cases := []struct {
		name      string
		setValues bool
		values    interface{}
		want      string
	}{
		{name: "omitted-values", want: "visible/none"},
		{name: "null-values", setValues: true, values: nil, want: "visible/none"},
		{name: "empty-values", setValues: true, values: map[string]interface{}{}, want: "visible/none"},
		{name: "populated-values", setValues: true, values: map[string]interface{}{"label": "configured"}, want: "visible/configured"},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			childTemplate := map[string]interface{}{
				"@type":     composite.TemplateConfigGetName(),
				"inline":    `{{with .Params}}{{.rid}}{{else}}missing{{end}}/{{with .label}}{{.}}{{else}}none{{end}}`,
				"querykeys": []string{"rid"},
			}
			if testCase.setValues {
				childTemplate["values"] = testCase.values
			}
			raw := headlessTemplateRoute(map[string]interface{}{"content": childTemplate})
			assertPlanParity(t, manager, raw, requestContext("visible"), testCase.want)
		})
	}
}

func TestTemplateQueryValuesYAMLReproduction(t *testing.T) {
	manager := newTestRenderManager()
	result, err := yamlparser.ProcessBytes([]byte(`
page:
  - type: hypermedia
  - route: query-example
  - content:
      - type: template
      - querykeys: [q]
      - inline: '<p>{{.Params.q}}</p>'
`), yamlparser.Options{})
	if err != nil {
		t.Fatalf("parse YAML reproduction: %v", err)
	}
	raw, ok := result.Materialized["page"].(map[string]interface{})
	if !ok {
		t.Fatalf("page has type %T", result.Materialized["page"])
	}
	output, renderErrors := manager.Render(composite.HyperMediaConfigGetName(), raw, requestURLContext("/query-example?q=hello"))
	if len(renderErrors) != 0 {
		t.Fatalf("render YAML reproduction: %v", renderErrors)
	}
	if !strings.Contains(output, "<p>hello</p>") {
		t.Fatalf("output %q does not contain %q", output, "<p>hello</p>")
	}
}

func TestStandaloneTemplateExposesQueryParamsWithoutValues(t *testing.T) {
	manager := newTestRenderManager()
	raw := map[string]interface{}{
		"@type":     composite.TemplateConfigGetName(),
		"inline":    `{{.Params.q}}`,
		"querykeys": []string{"q"},
	}
	output, renderErrors := manager.Render(
		composite.TemplateConfigGetName(),
		raw,
		requestURLContext("/?q=standalone"),
	)
	if len(renderErrors) != 0 || output != "standalone" {
		t.Fatalf("output=%q errors=%v, want standalone without errors", output, renderErrors)
	}
}

func TestCompiledPlanPreservesRootTemplateValueNormalization(t *testing.T) {
	manager := newTestRenderManager()
	t.Run("typed-nil-values", func(t *testing.T) {
		raw := map[string]interface{}{
			"@type": composite.HyperMediaConfigGetName(),
			"template": map[string]interface{}{
				"inline":    `{{with .Params}}{{.rid}}{{else}}missing{{end}}`,
				"querykeys": []string{"rid"},
				"values":    map[string]interface{}(nil),
			},
		}
		assertPlanParity(t, manager, raw, requestContext("root-visible"), "root-visible")
	})

	t.Run("weak-map-values", func(t *testing.T) {
		raw := map[string]interface{}{
			"@type": composite.HyperMediaConfigGetName(),
			"template": map[string]interface{}{
				"inline": `{{.label}}`,
				"values": map[string]string{"label": "preserved"},
			},
		}
		assertPlanParity(t, manager, raw, requestContext("unused"), "preserved")
	})
}

func TestCompiledPlanPreservesNamedTemplates(t *testing.T) {
	manager := newTestRenderManager()
	templates := map[string]string{
		"shell.tmpl": `[{{.content}}]`,
		"child.tmpl": `<strong>{{.label}}</strong>`,
	}
	provider := func(name string) (string, bool) {
		content, found := templates[name]
		return content, found
	}
	manager.GetRenderComponent(composite.TemplateConfigGetName()).(*composite.TemplateRenderer).TemplateProvider = provider
	raw := map[string]interface{}{
		"@type": composite.HyperMediaConfigGetName(),
		"template": map[string]interface{}{
			"template": "shell.tmpl",
			"values": map[string]interface{}{
				"content": map[string]interface{}{
					"@type":    composite.TemplateConfigGetName(),
					"template": "child.tmpl",
					"values": map[string]interface{}{
						"label": `safe <value>`,
					},
				},
			},
		},
	}

	plan, err := renderplan.Compile(manager, raw, provider)
	if err != nil {
		t.Fatalf("compile route plan: %v", err)
	}
	want, wantErrors := manager.Render(composite.HyperMediaConfigGetName(), raw, requestContext("unused"))
	got, gotErrors := plan.Render(requestContext("unused"))
	if got != want || got != `[<strong>safe &lt;value&gt;</strong>]` {
		t.Fatalf("compiled output = %q, legacy output = %q", got, want)
	}
	if len(gotErrors) != len(wantErrors) {
		t.Fatalf("compiled errors = %v, legacy errors = %v", gotErrors, wantErrors)
	}
}

func TestCompiledPlanPreservesHTMLTrimSpace(t *testing.T) {
	manager := newTestRenderManager()
	raw := headlessTemplateRoute(map[string]interface{}{
		"content": map[string]interface{}{
			"@type":     component.HTMLConfigGetName(),
			"value":     "  \n<strong>raw</strong>\n  ",
			"trimspace": true,
		},
	})
	assertPlanParity(t, manager, raw, requestContext("unused"), `<strong>raw</strong>`)
}

func TestCompiledPlanPreservesPerTemplateQueryAllowlists(t *testing.T) {
	manager := newTestRenderManager()
	raw := headlessTemplateRoute(map[string]interface{}{
		"content": map[string]interface{}{
			"@type":  composite.TreeRendererConfigGetName(),
			"@order": []string{"defaults", "custom", "none"},
			"defaults": map[string]interface{}{
				"@type":  composite.TemplateConfigGetName(),
				"inline": `default={{.Params.id}}/{{.Params.name}}/{{with .Params.secret}}{{.}}{{else}}hidden{{end}};`,
			},
			"custom": map[string]interface{}{
				"@type":     composite.TemplateConfigGetName(),
				"inline":    `tags={{range $index, $value := .Params.tag}}{{if $index}},{{end}}{{$value}}{{end}};id={{with .Params.id}}{{.}}{{else}}hidden{{end}}`,
				"querykeys": []string{"tag"},
			},
			"none": map[string]interface{}{
				"@type":     composite.TemplateConfigGetName(),
				"inline":    `;none={{with .Params.id}}{{.}}{{else}}hidden{{end}}`,
				"querykeys": []string{},
			},
		},
	})
	ctx := requestURLContext("/?id=42&name=Ada&secret=no&tag=one&tag=two")
	assertPlanParity(t, manager, raw, ctx, "default=42/Ada/hidden;tags=one,two;id=hidden;none=hidden")
}

func TestCompiledPlanPreservesConfiguredParamsWithoutRequest(t *testing.T) {
	manager := newTestRenderManager()
	raw := headlessTemplateRoute(map[string]interface{}{
		"content": map[string]interface{}{
			"@type":  composite.TemplateConfigGetName(),
			"inline": `{{.Params.rid}}`,
			"values": map[string]interface{}{
				"Params": map[string]interface{}{"rid": "configured"},
			},
		},
	})
	assertPlanParity(t, manager, raw, context.Background(), "configured")
}

func TestCompiledPlanProtectsStaticMapsAcrossConcurrentRenders(t *testing.T) {
	manager := newTestRenderManager()
	staticState := map[string]interface{}{}
	raw := headlessTemplateRoute(map[string]interface{}{
		"content": map[string]interface{}{
			"@type":     composite.TemplateConfigGetName(),
			"inline":    `{{$_ := set .state "rid" .Params.rid}}{{.state.rid}}`,
			"querykeys": []string{"rid"},
			"values": map[string]interface{}{
				"state": staticState,
			},
		},
	})
	plan, err := renderplan.Compile(manager, raw, nil)
	if err != nil {
		t.Fatalf("compile route plan: %v", err)
	}

	const requests = 256
	var wait sync.WaitGroup
	errors := make(chan error, requests)
	for index := 0; index < requests; index++ {
		requestID := fmt.Sprintf("mutation-%03d", index)
		wait.Add(1)
		go func() {
			defer wait.Done()
			output, renderErrors := plan.Render(requestContext(requestID))
			if len(renderErrors) != 0 || output != requestID {
				errors <- fmt.Errorf("%s output=%q errors=%v", requestID, output, renderErrors)
			}
		}()
	}
	wait.Wait()
	close(errors)
	for err := range errors {
		t.Error(err)
	}
	if _, mutated := staticState["rid"]; mutated {
		t.Fatal("source static values were mutated")
	}
}

func TestCompileRejectsRouteWithPluginChild(t *testing.T) {
	manager := newTestRenderManager()
	plugin := &requestPlugin{}
	manager.SetPlugin("renderplan-test", plugin)
	raw := headlessTemplateRoute(map[string]interface{}{
		"content": map[string]interface{}{
			"@type": composite.TreeRendererConfigGetName(),
			"plugin": map[string]interface{}{
				"@type":  component.PluginRenderGetName(),
				"plugin": "renderplan-test",
			},
		},
	})

	if _, err := renderplan.Compile(manager, raw, nil); !errors.Is(err, renderplan.ErrNotEligible) {
		t.Fatalf("compile error = %v, want ErrNotEligible", err)
	}
	output, renderErrors := manager.Render(composite.HyperMediaConfigGetName(), raw, requestContext("plugin-request"))
	if len(renderErrors) != 0 || output != "plugin-request" {
		t.Fatalf("legacy fallback output = %q, errors = %v", output, renderErrors)
	}
	if plugin.calls.Load() != 1 {
		t.Fatalf("plugin calls = %d, want one legacy render", plugin.calls.Load())
	}
}

func TestCompileRejectsRouteWithUnknownChild(t *testing.T) {
	manager := newTestRenderManager()
	raw := headlessTemplateRoute(map[string]interface{}{
		"content": map[string]interface{}{
			"@type":  composite.TreeRendererConfigGetName(),
			"@order": []string{"valid", "unknown"},
			"valid": map[string]interface{}{
				"@type": component.TextConfigGetName(),
				"value": "valid",
			},
			"unknown": map[string]interface{}{
				"@type": "<UNKNOWN>",
			},
		},
	})
	if _, err := renderplan.Compile(manager, raw, nil); !errors.Is(err, renderplan.ErrNotEligible) {
		t.Fatalf("compile error = %v, want ErrNotEligible", err)
	}
}

func assertPlanParity(
	t *testing.T,
	manager *render.RenderManager,
	raw map[string]interface{},
	ctx context.Context,
	wantOutput string,
) {
	t.Helper()
	plan, err := renderplan.Compile(manager, raw, nil)
	if err != nil {
		t.Fatalf("compile route plan: %v", err)
	}
	legacyOutput, legacyErrors := manager.Render(composite.HyperMediaConfigGetName(), raw, ctx)
	compiledOutput, compiledErrors := plan.Render(ctx)
	if compiledOutput != legacyOutput {
		t.Fatalf("compiled output differs\ngot:  %q\nwant: %q", compiledOutput, legacyOutput)
	}
	if compiledOutput != wantOutput {
		t.Fatalf("output = %q, want %q", compiledOutput, wantOutput)
	}
	if len(compiledErrors) != len(legacyErrors) {
		t.Fatalf("compiled errors = %v, legacy errors = %v", compiledErrors, legacyErrors)
	}
}

func assertComponentErrorParity(t *testing.T, gotError, wantError error) {
	t.Helper()
	got, gotOK := gotError.(shared.ComponentError)
	want, wantOK := wantError.(shared.ComponentError)
	if !gotOK || !wantOK {
		t.Fatalf("error types = %T and %T, want shared.ComponentError", gotError, wantError)
	}
	got.Hash = ""
	want.Hash = ""
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("compiled error = %#v, want %#v", got, want)
	}
}

func newTestRenderManager() *render.RenderManager {
	shared.Init_configuration()
	hbConfig := shared.GetHyperBricksConfiguration()
	hbConfig.Mode = shared.LIVE_MODE
	hbConfig.Development.FrontendErrors = false

	manager := render.NewRenderManager()
	treeRenderer := &composite.TreeRenderer{
		CompositeRenderer: renderer.CompositeRenderer{RenderManager: manager},
	}
	templateRenderer := &composite.TemplateRenderer{
		CompositeRenderer: renderer.CompositeRenderer{RenderManager: manager},
	}
	hyperMediaRenderer := &composite.HyperMediaRenderer{
		CompositeRenderer: renderer.CompositeRenderer{RenderManager: manager},
	}
	pluginRenderer := &component.PluginRenderer{
		CompositeRenderer: renderer.CompositeRenderer{RenderManager: manager},
	}

	manager.RegisterComponent(component.TextConfigGetName(), &component.TextRenderer{}, reflect.TypeOf(component.TextConfig{}))
	manager.RegisterComponent(component.HTMLConfigGetName(), &component.HTMLRenderer{}, reflect.TypeOf(component.HTMLConfig{}))
	manager.RegisterComponent(component.PluginRenderGetName(), pluginRenderer, reflect.TypeOf(component.PluginConfig{}))
	manager.RegisterComponent(composite.TreeRendererConfigGetName(), treeRenderer, reflect.TypeOf(composite.TreeConfig{}))
	manager.RegisterComponent(composite.TemplateConfigGetName(), templateRenderer, reflect.TypeOf(composite.TemplateConfig{}))
	manager.RegisterComponent(composite.HyperMediaConfigGetName(), hyperMediaRenderer, reflect.TypeOf(composite.HyperMediaConfig{}))
	return manager
}

func loadSSRProofRoute(t *testing.T) map[string]interface{} {
	t.Helper()
	return loadSSRProofRouteForTB(t)
}

func loadSSRProofRouteBenchmark(b *testing.B) map[string]interface{} {
	b.Helper()
	return loadSSRProofRouteForTB(b)
}

func loadSSRProofRouteForTB(tb testing.TB) map[string]interface{} {
	tb.Helper()
	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		tb.Fatal("cannot locate renderplan test")
	}
	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(currentFile), "../.."))
	result, err := yamlparser.ProcessFile(
		filepath.Join(repoRoot, "modules/ssr-proof-hyperbricks/hyperbricks/landing.hyperbricks.yaml"),
		yamlparser.Options{},
	)
	if err != nil {
		tb.Fatalf("load SSR proof config: %v", err)
	}
	raw, ok := result.Materialized["ssr_proof_page"].(map[string]interface{})
	if !ok {
		tb.Fatalf("ssr_proof_page has type %T", result.Materialized["ssr_proof_page"])
	}
	raw["hyperbricksfile"] = "landing"
	raw["hyperbrickskey"] = "ssr_proof_page"
	return raw
}

func headlessTemplateRoute(values map[string]interface{}) map[string]interface{} {
	return map[string]interface{}{
		"@type": composite.HyperMediaConfigGetName(),
		"template": map[string]interface{}{
			"inline": `{{.content}}`,
			"values": values,
		},
	}
}

func requestContext(requestID string) context.Context {
	return requestURLContext("/?rid=" + url.QueryEscape(requestID))
}

func requestURLContext(rawURL string) context.Context {
	request := httptest.NewRequest(http.MethodGet, rawURL, nil)
	return context.WithValue(request.Context(), shared.Request, request)
}

type requestPlugin struct {
	calls atomic.Int32
}

func (p *requestPlugin) Render(_ interface{}, ctx context.Context) (any, []error) {
	p.calls.Add(1)
	request, _ := ctx.Value(shared.Request).(*http.Request)
	if request == nil {
		return "missing-request", nil
	}
	return request.URL.Query().Get("rid"), nil
}
