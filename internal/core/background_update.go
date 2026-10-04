package core

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// WaitReporter is implemented by API clients that can report rate-limit waits
// to a callback instead of printing them to the terminal.
type WaitReporter interface {
	ReportWaits(func(until time.Time, reason string))
}

type BackgroundUpdate struct {
	path   string
	mu     sync.Mutex
	state  BackgroundState
	cancel context.CancelFunc
	done   chan struct{}
}

type BackgroundState struct {
	Running    bool
	Cancelled  bool
	Started    time.Time
	Finished   time.Time
	Progress   UpdateSnapshot
	WaitUntil  time.Time
	WaitReason string
	Err        error
}

func NewBackgroundUpdate(path string) *BackgroundUpdate {
	return &BackgroundUpdate{path: path}
}

func (b *BackgroundUpdate) Start(api RiotAPI) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.state.Running {
		return fmt.Errorf("update is already running")
	}
	ctx, cancel := context.WithCancel(context.Background())
	b.cancel = cancel
	b.done = make(chan struct{})
	b.state = BackgroundState{Running: true, Started: time.Now()}
	if reporter, ok := api.(WaitReporter); ok {
		reporter.ReportWaits(b.setWait)
	}
	go b.run(ctx, api, b.done)
	return nil
}

func (b *BackgroundUpdate) run(ctx context.Context, api RiotAPI, done chan struct{}) {
	defer close(done)
	store, err := OpenStore(b.path)
	if err == nil {
		err = updatePlayers(ctx, store, api, b.setProgress)
		if closeErr := store.Close(); err == nil {
			err = closeErr
		}
	}
	b.mu.Lock()
	b.state.Running = false
	b.state.Cancelled = ctx.Err() != nil
	b.state.Finished = time.Now()
	b.state.WaitUntil = time.Time{}
	b.state.Err = err
	b.mu.Unlock()
}

func (b *BackgroundUpdate) setProgress(progress UpdateSnapshot) {
	b.mu.Lock()
	b.state.Progress = progress
	b.mu.Unlock()
}

func (b *BackgroundUpdate) setWait(until time.Time, reason string) {
	b.mu.Lock()
	b.state.WaitUntil = until
	b.state.WaitReason = reason
	b.mu.Unlock()
}

func (b *BackgroundUpdate) Snapshot() BackgroundState {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.state
}

func (b *BackgroundUpdate) Stop() {
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
