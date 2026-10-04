package tui

import (
	"context"
	"fmt"
	"math"
	"os"
	"strings"
	"sync"
	"time"

	"matchup-lookup/internal/api"
	"matchup-lookup/internal/core"
)

// ImportSeeds saves seeds locally and prints progress every 50 IDs.
func ImportSeeds(store *core.Store, seeds []core.Seed) error {
	err := core.ImportSeeds(store, seeds, func(saved, total int) {
		if saved%50 == 0 || saved == total {
			fmt.Printf("[IMPORT] Saved %d/%d IDs locally\n", saved, total)
		}
	})
	if err == nil {
		fmt.Println("[IMPORT] Ready. Run update to resolve Riot IDs and fetch matches.")
	}
	return err
}

// UpdatePlayers runs a foreground update with a progress line and
// rate-limit countdown in the terminal.
func UpdatePlayers(ctx context.Context, store *core.Store, client *api.RiotClient) error {
	progress := newTerminalUpdate()
	client.ReportWaits(progress.Wait)
	defer progress.Close()
	return core.UpdatePlayers(ctx, store, client, progress)
}

func FormatWait(delay time.Duration) string {
	seconds := int((delay + time.Second - 1) / time.Second)
	return fmt.Sprintf("%02d:%02d", seconds/60, seconds%60)
}

// terminalUpdate redraws one status line on a terminal. When output is
// redirected it prints a plain line only when counters move noticeably.
type terminalUpdate struct {
	mu              sync.Mutex
	tty             bool
	last, lastPlain core.UpdateSnapshot
	printedPlain    bool
	waitUntil       time.Time
	waitReason      string
	stopCountdown   chan struct{}
}

func newTerminalUpdate() *terminalUpdate {
	info, err := os.Stdout.Stat()
	return &terminalUpdate{tty: err == nil && info.Mode()&os.ModeCharDevice != 0}
}

func (t *terminalUpdate) Progress(s core.UpdateSnapshot) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.last = s
	if t.tty {
		t.drawLocked()
		return
	}
	p := t.lastPlain
	if !t.printedPlain || s.Players != p.Players || s.Resolved/10 != p.Resolved/10 ||
		s.Matches/20 != p.Matches/20 || s.Loadouts/20 != p.Loadouts/20 {
		fmt.Println(progressLine(s))
		t.lastPlain, t.printedPlain = s, true
	}
}

func (t *terminalUpdate) Log(message string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.tty {
		fmt.Print("\r\x1b[2K")
	}
	fmt.Println(message)
	if t.tty {
		t.drawLocked()
	}
}

// Wait receives rate-limit waits from the Riot client; a zero time ends one.
func (t *terminalUpdate) Wait(until time.Time, reason string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.waitUntil, t.waitReason = until, reason
	if until.IsZero() {
		t.stopCountdownLocked()
	} else if !t.tty {
		fmt.Printf("[RATE LIMIT] %s; next request in %s\n", reason, FormatWait(time.Until(until)))
	} else if t.stopCountdown == nil {
		t.stopCountdown = make(chan struct{})
		go t.countdown(t.stopCountdown)
	}
	if t.tty {
		t.drawLocked()
	}
}

func (t *terminalUpdate) Close() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.stopCountdownLocked()
	t.waitUntil = time.Time{}
	if t.tty {
		t.drawLocked()
		fmt.Println()
	} else if t.last != t.lastPlain {
		fmt.Println(progressLine(t.last))
	}
}

func (t *terminalUpdate) countdown(stop chan struct{}) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
			t.mu.Lock()
			t.drawLocked()
			t.mu.Unlock()
		}
	}
}

func (t *terminalUpdate) stopCountdownLocked() {
	if t.stopCountdown != nil {
		close(t.stopCountdown)
		t.stopCountdown = nil
	}
}

func (t *terminalUpdate) drawLocked() {
	line := progressLine(t.last)
	if remaining := time.Until(t.waitUntil); !t.waitUntil.IsZero() && remaining > 0 {
		line += fmt.Sprintf("  · [RATE LIMIT] %s · next request in %s", t.waitReason, FormatWait(remaining))
	}
	fmt.Printf("\r\x1b[2K%s", line)
}

func progressLine(s core.UpdateSnapshot) string {
	const width = 20
	filled := int(math.Round(float64(s.Percent) / 100 * width))
	bar := strings.Repeat("█", filled) + strings.Repeat("·", width-filled)
	line := fmt.Sprintf("[UPDATE] [%s] %3d%%  IDs %d/%d  players %d/%d  matches %d/%d",
		bar, s.Percent, s.Resolved, s.ResolveTotal, s.Players, s.PlayerTotal, s.Matches, s.MatchTotal)
	if s.LoadoutTotal > 0 {
		line += fmt.Sprintf("  loadouts %d/%d", s.Loadouts, s.LoadoutTotal)
	}
	return line
}
