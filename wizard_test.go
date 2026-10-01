package main

import (
	"errors"
	"testing"

	"github.com/charmbracelet/huh"
)

func TestWizardEscapeReturnsToPreviousStep(t *testing.T) {
	var visited []string
	secondAttempt := 0
	err := runWizardSteps([]func() error{
		func() error { visited = append(visited, "server"); return nil },
		func() error { visited = append(visited, "champion"); return nil },
		func() error {
			visited = append(visited, "opponent")
			secondAttempt++
			if secondAttempt == 1 {
				return huh.ErrUserAborted
			}
			return nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"server", "champion", "opponent", "champion", "opponent"}
	if len(visited) != len(want) {
		t.Fatalf("visited %v, want %v", visited, want)
	}
	for i := range want {
		if visited[i] != want[i] {
			t.Fatalf("visited %v, want %v", visited, want)
		}
	}
}

func TestWizardEscapeAtFirstStepReturnsToMenu(t *testing.T) {
	err := runWizardSteps([]func() error{func() error { return huh.ErrUserAborted }})
	if !errors.Is(err, huh.ErrUserAborted) {
		t.Fatalf("got %v, want cancellation", err)
	}
}
