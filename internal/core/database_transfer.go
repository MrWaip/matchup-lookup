package core

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"matchup-lookup/internal/store"
)

// ExportDatabase writes a standalone snapshot of the open database.
func ExportDatabase(s *Store, output string) error {
	if output == "" {
		return fmt.Errorf("output path is required")
	}
	return store.Export(s.db, output)
}

// ImportDatabase replaces the local database after checking the portable
// snapshot. Existing data is kept in a timestamped backup beside the target.
// Call only while the application's Store is closed.
func ImportDatabase(target, source string) (string, error) {
	if source == "" {
		return "", fmt.Errorf("source path is required")
	}
	targetAbs, err := filepath.Abs(target)
	if err != nil {
		return "", err
	}
	sourceAbs, err := filepath.Abs(source)
	if err != nil {
		return "", err
	}
	if targetAbs == sourceAbs {
		return "", fmt.Errorf("source and destination are the same file")
	}
	if targetInfo, targetErr := os.Stat(targetAbs); targetErr == nil {
		if sourceInfo, sourceErr := os.Stat(sourceAbs); sourceErr == nil && os.SameFile(targetInfo, sourceInfo) {
			return "", fmt.Errorf("source and destination are the same file")
		}
	}
	for _, path := range []string{sourceAbs + "-wal", sourceAbs + "-shm", targetAbs + "-wal", targetAbs + "-shm"} {
		if _, err := os.Stat(path); err == nil {
			return "", fmt.Errorf("SQLite sidecar %s is present; close the app and use db-export for a standalone snapshot", path)
		} else if !os.IsNotExist(err) {
			return "", err
		}
	}
	if err := store.Verify(sourceAbs); err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(targetAbs), 0700); err != nil {
		return "", err
	}
	tmp, err := os.CreateTemp(filepath.Dir(targetAbs), ".matchup-import-*.db")
	if err != nil {
		return "", err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	in, err := os.Open(sourceAbs)
	if err != nil {
		tmp.Close()
		return "", err
	}
	_, copyErr := io.Copy(tmp, in)
	closeInErr := in.Close()
	syncErr := tmp.Sync()
	closeTmpErr := tmp.Close()
	for _, err := range []error{copyErr, closeInErr, syncErr, closeTmpErr} {
		if err != nil {
			return "", err
		}
	}
	if err := os.Chmod(tmpPath, 0600); err != nil {
		return "", err
	}
	if err := store.Verify(tmpPath); err != nil {
		return "", err
	}
	backup := ""
	if _, err := os.Stat(targetAbs); err == nil {
		backup = targetAbs + ".backup-" + time.Now().Format("20060102-150405.000000000") + ".db"
		if err := os.Rename(targetAbs, backup); err != nil {
			return "", fmt.Errorf("close the application before importing: %w", err)
		}
	} else if !os.IsNotExist(err) {
		return "", err
	}
	if err := os.Rename(tmpPath, targetAbs); err != nil {
		if backup != "" {
			_ = os.Rename(backup, targetAbs)
		}
		return "", err
	}
	return backup, nil
}
