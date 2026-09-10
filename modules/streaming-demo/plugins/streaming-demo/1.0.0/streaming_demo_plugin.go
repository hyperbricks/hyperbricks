package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/hyperbricks/hyperbricks/pkg/shared"
)

type streamingDemoPlugin struct {
	stepDelay     time.Duration
	streamTimeout time.Duration
	logger        *log.Logger
	requests      atomic.Uint64
}

var _ shared.PluginRenderer = (*streamingDemoPlugin)(nil)

func (p *streamingDemoPlugin) Render(_ interface{}, ctx context.Context) (any, []error) {
	request, _ := ctx.Value(shared.Request).(*http.Request)
	if request == nil {
		return "", []error{fmt.Errorf("streaming demo requires request context")}
	}
	if request.Method != http.MethodPost {
		return shared.HandledResponse{
			Status:      http.StatusMethodNotAllowed,
			ContentType: "text/plain; charset=utf-8",
			Headers:     map[string]string{"Allow": http.MethodPost, "Cache-Control": "no-store"},
			Body:        []byte("Method not allowed\n"),
			NoCache:     true,
		}, nil
	}

	// Prepare metadata only. HyperBricks invokes Stream with the live request
	// context after rendering; the renderer's context must not be retained.
	return shared.HandledResponse{
		Status:      http.StatusOK,
		ContentType: "text/event-stream",
		Headers:     map[string]string{"Cache-Control": "no-store"},
		NoCache:     true,
		Stream:      p.stream,
	}, nil
}

func (p *streamingDemoPlugin) stream(requestContext context.Context, output io.Writer, flush func() error) error {
	ctx, cancel := context.WithTimeout(requestContext, p.streamTimeout)
	defer cancel()
	requestID := p.requests.Add(1)
	p.logger.Printf("stream=%d START", requestID)
	complete := false
	defer func() {
		if !complete {
			p.logger.Printf("stream=%d CANCELLED", requestID)
		}
	}()

	steps := []struct {
		label   string
		percent int
		message string
	}{
		{"Started", 0, "The simulated build has started."},
		{"Preparing", 25, "Preparing the simulated build."},
		{"Processing", 65, "Processing the simulated build."},
		{"Finished", 100, "Demo complete. No project was built."},
	}
	for index, step := range steps {
		if index > 0 {
			if err := waitForStep(ctx, p.stepDelay); err != nil {
				return err
			}
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		// One line of HTML and a blank line delimit one unnamed SSE event.
		_, err := fmt.Fprintf(output, "data: <section class=\"stream-step\" data-step=\"%d\"><p class=\"step-label\">Step %d of 4 &middot; Simulated build</p><h2>%s</h2><progress max=\"100\" value=\"%d\" aria-label=\"Simulated build progress\">%d%%</progress><p class=\"step-progress\">%d%%</p><p>%s</p></section>\n\n", index+1, index+1, step.label, step.percent, step.percent, step.percent, step.message)
		if err != nil {
			return fmt.Errorf("write stream step %d: %w", index+1, err)
		}
		if err := flush(); err != nil {
			return fmt.Errorf("flush stream step %d: %w", index+1, err)
		}
		p.logger.Printf("stream=%d STEP %d", requestID, index+1)
	}
	complete = true
	p.logger.Printf("stream=%d COMPLETE", requestID)
	return nil
}

func waitForStep(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func Plugin() (shared.PluginRenderer, error) {
	return &streamingDemoPlugin{
		stepDelay:     time.Second,
		streamTimeout: 4 * time.Second,
		logger:        log.Default(),
	}, nil
}
