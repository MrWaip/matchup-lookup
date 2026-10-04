package core

import (
	"runtime/debug"
	"time"
)

// Set with -ldflags "-X" by builds that disable VCS stamping (wails build
// passes -buildvcs=false); see build-gui in the justfile.
var buildCommit, buildDate string

// Version describes the build: "2026-10-04 dbd759c", with "+dirty" for
// uncommitted changes when the Go toolchain stamped git data, or "dev" for
// go run and builds without git.
func Version() string {
	if buildCommit != "" {
		return buildDate + " " + buildCommit
	}
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "dev"
	}
	var revision, committed string
	var modified bool
	for _, setting := range info.Settings {
		switch setting.Key {
		case "vcs.revision":
			revision = setting.Value
		case "vcs.time":
			committed = setting.Value
		case "vcs.modified":
			modified = setting.Value == "true"
		}
	}
	if revision == "" {
		return "dev"
	}
	version := revision[:min(7, len(revision))]
	if at, err := time.Parse(time.RFC3339, committed); err == nil {
		version = at.Format("2006-01-02") + " " + version
	}
	if modified {
		version += "+dirty"
	}
	return version
}
