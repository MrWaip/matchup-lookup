package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"matchup-lookup/internal/core"
)

type menuOption struct{ label, action string }

var menuOptions = []menuOption{
	{"Find matchups", "find"},
	{"Repeat last search", "repeat"},
	{"Update recent matches", "update"},
	{"Cancel update", "cancel_update"},
	{"Import players from GitHub / URL / file", "import"},
	{"Export database for another computer", "db_export"},
	{"Import database from another computer", "db_import"},
	{"Show tracked players", "players"},
	{"Set / replace Riot API key", "key"},
	{"Show database location", "path"},
	{"Exit", "exit"},
}

type menuTick time.Time

func nextMenuTick() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg { return menuTick(t) })
}

type menuModel struct {
	background *core.BackgroundUpdate
	spinner    spinner.Model
	cursor     int
	action     string
	width      int
}

func pickMenuAction(background *core.BackgroundUpdate) (string, error) {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("63"))
	m := menuModel{background: background, spinner: s}
	result, err := tea.NewProgram(m, tea.WithAltScreen()).Run()
	if err != nil {
		return "", err
	}
	return result.(menuModel).action, nil
}

func (m menuModel) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, nextMenuTick())
}

func (m menuModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
	case menuTick:
		return m, nextMenuTick()
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q", "esc":
			m.action = "exit"
			return m, tea.Quit
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(menuOptions)-1 {
				m.cursor++
			}
		case "enter":
			m.action = menuOptions[m.cursor].action
			return m, tea.Quit
		}
	}
	var cmd tea.Cmd
	m.spinner, cmd = m.spinner.Update(msg)
	return m, cmd
}

func (m menuModel) View() string {
	title := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("63"))
	selected := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("42"))
	muted := lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	var left strings.Builder
	left.WriteString(title.Render("◆ MATCHUP LOOKUP") + "  " + muted.Render(core.Version()) + "\n\n")
	left.WriteString("What would you like to do?\n\n")
	for i, option := range menuOptions {
		if i == m.cursor {
			left.WriteString(selected.Render("› "+option.label) + "\n")
		} else {
			left.WriteString("  " + option.label + "\n")
		}
	}
	left.WriteString("\n" + muted.Render("↑/↓ choose · Enter select · q exit"))
	state := m.background.Snapshot()
	var right strings.Builder
	right.WriteString(title.Render("UPDATE STATUS") + "\n\n")
	switch {
	case state.Running:
		right.WriteString(m.spinner.View() + " Updating in background\n\n")
		p := state.Progress
		right.WriteString(fmt.Sprintf("%3d%%  players %d/%d\n", p.Percent, p.Players, p.PlayerTotal))
		right.WriteString(fmt.Sprintf("IDs %d/%d · matches %d/%d\n", p.Resolved, p.ResolveTotal, p.Matches, p.MatchTotal))
		if p.LoadoutTotal > 0 {
			right.WriteString(fmt.Sprintf("Old loadouts %d/%d\n", p.Loadouts, p.LoadoutTotal))
		}
		if remaining := time.Until(state.WaitUntil); remaining > 0 {
			right.WriteString("\nRiot limit: " + FormatWait(remaining) + "\n")
			right.WriteString(muted.Render(state.WaitReason))
		}
	case state.Cancelled:
		right.WriteString("Update cancelled\n")
	case !state.Finished.IsZero() && state.Err != nil:
		right.WriteString("Finished with errors\n\n" + state.Err.Error())
	case !state.Finished.IsZero():
		right.WriteString("✓ Update complete\n")
	default:
		right.WriteString(muted.Render("No update running"))
	}
	if m.width >= 90 {
		panel := lipgloss.NewStyle().Width(40).PaddingLeft(3).Render(right.String())
		return lipgloss.JoinHorizontal(lipgloss.Top, left.String(), panel) + "\n"
	}
	return left.String() + "\n\n" + right.String() + "\n"
}
