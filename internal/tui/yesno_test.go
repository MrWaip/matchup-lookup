package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestYesNoEnterSubmitsImmediately(t *testing.T) {
	for _, initial := range []bool{false, true} {
		m := yesNoModel{title: "Confirm?", yes: initial}
		updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
		result := updated.(yesNoModel)
		if !result.done || result.yes != initial || cmd == nil {
			t.Fatalf("Enter did not submit current choice: %+v", result)
		}
	}
	for key, want := range map[rune]bool{'y': true, 'n': false} {
		m := yesNoModel{title: "Confirm?", yes: !want}
		updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{key}})
		result := updated.(yesNoModel)
		if !result.done || result.yes != want || cmd == nil {
			t.Fatalf("%q did not submit immediately: %+v", key, result)
		}
	}
}
