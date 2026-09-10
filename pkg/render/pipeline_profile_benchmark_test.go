package render_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"testing"

	"github.com/hyperbricks/hyperbricks/pkg/component"
	"github.com/hyperbricks/hyperbricks/pkg/composite"
	hbrender "github.com/hyperbricks/hyperbricks/pkg/render"
	"github.com/hyperbricks/hyperbricks/pkg/renderer"
	"github.com/hyperbricks/hyperbricks/pkg/shared"
	yamlparser "github.com/hyperbricks/hyperbricks/pkg/yaml-parser"
	"github.com/mitchellh/mapstructure"
)

var (
	pipelineOutputSink string
	pipelineMapSink    map[string]interface{}
	pipelineParamsSink url.Values
)

func BenchmarkSSRProofRenderGraph(b *testing.B) {
	rm, hypermediaRenderer := benchmarkSSRRenderManager()
	nested := loadSSRProofConfig(b)
	flat := flatSSRProofConfig()
	ctx := benchmarkRequestContext("pipeline-profile")

	want, errs := rm.Render(composite.HyperMediaConfigGetName(), nested, ctx)
	if len(errs) != 0 {
		b.Fatalf("nested render errors: %v", errs)
	}
	assertProfileOutput(b, rm, flat, ctx, want)

	var typedRoot composite.HyperMediaConfig
	if err := mapstructure.WeakDecode(nested, &typedRoot); err != nil {
		b.Fatalf("decode typed root: %v", err)
	}
	typedOutput, typedErrs := hypermediaRenderer.Render(typedRoot, ctx)
	if len(typedErrs) != 0 {
		b.Fatalf("typed root render errors: %v", typedErrs)
	}
	if typedOutput != want {
		b.Fatalf("typed root output differs: got %d bytes, want %d", len(typedOutput), len(want))
	}

	var typedFlatRoot composite.HyperMediaConfig
	if err := mapstructure.WeakDecode(flat, &typedFlatRoot); err != nil {
		b.Fatalf("decode flat typed root: %v", err)
	}
	flatTypedOutput, flatTypedErrs := hypermediaRenderer.Render(typedFlatRoot, ctx)
	if len(flatTypedErrs) != 0 || flatTypedOutput != want {
		b.Fatalf("flat typed root differs: output=%d bytes errors=%v", len(flatTypedOutput), flatTypedErrs)
	}

	flatTemplateConfig := composite.TemplateConfig{TemplateOptions: *typedFlatRoot.Template}
	flatTemplateRenderer := &composite.TemplateRenderer{}
	flatTemplateOutput, flatTemplateErrs := flatTemplateRenderer.Render(flatTemplateConfig, ctx)
	if len(flatTemplateErrs) != 0 || flatTemplateOutput != want {
		b.Fatalf("flat typed template differs: output=%d bytes errors=%v", len(flatTemplateOutput), flatTemplateErrs)
	}

	cases := []struct {
		name   string
		render func() (string, []error)
	}{
		{
			name: "nested-current-map-pipeline",
			render: func() (string, []error) {
				return rm.Render(composite.HyperMediaConfigGetName(), nested, ctx)
			},
		},
		{
			name: "nested-pretyped-root-only",
			render: func() (string, []error) {
				return hypermediaRenderer.Render(typedRoot, ctx)
			},
		},
		{
			name: "flat-current-map-pipeline",
			render: func() (string, []error) {
				return rm.Render(composite.HyperMediaConfigGetName(), flat, ctx)
			},
		},
		{
			name: "flat-pretyped-root",
			render: func() (string, []error) {
				return hypermediaRenderer.Render(typedFlatRoot, ctx)
			},
		},
		{
			name: "flat-pretyped-template",
			render: func() (string, []error) {
				return flatTemplateRenderer.Render(flatTemplateConfig, ctx)
			},
		},
	}

	for _, benchCase := range cases {
		b.Run(benchCase.name, func(b *testing.B) {
			b.ReportAllocs()
			b.SetBytes(int64(len(want)))
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				output, renderErrs := benchCase.render()
				if len(renderErrs) != 0 {
					b.Fatalf("render errors: %v", renderErrs)
				}
				pipelineOutputSink = output
			}
		})
	}
}

func BenchmarkSSRProofTreeStrategyParallelRequests(b *testing.B) {
	nested := loadSSRProofConfig(b)
	ctx := benchmarkRequestContext("tree-strategy")

	currentRM, _ := benchmarkSSRRenderManager()
	typedConcurrentRM := benchmarkSSRRenderManagerWithTypedTree(true)
	typedSequentialRM := benchmarkSSRRenderManagerWithTypedTree(false)

	want, errs := currentRM.Render(composite.HyperMediaConfigGetName(), nested, ctx)
	if len(errs) != 0 {
		b.Fatalf("current render errors: %v", errs)
	}
	assertProfileOutput(b, typedConcurrentRM, nested, ctx, want)
	assertProfileOutput(b, typedSequentialRM, nested, ctx, want)

	cases := []struct {
		name string
		rm   *hbrender.RenderManager
	}{
		{name: "current-decode-and-goroutines", rm: currentRM},
		{name: "typed-tree-with-goroutines", rm: typedConcurrentRM},
		{name: "typed-tree-sequential", rm: typedSequentialRM},
	}

	for _, benchCase := range cases {
		b.Run(benchCase.name, func(b *testing.B) {
			b.ReportAllocs()
			b.SetBytes(int64(len(want)))
			b.RunParallel(func(pb *testing.PB) {
				for pb.Next() {
					output, renderErrs := benchCase.rm.Render(composite.HyperMediaConfigGetName(), nested, ctx)
					if len(renderErrs) != 0 || output == "" {
						b.Fatalf("render failed: output=%d bytes errors=%v", len(output), renderErrs)
					}
				}
			})
		})
	}
}

func BenchmarkPipelineDecodeBoundary(b *testing.B) {
	rm := benchmarkRenderManager()
	ctx := benchmarkRequestContext("decode-boundary")

	textMap := map[string]interface{}{
		"@type": component.TextConfigGetName(),
		"value": "A fast HyperBricks text component.",
	}
	textConfig := component.TextConfig{Value: "A fast HyperBricks text component."}
	textRenderer := &component.TextRenderer{}

	templateMap := map[string]interface{}{
		"@type":     composite.TemplateConfigGetName(),
		"inline":    `<p>Request {{.Params.rid}} stays isolated.</p>`,
		"querykeys": []interface{}{"rid"},
		"values":    map[string]interface{}{},
	}
	templateConfig := composite.TemplateConfig{
		TemplateOptions: composite.TemplateOptions{
			Inline:           `<p>Request {{.Params.rid}} stays isolated.</p>`,
			AllowedQueryKeys: []string{"rid"},
			Values:           map[string]interface{}{},
		},
	}
	templateRenderer := &composite.TemplateRenderer{}

	cases := []struct {
		name   string
		render func() (string, []error)
	}{
		{
			name: "text-current-map-pipeline",
			render: func() (string, []error) {
				return rm.Render(component.TextConfigGetName(), textMap, ctx)
			},
		},
		{
			name: "text-pretyped-renderer",
			render: func() (string, []error) {
				return textRenderer.Render(textConfig, ctx)
			},
		},
		{
			name: "template-current-map-pipeline",
			render: func() (string, []error) {
				return rm.Render(composite.TemplateConfigGetName(), templateMap, ctx)
			},
		},
		{
			name: "template-pretyped-renderer",
			render: func() (string, []error) {
				return templateRenderer.Render(templateConfig, ctx)
			},
		},
	}

	for _, benchCase := range cases {
		output, errs := benchCase.render()
		if len(errs) != 0 || output == "" {
			b.Fatalf("%s validation failed: output=%q errors=%v", benchCase.name, output, errs)
		}
		b.Run(benchCase.name, func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				output, renderErrs := benchCase.render()
				if len(renderErrs) != 0 {
					b.Fatalf("render errors: %v", renderErrs)
				}
				pipelineOutputSink = output
			}
		})
	}
}

func BenchmarkPipelineRequestClone(b *testing.B) {
	nested := loadSSRProofConfig(b)
	templateMap := nested["template"].(map[string]interface{})
	values := templateMap["values"].(map[string]interface{})

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pipelineMapSink = shared.CloneMapDeep(values)
	}
}

func BenchmarkPipelineRequestParameterPreparation(b *testing.B) {
	request := httptest.NewRequest(
		http.MethodGet,
		"/?rid=request-42&campaign=proof&locale=nl&ignored=value",
		nil,
	)
	allowed := []string{"rid"}
	prepared := url.Values{"rid": []string{"request-42"}}

	b.Run("filter-at-three-template-boundaries", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			pipelineParamsSink = composite.FilterAllowedQueryParams(request, allowed)
			pipelineParamsSink = composite.FilterAllowedQueryParams(request, allowed)
			pipelineParamsSink = composite.FilterAllowedQueryParams(request, allowed)
		}
	})

	b.Run("filter-once-per-request", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			pipelineParamsSink = composite.FilterAllowedQueryParams(request, allowed)
		}
	})

	b.Run("reuse-request-scoped-values", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			pipelineParamsSink = prepared
		}
	})
}

func BenchmarkPipelineTemplateValuePreparation(b *testing.B) {
	nested := loadSSRProofConfig(b)
	templateMap := nested["template"].(map[string]interface{})
	values := templateMap["values"].(map[string]interface{})
	params := map[string]interface{}{"rid": "request-42"}

	b.Run("deep-clone-static-graph", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			prepared := shared.CloneMapDeep(values)
			prepared["Params"] = params
			pipelineMapSink = prepared
		}
	})

	b.Run("shallow-request-overlay", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			prepared := make(map[string]interface{}, len(values)+1)
			for key, value := range values {
				prepared[key] = value
			}
			prepared["Params"] = params
			pipelineMapSink = prepared
		}
	})
}

func BenchmarkTreeScheduling(b *testing.B) {
	for _, children := range []int{3, 16, 32} {
		children := children
		b.Run(fmt.Sprintf("children-%02d", children), func(b *testing.B) {
			rm := benchmarkRenderManager()
			data := benchmarkTreeData(children)
			ctx := context.Background()
			concurrentRenderer := &composite.TreeRenderer{
				CompositeRenderer: renderer.CompositeRenderer{RenderManager: rm},
			}

			concurrentOutput, concurrentErrs := concurrentRenderer.Render(data, ctx)
			sequentialOutput, sequentialErrs := renderTreeSequentialProfile(rm, data, ctx)
			if len(concurrentErrs) != 0 || len(sequentialErrs) != 0 {
				b.Fatalf("tree validation errors: concurrent=%v sequential=%v", concurrentErrs, sequentialErrs)
			}
			if concurrentOutput != sequentialOutput {
				b.Fatalf("tree outputs differ: concurrent=%q sequential=%q", concurrentOutput, sequentialOutput)
			}

			b.Run("goroutine-per-child", func(b *testing.B) {
				b.ReportAllocs()
				for i := 0; i < b.N; i++ {
					output, errs := concurrentRenderer.Render(data, ctx)
					if len(errs) != 0 {
						b.Fatalf("render errors: %v", errs)
					}
					pipelineOutputSink = output
				}
			})

			b.Run("sequential", func(b *testing.B) {
				b.ReportAllocs()
				for i := 0; i < b.N; i++ {
					output, errs := renderTreeSequentialProfile(rm, data, ctx)
					if len(errs) != 0 {
						b.Fatalf("render errors: %v", errs)
					}
					pipelineOutputSink = output
				}
			})
		})
	}
}

func BenchmarkTreeSchedulingParallelRequests(b *testing.B) {
	for _, children := range []int{3, 16} {
		children := children
		b.Run(fmt.Sprintf("children-%02d", children), func(b *testing.B) {
			rm := benchmarkRenderManager()
			data := benchmarkTreeData(children)
			ctx := context.Background()
			concurrentRenderer := &composite.TreeRenderer{
				CompositeRenderer: renderer.CompositeRenderer{RenderManager: rm},
			}

			b.Run("goroutine-per-child", func(b *testing.B) {
				b.ReportAllocs()
				b.RunParallel(func(pb *testing.PB) {
					for pb.Next() {
						output, errs := concurrentRenderer.Render(data, ctx)
						if len(errs) != 0 {
							b.Fatalf("render errors: %v", errs)
						}
						pipelineOutputSink = output
					}
				})
			})

			b.Run("sequential", func(b *testing.B) {
				b.ReportAllocs()
				b.RunParallel(func(pb *testing.PB) {
					for pb.Next() {
						output, errs := renderTreeSequentialProfile(rm, data, ctx)
						if len(errs) != 0 {
							b.Fatalf("render errors: %v", errs)
						}
						pipelineOutputSink = output
					}
				})
			})
		})
	}
}

func benchmarkSSRRenderManager() (*hbrender.RenderManager, *composite.HyperMediaRenderer) {
	rm := benchmarkRenderManager()
	hbConfig := shared.GetHyperBricksConfiguration()
	hbConfig.Mode = shared.LIVE_MODE
	hbConfig.Development.FrontendErrors = false

	hypermediaRenderer := &composite.HyperMediaRenderer{
		CompositeRenderer: renderer.CompositeRenderer{RenderManager: rm},
	}
	rm.RegisterComponent(
		composite.HyperMediaConfigGetName(),
		hypermediaRenderer,
		reflect.TypeOf(composite.HyperMediaConfig{}),
	)
	return rm, hypermediaRenderer
}

func benchmarkSSRRenderManagerWithTypedTree(concurrent bool) *hbrender.RenderManager {
	rm, _ := benchmarkSSRRenderManager()
	rm.RegisterComponent(
		composite.TreeRendererConfigGetName(),
		&profileTypedTreeRenderer{rm: rm, concurrent: concurrent},
		reflect.TypeOf(composite.TreeConfig{}),
	)
	return rm
}

func loadSSRProofConfig(tb testing.TB) map[string]interface{} {
	tb.Helper()
	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		tb.Fatal("cannot locate pipeline benchmark source")
	}
	root := filepath.Clean(filepath.Join(filepath.Dir(currentFile), "../.."))
	result, err := yamlparser.ProcessFile(
		filepath.Join(root, "modules/ssr-proof-hyperbricks/hyperbricks/landing.hyperbricks.yaml"),
		yamlparser.Options{},
	)
	if err != nil {
		tb.Fatalf("load SSR proof config: %v", err)
	}
	config, ok := result.Materialized["ssr_proof_page"].(map[string]interface{})
	if !ok {
		tb.Fatalf("ssr_proof_page has type %T", result.Materialized["ssr_proof_page"])
	}
	return config
}

func flatSSRProofConfig() map[string]interface{} {
	return map[string]interface{}{
		"@type":        composite.HyperMediaConfigGetName(),
		"route":        "index",
		"nocache":      true,
		"beautify":     false,
		"content_type": "text/html; charset=utf-8",
		"headers": map[string]interface{}{
			"Cache-Control": "no-store",
		},
		"template": map[string]interface{}{
			"inline":    `<!DOCTYPE html><html><head><title>SSR Proof Landing</title></head><body><main id="landing" data-request-id="{{.Params.rid}}"><section class="hero"><h1>SSR Proof Landing</h1><p>Nested server-rendered output with request isolation.</p><a href="/start">Start</a></section><section class="features"><div class="feature-list"><article class="feature-card"><h2>Native rendering</h2><p>HTML is generated on the server.</p></article><article class="feature-card"><h2>Nested composition</h2><p>Components contain components across three levels.</p></article><article class="feature-card"><h2>Request state</h2><p>Request {{.Params.rid}} stays isolated.</p></article></div></section><section class="proof"><dl class="metrics"><div class="metric"><dt>Request</dt><dd>{{.Params.rid}}</dd></div><div class="metric"><dt>Mode</dt><dd>SSR</dd></div></dl></section></main></body></html>`,
			"querykeys": []interface{}{"rid"},
			"values":    map[string]interface{}{},
		},
	}
}

func benchmarkRequestContext(requestID string) context.Context {
	request := httptest.NewRequest(http.MethodGet, "/?rid="+requestID, nil)
	return context.WithValue(request.Context(), shared.Request, request)
}

func assertProfileOutput(
	b *testing.B,
	rm *hbrender.RenderManager,
	config map[string]interface{},
	ctx context.Context,
	want string,
) {
	b.Helper()
	output, errs := rm.Render(composite.HyperMediaConfigGetName(), config, ctx)
	if len(errs) != 0 {
		b.Fatalf("render errors: %v", errs)
	}
	if output != want {
		b.Fatalf("output differs: got %d bytes, want %d", len(output), len(want))
	}
}

func renderTreeSequentialProfile(
	rm *hbrender.RenderManager,
	data map[string]interface{},
	ctx context.Context,
) (string, []error) {
	var config composite.TreeConfig
	if err := mapstructure.Decode(data, &config); err != nil {
		return "", []error{err}
	}

	keys := shared.SortedUniqueKeys(config.Items)
	var output strings.Builder
	var renderErrs []error
	for _, key := range keys {
		componentConfig, ok := config.Items[key].(map[string]interface{})
		if !ok {
			continue
		}
		componentType, ok := componentConfig["@type"].(string)
		if !ok {
			continue
		}

		localConfig := make(map[string]interface{}, len(componentConfig)+3)
		for name, value := range componentConfig {
			localConfig[name] = value
		}
		localConfig["hyperbrickskey"] = key
		localConfig["hyperbricksfile"] = config.HyperBricksFile
		localConfig["hyperbrickspath"] = config.HyperBricksPath + "." + key

		rendered, errs := rm.Render(componentType, localConfig, ctx)
		output.WriteString(rendered)
		renderErrs = append(renderErrs, errs...)
	}
	return shared.EncloseContent(config.Enclose, output.String()), renderErrs
}

type profileTypedTreeRenderer struct {
	rm         *hbrender.RenderManager
	concurrent bool
}

func (r *profileTypedTreeRenderer) Types() []string {
	return []string{composite.TreeRendererConfigGetName()}
}

func (r *profileTypedTreeRenderer) Render(instance interface{}, ctx context.Context) (string, []error) {
	var config composite.TreeConfig
	switch typed := instance.(type) {
	case composite.TreeConfig:
		config = typed
	case *composite.TreeConfig:
		if typed != nil {
			config = *typed
		}
	default:
		if err := mapstructure.WeakDecode(instance, &config); err != nil {
			return "", []error{err}
		}
	}

	keys := profileOrderedTreeKeys(config.Items)
	if !r.concurrent {
		var output strings.Builder
		var renderErrs []error
		for _, key := range keys {
			rendered, errs := renderProfileTreeChild(r.rm, config, key, ctx)
			output.WriteString(rendered)
			renderErrs = append(renderErrs, errs...)
		}
		return shared.EncloseContent(config.Enclose, output.String()), renderErrs
	}

	outputs := make([]string, len(keys))
	errorsByChild := make([][]error, len(keys))
	var wg sync.WaitGroup
	for index, key := range keys {
		wg.Add(1)
		go func(index int, key string) {
			defer wg.Done()
			outputs[index], errorsByChild[index] = renderProfileTreeChild(r.rm, config, key, ctx)
		}(index, key)
	}
	wg.Wait()

	var output strings.Builder
	var renderErrs []error
	for index := range outputs {
		output.WriteString(outputs[index])
		renderErrs = append(renderErrs, errorsByChild[index]...)
	}
	return shared.EncloseContent(config.Enclose, output.String()), renderErrs
}

func renderProfileTreeChild(
	rm *hbrender.RenderManager,
	config composite.TreeConfig,
	key string,
	ctx context.Context,
) (string, []error) {
	componentConfig, ok := config.Items[key].(map[string]interface{})
	if !ok {
		return "", nil
	}
	componentType, ok := componentConfig["@type"].(string)
	if !ok {
		return "", nil
	}

	localConfig := make(map[string]interface{}, len(componentConfig)+3)
	for name, value := range componentConfig {
		localConfig[name] = value
	}
	localConfig["hyperbrickskey"] = key
	localConfig["hyperbricksfile"] = config.HyperBricksFile
	localConfig["hyperbrickspath"] = strings.Trim(config.HyperBricksPath+"."+key, ".")
	return rm.Render(componentType, localConfig, ctx)
}

func profileOrderedTreeKeys(items map[string]interface{}) []string {
	if rawOrder, ok := items["@order"]; ok {
		var ordered []string
		switch values := rawOrder.(type) {
		case []string:
			ordered = append(ordered, values...)
		case []interface{}:
			for _, value := range values {
				if key, ok := value.(string); ok {
					ordered = append(ordered, key)
				}
			}
		}
		if len(ordered) > 0 {
			return ordered
		}
	}
	return shared.SortedUniqueKeys(items)
}
