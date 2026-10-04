package core

import (
	"fmt"
	"math"
	"sync"
)

// UpdateReporter receives progress and non-fatal problems from UpdatePlayers.
// Log may be called concurrently from collection workers.
type UpdateReporter interface {
	Progress(UpdateSnapshot)
	Log(message string)
}

type UpdateSnapshot struct {
	Resolved, ResolveTotal int
	Players, PlayerTotal   int
	Matches, MatchTotal    int
	Loadouts, LoadoutTotal int
	Percent                int
}

type updateProgress struct {
	resolved, resolveTotal int
	players, playerTotal   int
	matches, matchTotal    int
	current, currentTotal  int
	loadouts, loadoutTotal int
	reporter               UpdateReporter
	logMu                  sync.Mutex
}

func newUpdateProgress(resolveTotal, playerTotal int, reporter UpdateReporter) *updateProgress {
	return &updateProgress{resolveTotal: resolveTotal, playerTotal: playerTotal, reporter: reporter}
}

func (p *updateProgress) render() {
	if p.reporter == nil {
		return
	}
	resolveWeight := 0.0
	if p.resolveTotal > 0 {
		resolveWeight = 0.1
	}
	resolved := 1.0
	if p.resolveTotal > 0 {
		resolved = float64(p.resolved) / float64(p.resolveTotal)
	}
	collected := 0.0
	if p.playerTotal > 0 {
		fraction := 0.0
		if p.currentTotal > 0 {
			fraction = float64(p.current) / float64(p.currentTotal)
		}
		collected = (float64(p.players) + fraction) / float64(p.playerTotal)
	} else if p.resolved == p.resolveTotal {
		collected = 1
	}
	if p.loadoutTotal > 0 {
		collected = 0.8*collected + 0.2*float64(p.loadouts)/float64(p.loadoutTotal)
	}
	progress := resolveWeight*resolved + (1-resolveWeight)*collected
	progress = math.Max(0, math.Min(1, progress))
	p.reporter.Progress(UpdateSnapshot{p.resolved, p.resolveTotal, p.players, p.playerTotal,
		p.matches, p.matchTotal, p.loadouts, p.loadoutTotal, int(math.Round(progress * 100))})
}

func (p *updateProgress) logf(format string, args ...any) {
	if p.reporter == nil {
		return
	}
	p.logMu.Lock()
	defer p.logMu.Unlock()
	p.reporter.Log(fmt.Sprintf(format, args...))
}
