package component

import (
	"context"
	"io"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/hyperbricks/hyperbricks/pkg/shared"
)

type responsePlugin struct{ response any }

func (p responsePlugin) Render(_ interface{}, _ context.Context) (any, []error) {
	return p.response, nil
}

func TestPluginStreamIsCapturedWithoutExecutingOrWrapping(t *testing.T) {
	var calls atomic.Int32
	response := shared.HandledResponse{
		Status: 201, ContentType: "application/octet-stream",
		Headers: map[string]string{"X-Example": "original"}, Cookies: []string{"session=original"},
		Stream: func(_ context.Context, output io.Writer, flush func() error) error {
			calls.Add(1)
			if _, err := io.WriteString(output, "chunk"); err != nil {
				return err
			}
			return flush()
		},
	}
	for _, value := range []any{response, &response} {
		capture := &shared.HandledResponseCapture{}
		ctx := context.WithValue(context.Background(), shared.HandledResponseCaptureKey, capture)
		config := PluginConfig{Classes: []string{"wrapper"}}
		config.Enclose = "<div>|</div>"
		output, errs := (&PluginRenderer{}).renderAndWrap(responsePlugin{value}, config, config, ctx, nil)
		if len(errs) > 0 || output != "" || calls.Load() != 0 {
			t.Fatalf("capture executed/wrapped stream: output=%q calls=%d errors=%v", output, calls.Load(), errs)
		}
		got, err := capture.Result()
		if err != nil || got == nil || got.Stream == nil || got.Status != 201 {
			t.Fatalf("response lost during capture: %#v, %v", got, err)
		}
		got.Headers["X-Example"] = "changed"
		got.Cookies[0] = "session=changed"
		if response.Headers["X-Example"] != "original" || response.Cookies[0] != "session=original" {
			t.Fatal("capture shares mutable response metadata with plugin")
		}
	}
}

func TestPluginStreamRequiresCapture(t *testing.T) {
	response := shared.HandledResponse{Stream: func(context.Context, io.Writer, func() error) error {
		t.Fatal("stream executed outside HTTP lifecycle")
		return nil
	}}
	output, errs := (&PluginRenderer{}).renderAndWrap(responsePlugin{response}, PluginConfig{}, nil, context.Background(), nil)
	if len(errs) != 1 || !strings.Contains(output, "without capture context") {
		t.Fatalf("output=%q errors=%v", output, errs)
	}
}

func TestPluginStreamRejectsAmbiguousBody(t *testing.T) {
	capture := &shared.HandledResponseCapture{}
	ctx := context.WithValue(context.Background(), shared.HandledResponseCaptureKey, capture)
	response := shared.HandledResponse{Body: []byte("buffered"), Stream: func(context.Context, io.Writer, func() error) error {
		t.Fatal("invalid stream was executed")
		return nil
	}}
	_, errs := (&PluginRenderer{}).renderAndWrap(responsePlugin{response}, PluginConfig{}, nil, ctx, nil)
	if len(errs) != 1 {
		t.Fatalf("errors=%v", errs)
	}
	if got, err := capture.Result(); got != nil || err == nil {
		t.Fatalf("ambiguous response captured: %v, %v", got, err)
	}
}

func TestPluginConcurrentResponseOwnersInvalidateCapture(t *testing.T) {
	capture := &shared.HandledResponseCapture{}
	ctx := context.WithValue(context.Background(), shared.HandledResponseCaptureKey, capture)
	var wg sync.WaitGroup
	for range 16 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			response := shared.HandledResponse{Stream: func(context.Context, io.Writer, func() error) error {
				t.Error("conflicting stream executed")
				return nil
			}}
			(&PluginRenderer{}).renderAndWrap(responsePlugin{response}, PluginConfig{}, nil, ctx, nil)
		}()
	}
	wg.Wait()
	got, err := capture.Result()
	if got != nil || err == nil || !strings.Contains(err.Error(), "multiple plugins") {
		t.Fatalf("concurrent response ownership was not rejected: %v, %v", got, err)
	}
}
