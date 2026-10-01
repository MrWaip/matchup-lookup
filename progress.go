package main

import (
	"fmt"
	"math"
	"os"
	"strings"
)

type updateProgress struct {
	resolved, resolveTotal                                int
	players, playerTotal                                  int
	matches, matchTotal                                   int
	current, currentTotal                                 int
	lastPlainPlayers, lastPlainResolves, lastPlainMatches int
}

func newUpdateProgress(resolveTotal, playerTotal int) *updateProgress {
	return &updateProgress{resolveTotal: resolveTotal, playerTotal: playerTotal,
		lastPlainPlayers: -1, lastPlainResolves: -1, lastPlainMatches: -1}
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
	progress := resolveWeight*resolved + (1-resolveWeight)*collected
	progress = math.Max(0, math.Min(1, progress))
	percent := int(math.Round(progress * 100))
	const width = 20
	filled := int(math.Round(progress * width))
	bar := strings.Repeat("█", filled) + strings.Repeat("·", width-filled)
	line := fmt.Sprintf("[UPDATE] [%s] %3d%%  IDs %d/%d  players %d/%d  matches %d/%d",
		bar, percent, p.resolved, p.resolveTotal, p.players, p.playerTotal, p.matches, p.matchTotal)
	info, err := os.Stdout.Stat()
	if err != nil || info.Mode()&os.ModeCharDevice == 0 {
		if p.players != p.lastPlainPlayers || p.resolved/10 != p.lastPlainResolves/10 || p.matches/20 != p.lastPlainMatches/20 {
			fmt.Println(line)
			p.lastPlainPlayers, p.lastPlainResolves, p.lastPlainMatches = p.players, p.resolved, p.matches
		}
		return
	}
	fmt.Printf("\r\x1b[2K%s", line)
}

func (p *updateProgress) close() {
	p.render()
	fmt.Println()
}
