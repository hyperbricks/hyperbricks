package shared

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
)

func TestResponseStatusOutcomes(t *testing.T) {
	policy := &ResponseStatusConfig{Required: true, Map: map[string]interface{}{"404": 404, "409": "ignore", "502": 503}}
	tests := []struct {
		name    string
		outcome APIResponseOutcome
		want    int
	}{
		{"success", APIResponseOutcome{Received: true, Status: 200}, 0},
		{"mapped missing", APIResponseOutcome{Received: true, Status: 404}, 404},
		{"ignored conflict", APIResponseOutcome{Received: true, Status: 409}, 0},
		{"actual gateway response is mapped", APIResponseOutcome{Received: true, Status: 502}, 503},
		{"network failure is not mapped", APIResponseOutcome{Status: 502, Failure: APIFailureTransport, Err: errors.New("connection refused")}, 502},
		{"timeout", APIResponseOutcome{Status: 502, Failure: APIFailureTransport, Err: fmt.Errorf("wrapped: %w", context.DeadlineExceeded)}, 504},
		{"body read timeout", APIResponseOutcome{Received: true, Status: 404, Failure: APIFailureDecode, Err: context.DeadlineExceeded}, 504},
		{"broken JSON404", APIResponseOutcome{Received: true, Status: 404, Failure: APIFailureDecode, Err: errors.New("decode")}, 502},
		{"broken template404", APIResponseOutcome{Received: true, Status: 404, Failure: APIFailureTemplate}, 500},
		{"unmapped response", APIResponseOutcome{Received: true, Status: 503}, 502},
		{"request canceled", APIResponseOutcome{Failure: APIFailureTransport, Err: context.Canceled}, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &ResponseStatusCapture{}
			ctx := context.WithValue(context.Background(), ResponseStatusCaptureKey, c)
			CaptureResponseStatus(ctx, policy, Meta{HyperBricksPath: "page.api"}, tt.outcome)
			got, _ := c.Result()
			if got != tt.want {
				t.Fatalf("status=%d want%d", got, tt.want)
			}
		})
	}
}

func TestResponseStatusOptionalAndDisabled(t *testing.T) {
	disabled := false
	for _, policy := range []*ResponseStatusConfig{nil, {Enabled: &disabled, Required: true, Map: map[string]interface{}{"404": 404}}, {Map: map[string]interface{}{"404": 404}}} {
		c := &ResponseStatusCapture{}
		ctx := context.WithValue(context.Background(), ResponseStatusCaptureKey, c)
		CaptureResponseStatus(ctx, policy, Meta{}, APIResponseOutcome{Failure: APIFailureTransport, Err: errors.New("offline")})
		if status, diagnostics := c.Result(); status != 0 || len(diagnostics) != 0 {
			t.Fatalf("optional policy changed failure: %d %v", status, diagnostics)
		}
	}
	c := &ResponseStatusCapture{}
	ctx := context.WithValue(context.Background(), ResponseStatusCaptureKey, c)
	CaptureResponseStatus(ctx, &ResponseStatusConfig{Enabled: &disabled, Map: map[string]interface{}{"404": 404}}, Meta{}, APIResponseOutcome{Received: true, Status: 404})
	if status, _ := c.Result(); status != 0 {
		t.Fatal(status)
	}
	// Rendering outside HTTP, or skipping a component entirely, is a no-op.
	CaptureResponseStatus(context.Background(), &ResponseStatusConfig{Required: true}, Meta{}, APIResponseOutcome{Failure: APIFailureLocal})
	if status, _ := (&ResponseStatusCapture{}).Result(); status != 0 {
		t.Fatal(status)
	}
}

func TestResponseStatusConcurrentSelection(t *testing.T) {
	for _, same := range []bool{false, true} {
		for iteration := 0; iteration < 20; iteration++ {
			c := &ResponseStatusCapture{}
			ctx := context.WithValue(context.Background(), ResponseStatusCaptureKey, c)
			var wg sync.WaitGroup
			for i := 0; i < 12; i++ {
				wg.Add(1)
				go func(i int) {
					defer wg.Done()
					status := 404
					if !same && i%2 == 0 {
						status = 503
					}
					p := &ResponseStatusConfig{Priority: 10, Map: map[string]interface{}{fmt.Sprint(status): status}}
					CaptureResponseStatus(ctx, p, Meta{HyperBricksPath: fmt.Sprintf("page.api%d", i)}, APIResponseOutcome{Received: true, Status: status})
				}(i)
			}
			wg.Wait()
			status, diags := c.Result()
			if same && (status != 404 || len(diags) != 0) {
				t.Fatalf("same status tie=%d %v", status, diags)
			}
			if !same && (status != 500 || len(diags) == 0) {
				t.Fatalf("conflict=%d %v", status, diags)
			}
		}
	}
}

func TestResponseStatusPriorityAndRequiredFailure(t *testing.T) {
	for _, withFailure := range []bool{false, true} {
		c := &ResponseStatusCapture{}
		ctx := context.WithValue(context.Background(), ResponseStatusCaptureKey, c)
		CaptureResponseStatus(ctx, &ResponseStatusConfig{Priority: 100, Map: map[string]interface{}{"404": 404}}, Meta{HyperBricksPath: "page.feed"}, APIResponseOutcome{Received: true, Status: 404})
		CaptureResponseStatus(ctx, &ResponseStatusConfig{Priority: 50, Map: map[string]interface{}{"503": 503}}, Meta{HyperBricksPath: "page.pricing"}, APIResponseOutcome{Received: true, Status: 503})
		want := 404
		if withFailure {
			CaptureResponseStatus(ctx, &ResponseStatusConfig{Required: true}, Meta{HyperBricksPath: "page.other"}, APIResponseOutcome{Failure: APIFailureTransport})
			want = 502
		}
		if status, _ := c.Result(); status != want {
			t.Fatalf("status=%d want%d", status, want)
		}
	}
	// Expected pricing404 must not override the primary resource404.
	c := &ResponseStatusCapture{}
	ctx := context.WithValue(context.Background(), ResponseStatusCaptureKey, c)
	CaptureResponseStatus(ctx, &ResponseStatusConfig{Required: true, Map: map[string]interface{}{"404": 404}}, Meta{HyperBricksPath: "feed"}, APIResponseOutcome{Received: true, Status: 404})
	CaptureResponseStatus(ctx, &ResponseStatusConfig{Required: true, Map: map[string]interface{}{"404": "ignore"}}, Meta{HyperBricksPath: "pricing"}, APIResponseOutcome{Received: true, Status: 404})
	if status, diags := c.Result(); status != 404 || len(diags) > 0 {
		t.Fatalf("404/404=%d %v", status, diags)
	}
}

func TestResponseStatusFailureOrderAndDuplicate(t *testing.T) {
	c := &ResponseStatusCapture{}
	ctx := context.WithValue(context.Background(), ResponseStatusCaptureKey, c)
	p := &ResponseStatusConfig{Required: true}
	for i, outcome := range []APIResponseOutcome{{Failure: APIFailureTransport}, {Failure: APIFailureTransport, Err: context.DeadlineExceeded}, {Failure: APIFailureLocal}} {
		CaptureResponseStatus(ctx, p, Meta{HyperBricksPath: fmt.Sprint(i)}, outcome)
	}
	if status, diags := c.Result(); status != 500 || len(diags) != 3 {
		t.Fatalf("failure order %d %v", status, diags)
	}
	c = &ResponseStatusCapture{}
	ctx = context.WithValue(context.Background(), ResponseStatusCaptureKey, c)
	p = &ResponseStatusConfig{Map: map[string]interface{}{"404": 404}}
	for i := 0; i < 2; i++ {
		CaptureResponseStatus(ctx, p, Meta{HyperBricksPath: "same"}, APIResponseOutcome{Received: true, Status: 404})
	}
	if status, diags := c.Result(); status != 500 || len(diags) != 1 {
		t.Fatalf("duplicate %d %v", status, diags)
	}
	c = &ResponseStatusCapture{}
	ctx = context.WithValue(context.Background(), ResponseStatusCaptureKey, c)
	CaptureResponseStatus(ctx, p, Meta{HyperBricksPath: "same"}, APIResponseOutcome{Failure: APIFailureTransport})
	CaptureResponseStatus(ctx, p, Meta{HyperBricksPath: "same"}, APIResponseOutcome{Received: true, Status: 404})
	if status, diags := c.Result(); status != 500 || len(diags) != 1 {
		t.Fatalf("duplicate after optional failure %d %v", status, diags)
	}
}
