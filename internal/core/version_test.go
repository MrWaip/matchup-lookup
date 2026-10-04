package core

import "testing"

func TestVersionFromLinkerFlags(t *testing.T) {
	buildCommit, buildDate = "dbd759c", "2026-10-04"
	t.Cleanup(func() { buildCommit, buildDate = "", "" })
	if got := Version(); got != "2026-10-04 dbd759c" {
		t.Fatalf("version %q", got)
	}
}
