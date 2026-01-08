package httputil

import (
	"context"
	"testing"
	"time"
)

func TestRateLimiterWithoutDelay(t *testing.T) {
	rl := NewRateLimiter(context.Background(), 0)
	if !rl.Wait(context.Background()) {
		t.Fatalf("rate limiter without delay should allow immediately")
	}
}

func TestRateLimiterWithDelay(t *testing.T) {
	ctx := context.Background()
	rl := NewRateLimiter(ctx, 15*time.Millisecond)

	start := time.Now()
	if !rl.Wait(ctx) {
		t.Fatalf("expected Wait to succeed")
	}
	if elapsed := time.Since(start); elapsed < 10*time.Millisecond {
		t.Fatalf("expected Wait to block for at least ~10ms, got %v", elapsed)
	}
}

func TestRateLimiterCancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	rl := NewRateLimiter(context.Background(), 50*time.Millisecond)
	if rl.Wait(ctx) {
		t.Fatalf("expected Wait to fail when context cancelled")
	}
}
