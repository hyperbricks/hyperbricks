package main

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/hyperbricks/hyperbricks/pkg/component"
	"github.com/hyperbricks/hyperbricks/pkg/composite"
	"github.com/hyperbricks/hyperbricks/pkg/parser"
	"github.com/hyperbricks/hyperbricks/pkg/renderplan"
	"github.com/hyperbricks/hyperbricks/pkg/shared"
)

type nativeStreamTestPlugin struct {
	response func(context.Context) shared.HandledResponse
	calls    atomic.Int32
}

func (p *nativeStreamTestPlugin) Render(_ interface{}, ctx context.Context) (any, []error) {
	p.calls.Add(1)
	return p.response(ctx), nil
}

func TestServeContent_NativeStreamFlushesBeforeCompletion(t *testing.T) {
	for _, compiled := range []bool{false, true} {
		for _, live := range []bool{false, true} {
			t.Run(fmt.Sprintf("compile-requested=%t/live=%t", compiled, live), func(t *testing.T) {
				setupNativeStreamMode(t, live)
				release := make(chan struct{})
				finished := make(chan struct{})
				var releaseOnce sync.Once
				unblock := func() { releaseOnce.Do(func() { close(release) }) }
				defer unblock()
				plugin := &nativeStreamTestPlugin{response: func(context.Context) shared.HandledResponse {
					return shared.HandledResponse{
						Status: http.StatusAccepted, ContentType: "text/event-stream",
						Headers: map[string]string{"X-Priority": "plugin", "Content-Length": "999", "ETag": "obsolete", "Set-Cookie": "plugin-header=1; Path=/"},
						Cookies: []string{"plugin=1; Path=/"},
						Stream: func(ctx context.Context, w io.Writer, flush func() error) error {
							defer close(finished)
							if _, err := io.WriteString(w, "first\n"); err != nil {
								return err
							}
							if err := flush(); err != nil {
								return err
							}
							select {
							case <-release:
							case <-ctx.Done():
								return ctx.Err()
							}
							if _, err := io.WriteString(w, "last\n"); err != nil {
								return err
							}
							return flush()
						},
					}
				}}
				config := nativeStreamRouteConfig()
				config["response"] = map[string]interface{}{"status": http.StatusCreated, "headers": map[string]interface{}{"X-Route": "retained", "X-Priority": "route", "Content-Length": "888", "Cache-Control": "public"}}
				config["cookies"] = []interface{}{"route=1; Path=/"}
				installNativeStreamRoute(t, compiled, config, plugin)
				server := newNativeStreamHTTPServer(t)
				response := requestNativeStream(t, server, http.MethodGet, nil)
				defer response.Body.Close()
				if response.StatusCode != http.StatusAccepted || response.Header.Get("Content-Type") != "text/event-stream" || response.Header.Get("X-Route") != "retained" || response.Header.Get("X-Priority") != "plugin" {
					t.Fatalf("stream metadata status=%d headers=%v", response.StatusCode, response.Header)
				}
				assertNativeStreamUncachedHeaders(t, response)
				assertHTTPCookies(t, response.Header, "plugin-header=1; Path=/", "route=1; Path=/", "plugin=1; Path=/")
				reader := bufio.NewReader(response.Body)
				if first, err := reader.ReadString('\n'); err != nil || first != "first\n" {
					t.Fatalf("first chunk=%q, err=%v", first, err)
				}
				select {
				case <-finished:
					t.Fatal("callback completed before the test released the remaining response")
				default:
				}
				unblock()
				if rest, err := io.ReadAll(reader); err != nil || string(rest) != "last\n" {
					t.Fatalf("stream suffix=%q, err=%v; no surrounding page HTML is allowed", rest, err)
				}
				if plugin.calls.Load() != 1 {
					t.Fatalf("plugin calls=%d, want 1", plugin.calls.Load())
				}
			})
		}
	}
}

func TestServeContent_NativeStreamClientDisconnectCancelsCallback(t *testing.T) {
	for _, compiled := range []bool{false, true} {
		t.Run(fmt.Sprintf("compile-requested=%t", compiled), func(t *testing.T) {
			setupLiveModeServeContentTest(t)
			cancelled := make(chan error, 1)
			plugin := &nativeStreamTestPlugin{response: func(context.Context) shared.HandledResponse {
				return shared.HandledResponse{ContentType: "text/plain", Stream: func(ctx context.Context, w io.Writer, flush func() error) error {
					if _, err := io.WriteString(w, "ready\n"); err != nil {
						return err
					}
					if err := flush(); err != nil {
						return err
					}
					<-ctx.Done()
					cancelled <- ctx.Err()
					return ctx.Err()
				}}
			}}
			installNativeStreamRoute(t, compiled, nativeStreamRouteConfig(), plugin)
			server := newNativeStreamHTTPServer(t)
			response := requestNativeStream(t, server, http.MethodGet, nil)
			if first, err := bufio.NewReader(response.Body).ReadString('\n'); err != nil || first != "ready\n" {
				response.Body.Close()
				t.Fatalf("first chunk=%q, err=%v", first, err)
			}
			// The request deadline is ten seconds; cancellation must come from
			// this closed connection within the shorter observation deadline.
			response.Body.Close()
			select {
			case err := <-cancelled:
				if !errors.Is(err, context.Canceled) {
					t.Fatalf("callback context error=%v, want client cancellation", err)
				}
			case <-time.After(2 * time.Second):
				t.Fatal("client disconnect did not stop callback")
			}
		})
	}
}

func TestServeContent_NativeStreamLiveCacheNeverRetainsProducer(t *testing.T) {
	for _, compiled := range []bool{false, true} {
		t.Run(fmt.Sprintf("compile-requested=%t", compiled), func(t *testing.T) {
			setupLiveModeServeContentTest(t)
			var producers atomic.Int32
			plugin := &nativeStreamTestPlugin{response: func(ctx context.Context) shared.HandledResponse {
				request := ctx.Value(shared.Request).(*http.Request)
				requestValue := request.Header.Get("X-Stream-Request")
				return shared.HandledResponse{ContentType: "text/plain", Stream: func(_ context.Context, w io.Writer, flush func() error) error {
					producers.Add(1)
					if _, err := io.WriteString(w, requestValue); err != nil {
						return err
					}
					return flush()
				}}
			}}
			installNativeStreamRoute(t, compiled, nativeStreamRouteConfig(), plugin)
			server := newNativeStreamHTTPServer(t)
			for _, value := range []string{"first-request", "second-request"} {
				response := requestNativeStream(t, server, http.MethodGet, http.Header{"X-Stream-Request": []string{value}})
				body, err := io.ReadAll(response.Body)
				response.Body.Close()
				if err != nil || string(body) != value {
					t.Fatalf("request %s returned %q, err=%v", value, body, err)
				}
				assertNativeStreamUncachedHeaders(t, response)
			}
			if plugin.calls.Load() != 2 || producers.Load() != 2 {
				t.Fatalf("plugin calls=%d producers=%d, want a fresh pair per request", plugin.calls.Load(), producers.Load())
			}
			htmlCacheMutex.RLock()
			defer htmlCacheMutex.RUnlock()
			if len(htmlCache) != 0 {
				t.Fatalf("stream requests populated %d cache entries", len(htmlCache))
			}
		})
	}
}

func TestServeContent_NativeStreamGuardDenialPreventsPluginAndProducer(t *testing.T) {
	for _, compiled := range []bool{false, true} {
		t.Run(fmt.Sprintf("compile-requested=%t", compiled), func(t *testing.T) {
			setupLiveModeServeContentTest(t)
			var producers atomic.Int32
			plugin := &nativeStreamTestPlugin{response: func(context.Context) shared.HandledResponse {
				return shared.HandledResponse{Stream: func(context.Context, io.Writer, func() error) error {
					producers.Add(1)
					return nil
				}}
			}}
			config := nativeStreamRouteConfig()
			config["guard"] = map[string]interface{}{
				"enabled": true, "require": map[string]interface{}{"authenticated": true},
				"on_unauthenticated": map[string]interface{}{"default": map[string]interface{}{"status": http.StatusUnauthorized, "headers": map[string]interface{}{"X-Denied": "guard"}}},
			}
			installNativeStreamRoute(t, compiled, config, plugin)
			response := requestNativeStream(t, newNativeStreamHTTPServer(t), http.MethodGet, nil)
			defer response.Body.Close()
			body, err := io.ReadAll(response.Body)
			if err != nil || response.StatusCode != http.StatusUnauthorized || response.Header.Get("X-Denied") != "guard" {
				t.Fatalf("denial status=%d headers=%v body=%q err=%v", response.StatusCode, response.Header, body, err)
			}
			if plugin.calls.Load() != 0 || producers.Load() != 0 {
				t.Fatalf("guard invoked plugin=%d producer=%d", plugin.calls.Load(), producers.Load())
			}
		})
	}
}

func TestServeContent_NativeStreamHEADAndBodylessSkipProducer(t *testing.T) {
	for _, compiled := range []bool{false, true} {
		for _, tc := range []struct {
			method string
			status int
		}{{http.MethodHead, http.StatusOK}, {http.MethodGet, http.StatusNoContent}, {http.MethodGet, http.StatusResetContent}, {http.MethodGet, http.StatusNotModified}} {
			t.Run(fmt.Sprintf("compile-requested=%t/%s/%d", compiled, tc.method, tc.status), func(t *testing.T) {
				setupLiveModeServeContentTest(t)
				var producers atomic.Int32
				plugin := &nativeStreamTestPlugin{response: func(context.Context) shared.HandledResponse {
					return shared.HandledResponse{Status: tc.status, Headers: map[string]string{"X-Bodyless": "kept"}, Stream: func(context.Context, io.Writer, func() error) error {
						producers.Add(1)
						return nil
					}}
				}}
				installNativeStreamRoute(t, compiled, nativeStreamRouteConfig(), plugin)
				response := requestNativeStream(t, newNativeStreamHTTPServer(t), tc.method, nil)
				body, err := io.ReadAll(response.Body)
				response.Body.Close()
				if err != nil || response.StatusCode != tc.status || len(body) != 0 || response.Header.Get("X-Bodyless") != "kept" || producers.Load() != 0 {
					t.Fatalf("status=%d body=%q producers=%d headers=%v err=%v", response.StatusCode, body, producers.Load(), response.Header, err)
				}
			})
		}
	}
}

func TestServeContent_NativeStreamRejectsBufferedBody(t *testing.T) {
	for _, compiled := range []bool{false, true} {
		t.Run(fmt.Sprintf("compile-requested=%t", compiled), func(t *testing.T) {
			setupDevelopmentModeServeContentTest(t, true)
			var producers atomic.Int32
			plugin := &nativeStreamTestPlugin{response: func(context.Context) shared.HandledResponse {
				return shared.HandledResponse{Body: []byte("invalid-buffered-body"), Stream: func(context.Context, io.Writer, func() error) error {
					producers.Add(1)
					return nil
				}}
			}}
			installNativeStreamRoute(t, compiled, nativeStreamRouteConfig(), plugin)
			response := requestNativeStream(t, newNativeStreamHTTPServer(t), http.MethodGet, nil)
			body, err := io.ReadAll(response.Body)
			response.Body.Close()
			if err != nil || response.StatusCode != http.StatusInternalServerError || response.Header.Get("Cache-Control") != "no-store" || producers.Load() != 0 {
				t.Fatalf("mixed response status=%d headers=%v producers=%d err=%v", response.StatusCode, response.Header, producers.Load(), err)
			}
			for _, forbidden := range []string{"invalid-buffered-body", "page-before", "page-after", "<html", "<script"} {
				if bytes.Contains(body, []byte(forbidden)) {
					t.Errorf("invalid stream response leaked %q: %s", forbidden, body)
				}
			}
		})
	}
}

func TestServeContent_NativeStreamRejectsMultipleOwnersInFragment(t *testing.T) {
	for _, live := range []bool{false, true} {
		t.Run(fmt.Sprintf("live=%t", live), func(t *testing.T) {
			setupNativeStreamMode(t, live)
			var producers atomic.Int32
			plugin := &nativeStreamTestPlugin{response: func(context.Context) shared.HandledResponse {
				return shared.HandledResponse{Stream: func(context.Context, io.Writer, func() error) error {
					producers.Add(1)
					return nil
				}}
			}}
			rm.SetPlugin("native_stream_test", plugin)
			config := map[string]interface{}{
				"@type": composite.FragmentConfigGetName(), "route": "native-stream",
				"10": map[string]interface{}{"@type": component.PluginRenderGetName(), "plugin": "native_stream_test"},
				"20": map[string]interface{}{"@type": component.PluginRenderGetName(), "plugin": "native_stream_test"},
				"30": map[string]interface{}{"@type": component.HTMLConfigGetName(), "value": "page-after"},
			}
			// Fragment roots are deliberately interpreted in the current plan
			// compiler. Neither plugin may acquire the response after rendering.
			if plan, err := renderplan.Compile(rm, config, parser.GetTemplate); !errors.Is(err, renderplan.ErrNotEligible) || plan != nil {
				t.Fatalf("fragment plan=%v err=%v, want explicit interpreter fallback", plan, err)
			}
			setTestRouteConfig("native-stream", config)
			response := requestNativeStream(t, newNativeStreamHTTPServer(t), http.MethodGet, nil)
			body, err := io.ReadAll(response.Body)
			response.Body.Close()
			if err != nil || response.StatusCode != http.StatusInternalServerError || response.Header.Get("Cache-Control") != "no-store" || plugin.calls.Load() != 2 || producers.Load() != 0 {
				t.Fatalf("multiple owners status=%d plugins=%d producers=%d headers=%v err=%v", response.StatusCode, plugin.calls.Load(), producers.Load(), response.Header, err)
			}
			if bytes.Contains(body, []byte("page-after")) || bytes.Contains(body, []byte("<html")) || bytes.Contains(body, []byte("<script")) {
				t.Fatalf("ambiguous stream leaked rendered HTML: %s", body)
			}
		})
	}
}

func TestServeContent_NativeStreamRequiresFlushing(t *testing.T) {
	setupLiveModeServeContentTest(t)
	var producers atomic.Int32
	plugin := &nativeStreamTestPlugin{response: func(context.Context) shared.HandledResponse {
		return shared.HandledResponse{Stream: func(context.Context, io.Writer, func() error) error {
			producers.Add(1)
			return nil
		}}
	}}
	installNativeStreamRoute(t, false, nativeStreamRouteConfig(), plugin)
	writer := &nativeStreamPlainWriter{header: make(http.Header)}
	ServeContent(writer, httptest.NewRequest(http.MethodGet, "/native-stream", nil))
	if writer.status != http.StatusInternalServerError || writer.header.Get("Cache-Control") != "no-store" || producers.Load() != 0 {
		t.Fatalf("unsupported writer status=%d headers=%v producers=%d", writer.status, writer.header, producers.Load())
	}
	if strings.Contains(writer.body.String(), "page-") || strings.Contains(writer.body.String(), "<html") {
		t.Fatalf("unsupported streaming appended rendered HTML: %s", writer.body.String())
	}
}

func TestWriteStreamResponseIOErrorsCancelAndRemainSticky(t *testing.T) {
	for _, tc := range []struct {
		name        string
		writeError  bool
		flushFailAt int
		wantBody    string
	}{{"write", true, 0, ""}, {"callback-flush", false, 2, "first\n"}} {
		t.Run(tc.name, func(t *testing.T) {
			failure := errors.New("transport failed")
			writer := &nativeStreamFaultWriter{nativeStreamPlainWriter: nativeStreamPlainWriter{header: make(http.Header)}, failure: failure, writeError: tc.writeError, flushFailAt: tc.flushFailAt}
			var contextError, secondWriteError, secondFlushError error
			response := RenderContent{Content: "page HTML must not be appended", Status: http.StatusOK, Handled: &shared.HandledResponse{Stream: func(ctx context.Context, w io.Writer, flush func() error) error {
				_, err := io.WriteString(w, "first\n")
				if err == nil {
					err = flush()
				}
				if !errors.Is(err, failure) {
					t.Errorf("first I/O error=%v, want transport failure", err)
				}
				contextError = ctx.Err()
				_, secondWriteError = io.WriteString(w, "late bytes must not be sent")
				secondFlushError = flush()
				// Even a producer that ignores its I/O error cannot hide the
				// transport failure or continue writing through this wrapper.
				return nil
			}}}
			err := writeStreamResponse(writer, httptest.NewRequest(http.MethodGet, "/native-stream", nil), response)
			if !errors.Is(err, failure) || !errors.Is(secondWriteError, failure) || !errors.Is(secondFlushError, failure) || !errors.Is(contextError, context.Canceled) {
				t.Fatalf("helper=%v repeated write=%v flush=%v context=%v", err, secondWriteError, secondFlushError, contextError)
			}
			if got := writer.body.String(); got != tc.wantBody {
				t.Fatalf("transport body=%q, want %q without a suffix", got, tc.wantBody)
			}
		})
	}
}

func TestWriteStreamResponseInitialFlushFailureSkipsProducer(t *testing.T) {
	failure := errors.New("initial flush failed")
	writer := &nativeStreamFaultWriter{nativeStreamPlainWriter: nativeStreamPlainWriter{header: make(http.Header)}, failure: failure, flushFailAt: 1}
	called := false
	response := RenderContent{Content: "page-after", Handled: &shared.HandledResponse{Stream: func(context.Context, io.Writer, func() error) error {
		called = true
		return nil
	}}}
	err := writeStreamResponse(writer, httptest.NewRequest(http.MethodGet, "/native-stream", nil), response)
	if !errors.Is(err, failure) || called || writer.body.Len() != 0 {
		t.Fatalf("initial flush error=%v producer=%t body=%q", err, called, writer.body.String())
	}
}

func TestServeContent_NativeStreamProducerFailureDoesNotAppendHTML(t *testing.T) {
	setupDevelopmentModeServeContentTest(t, true)
	plugin := &nativeStreamTestPlugin{response: func(context.Context) shared.HandledResponse {
		return shared.HandledResponse{Stream: func(_ context.Context, w io.Writer, flush func() error) error {
			if _, err := io.WriteString(w, "partial\n"); err != nil {
				return err
			}
			if err := flush(); err != nil {
				return err
			}
			return errors.New("producer failed")
		}}
	}}
	installNativeStreamRoute(t, false, nativeStreamRouteConfig(), plugin)
	response := requestNativeStream(t, newNativeStreamHTTPServer(t), http.MethodGet, nil)
	body, err := io.ReadAll(response.Body)
	response.Body.Close()
	if err != nil || string(body) != "partial\n" || response.StatusCode != http.StatusOK {
		t.Fatalf("failed producer status=%d body=%q err=%v", response.StatusCode, body, err)
	}
}

func TestServeContent_NativeStreamKeepsBufferedAndNormalResponses(t *testing.T) {
	for _, compiled := range []bool{false, true} {
		for _, buffered := range []bool{false, true} {
			t.Run(fmt.Sprintf("compile-requested=%t/buffered=%t", compiled, buffered), func(t *testing.T) {
				setupLiveModeServeContentTest(t)
				plugin := &nativeStreamTestPlugin{response: func(context.Context) shared.HandledResponse {
					return shared.HandledResponse{Status: http.StatusCreated, ContentType: "application/octet-stream", Body: []byte{0, 'A', 'B'}}
				}}
				config := nativeStreamRouteConfig()
				if !buffered {
					template := config["template"].(map[string]interface{})
					delete(template["values"].(map[string]interface{}), "stream")
					template["inline"] = "{{.before}}{{.after}}"
				}
				installNativeStreamRoute(t, compiled, config, plugin)
				response := requestNativeStream(t, newNativeStreamHTTPServer(t), http.MethodGet, nil)
				body, err := io.ReadAll(response.Body)
				response.Body.Close()
				if err != nil || response.Header.Get("Content-Length") != strconv.Itoa(len(body)) {
					t.Fatalf("buffered=%t length=%q body=%q err=%v", buffered, response.Header.Get("Content-Length"), body, err)
				}
				if buffered {
					if response.StatusCode != http.StatusCreated || response.Header.Get("Content-Type") != "application/octet-stream" || !bytes.Equal(body, []byte{0, 'A', 'B'}) {
						t.Fatalf("buffered response changed: status=%d headers=%v body=%v", response.StatusCode, response.Header, body)
					}
				} else if response.StatusCode != http.StatusOK || !bytes.Contains(body, []byte("page-before")) || !bytes.Contains(body, []byte("page-after")) || response.Header.Get("ETag") == "" {
					t.Fatalf("normal response changed: status=%d headers=%v body=%q", response.StatusCode, response.Header, body)
				}
			})
		}
	}
}

func setupNativeStreamMode(t *testing.T, live bool) {
	t.Helper()
	if live {
		setupLiveModeServeContentTest(t)
	} else {
		setupDevelopmentModeServeContentTest(t, true)
	}
}

func nativeStreamRouteConfig() map[string]interface{} {
	return map[string]interface{}{
		"@type": composite.HyperMediaConfigGetName(), "route": "native-stream",
		"template": map[string]interface{}{
			"@type": composite.TemplateConfigGetName(), "inline": "{{.before}}{{.stream}}{{.after}}",
			"values": map[string]interface{}{
				"before": map[string]interface{}{"@type": component.HTMLConfigGetName(), "value": "<p>page-before</p>"},
				"stream": map[string]interface{}{"@type": component.PluginRenderGetName(), "plugin": "native_stream_test"},
				"after":  map[string]interface{}{"@type": component.HTMLConfigGetName(), "value": "<p>page-after</p>"},
			},
		},
	}
}

func installNativeStreamRoute(t *testing.T, compiled bool, config map[string]interface{}, plugin *nativeStreamTestPlugin) {
	t.Helper()
	rm.SetPlugin("native_stream_test", plugin)
	setTestRouteConfig("native-stream", config)
	if compiled {
		plan, err := renderplan.Compile(rm, config, parser.GetTemplate)
		_, hasPlugin := config["template"].(map[string]interface{})["values"].(map[string]interface{})["stream"]
		if hasPlugin {
			// The current compiler intentionally leaves plugin components to
			// the interpreter. Verify that exact eligibility decision instead
			// of pretending the stream callback ran in a compiled plugin node.
			if !errors.Is(err, renderplan.ErrNotEligible) || !strings.Contains(err.Error(), component.PluginRenderGetName()) || plan != nil {
				t.Fatalf("plugin route must fall back specifically for PLUGIN_RENDER: plan=%v err=%v", plan, err)
			}
			return
		}
		if err != nil {
			t.Fatalf("compile normal template-backed route: %v", err)
		}
		configMutex.Lock()
		routePlans["native-stream"] = plan
		configMutex.Unlock()
	}
}

func newNativeStreamHTTPServer(t *testing.T) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(ServeContent))
	t.Cleanup(server.Close)
	return server
}

func requestNativeStream(t *testing.T, server *httptest.Server, method string, headers http.Header) *http.Response {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)
	request, err := http.NewRequestWithContext(ctx, method, server.URL+"/native-stream", nil)
	if err != nil {
		t.Fatal(err)
	}
	request.Header = headers.Clone()
	response, err := server.Client().Do(request)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { response.Body.Close() })
	return response
}

func assertNativeStreamUncachedHeaders(t *testing.T, response *http.Response) {
	t.Helper()
	for _, header := range []string{"Content-Length", "ETag", liveCacheRenderedAtHeader, liveCacheExpiresAtHeader} {
		if got := response.Header.Get(header); got != "" {
			t.Errorf("stream retained %s=%q", header, got)
		}
	}
	if response.ContentLength != -1 || response.Header.Get("Cache-Control") != "no-store" {
		t.Errorf("stream ContentLength=%d Cache-Control=%q", response.ContentLength, response.Header.Get("Cache-Control"))
	}
}

type nativeStreamPlainWriter struct {
	header http.Header
	status int
	body   bytes.Buffer
}

func (w *nativeStreamPlainWriter) Header() http.Header { return w.header }
func (w *nativeStreamPlainWriter) WriteHeader(status int) {
	if w.status == 0 {
		w.status = status
	}
}
func (w *nativeStreamPlainWriter) Write(body []byte) (int, error) {
	if w.status == 0 {
		w.WriteHeader(http.StatusOK)
	}
	return w.body.Write(body)
}

type nativeStreamFaultWriter struct {
	nativeStreamPlainWriter
	failure     error
	writeError  bool
	flushFailAt int
	flushes     int
}

func (w *nativeStreamFaultWriter) Write(body []byte) (int, error) {
	if w.writeError {
		return 0, w.failure
	}
	return w.nativeStreamPlainWriter.Write(body)
}
func (w *nativeStreamFaultWriter) Flush() { _ = w.FlushError() }
func (w *nativeStreamFaultWriter) FlushError() error {
	w.flushes++
	if w.flushFailAt > 0 && w.flushes >= w.flushFailAt {
		return w.failure
	}
	return nil
}
