package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
)

type yesNoModel struct {
	title     string
	yes       bool
	done      bool
	cancelled bool
}

func promptYesNo(title string, initial bool) (bool, error) {
	final, err := tea.NewProgram(yesNoModel{title: title, yes: initial}).Run()
	if err != nil {
		return false, err
	}
	result := final.(yesNoModel)
	if result.cancelled {
		return false, huh.ErrUserAborted
	}
	return result.yes, nil
}

func (yesNoModel) Init() tea.Cmd { return nil }

func (m yesNoModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.Type {
		case tea.KeyEnter:
			m.done = true
			return m, tea.Quit
		case tea.KeyEsc, tea.KeyCtrlC:
			m.cancelled = true
			return m, tea.Quit
		case tea.KeyLeft, tea.KeyRight, tea.KeyTab, tea.KeySpace:
			m.yes = !m.yes
		}
		switch key.String() {
		case "y", "Y":
			m.yes, m.done = true, true
			return m, tea.Quit
		case "n", "N":
			m.yes, m.done = false, true
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m yesNoModel) View() string {
	if m.done || m.cancelled {
		return ""
	}
	selected := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("42"))
	options := "  Yes     No"
	if m.yes {
		options = selected.Render("› Yes") + "     No"
	} else {
		options = "  Yes   " + selected.Render("› No")
	}
	return m.title + "\n" + options + "\n←/→ choose · Enter confirm · y/n answer · Esc back\n"
}
