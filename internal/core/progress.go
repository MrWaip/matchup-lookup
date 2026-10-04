package core

import (
	"fmt"
	"math"
	"os"
	"strings"
)

type updateProgress struct {
	resolved, resolveTotal                                                   int
	players, playerTotal                                                     int
	matches, matchTotal                                                      int
	current, currentTotal                                                    int
	loadouts, loadoutTotal                                                   int
	lastPlainPlayers, lastPlainResolves, lastPlainMatches, lastPlainLoadouts int
	onChange                                                                 func(UpdateSnapshot)
}

type UpdateSnapshot struct {
	Resolved, ResolveTotal int
	Players, PlayerTotal   int
	Matches, MatchTotal    int
	Loadouts, LoadoutTotal int
	Percent                int
}

func newUpdateProgress(resolveTotal, playerTotal int) *updateProgress {
	return &updateProgress{resolveTotal: resolveTotal, playerTotal: playerTotal,
		lastPlainPlayers: -1, lastPlainResolves: -1, lastPlainMatches: -1, lastPlainLoadouts: -1}
}

func (p *updateProgress) render() {
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
	percent := int(math.Round(progress * 100))
	const width = 20
	filled := int(math.Round(progress * width))
	bar := strings.Repeat("█", filled) + strings.Repeat("·", width-filled)
	line := fmt.Sprintf("[UPDATE] [%s] %3d%%  IDs %d/%d  players %d/%d  matches %d/%d",
		bar, percent, p.resolved, p.resolveTotal, p.players, p.playerTotal, p.matches, p.matchTotal)
	if p.loadoutTotal > 0 {
		line += fmt.Sprintf("  loadouts %d/%d", p.loadouts, p.loadoutTotal)
	}
	if p.onChange != nil {
		p.onChange(UpdateSnapshot{p.resolved, p.resolveTotal, p.players, p.playerTotal, p.matches, p.matchTotal, p.loadouts, p.loadoutTotal, percent})
		return
	}
	info, err := os.Stdout.Stat()
	if err != nil || info.Mode()&os.ModeCharDevice == 0 {
		if p.players != p.lastPlainPlayers || p.resolved/10 != p.lastPlainResolves/10 || p.matches/20 != p.lastPlainMatches/20 || p.loadouts/20 != p.lastPlainLoadouts/20 {
			fmt.Println(line)
			p.lastPlainPlayers, p.lastPlainResolves, p.lastPlainMatches, p.lastPlainLoadouts = p.players, p.resolved, p.matches, p.loadouts
		}
		return
	}
	fmt.Printf("\r\x1b[2K%s", line)
}

func (p *updateProgress) close() {
	p.render()
	if p.onChange == nil {
		fmt.Println()
	}
}

func (p *updateProgress) logf(format string, args ...any) {
	if p.onChange == nil {
		fmt.Printf(format, args...)
	}
}
