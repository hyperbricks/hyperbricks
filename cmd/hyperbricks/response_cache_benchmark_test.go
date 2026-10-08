package main

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hyperbricks/hyperbricks/pkg/composite"
)

func BenchmarkResponseCacheBackendHit(b *testing.B) {
	for _, policy := range []struct {
		name  string
		value interface{}
	}{
		{"default-memory", nil},
		{"scalar-memory", "10m"},
		{"mapping-memory", map[string]interface{}{"storage": "mem", "expire": "10m"}},
		{"disk", map[string]interface{}{"storage": "disk", "expire": "10m"}},
	} {
		b.Run(policy.name, func(b *testing.B) {
			setupLiveModeServeContentTest(b)
			module := b.TempDir()
			if err := configureResponseCache(module, filepath.Join(module, ".cache"), 256<<20, 10000); err != nil {
				b.Fatal(err)
			}
			b.Cleanup(func() { _ = closeResponseCache() })
			body := strings.Repeat("cache benchmark ", 1024)
			config := map[string]interface{}{
				"@type": composite.FragmentConfigGetName(), "route": "bench", "content_type": "text/plain", "beautify": false,
				"10": map[string]interface{}{"@type": "<TEXT>", "value": body},
			}
			if policy.value != nil {
				config["cache"] = policy.value
			}
			setTestRouteConfig("bench", config)
			request := httptest.NewRequest(http.MethodGet, "/bench", nil)
			writer := newBenchmarkResponseWriter()
			ServeContent(writer, request)
			if writer.status != http.StatusOK || writer.bytes != len(body) {
				b.Fatalf("warm response: status=%d bytes=%d, want %d", writer.status, writer.bytes, len(body))
			}
			entry, found := cachedEntry("bench")
			if !found {
				b.Fatal("cache did not warm")
			}
			b.ReportAllocs()
			b.SetBytes(int64(len(body)))
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				writer.reset()
				ServeContent(writer, request)
				if writer.bytes != len(body) {
					b.Fatal("cache hit body changed")
				}
			}
			b.StopTimer()
			b.ReportMetric(float64(len(entry.Content)), "retained-body-B")
		})
	}
}
