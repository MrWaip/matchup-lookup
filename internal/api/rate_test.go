package api

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestRateWaitCancellation(t *testing.T) {
	limiter := &rateLimiter{}
	limiter.block(3 * time.Second)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := limiter.wait(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancel: %v", err)
	}
}
