package main

import (
	"testing"

	"github.com/hyperbricks/hyperbricks/pkg/shared"
	"github.com/yosssi/gohtml"
)

func BenchmarkBeautifyMeasurement(b *testing.B) {
	fixture := setupSSRProofPipelineBenchmark(b)
	config := shared.GetHyperBricksConfiguration()
	formatted := gohtml.Format(fixture.expected)

	for _, enabled := range []bool{false, true} {
		name := "Off"
		expected := fixture.expected
		if enabled {
			name = "On"
			expected = formatted
		}

		b.Run(name, func(b *testing.B) {
			config.Server.Beautify = enabled
			configMutex.Lock()
			delete(configs["index"], "beautify")
			configMutex.Unlock()

			writer := newBenchmarkResponseWriter()
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				benchmarkSSRProofRenderResult = renderContent(writer, "index", fixture.request, "benchmark-request")
			}
			b.StopTimer()

			if benchmarkSSRProofRenderResult.Content != expected {
				b.Fatal("unexpected rendered output")
			}
			b.ReportMetric(float64(len(fixture.expected)), "input-bytes")
			b.ReportMetric(float64(len(expected)), "output-bytes")
		})
	}

	b.Run("FormatOnly", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			benchmarkSSRProofOutput = gohtml.Format(fixture.expected)
		}
	})
}
