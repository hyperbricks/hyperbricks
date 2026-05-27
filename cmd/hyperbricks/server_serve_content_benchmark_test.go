package main

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/hyperbricks/hyperbricks/pkg/composite"
	"github.com/hyperbricks/hyperbricks/pkg/shared"
)

type benchmarkResponseWriter struct {
	header http.Header
	status int
	bytes  int
}

func newBenchmarkResponseWriter() *benchmarkResponseWriter {
	return &benchmarkResponseWriter{
		header: make(http.Header, 8),
	}
}

func (w *benchmarkResponseWriter) Header() http.Header {
	return w.header
}

func (w *benchmarkResponseWriter) Write(body []byte) (int, error) {
	w.bytes += len(body)
	return len(body), nil
}

func (w *benchmarkResponseWriter) WriteString(body string) (int, error) {
	w.bytes += len(body)
	return len(body), nil
}

func (w *benchmarkResponseWriter) WriteHeader(status int) {
	w.status = status
}

func (w *benchmarkResponseWriter) reset() {
	w.status = 0
	w.bytes = 0
}

func BenchmarkWriteRenderResponseHTML16KB(b *testing.B) {
	content := strings.Repeat("<section>stable output</section>", 512)
	contentLength := strconv.Itoa(len(content))
	writer := newBenchmarkResponseWriter()

	b.ReportAllocs()
	b.SetBytes(int64(len(content)))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		writer.reset()
		if !writeRenderResponse(writer, "benchmark", "hb-"+strconv.Itoa(i), content, contentLength, nil, nil, nil, "text/html; charset=utf-8", http.StatusOK, 0) {
			b.Fatal("writeRenderResponse returned false")
		}
	}
}

func BenchmarkWriteRenderResponseHandled16KB(b *testing.B) {
	body := []byte(strings.Repeat("runtime-proxy-output", 1024))
	handled := &shared.HandledResponse{
		Status:      http.StatusOK,
		ContentType: "application/octet-stream",
		Body:        body,
	}
	writer := newBenchmarkResponseWriter()

	b.ReportAllocs()
	b.SetBytes(int64(len(body)))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		writer.reset()
		if !writeRenderResponse(writer, "benchmark", "hb-"+strconv.Itoa(i), "", "", handled, nil, nil, "", http.StatusOK, 0) {
			b.Fatal("writeRenderResponse returned false")
		}
	}
}

func BenchmarkServeContentLiveCacheHitHTML(b *testing.B) {
	setupLiveModeServeContentTest(b)

	setTestRouteConfig("bench-live-cache", map[string]interface{}{
		"@type": composite.FragmentConfigGetName(),
		"route": "bench-live-cache",
		"template": map[string]interface{}{
			"@type":  composite.TemplateConfigGetName(),
			"inline": strings.Repeat("<section>{{ .copy }}</section>", 256),
			"values": map[string]interface{}{
				"copy": "stable cached output",
			},
		},
	})

	prewarmWriter := httptest.NewRecorder()
	prewarmRequest := httptest.NewRequest(http.MethodGet, "/bench-live-cache", nil)
	ServeContent(prewarmWriter, prewarmRequest)
	if prewarmWriter.Code != http.StatusOK {
		b.Fatalf("prewarm status = %d, want 200", prewarmWriter.Code)
	}
	if _, found := cachedEntry("bench-live-cache"); !found {
		b.Fatal("expected prewarm request to populate live cache")
	}

	request := httptest.NewRequest(http.MethodGet, "/bench-live-cache", nil)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		writer := httptest.NewRecorder()
		ServeContent(writer, request)
		if writer.Code != http.StatusOK {
			b.Fatalf("status = %d, want 200", writer.Code)
		}
	}
}

func BenchmarkServeContentLiveCacheNotModified(b *testing.B) {
	setupLiveModeServeContentTest(b)

	setTestRouteConfig("bench-live-etag", map[string]interface{}{
		"@type": composite.FragmentConfigGetName(),
		"route": "bench-live-etag",
		"template": map[string]interface{}{
			"@type":  composite.TemplateConfigGetName(),
			"inline": strings.Repeat("<section>{{ .copy }}</section>", 256),
			"values": map[string]interface{}{
				"copy": "stable cached output",
			},
		},
	})

	prewarmWriter := httptest.NewRecorder()
	prewarmRequest := httptest.NewRequest(http.MethodGet, "/bench-live-etag", nil)
	ServeContent(prewarmWriter, prewarmRequest)
	etag := prewarmWriter.Header().Get("ETag")
	if etag == "" {
		b.Fatal("expected prewarm request to return an ETag")
	}

	request := httptest.NewRequest(http.MethodGet, "/bench-live-etag", nil)
	request.Header.Set("If-None-Match", etag)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		writer := httptest.NewRecorder()
		ServeContent(writer, request)
		if writer.Code != http.StatusNotModified {
			b.Fatalf("status = %d, want 304", writer.Code)
		}
	}
}
