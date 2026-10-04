package api

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestRateWaitDisplayAndCancellation(t *testing.T) {
	if got := FormatWait(65 * time.Second); got != "01:05" {
		t.Fatalf("countdown %q", got)
	}
	limiter := &rateLimiter{}
	limiter.block(3 * time.Second)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := limiter.wait(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancel: %v", err)
	}
}
