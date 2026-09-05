package main

import (
	"testing"

	"github.com/hyperbricks/hyperbricks/pkg/shared"
)

func TestNewRequestRateLimiterCanBeDisabled(t *testing.T) {
	config := shared.RateLimitConfig{
		Enabled:           false,
		RequestsPerSecond: 1_000_000,
		Burst:             1_000_000,
	}

	if limiter := newRequestRateLimiter(config); limiter != nil {
		t.Fatal("expected disabled rate limiter to return nil")
	}
}

func TestNewRequestRateLimiterUsesConfiguredLimit(t *testing.T) {
	config := shared.RateLimitConfig{
		Enabled:           true,
		RequestsPerSecond: 1,
		Burst:             1,
	}

	limiter := newRequestRateLimiter(config)
	if limiter == nil {
		t.Fatal("expected enabled rate limiter")
	}
	if !limiter.Allow() {
		t.Fatal("expected the initial burst token to be available")
	}
	if limiter.Allow() {
		t.Fatal("expected a second immediate request to exceed the configured burst")
	}
}
