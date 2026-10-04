package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"matchup-lookup/internal/core"
)

type replayBrowserModel struct {
	rows      []core.Result
	cursor    int
	chosen    int
	width     int
	height    int
	cancelled bool
}

func browseReplay(rows []core.Result, cursor int) (int, error) {
	m := replayBrowserModel{rows: rows, cursor: cursor, chosen: -1}
	final, err := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseAllMotion()).Run()
	if err != nil {
		return -1, err
	}
	result := final.(replayBrowserModel)
	if result.cancelled {
		return -1, nil
	}
	return result.chosen, nil
}

func (replayBrowserModel) Init() tea.Cmd { return nil }

func (m replayBrowserModel) Update(message tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := message.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
	case tea.MouseMsg:
		if msg.Button == tea.MouseButtonWheelUp {
			m.cursor = max(0, m.cursor-1)
			return m, nil
		}
		if msg.Button == tea.MouseButtonWheelDown {
			m.cursor = min(len(m.rows)-1, m.cursor+1)
			return m, nil
		}
		w, _ := m.dimensions()
		leftWidth := w
		if m.wide() {
			leftWidth = (w - 2) / 2
		}
		if msg.X >= 0 && msg.X < leftWidth && msg.Y >= 3 && msg.Y < 3+m.pageSize() {
			index := m.cursor/m.pageSize()*m.pageSize() + msg.Y - 3
			if index < len(m.rows) {
				m.cursor = index
				if msg.Button == tea.MouseButtonLeft && msg.Action == tea.MouseActionPress {
					m.chosen = index
					return m, tea.Quit
				}
			}
		}
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyEsc:
			m.cancelled = true
			return m, tea.Quit
		case tea.KeyUp:
			m.cursor = max(0, m.cursor-1)
		case tea.KeyDown:
			m.cursor = min(len(m.rows)-1, m.cursor+1)
		case tea.KeyLeft, tea.KeyPgUp:
			m.cursor = max(0, m.cursor-m.pageSize())
		case tea.KeyRight, tea.KeyPgDown:
			m.cursor = min(len(m.rows)-1, m.cursor+m.pageSize())
		case tea.KeyHome:
			m.cursor = 0
		case tea.KeyEnd:
			m.cursor = len(m.rows) - 1
		case tea.KeyEnter:
			m.chosen = m.cursor
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m replayBrowserModel) dimensions() (int, int) {
	w, h := m.width, m.height
	if w <= 0 {
		w = 80
	}
	if h <= 0 {
		h = 24
	}
	return w, h
}

func (m replayBrowserModel) wide() bool {
	w, _ := m.dimensions()
	return w >= 100
}

func (m replayBrowserModel) detailHeight() int {
	if m.wide() {
		return 0
	}
	_, h := m.dimensions()
	return min(9, max(3, h-7))
}

func (m replayBrowserModel) pageSize() int {
	_, h := m.dimensions()
	return max(1, h-5-m.detailHeight())
}

func (m replayBrowserModel) View() string {
	w, h := m.dimensions()
	if w < 35 || h < 9 {
		return "Enlarge the terminal to browse matches.\nEsc: back"
	}
	selected := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("42"))
	title := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("63"))
	muted := lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	pageSize := m.pageSize()
	start := m.cursor / pageSize * pageSize
	end := min(start+pageSize, len(m.rows))
	leftWidth := w
	if m.wide() {
		leftWidth = (w - 2) / 2
	}
	var lines []string
	lines = append(lines, fmt.Sprintf("MATCHES %d–%d / %d", start+1, end, len(m.rows)))
	for i := start; i < end; i++ {
		r := m.rows[i]
		outcome := "L"
		if r.Win {
			outcome = "W"
		}
		label := fmt.Sprintf("%d %s %s %s vs %s %s", i+1, outcome, r.Champion, r.PlayerID, r.Opponent, r.OpponentID)
		label = ansi.Truncate(label, leftWidth-3, "…")
		if i == m.cursor {
			lines = append(lines, selected.Render("› "+label))
		} else {
			lines = append(lines, "  "+label)
		}
	}
	for len(lines) < pageSize+1 {
		lines = append(lines, "")
	}
	left := strings.Join(lines, "\n")
	var body string
	if m.wide() {
		rightWidth := w - leftWidth - 2
		details := m.detailLines(rightWidth)
		for i := range details {
			details[i] = "  " + details[i]
		}
		body = lipgloss.JoinHorizontal(lipgloss.Top, lipgloss.NewStyle().Width(leftWidth).Render(left), strings.Join(details, "\n"))
	} else {
		details := m.detailLines(w)
		body = left + "\n" + strings.Join(details[:m.detailHeight()], "\n")
	}
	page := m.cursor/pageSize + 1
	pages := (len(m.rows) + pageSize - 1) / pageSize
	return title.Render("◆ MATCHUP LOOKUP") + "\n" + muted.Render(fmt.Sprintf("Page %d/%d · %d matches", page, pages, len(m.rows))) + "\n" +
		body + "\n" + muted.Render("↑/↓ choose · ←/→ page · Enter replay · Esc menu") + "\n"
}

func (m replayBrowserModel) detailLines(width int) []string {
	r := m.rows[m.cursor]
	result := "LOSS"
	if r.Win {
		result = "WIN"
	}
	lines := []string{
		"MATCH DETAILS · " + result,
		"Player: " + r.PlayerID + " (" + r.Champion + ")",
		"Opponent: " + r.OpponentID + " (" + r.Opponent + ")",
		"Date: " + r.Date.Format("02 Jan 2006 15:04") + " · " + r.Region,
		"K/D/A: " + r.KDA + " vs " + r.OpponentKDA + " · CS " + fmt.Sprint(r.CS),
		"Spells: " + r.Spells,
		"Keystone: " + r.Keystone,
		"Secondary: " + r.SecondaryRunes,
		"Match ID: " + r.MatchID,
	}
	for i := range lines {
		lines[i] = ansi.Truncate(lines[i], width, "…")
	}
	return lines
}
