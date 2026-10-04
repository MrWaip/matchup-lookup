package tui

import (
	"testing"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"matchup-lookup/internal/core"
)

func TestReturnPromptAcceptsOneEnter(t *testing.T) {
	m := returnToMenuModel{}
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if !updated.(returnToMenuModel).done || cmd == nil {
		t.Fatal("one Enter should finish the return-to-menu prompt")
	}
}

func TestMenuAndChampionAcceptOneEnter(t *testing.T) {
	menu, menuCmd := (menuModel{}).Update(tea.KeyMsg{Type: tea.KeyEnter})
	if menu.(menuModel).action != "find" || menuCmd == nil {
		t.Fatal("menu should activate selected action on one Enter")
	}
	input := textinput.New()
	input.SetValue("fiora")
	champion := championPicker{input: input, all: []core.Champion{{ID: "Fiora", Name: "Fiora"}}}
	champion.filter()
	chosen, pickerCmd := champion.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if chosen.(championPicker).selected != "Fiora" || pickerCmd == nil {
		t.Fatal("champion picker should select current option on one Enter")
	}
}
