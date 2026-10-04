package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type returnToMenuModel struct{ done bool }

func (returnToMenuModel) Init() tea.Cmd { return nil }

func (m returnToMenuModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.Type {
		case tea.KeyEnter, tea.KeyEsc, tea.KeyCtrlC:
			m.done = true
			return m, tea.Quit
		}
	}
	return m, nil
}

func (returnToMenuModel) View() string {
	return lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("63")).Render("Press Enter to return to menu") + "\n"
}

func waitForReturnToMenu() error {
	// Use the same terminal reader as the rest of the interactive UI.
	_, err := tea.NewProgram(returnToMenuModel{}).Run()
	return err
}
