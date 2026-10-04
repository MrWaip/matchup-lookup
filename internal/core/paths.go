package core

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

// DBPath returns MATCHUP_DB_PATH if set, otherwise DefaultDBPath.
func DBPath() (string, error) {
	if custom := os.Getenv("MATCHUP_DB_PATH"); custom != "" {
		return custom, nil
	}
	return DefaultDBPath()
}

func DefaultDBPath() (string, error) {
	var root string
	switch runtime.GOOS {
	case "windows":
		root = os.Getenv("LOCALAPPDATA")
		if root == "" {
			return "", fmt.Errorf("LOCALAPPDATA is not set")
		}
	case "darwin":
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		root = filepath.Join(home, "Library", "Application Support")
	default:
		root = os.Getenv("XDG_DATA_HOME")
		if root == "" {
			home, err := os.UserHomeDir()
			if err != nil {
				return "", err
			}
			root = filepath.Join(home, ".local", "share")
		}
	}
	return filepath.Join(root, "matchup-lookup", "matches.db"), nil
}
