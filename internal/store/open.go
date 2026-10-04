// Package store owns the SQLite schema, its migrations and the queries used by
// the application. Queries live in queries/*.sql and are compiled to Go by sqlc
// (see sqlc.yaml at the repository root); run `just generate` after editing
// them or the migrations.
package store

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/pressly/goose/v3"
	_ "modernc.org/sqlite"
)

//go:embed migrations/*.sql
var migrationFiles embed.FS

// Open opens the database at path, creating it if needed, and applies any
// pending migrations. An existing database is first copied to a timestamped
// backup next to it, so a failed or unwanted upgrade can be undone by hand.
func Open(path string) (*sql.DB, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return nil, err
	}
	_, statErr := os.Stat(path)
	existed := statErr == nil
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	if err := setup(db, path, existed); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}

func setup(db *sql.DB, path string, existed bool) error {
	db.SetMaxOpenConns(1)
	for _, pragma := range []string{"PRAGMA foreign_keys=ON", "PRAGMA busy_timeout=5000"} {
		if _, err := db.Exec(pragma); err != nil {
			return err
		}
	}
	ctx := context.Background()
	provider, err := newMigrator(db)
	if err != nil {
		return err
	}
	pending, err := provider.HasPending(ctx)
	if err != nil {
		return err
	}
	if pending && existed {
		backup := path + ".pre-migration-" + time.Now().Format("20060102-150405") + ".db"
		if err := Export(db, backup); err != nil {
			return fmt.Errorf("back up database before migration: %w", err)
		}
	}
	if _, err := provider.Up(ctx); err != nil {
		return fmt.Errorf("migrate database: %w", err)
	}
	return os.Chmod(path, 0600)
}

func newMigrator(db *sql.DB) (*goose.Provider, error) {
	files, err := fs.Sub(migrationFiles, "migrations")
	if err != nil {
		return nil, err
	}
	return goose.NewProvider(goose.DialectSQLite3, db, files,
		goose.WithGoMigrations(goose.NewGoMigration(3, &goose.GoFunc{RunTx: extractMatchJSON}, nil)))
}

// Export writes a consistent standalone snapshot of db to output.
// VACUUM INTO includes committed changes that may still live in a WAL file.
func Export(db *sql.DB, output string) error {
	if err := os.MkdirAll(filepath.Dir(output), 0700); err != nil {
		return err
	}
	if _, err := os.Stat(output); err == nil {
		return fmt.Errorf("export file already exists: %s", output)
	} else if !os.IsNotExist(err) {
		return err
	}
	if _, err := db.Exec(`VACUUM INTO ?`, output); err != nil {
		return err
	}
	return os.Chmod(output, 0600)
}

// Verify checks that path holds an intact matchup-lookup database of any
// schema version; Open migrates it afterwards.
func Verify(path string) error {
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
