package core

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"matchup-lookup/internal/store"
)

type Filters struct {
	Champion   string
	Opponent   string
	Result     string
	KDACompare string
	Rank       string
	Days       int
	MinMinutes int
	Patch      string
	Player     string
	Region     string
	Limit      int
}

type Result struct {
	Date                                                                                             time.Time
	PlayerID, Champion, Rank, Opponent, OpponentID, Status, KDA, OpponentKDA, Patch, MatchID, Region string
	Spells, Keystone, SecondaryRunes                                                                 string
	Win                                                                                              bool
	CS                                                                                               int
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

func Search(s *Store, f Filters) ([]Result, int, int, int, int, error) {
	threshold, err := rankThreshold(f.Rank)
	if err != nil {
		return nil, 0, 0, 0, 0, err
	}
	if f.Days < 0 || f.MinMinutes < 0 || f.Limit < 1 {
		return nil, 0, 0, 0, 0, fmt.Errorf("days/minutes must be nonnegative; limit must be positive")
	}
	params := store.SearchGamesParams{
		Region:      strings.ToLower(f.Region),
		Champion:    f.Champion,
		Opponent:    f.Opponent,
		MinRank:     int64(threshold),
		MinDuration: int64(f.MinMinutes) * 60,
		Player:      f.Player,
	}
	switch kda := strings.ToLower(f.KDACompare); kda {
	case "", "any":
	case "ge", "gt":
		params.Kda = kda
	default:
		return nil, 0, 0, 0, 0, fmt.Errorf("KDA comparison must be any, ge, or gt")
	}
	switch result := strings.ToLower(f.Result); result {
	case "", "any":
	case "win", "loss":
		params.Result = result
	default:
		return nil, 0, 0, 0, 0, fmt.Errorf("result must be any, win, or loss")
	}
	if f.Days > 0 {
		params.CreatedSince = time.Now().AddDate(0, 0, -f.Days).UnixMilli()
	}
	if f.Patch != "" {
		params.PatchMajor, params.PatchMinor = store.ParsePatch(rawPatch(f.Patch))
		if params.PatchMajor == 0 {
			return nil, 0, 0, 0, 0, fmt.Errorf("patch must look like 26.19")
		}
	}
	ctx := context.Background()
	players, err := s.q.CountPlayers(ctx)
	if err != nil {
		return nil, 0, 0, 0, 0, err
	}
	total, err := s.q.CountTrackedGames(ctx)
	if err != nil {
		return nil, 0, 0, 0, 0, err
	}
	rows, err := s.q.SearchGames(ctx, params)
	if err != nil {
		return nil, 0, 0, 0, 0, err
	}
	wins := 0
	for _, r := range rows {
		if r.Win {
			wins++
		}
	}
	matching := len(rows)
	if len(rows) > f.Limit {
		rows = rows[:f.Limit]
	}
	out := make([]Result, 0, len(rows))
	for _, r := range rows {
		out = append(out, resultFromRow(r))
	}
	return out, int(players), int(total), matching, wins, nil
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
	}
}

// Riot's 2026 API version is 16.x while its player-facing patch name is 26.x.
// Keep the raw version in SQLite and accept either name in the patch filter.
func rawPatch(patch string) string {
	parts := strings.Split(patch, ".")
	if len(parts) < 2 {
		return patch
	}
	major, err := strconv.Atoi(parts[0])
	if err != nil || major < 25 || major > 39 {
		return patch
	}
	parts[0] = strconv.Itoa(major - 10)
	return strings.Join(parts, ".")
}

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
