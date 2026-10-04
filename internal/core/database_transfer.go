package core

import (
	"database/sql"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

// VACUUM INTO creates a consistent standalone SQLite snapshot, including any
// committed changes that may currently live in a WAL file.
func ExportDatabase(store *Store, output string) error {
	if output == "" {
		return fmt.Errorf("output path is required")
	}
	if err := os.MkdirAll(filepath.Dir(output), 0700); err != nil {
		return err
	}
	if _, err := os.Stat(output); err == nil {
		return fmt.Errorf("export file already exists: %s", output)
	} else if !os.IsNotExist(err) {
		return err
	}
	if _, err := store.DB.Exec(`VACUUM INTO ?`, output); err != nil {
		return err
	}
	return os.Chmod(output, 0600)
}

func verifyDatabase(path string) error {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return err
	}
	defer db.Close()
	var check string
	if err := db.QueryRow(`PRAGMA quick_check`).Scan(&check); err != nil {
		return err
	}
	if check != "ok" {
		return fmt.Errorf("SQLite integrity check failed: %s", check)
	}
	for _, table := range []string{"players", "matches", "tracked_games"} {
		var found int
		if err := db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?`, table).Scan(&found); err != nil {
			return err
		}
		if found == 0 {
			return fmt.Errorf("not a matchup-lookup database: missing %s", table)
		}
	}
	return nil
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
	if err := verifyDatabase(sourceAbs); err != nil {
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
	if err := verifyDatabase(tmpPath); err != nil {
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
