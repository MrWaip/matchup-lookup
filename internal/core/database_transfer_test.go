package core

import (
	"os"
	"path/filepath"
	"testing"
)

func TestExportImportDatabaseOffline(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "mac.db")
	store, err := OpenStore(source)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.AddRiotKey("personal", "RGAPI-test-key"); err != nil {
		t.Fatal(err)
	}
	if err := store.QueueSeed(Seed{GameName: "Example", TagLine: "EUW", Region: "euw1", Champion: "Fiora"}); err != nil {
		t.Fatal(err)
	}
	snapshot := filepath.Join(dir, "snapshot.db")
	if err := ExportDatabase(store, snapshot); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(dir, "windows", "matches.db")
	old, err := OpenStore(target)
	if err != nil {
		t.Fatal(err)
	}
	if err := old.AddRiotKey("old", "old-key"); err != nil {
		t.Fatal(err)
	}
	old.Close()
	backup, err := ImportDatabase(target, snapshot)
	if err != nil {
		t.Fatal(err)
	}
	if backup == "" {
		t.Fatal("expected backup of existing database")
	}
	if _, err := os.Stat(backup); err != nil {
		t.Fatal(err)
	}
	loaded, err := OpenStore(target)
	if err != nil {
		t.Fatal(err)
	}
	defer loaded.Close()
	keys, err := loaded.RiotKeys()
	if err != nil || len(keys) != 1 || keys[0].Value != "RGAPI-test-key" {
		t.Fatalf("imported keys: %+v %v", keys, err)
	}
	pending, err := loaded.PendingSeeds()
	if err != nil || len(pending) != 1 || pending[0].GameName != "Example" {
		t.Fatalf("imported player seed: %+v %v", pending, err)
	}
	if _, err := ImportDatabase(target, filepath.Join(dir, "missing.db")); err == nil {
		t.Fatal("invalid source accepted")
	}
}
