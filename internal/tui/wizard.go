package tui

import (
	"errors"

	"github.com/charmbracelet/huh"
)

// Each field runs separately so Esc can return to the preceding field. At the
// first field, Esc returns to the main menu via RunInteractive.
func runWizardSteps(steps []func() error) error {
	for i := 0; i < len(steps); {
		err := steps[i]()
		if errors.Is(err, huh.ErrUserAborted) {
			if i == 0 {
				return err
			}
			i--
			continue
		}
		if err != nil {
			return err
		}
		i++
	}
	return nil
}
