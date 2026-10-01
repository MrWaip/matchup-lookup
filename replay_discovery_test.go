package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLockfileFollowsRunningLeagueClient(t *testing.T) {
	install := filepath.Join(t.TempDir(), "Games", "League of Legends")
	if err := os.MkdirAll(install, 0700); err != nil {
		t.Fatal(err)
	}
	lockfile := filepath.Join(install, "lockfile")
	if err := os.WriteFile(lockfile, []byte("LeagueClient:42:12345:temporary-password:https"), 0600); err != nil {
		t.Fatal(err)
	}
	executable := filepath.Join(install, "LeagueClient.exe")
	path, err := resolveLeagueLockfile("", []string{executable}, filepath.Join(t.TempDir(), "missing", "lockfile"))
	if err != nil || path != lockfile {
		t.Fatalf("got %q, %v; want %q", path, err, lockfile)
	}
	override := filepath.Join(t.TempDir(), "custom-lockfile")
	path, err = resolveLeagueLockfile(override, []string{executable}, "")
	if err != nil || path != override {
		t.Fatalf("override got %q, %v; want %q", path, err, override)
	}
}
