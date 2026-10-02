package main

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func TestSearchReturnsDirectlyToMenu(t *testing.T) {
	for _, action := range []string{"find", "repeat"} {
		if requiresReturnPrompt(action, true, nil) {
			t.Fatalf("%s should return directly to menu after replay choice", action)
		}
		if !requiresReturnPrompt(action, false, nil) {
			t.Fatalf("%s with no matches should leave the empty result readable", action)
		}
	}
	if !requiresReturnPrompt("players", false, nil) {
		t.Fatal("player list still needs a pause so it remains readable")
	}
}

func TestReplayBrowserArrowNavigationAndAdaptiveDetails(t *testing.T) {
	rows := make([]Result, 40)
	for i := range rows {
		rows[i] = Result{Champion: "Fiora", PlayerID: "Blue#EUW", Opponent: "Darius", OpponentID: "Red#EUW", SecondaryRunes: "Bone Plating + Overgrowth"}
	}
	rows[1].SecondaryRunes = "Second Wind + Overgrowth"
	m := replayBrowserModel{rows: rows, width: 110, height: 24}
	if m.pageSize() <= 4 {
		t.Fatalf("screen should fit more than four matches: %d", m.pageSize())
	}
	view := m.View()
	if !strings.Contains(view, "Bone Plating") || !strings.Contains(view, "Blue#EUW") || !strings.Contains(view, "Red#EUW") {
		t.Fatalf("selected match details missing: %q", view)
	}
	if lipgloss.Height(view) > m.height {
		t.Fatalf("view exceeds terminal height: %d > %d", lipgloss.Height(view), m.height)
	}
	if lipgloss.Width(view) > m.width {
		t.Fatalf("view exceeds terminal width: %d > %d", lipgloss.Width(view), m.width)
	}
	updated, _ := m.Update(tea.MouseMsg{X: 5, Y: 4, Action: tea.MouseActionMotion, Type: tea.MouseMotion})
	m = updated.(replayBrowserModel)
	if m.cursor != 1 || !strings.Contains(m.View(), "Second Wind") {
		t.Fatalf("hover should update the selected details: cursor=%d", m.cursor)
	}
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})
	m = updated.(replayBrowserModel)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRight})
	m = updated.(replayBrowserModel)
	if m.cursor != m.pageSize() {
		t.Fatalf("right arrow should retain row position on next page: cursor=%d page size=%d", m.cursor, m.pageSize())
	}
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyLeft})
	m = updated.(replayBrowserModel)
	if m.cursor != 0 {
		t.Fatalf("left arrow should return to prior page, got %d", m.cursor)
	}
	updated, _ = m.Update(tea.WindowSizeMsg{Width: 80, Height: 20})
	m = updated.(replayBrowserModel)
	if lipgloss.Height(m.View()) > 20 {
		t.Fatalf("narrow view exceeds terminal height: %d", lipgloss.Height(m.View()))
	}
	if lipgloss.Width(m.View()) > 80 {
		t.Fatalf("narrow view exceeds terminal width: %d", lipgloss.Width(m.View()))
	}
	if !strings.Contains(m.View(), "Bone Plating") {
		t.Fatal("narrow view hides selected match details")
	}
}
