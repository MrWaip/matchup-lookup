package main

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"
)

// One limiter is shared by all workers. The headroom below Riot's common
// 20/second and 100/two-minutes limits leaves room for clock skew and retries.
type rateLimiter struct {
	mu           sync.Mutex
	requests     []time.Time
	blockedUntil time.Time
	reporting    bool
	onWait       func(time.Time, string)
}

func (l *rateLimiter) wait(ctx context.Context) error {
	for {
		l.mu.Lock()
		now := time.Now()
		cutoff := now.Add(-2 * time.Minute)
		first := 0
		for first < len(l.requests) && !l.requests[first].After(cutoff) {
			first++
		}
		l.requests = l.requests[first:]
		var delay time.Duration
		reason := ""
		if d := time.Until(l.blockedUntil); d > delay {
			delay = d
			reason = "Retry-After from Riot"
		}
		if len(l.requests) >= 90 {
			if d := time.Until(l.requests[0].Add(2 * time.Minute)); d > delay {
				delay = d
				reason = "90 requests / 2 min"
			}
		}
		oneSecond := now.Add(-time.Second)
		recent := 0
		for _, at := range l.requests {
			if at.After(oneSecond) {
				recent++
			}
		}
		if recent >= 18 {
			at := l.requests[len(l.requests)-recent]
			if d := time.Until(at.Add(time.Second)); d > delay {
				delay = d
				reason = "18 requests / 1 sec"
			}
		}
		if delay <= 0 {
			l.requests = append(l.requests, now)
			l.mu.Unlock()
			return nil
		}
		l.mu.Unlock()
		if err := l.pauseWithNotice(ctx, delay, reason); err != nil {
			return err
		}
	}
}

func (l *rateLimiter) pauseWithNotice(ctx context.Context, delay time.Duration, reason string) error {
	if delay < 2*time.Second {
		return pause(ctx, delay)
	}
	if l.onWait != nil {
		l.onWait(time.Now().Add(delay), reason)
		defer l.onWait(time.Time{}, "")
		return pause(ctx, delay)
	}
	l.mu.Lock()
	report := !l.reporting
	if report {
		l.reporting = true
	}
	l.mu.Unlock()
	if !report {
		return pause(ctx, delay)
	}
	info, statErr := os.Stdout.Stat()
	if statErr != nil || info.Mode()&os.ModeCharDevice == 0 {
		fmt.Printf("[RATE LIMIT] %s; next request in %s\n", reason, formatWait(delay))
		defer func() { l.mu.Lock(); l.reporting = false; l.mu.Unlock() }()
		return pause(ctx, delay)
	}
	defer func() {
		l.mu.Lock()
		l.reporting = false
		l.mu.Unlock()
		fmt.Print("\r\x1b[2K")
	}()
	deadline := time.Now().Add(delay)
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return nil
		}
		fmt.Printf("\r\x1b[2K[RATE LIMIT] %s · next request in %s", reason, formatWait(remaining))
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

func formatWait(delay time.Duration) string {
	seconds := int((delay + time.Second - 1) / time.Second)
	return fmt.Sprintf("%02d:%02d", seconds/60, seconds%60)
}

func (l *rateLimiter) block(delay time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if until := time.Now().Add(delay); until.After(l.blockedUntil) {
		l.blockedUntil = until
	}
}
