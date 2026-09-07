package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestDemoEventsFlushFirstEventBeforeLaterSteps(t *testing.T) {
	logs := newDemoLogCapture()
	server := newDemoTestServer(t, logs, time.Hour)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, server.URL+"/demo/events", nil)
	if err != nil {
		t.Fatal(err)
	}
	response, err := server.Client().Do(request)
	if err != nil {
		t.Fatalf("first flushed response must arrive before the later steps: %v", err)
	}
	defer response.Body.Close()
	assertDemoStreamHeaders(t, response)
	first := readDemoEvent(t, bufio.NewReader(response.Body))
	assertDemoEvent(t, first, 1, 0, "Started")
	waitForDemoLog(t, logs, "STEP 1")
	if got := logs.String(); strings.Contains(got, "STEP 2") || strings.Contains(got, "COMPLETE") {
		t.Fatalf("later steps ran while their delay was pending: %s", got)
	}
}

func TestDemoEventsDisconnectCancelsPendingSteps(t *testing.T) {
	logs := newDemoLogCapture()
	server := newDemoTestServer(t, logs, time.Hour)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, server.URL+"/demo/events", nil)
	if err != nil {
		t.Fatal(err)
	}
	response, err := server.Client().Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	readDemoEvent(t, bufio.NewReader(response.Body))
	waitForDemoLog(t, logs, "STEP 1")
	// Closing an unfinished response disconnects this real HTTP client.
	if err := response.Body.Close(); err != nil {
		t.Fatal(err)
	}
	waitForDemoLog(t, logs, "CANCELLED")
	if got := logs.String(); strings.Contains(got, "STEP 2") || strings.Contains(got, "COMPLETE") {
		t.Fatalf("disconnected stream continued: %s", got)
	}
}

func TestDemoEventsCompleteAndUseDistinctStreamIDs(t *testing.T) {
	logs := newDemoLogCapture()
	server := newDemoTestServer(t, logs, 0)
	for requestNumber := 0; requestNumber < 2; requestNumber++ {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		request, err := http.NewRequestWithContext(ctx, http.MethodPost, server.URL+"/demo/events", nil)
		if err != nil {
			cancel()
			t.Fatal(err)
		}
		response, err := server.Client().Do(request)
		if err != nil {
			cancel()
			t.Fatal(err)
		}
		assertDemoStreamHeaders(t, response)
		reader := bufio.NewReader(response.Body)
		labels := []string{"Started", "Preparing", "Processing", "Finished"}
		progress := []int{0, 25, 65, 100}
		for i, label := range labels {
			event := readDemoEvent(t, reader)
			assertDemoEvent(t, event, i+1, progress[i], label)
			if i == 3 && !strings.Contains(event, "Demo complete. No project was built.") {
				t.Fatalf("final event is missing the demo result: %s", event)
			}
		}
		rest, err := io.ReadAll(reader)
		response.Body.Close()
		cancel()
		if err != nil || len(rest) != 0 {
			t.Fatalf("want EOF after four events, got %q, %v", rest, err)
		}
	}
	streamIDs := map[string]bool{}
	for _, match := range regexp.MustCompile(`stream=(\d+) START`).FindAllStringSubmatch(logs.String(), -1) {
		streamIDs[match[1]] = true
	}
	if len(streamIDs) != 2 {
		t.Fatalf("requests must have distinct stream IDs: %s", logs.String())
	}
	for id := range streamIDs {
		for _, action := range []string{"START", "STEP 1", "STEP 2", "STEP 3", "STEP 4", "COMPLETE"} {
			if !strings.Contains(logs.String(), "stream="+id+" "+action) {
				t.Errorf("stream %s missing %s: %s", id, action, logs.String())
			}
		}
	}
}

func TestDemoEventsRequirePOST(t *testing.T) {
	writer := httptest.NewRecorder()
	newDemoHandler(&url.URL{Scheme: "http", Host: "127.0.0.1:1"}, log.New(io.Discard, "", 0), 0).
		ServeHTTP(writer, httptest.NewRequest(http.MethodGet, "/demo/events", nil))
	if writer.Code != http.StatusMethodNotAllowed || writer.Header().Get("Allow") != http.MethodPost {
		t.Fatalf("GET status=%d Allow=%q, want 405 and POST", writer.Code, writer.Header().Get("Allow"))
	}
}

func TestDemoEventsRejectWriterWithoutFlusher(t *testing.T) {
	writer := &plainDemoResponseWriter{header: make(http.Header)}
	newDemoHandler(&url.URL{Scheme: "http", Host: "127.0.0.1:1"}, log.New(io.Discard, "", 0), 0).
		ServeHTTP(writer, httptest.NewRequest(http.MethodPost, "/demo/events", nil))
	if writer.status != http.StatusInternalServerError {
		t.Fatalf("status=%d, want 500 when flushing is unavailable", writer.status)
	}
}

func TestDemoEventsWriteFailureCancelsStream(t *testing.T) {
	logs := newDemoLogCapture()
	writer := &failedDemoResponseWriter{plainDemoResponseWriter{header: make(http.Header)}}
	newDemoHandler(&url.URL{Scheme: "http", Host: "127.0.0.1:1"}, log.New(logs, "", 0), 0).
		ServeHTTP(writer, httptest.NewRequest(http.MethodPost, "/demo/events", nil))
	got := logs.String()
	if !strings.Contains(got, "START") || !strings.Contains(got, "CANCELLED") || strings.Contains(got, "STEP 1") || strings.Contains(got, "COMPLETE") {
		t.Fatalf("write failure must cancel before logging a completed step: %s", got)
	}
}

func TestDemoProxyPreservesRequestAndUpstreamResponse(t *testing.T) {
	type receivedRequest struct{ method, uri, body, header string }
	received := make(chan receivedRequest, 1)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		received <- receivedRequest{r.Method, r.URL.RequestURI(), string(body), r.Header.Get("X-Demo-Request")}
		w.Header().Set("X-Demo-Upstream", "preserved")
		w.WriteHeader(http.StatusCreated)
		io.WriteString(w, "upstream response")
	}))
	t.Cleanup(upstream.Close)
	target, err := url.Parse(upstream.URL)
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(newDemoHandler(target, log.New(io.Discard, "", 0), 0))
	t.Cleanup(server.Close)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, http.MethodPatch, server.URL+"/some/route?value=one", strings.NewReader("request body"))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("X-Demo-Request", "forwarded")
	response, err := server.Client().Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusCreated || response.Header.Get("X-Demo-Upstream") != "preserved" || string(body) != "upstream response" {
		t.Fatalf("proxy response status=%d headers=%v body=%q", response.StatusCode, response.Header, body)
	}
	select {
	case got := <-received:
		want := receivedRequest{http.MethodPatch, "/some/route?value=one", "request body", "forwarded"}
		if got != want {
			t.Fatalf("upstream received %#v, want %#v", got, want)
		}
	case <-ctx.Done():
		t.Fatal("upstream did not receive request")
	}
}

func TestDemoHealth(t *testing.T) {
	writer := httptest.NewRecorder()
	newDemoHandler(&url.URL{Scheme: "http", Host: "127.0.0.1:1"}, log.New(io.Discard, "", 0), 0).
		ServeHTTP(writer, httptest.NewRequest(http.MethodGet, "/demo/health", nil))
	mediaType, _, err := mime.ParseMediaType(writer.Header().Get("Content-Type"))
	if writer.Code != http.StatusOK || err != nil || mediaType != "application/json" {
		t.Fatalf("health status=%d Content-Type=%q", writer.Code, writer.Header().Get("Content-Type"))
	}
	var body map[string]any
	if err := json.Unmarshal(writer.Body.Bytes(), &body); err != nil || body["ok"] != true {
		t.Fatalf("health body=%q, err=%v", writer.Body.String(), err)
	}
}

func newDemoTestServer(t *testing.T, logs *demoLogCapture, delay time.Duration) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(newDemoHandler(&url.URL{Scheme: "http", Host: "127.0.0.1:1"}, log.New(logs, "", 0), delay))
	t.Cleanup(server.Close)
	return server
}

func assertDemoStreamHeaders(t *testing.T, response *http.Response) {
	t.Helper()
	mediaType, _, err := mime.ParseMediaType(response.Header.Get("Content-Type"))
	if response.StatusCode != http.StatusOK || err != nil || mediaType != "text/event-stream" || response.Header.Get("Cache-Control") != "no-store" {
		t.Fatalf("stream status=%d headers=%v", response.StatusCode, response.Header)
	}
}

func readDemoEvent(t *testing.T, reader *bufio.Reader) string {
	t.Helper()
	line, err := reader.ReadString('\n')
	if err != nil {
		t.Fatalf("read SSE data: %v", err)
	}
	data, ok := strings.CutPrefix(strings.TrimSuffix(line, "\n"), "data:")
	if !ok {
		t.Fatalf("want unnamed SSE data event, got %q", line)
	}
	separator, err := reader.ReadString('\n')
	if err != nil || strings.TrimSpace(separator) != "" {
		t.Fatalf("want one data line followed by an empty line, got %q, %v", separator, err)
	}
	return strings.TrimSpace(data)
}

func assertDemoEvent(t *testing.T, event string, step, progress int, label string) {
	t.Helper()
	if !strings.Contains(event, fmt.Sprintf(`data-step="%d"`, step)) || !strings.Contains(event, label) {
		t.Fatalf("event missing step %d or label %q: %s", step, label, event)
	}
	progressTag := regexp.MustCompile(`<progress\b[^>]*>`).FindString(event)
	for attribute, value := range map[string]int{"value": progress, "max": 100} {
		if !strings.Contains(progressTag, fmt.Sprintf(`%s="%d"`, attribute, value)) {
			t.Errorf("progress missing %s=%d: %s", attribute, value, event)
		}
	}
}

type demoLogCapture struct {
	mu      sync.Mutex
	buffer  bytes.Buffer
	changed chan struct{}
}

func newDemoLogCapture() *demoLogCapture {
	return &demoLogCapture{changed: make(chan struct{}, 1)}
}

func (l *demoLogCapture) Write(p []byte) (int, error) {
	l.mu.Lock()
	n, err := l.buffer.Write(p)
	l.mu.Unlock()
	select {
	case l.changed <- struct{}{}:
	default:
	}
	return n, err
}

func (l *demoLogCapture) String() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.buffer.String()
}

func waitForDemoLog(t *testing.T, logs *demoLogCapture, text string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	for !strings.Contains(logs.String(), text) {
		select {
		case <-logs.changed:
		case <-ctx.Done():
			t.Fatalf("missing log %q: %s", text, logs.String())
		}
	}
}

type plainDemoResponseWriter struct {
	header http.Header
	status int
	bytes.Buffer
}

func (w *plainDemoResponseWriter) Header() http.Header    { return w.header }
func (w *plainDemoResponseWriter) WriteHeader(status int) { w.status = status }

type failedDemoResponseWriter struct{ plainDemoResponseWriter }

func (w *failedDemoResponseWriter) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }
func (w *failedDemoResponseWriter) Flush()                    {}
