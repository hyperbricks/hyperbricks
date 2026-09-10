package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/hyperbricks/hyperbricks/pkg/shared"
)

func TestRenderDefersStreamAndUsesCallbackContext(t *testing.T) {
	var logs bytes.Buffer
	plugin := newTestPlugin(0)
	plugin.logger = log.New(&logs, "", 0)
	renderCtx, cancel := context.WithCancel(context.Background())
	response := renderTestResponse(t, plugin, renderCtx, http.MethodPost)
	if logs.Len() != 0 || plugin.requests.Load() != 0 {
		t.Fatal("Render must not start producing stream output")
	}
	if response.Status != http.StatusOK || response.ContentType != "text/event-stream" || response.Headers["Cache-Control"] != "no-store" || !response.NoCache {
		t.Fatalf("unexpected stream metadata: %#v", response)
	}
	if response.Stream == nil || len(response.Body) != 0 {
		t.Fatal("response must contain only the stream callback, without a buffered body")
	}
	// Rendering is finished. Its cancellation must not cancel the later stream.
	cancel()
	var output bytes.Buffer
	flushed := make([]string, 0, 4)
	err := response.Stream(context.Background(), &output, func() error {
		flushed = append(flushed, output.String())
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	labels := []string{"Started", "Preparing", "Processing", "Finished"}
	percentages := []int{0, 25, 65, 100}
	if len(flushed) != 4 {
		t.Fatalf("flushes = %d, want 4", len(flushed))
	}
	for i, chunk := range flushed {
		if strings.Count(chunk, "\n\n") != i+1 || !strings.Contains(chunk, "<h2>"+labels[i]+"</h2>") || !strings.Contains(chunk, fmt.Sprintf(`value="%d"`, percentages[i])) {
			t.Fatalf("flush %d does not expose its complete SSE message: %s", i+1, chunk)
		}
		if i < 3 && strings.Contains(chunk, "<h2>"+labels[i+1]+"</h2>") {
			t.Fatalf("flush %d already contains a later step: %s", i+1, chunk)
		}
	}
	if !strings.Contains(output.String(), "Demo complete. No project was built.") || !strings.Contains(logs.String(), "COMPLETE") {
		t.Fatal("finished stream is missing its completion result")
	}
}

func TestStreamCancellationStopsPendingSteps(t *testing.T) {
	plugin := newTestPlugin(time.Hour)
	response := renderTestResponse(t, plugin, context.Background(), http.MethodPost)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var output bytes.Buffer
	firstFlushed := make(chan struct{})
	done := make(chan error, 1)
	go func() {
		done <- response.Stream(ctx, &output, func() error {
			close(firstFlushed)
			return nil
		})
	}()
	select {
	case <-firstFlushed:
	case <-time.After(2 * time.Second):
		t.Fatal("first event was not flushed before the later steps")
	}
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("stream error = %v, want cancellation", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("stream did not stop waiting after cancellation")
	}
	if strings.Count(output.String(), "\n\n") != 1 {
		t.Fatalf("cancelled stream continued after first flush: %s", output.String())
	}
}

func TestStreamDeadlineBoundsPendingSteps(t *testing.T) {
	plugin := newTestPlugin(time.Hour)
	plugin.streamTimeout = 20 * time.Millisecond
	response := renderTestResponse(t, plugin, context.Background(), http.MethodPost)
	if err := response.Stream(context.Background(), io.Discard, func() error { return nil }); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("stream error = %v, want deadline exceeded", err)
	}
}

func TestStreamReturnsWriteAndFlushErrors(t *testing.T) {
	for _, failure := range []string{"write", "flush"} {
		t.Run(failure, func(t *testing.T) {
			plugin := newTestPlugin(0)
			response := renderTestResponse(t, plugin, context.Background(), http.MethodPost)
			writes, flushes := 0, 0
			err := response.Stream(context.Background(), writeFunc(func(data []byte) (int, error) {
				writes++
				if failure == "write" {
					return 0, io.ErrClosedPipe
				}
				return len(data), nil
			}), func() error {
				flushes++
				return io.ErrClosedPipe
			})
			if !errors.Is(err, io.ErrClosedPipe) || writes != 1 || (failure == "write" && flushes != 0) || (failure == "flush" && flushes != 1) {
				t.Fatalf("failure=%s err=%v writes=%d flushes=%d", failure, err, writes, flushes)
			}
		})
	}
}

func TestStreamRejectsCanceledContextBeforeWriting(t *testing.T) {
	plugin := newTestPlugin(0)
	response := renderTestResponse(t, plugin, context.Background(), http.MethodPost)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	var output bytes.Buffer
	err := response.Stream(ctx, &output, func() error {
		t.Fatal("cancelled stream must not flush")
		return nil
	})
	if !errors.Is(err, context.Canceled) || output.Len() != 0 {
		t.Fatalf("error=%v output=%q", err, output.String())
	}
}

func TestConcurrentStreamsKeepIndependentSequences(t *testing.T) {
	plugin := newTestPlugin(0)
	var workers sync.WaitGroup
	results := make(chan error, 4)
	for i := 0; i < cap(results); i++ {
		response := renderTestResponse(t, plugin, context.Background(), http.MethodPost)
		workers.Add(1)
		go func() {
			defer workers.Done()
			var output bytes.Buffer
			flushes := 0
			err := response.Stream(context.Background(), &output, func() error { flushes++; return nil })
			if err == nil && (flushes != 4 || strings.Count(output.String(), "\n\n") != 4 || strings.Count(output.String(), `data-step="1"`) != 1 || strings.Count(output.String(), `data-step="4"`) != 1) {
				err = fmt.Errorf("incomplete or shared sequence: flushes=%d output=%s", flushes, output.String())
			}
			results <- err
		}()
	}
	workers.Wait()
	close(results)
	for err := range results {
		if err != nil {
			t.Error(err)
		}
	}
	if plugin.requests.Load() != 4 {
		t.Fatalf("started %d requests, want 4", plugin.requests.Load())
	}
}

func TestRenderRequiresPOSTAndRequestContext(t *testing.T) {
	plugin := newTestPlugin(0)
	for _, method := range []string{http.MethodGet, http.MethodHead, http.MethodPut} {
		response := renderTestResponse(t, plugin, context.Background(), method)
		if response.Status != http.StatusMethodNotAllowed || response.Headers["Allow"] != http.MethodPost || response.Stream != nil {
			t.Fatalf("method %s returned %#v", method, response)
		}
	}
	if _, errs := plugin.Render(nil, context.Background()); len(errs) == 0 {
		t.Fatal("missing request context must reject rendering")
	}
}

func newTestPlugin(delay time.Duration) *streamingDemoPlugin {
	return &streamingDemoPlugin{stepDelay: delay, streamTimeout: 4 * time.Second, logger: log.New(io.Discard, "", 0)}
}

func renderTestResponse(t *testing.T, plugin *streamingDemoPlugin, ctx context.Context, method string) shared.HandledResponse {
	t.Helper()
	request := httptest.NewRequest(method, "/demo/events", nil)
	value, errs := plugin.Render(nil, context.WithValue(ctx, shared.Request, request))
	if len(errs) != 0 {
		t.Fatalf("Render returned errors: %v", errs)
	}
	response, ok := value.(shared.HandledResponse)
	if !ok {
		t.Fatalf("Render returned %T, want shared.HandledResponse", value)
	}
	return response
}

type writeFunc func([]byte) (int, error)

func (write writeFunc) Write(data []byte) (int, error) { return write(data) }
