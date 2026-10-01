package main

import (
	"context"
	"fmt"
	"sync"
	"time"
)

type backgroundUpdate struct {
	path   string
	mu     sync.Mutex
	state  backgroundState
	cancel context.CancelFunc
	done   chan struct{}
}

type backgroundState struct {
	running    bool
	cancelled  bool
	started    time.Time
	finished   time.Time
	progress   updateSnapshot
	waitUntil  time.Time
	waitReason string
	err        error
}

func newBackgroundUpdate(path string) *backgroundUpdate {
	return &backgroundUpdate{path: path}
}

func (b *backgroundUpdate) start(api RiotAPI) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.state.running {
		return fmt.Errorf("update is already running")
	}
	ctx, cancel := context.WithCancel(context.Background())
	b.cancel = cancel
	b.done = make(chan struct{})
	b.state = backgroundState{running: true, started: time.Now()}
	if client, ok := api.(*RiotClient); ok {
		client.quiet = true
		client.limit.onWait = b.setWait
	}
	go b.run(ctx, api, b.done)
	return nil
}

func (b *backgroundUpdate) run(ctx context.Context, api RiotAPI, done chan struct{}) {
	defer close(done)
	store, err := OpenStore(b.path)
	if err == nil {
		err = updatePlayers(ctx, store, api, b.setProgress)
		if closeErr := store.Close(); err == nil {
			err = closeErr
		}
	}
	b.mu.Lock()
	b.state.running = false
	b.state.cancelled = ctx.Err() != nil
	b.state.finished = time.Now()
	b.state.waitUntil = time.Time{}
	b.state.err = err
	b.mu.Unlock()
}

func (b *backgroundUpdate) setProgress(progress updateSnapshot) {
	b.mu.Lock()
	b.state.progress = progress
	b.mu.Unlock()
}

func (b *backgroundUpdate) setWait(until time.Time, reason string) {
	b.mu.Lock()
	b.state.waitUntil = until
	b.state.waitReason = reason
	b.mu.Unlock()
}

func (b *backgroundUpdate) snapshot() backgroundState {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.state
}

func (b *backgroundUpdate) stop() {
	b.mu.Lock()
	cancel, done := b.cancel, b.done
	b.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	if done != nil {
		<-done
	}
}
