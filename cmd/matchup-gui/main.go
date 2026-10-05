// Command matchup-gui is the desktop version of MatchupFinder.gg.
// Build it with `just build-gui` (wails build); plain `go build` lacks the
// Wails build tags and Windows resources.
package main

import (
	"embed"
	"fmt"
	"io/fs"
	"os"

	"matchup-lookup/internal/core"
	"matchup-lookup/internal/gui"
)

//go:embed all:frontend
var assets embed.FS

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}

func run() error {
	path, err := core.DBPath()
	if err != nil {
		return err
	}
	frontend, err := fs.Sub(assets, "frontend")
	if err != nil {
		return err
	}
	return gui.Run(path, frontend)
}
