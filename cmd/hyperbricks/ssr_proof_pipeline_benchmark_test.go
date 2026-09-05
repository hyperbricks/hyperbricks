package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/hyperbricks/hyperbricks/pkg/parser"
	"github.com/hyperbricks/hyperbricks/pkg/renderplan"
	"github.com/hyperbricks/hyperbricks/pkg/shared"
	yamlparser "github.com/hyperbricks/hyperbricks/pkg/yaml-parser"
)

var (
	benchmarkSSRProofOutput       string
	benchmarkSSRProofRenderResult RenderContent
	benchmarkSSRProofCacheResult  CacheEntry
	benchmarkSSRProofWrittenBytes int
)

type ssrProofPipelineBenchmark struct {
	plan     *renderplan.Plan
	request  *http.Request
	planCtx  context.Context
	expected string
}

func BenchmarkSSRProofCompiledLivePipeline(b *testing.B) {
	fixture := setupSSRProofPipelineBenchmark(b)

	b.Run("PlanRender", func(b *testing.B) {
		reportSSRProofPipelineMetrics(b, len(fixture.expected))
		var output string
		var renderErrors []error
		b.ResetTimer()
		for b.Loop() {
			output, renderErrors = fixture.plan.Render(fixture.planCtx)
		}
		b.StopTimer()
		reportSSRProofGOMAXPROCS(b)
		if len(renderErrors) != 0 {
			b.Fatalf("Plan.Render errors: %v", renderErrors)
		}
		if output != fixture.expected {
			b.Fatalf("Plan.Render output changed: got %d bytes, want %d", len(output), len(fixture.expected))
		}
		benchmarkSSRProofOutput = output
	})

	b.Run("RenderContent", func(b *testing.B) {
		writer := newBenchmarkResponseWriter()
		var result RenderContent
		reportSSRProofPipelineMetrics(b, len(fixture.expected))
		b.ResetTimer()
		for b.Loop() {
			result = renderContent(writer, "index", fixture.request, "benchmark-request")
		}
		b.StopTimer()
		reportSSRProofGOMAXPROCS(b)
		assertSSRProofRenderContent(b, result, fixture.expected)
		benchmarkSSRProofRenderResult = result
	})

	b.Run("HandleLiveModeNoCache", func(b *testing.B) {
		writer := newBenchmarkResponseWriter()
		var result CacheEntry
		reportSSRProofPipelineMetrics(b, len(fixture.expected))
		b.ResetTimer()
		for b.Loop() {
			result = handleLiveMode(writer, "index", fixture.request, "benchmark-request")
		}
		b.StopTimer()
		reportSSRProofGOMAXPROCS(b)
		assertSSRProofCacheEntry(b, result, fixture.expected)
		benchmarkSSRProofCacheResult = result
	})

	b.Run("ServeContentNoCache", func(b *testing.B) {
		writer := newBenchmarkResponseWriter()
		reportSSRProofPipelineMetrics(b, len(fixture.expected))
		b.ResetTimer()
		for b.Loop() {
			writer.reset()
			ServeContent(writer, fixture.request)
		}
		b.StopTimer()
		reportSSRProofGOMAXPROCS(b)
		if writer.status != http.StatusOK {
			b.Fatalf("ServeContent status = %d, want %d", writer.status, http.StatusOK)
		}
		if writer.bytes != len(fixture.expected) {
			b.Fatalf("ServeContent wrote %d bytes, want %d", writer.bytes, len(fixture.expected))
		}
		benchmarkSSRProofWrittenBytes = writer.bytes
	})
}

func setupSSRProofPipelineBenchmark(b testing.TB) ssrProofPipelineBenchmark {
	b.Helper()
	previousProcs := runtime.GOMAXPROCS(4)
	b.Cleanup(func() {
		runtime.GOMAXPROCS(previousProcs)
	})

	setupLiveModeServeContentTest(b)
	hbConfig := shared.GetHyperBricksConfiguration()
	oldBeautify := hbConfig.Server.Beautify
	oldFrontendErrors := hbConfig.Development.FrontendErrors
	hbConfig.Server.Beautify = false
	hbConfig.Development.FrontendErrors = false
	b.Cleanup(func() {
		hbConfig.Server.Beautify = oldBeautify
		hbConfig.Development.FrontendErrors = oldFrontendErrors
	})

	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		b.Fatal("cannot locate SSR proof benchmark")
	}
	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(currentFile), "../.."))
	result, err := yamlparser.ProcessFile(
		filepath.Join(repoRoot, "modules/ssr-proof-hyperbricks/hyperbricks/landing.hyperbricks.yaml"),
		yamlparser.Options{},
	)
	if err != nil {
		b.Fatalf("load SSR proof route: %v", err)
	}
	raw, ok := result.Materialized["ssr_proof_page"].(map[string]interface{})
	if !ok {
		b.Fatalf("ssr_proof_page has type %T", result.Materialized["ssr_proof_page"])
	}
	raw["hyperbricksfile"] = "landing"
	raw["hyperbrickskey"] = "ssr_proof_page"

	plan, err := renderplan.Compile(rm, raw, parser.GetTemplate)
	if err != nil {
		b.Fatalf("compile SSR proof route: %v", err)
	}
	configMutex.Lock()
	configs["index"] = raw
	routePlans["index"] = plan
	configMutex.Unlock()

	if !routeConfiguredNoCache("index") {
		b.Fatal("benchmark contract requires index to be nocache")
	}
	_, registeredPlan, found := getConfigAndPlan("index")
	if !found || registeredPlan != plan {
		b.Fatal("benchmark contract requires the compiled index route plan")
	}

	request := httptest.NewRequest(http.MethodGet, "/?rid=benchmark-request", nil)
	planCtx := context.WithValue(request.Context(), shared.Request, request)
	expected, renderErrors := plan.Render(planCtx)
	if len(renderErrors) != 0 {
		b.Fatalf("preflight Plan.Render errors: %v", renderErrors)
	}
	if strings.Count(expected, "benchmark-request") != 3 {
		b.Fatalf("preflight request isolation marker count = %d, want 3", strings.Count(expected, "benchmark-request"))
	}

	rendered := renderContent(newBenchmarkResponseWriter(), "index", request, "benchmark-request")
	assertSSRProofRenderContent(b, rendered, expected)
	live := handleLiveMode(newBenchmarkResponseWriter(), "index", request, "benchmark-request")
	assertSSRProofCacheEntry(b, live, expected)

	response := httptest.NewRecorder()
	ServeContent(response, request)
	if response.Code != http.StatusOK {
		b.Fatalf("preflight ServeContent status = %d, want %d", response.Code, http.StatusOK)
	}
	if response.Body.String() != expected {
		b.Fatalf("preflight ServeContent output changed: got %d bytes, want %d", response.Body.Len(), len(expected))
	}

	htmlCacheMutex.RLock()
	cachedResponses := len(htmlCache)
	htmlCacheMutex.RUnlock()
	if cachedResponses != 0 {
		b.Fatalf("benchmark contract requires no live cache entries, got %d", cachedResponses)
	}

	return ssrProofPipelineBenchmark{
		plan:     plan,
		request:  request,
		planCtx:  planCtx,
		expected: expected,
	}
}

func reportSSRProofPipelineMetrics(b *testing.B, outputBytes int) {
	b.Helper()
	b.ReportAllocs()
	b.SetBytes(int64(outputBytes))
}

func reportSSRProofGOMAXPROCS(b *testing.B) {
	b.Helper()
	b.ReportMetric(float64(runtime.GOMAXPROCS(0)), "gomaxprocs")
}

func assertSSRProofRenderContent(tb testing.TB, result RenderContent, expected string) {
	tb.Helper()
	if result.Status != http.StatusOK || result.ErrorCount != 0 || !result.NoCache {
		tb.Fatalf(
			"renderContent contract changed: status=%d errors=%d nocache=%t",
			result.Status,
			result.ErrorCount,
			result.NoCache,
		)
	}
	if result.Content != expected {
		tb.Fatalf("renderContent output changed: got %d bytes, want %d", len(result.Content), len(expected))
	}
}

func assertSSRProofCacheEntry(tb testing.TB, result CacheEntry, expected string) {
	tb.Helper()
	if result.Status != http.StatusOK || result.ErrorCount != 0 {
		tb.Fatalf("handleLiveMode contract changed: status=%d errors=%d", result.Status, result.ErrorCount)
	}
	if result.Content != expected {
		tb.Fatalf("handleLiveMode output changed: got %d bytes, want %d", len(result.Content), len(expected))
	}
	if result.ETag != "" || result.ContentLength != "" {
		tb.Fatalf("nocache path populated cache metadata: etag=%q content_length=%q", result.ETag, result.ContentLength)
	}
}
