package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"

	"matchup-lookup/internal/core"
)

type championPicker struct {
	title     string
	input     textinput.Model
	all       []core.Champion
	matches   []core.Champion
	cursor    int
	selected  string
	cancelled bool
}

func PickChampion(title string, all []core.Champion, initial string) (string, error) {
	input := textinput.New()
	input.Placeholder = "type to search, e.g. fiora or fiora typo"
	input.Focus()
	input.SetValue(initial)
	input.CharLimit = 40
	m := championPicker{title: title, input: input, all: all}
	m.filter()
	final, err := tea.NewProgram(m, tea.WithAltScreen()).Run()
	if err != nil {
		return "", err
	}
	chosen := final.(championPicker)
	if chosen.cancelled {
		return "", huh.ErrUserAborted
	}
	return chosen.selected, nil
}

func (m championPicker) Init() tea.Cmd { return textinput.Blink }

func (m championPicker) Update(message tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := message.(tea.KeyMsg); ok {
		switch key.Type {
		case tea.KeyCtrlC, tea.KeyEsc:
			m.cancelled = true
			return m, tea.Quit
		case tea.KeyUp:
			if m.cursor > 0 {
				m.cursor--
			}
			return m, nil
		case tea.KeyDown:
			if m.cursor+1 < len(m.matches) {
				m.cursor++
			}
			return m, nil
		case tea.KeyEnter:
			if len(m.matches) > 0 {
				m.selected = m.matches[m.cursor].ID
				return m, tea.Quit
			}
		}
	}
	old := m.input.Value()
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(message)
	if m.input.Value() != old {
		m.cursor = 0
		m.filter()
	}
	return m, cmd
}

func (m championPicker) View() string {
	title := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("63"))
	selected := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("42"))
	muted := lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	var b strings.Builder
	b.WriteString(title.Render(m.title) + "\n\n" + m.input.View() + "\n\n")
	if len(m.matches) == 0 {
		b.WriteString(muted.Render("No champions found") + "\n")
	}
	start := m.cursor - 4
	if start < 0 {
		start = 0
	}
	end := start + 9
	if end > len(m.matches) {
		end = len(m.matches)
	}
	for i := start; i < end; i++ {
		label := m.matches[i].Name
		if m.matches[i].ID != "" && !strings.EqualFold(label, m.matches[i].ID) {
			label += "  (" + m.matches[i].ID + ")"
		}
		if i == m.cursor {
			b.WriteString(selected.Render("› "+label) + "\n")
		} else {
			b.WriteString("  " + label + "\n")
		}
	}
	b.WriteString("\n" + muted.Render("Type to search · ↑/↓ choose · Enter select · Esc back"))
	return b.String()
}

func (m *championPicker) filter() {
	m.matches = m.matches[:0]
	if core.NormalizeChampion(m.input.Value()) == "" {
		m.matches = append(m.matches, core.Champion{Name: "Any champion"})
	}
	m.matches = append(m.matches, core.RankChampions(m.all, m.input.Value())...)
}
