package core

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"matchup-lookup/internal/store"
)

// Filters narrow a search within the current patch. They are saved as JSON
// with Go field names; fields of older versions (Days, Patch) are ignored.
type Filters struct {
	Champion   string
	Opponent   string
	Result     string
	KDACompare string
	Rank       string
	MinMinutes int
	Player     string
	Region     string
	Limit      int
}

// SearchResult is one page of matches plus counts over all of them.
type SearchResult struct {
	Results  []Result // newest first, at most Filters.Limit
	Patch    Patch    // the current patch searched; zero if nothing is known yet
	Players  int      // tracked players
	Stored   int      // tracked games of the current patch
	Matching int
	Wins     int
}

type Result struct {
	Date                                                                                             time.Time
	PlayerID, Champion, Rank, Opponent, OpponentID, Status, KDA, OpponentKDA, Patch, MatchID, Region string
	Spells, Keystone, SecondaryRunes                                                                 string
	Win                                                                                              bool
	CS                                                                                               int
	// Data Dragon IDs behind Spells, Keystone and SecondaryRunes; 0 if unknown.
	SpellIDs     [2]int64
	KeystoneID   int64
	SecondaryIDs [2]int64
}

func rankThreshold(tier string) (int, error) {
	switch strings.ToLower(strings.TrimSuffix(tier, "+")) {
	case "", "any":
		return 0, nil
	case "emerald":
		return 1, nil
	case "diamond":
		return 2, nil
	case "master":
		return 3, nil
	case "grandmaster":
		return 4, nil
	case "challenger":
		return 5, nil
	default:
		return 0, fmt.Errorf("unknown rank %q", tier)
	}
}

// Search finds matches of the current patch, the only ones whose replays can
// still be opened, from both sides of each tracked game.
func Search(s *Store, f Filters) (SearchResult, error) {
	threshold, err := rankThreshold(f.Rank)
	if err != nil {
		return SearchResult{}, err
	}
	if f.MinMinutes < 0 || f.Limit < 1 {
		return SearchResult{}, fmt.Errorf("minutes must be nonnegative; limit must be positive")
	}
	patch, err := s.CurrentPatch()
	if err != nil {
		return SearchResult{}, err
	}
	params := store.SearchGamesParams{
		Region:      strings.ToLower(f.Region),
		Champion:    f.Champion,
		Opponent:    f.Opponent,
		MinRank:     int64(threshold),
		MinDuration: int64(f.MinMinutes) * 60,
		Player:      f.Player,
		PatchMajor:  patch.Major,
		PatchMinor:  patch.Minor,
	}
	switch kda := strings.ToLower(f.KDACompare); kda {
	case "", "any":
	case "ge", "gt":
		params.Kda = kda
	default:
		return SearchResult{}, fmt.Errorf("KDA comparison must be any, ge, or gt")
	}
	switch result := strings.ToLower(f.Result); result {
	case "", "any":
	case "win", "loss":
		params.Result = result
	default:
		return SearchResult{}, fmt.Errorf("result must be any, win, or loss")
	}
	ctx := context.Background()
	players, err := s.q.CountPlayers(ctx)
	if err != nil {
		return SearchResult{}, err
	}
	stored, err := s.q.CountTrackedGamesInPatch(ctx, store.CountTrackedGamesInPatchParams{
		PatchMajor: patch.Major, PatchMinor: patch.Minor})
	if err != nil {
		return SearchResult{}, err
	}
	rows, err := s.q.SearchGames(ctx, params)
	if err != nil {
		return SearchResult{}, err
	}
	out := SearchResult{Patch: patch, Players: int(players), Stored: int(stored), Matching: len(rows)}
	for _, r := range rows {
		if r.Win {
			out.Wins++
		}
	}
	for _, r := range rows[:min(len(rows), f.Limit)] {
		out.Results = append(out.Results, resultFromRow(r))
	}
	return out, nil
}

func resultFromRow(r store.SearchGamesRow) Result {
	gameName, tagLine := r.RiotIDGameName, r.RiotIDTagline
	if gameName == "" {
		gameName, tagLine = r.PlayerGameName, r.PlayerTagLine
	}
	opponent := "?"
	if r.OpponentChampion.Valid && r.OpponentChampion.String != "" {
		opponent = r.OpponentChampion.String
	}
	opponentID := "?"
	if r.OpponentGameName.Valid && r.OpponentGameName.String != "" {
		opponentID = r.OpponentGameName.String + "#" + r.OpponentTagLine.String
	}
	opponentKDA := "?"
	if r.OpponentKills.Valid && r.OpponentDeaths.Valid && r.OpponentAssists.Valid {
		opponentKDA = fmt.Sprintf("%d/%d/%d", r.OpponentKills.Int64, r.OpponentDeaths.Int64, r.OpponentAssists.Int64)
	}
	rank := strings.TrimSpace(r.RankTier + " " + r.RankDivision)
	if rank == "" {
		rank = "unknown"
	}
	return Result{
		Date: time.UnixMilli(r.GameCreation).Local(), Region: PlatformLabel(r.Platform),
		PlayerID: gameName + "#" + tagLine, Champion: r.Champion, Rank: rank,
		Opponent: opponent, OpponentID: opponentID, OpponentKDA: opponentKDA,
		Status: r.MatchupStatus, Win: r.Win, KDA: fmt.Sprintf("%d/%d/%d", r.Kills, r.Deaths, r.Assists),
		CS: int(r.Cs), Patch: displayPatch(r.GameVersion), MatchID: r.MatchID,
		Spells:         loadoutPair(spellName, r.Summoner1ID, r.Summoner2ID),
		Keystone:       runeName(r.Keystone),
		SecondaryRunes: loadoutPair(runeName, r.SecondaryRune1, r.SecondaryRune2),
		SpellIDs:       [2]int64{r.Summoner1ID.Int64, r.Summoner2ID.Int64},
		KeystoneID:     r.Keystone.Int64,
		SecondaryIDs:   [2]int64{r.SecondaryRune1.Int64, r.SecondaryRune2.Int64},
	}
}

// Riot's 2026 API version is 16.x while its player-facing patch name is 26.x.
// SQLite keeps the raw version; only display uses the player-facing name.
func displayPatch(version string) string {
	parts := strings.Split(version, ".")
	if len(parts) < 2 {
		return version
	}
	major, err := strconv.Atoi(parts[0])
	if err == nil && major >= 15 && major <= 29 {
		return fmt.Sprintf("%d.%s", major+10, parts[1])
	}
	return parts[0] + "." + parts[1]
}
