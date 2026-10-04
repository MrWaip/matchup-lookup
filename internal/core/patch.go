package core

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"time"

	"matchup-lookup/internal/store"
)

// Patch is a game version's major.minor in Riot's API numbering (16.19 for
// the patch players call 26.19). The zero Patch means unknown.
type Patch struct{ Major, Minor int64 }

func (p Patch) IsZero() bool { return p == Patch{} }

func (p Patch) Before(other Patch) bool {
	return p.Major < other.Major || (p.Major == other.Major && p.Minor < other.Minor)
}

// String returns the player-facing name, e.g. "26.19".
func (p Patch) String() string {
	if p.IsZero() {
		return "unknown"
	}
	return displayPatch(fmt.Sprintf("%d.%d", p.Major, p.Minor))
}

const livePatchTTL = time.Hour

// RefreshLivePatch saves the live patch from Data Dragon at most once per
// livePatchTTL. Callers may ignore its error: CurrentPatch then falls back to
// the last saved live patch and the stored matches.
func RefreshLivePatch(ctx context.Context, s *Store) error {
	checked, found, err := s.setting("live_patch_checked_at")
	if err != nil {
		return err
	}
	if sec, parseErr := strconv.ParseInt(checked, 10, 64); found && parseErr == nil && time.Since(time.Unix(sec, 0)) < livePatchTTL {
		return nil
	}
	version, err := fetchLiveVersion(ctx)
	if err != nil {
		return err
	}
	if err := s.setSetting("live_patch", version); err != nil {
		return err
	}
	return s.setSetting("live_patch_checked_at", strconv.FormatInt(time.Now().Unix(), 10))
}

// CurrentPatch is the patch whose replays the client can still open: the
// newer of the saved live patch and the newest stored match.
func (s *Store) CurrentPatch() (Patch, error) {
	stored, err := s.newestStoredPatch()
	if err != nil {
		return Patch{}, err
	}
	version, _, err := s.setting("live_patch")
	if err != nil {
		return Patch{}, err
	}
	major, minor := store.ParsePatch(version)
	if live := (Patch{major, minor}); stored.Before(live) {
		return live, nil
	}
	return stored, nil
}

func (s *Store) newestStoredPatch() (Patch, error) {
	latest, err := s.q.LatestPatch(context.Background())
	if errors.Is(err, sql.ErrNoRows) {
		return Patch{}, nil
	}
	return Patch{latest.PatchMajor, latest.PatchMinor}, err
}

func fetchLiveVersion(ctx context.Context) (string, error) {
	var versions []string
	if err := catalogGET(ctx, "https://ddragon.leagueoflegends.com/api/versions.json", &versions); err != nil {
		return "", err
	}
	if len(versions) == 0 {
		return "", fmt.Errorf("Data Dragon returned no versions")
	}
	return versions[0], nil
}
