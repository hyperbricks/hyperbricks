package main

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/hyperbricks/hyperbricks/pkg/parser"
	"github.com/hyperbricks/hyperbricks/pkg/renderplan"
	"github.com/hyperbricks/hyperbricks/pkg/shared"
	yamlparser "github.com/hyperbricks/hyperbricks/pkg/yaml-parser"
)

func BenchmarkSourceAwareRender(b *testing.B) {
	for _, mode := range []string{shared.LIVE_MODE, shared.DEVELOPMENT_MODE} {
		b.Run(mode, func(b *testing.B) {
			fixture := setupSSRProofPipelineBenchmark(b)
			getHyperBricksConfiguration().Mode = mode
			_, file, _, _ := runtime.Caller(0)
			module := filepath.Join(filepath.Dir(file), "../../modules/ssr-proof-hyperbricks")
			result, err := yamlparser.ProcessFile(filepath.Join(module, "hyperbricks/landing.hyperbricks.yaml"),
				yamlparser.Options{IncludeSourceMetadata: yamlRuntimeOptions().IncludeSourceMetadata, Paths: yamlparser.PathMarkers{Module: module}})
			if err != nil {
				b.Fatal(err)
			}
			raw := result.Materialized["ssr_proof_page"].(map[string]interface{})
			if err := prepareSourceMetadata(raw); err != nil {
				b.Fatal(err)
			}
			plans := make(map[string]*renderplan.Plan)
			if mode == shared.LIVE_MODE {
				plan, err := renderplan.Compile(rm, raw, parser.GetTemplate)
				if err != nil {
					b.Fatal(err)
				}
				plans["index"] = plan
			}
			updateGlobalRoutes(map[string]map[string]interface{}{"index": raw}, plans)
			writer := httptest.NewRecorder()
			ServeContent(writer, fixture.request)
			if writer.Code != http.StatusOK || writer.Header().Get(renderErrorCountHeader) != "0" || writer.Body.String() != fixture.expected {
				b.Fatalf("source-aware render changed output: status=%d diagnostics=%s", writer.Code, writer.Header().Get(renderErrorCountHeader))
			}
			output := newBenchmarkResponseWriter()
			b.ReportAllocs()
			b.SetBytes(int64(len(fixture.expected)))
			b.ResetTimer()
			for b.Loop() {
				output.reset()
				ServeContent(output, fixture.request)
			}
			b.StopTimer()
			if output.bytes != len(fixture.expected) || output.status != http.StatusOK {
				b.Fatal("source-aware render contract changed")
			}
		})
	}
}

func BenchmarkCurrentDiagnosticsSnapshot(b *testing.B) {
	setupDevelopmentModeServeContentTest(b, false)
	routes := make(map[string]map[string]interface{})
	for index := 0; index < maxRenderDiagnostics; index++ {
		id := nextRenderRequestID()
		routes[id] = nil
	}
	updateGlobalRoutes(routes, nil)
	for route := range routes {
		recordRenderDiagnostics(nil, nextRenderRequestID(), route, []error{shared.ComponentError{Level: "WARNING", Err: "benchmark warning"}})
	}
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		if snapshot := collectCurrentRenderDiagnostics(); len(snapshot.Records) != maxRenderDiagnostics {
			b.Fatal("diagnostics snapshot lost records")
		}
	}
}
