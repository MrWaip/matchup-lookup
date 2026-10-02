package main

import (
	"database/sql"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
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
	var players, total int
	if err := s.DB.QueryRow(`SELECT COUNT(*) FROM players`).Scan(&players); err != nil {
		return nil, 0, 0, 0, 0, err
	}
	if err := s.DB.QueryRow(`SELECT COUNT(*) FROM tracked_games`).Scan(&total); err != nil {
		return nil, 0, 0, 0, 0, err
	}
	where := []string{"1=1"}
	args := []any{}
	if f.Region != "" {
		where = append(where, "m.platform=? COLLATE NOCASE")
		args = append(args, strings.ToLower(f.Region))
	}
	if f.Opponent != "" {
		where = append(where, "g.opponent_champion=? COLLATE NOCASE")
		args = append(args, f.Opponent)
	}
	if f.Champion != "" {
		where = append(where, "g.champion=? COLLATE NOCASE")
		args = append(args, f.Champion)
	}
	// Compare exact KDA ratios without floating point rounding. Unknown opponent KDA never passes.
	switch strings.ToLower(f.KDACompare) {
	case "", "any":
	case "ge", "gt":
		op := ">="
		if strings.EqualFold(f.KDACompare, "gt") {
			op = ">"
		}
		where = append(where, "g.opponent_kills IS NOT NULL AND (g.kills+g.assists)*MAX(1,g.opponent_deaths) "+op+" (g.opponent_kills+g.opponent_assists)*MAX(1,g.deaths)")
	default:
		return nil, 0, 0, 0, 0, fmt.Errorf("KDA comparison must be any, ge, or gt")
	}
	switch strings.ToLower(f.Result) {
	case "", "any":
	case "win":
		where = append(where, "g.win=1")
	case "loss":
		where = append(where, "g.win=0")
	default:
		return nil, 0, 0, 0, 0, fmt.Errorf("result must be any, win, or loss")
	}
	if threshold > 0 {
		where = append(where, `CASE UPPER(g.player_rank_tier)
        WHEN 'EMERALD' THEN 1 WHEN 'DIAMOND' THEN 2 WHEN 'MASTER' THEN 3
        WHEN 'GRANDMASTER' THEN 4 WHEN 'CHALLENGER' THEN 5 ELSE 0 END >= ?`)
		args = append(args, threshold)
	}
	if f.Days > 0 {
		where = append(where, "m.game_creation>=?")
		args = append(args, time.Now().AddDate(0, 0, -f.Days).UnixMilli())
	}
	if f.MinMinutes > 0 {
		where = append(where, "m.game_duration>=?")
		args = append(args, f.MinMinutes*60)
	}
	if f.Patch != "" {
		where = append(where, "m.game_version LIKE ?")
		args = append(args, rawPatch(f.Patch)+".%")
	}
	if f.Player != "" {
		where = append(where, "(g.player_game_name=? COLLATE NOCASE OR p.game_name=? COLLATE NOCASE OR g.player_puuid=?)")
		args = append(args, f.Player, f.Player, f.Player)
	}
	// A tracked game is stored from the seed player's point of view. Project the
	// cached lane opponent as a second, searchable point of view. Match-v5 has
	// no opponent rank snapshot, so only a separately tracked opponent has rank.
	const perspectives = `WITH search_games AS (
      SELECT g.match_id, g.player_puuid, g.player_game_name, g.player_tag_line,
        g.player_rank_tier, g.player_rank_division, g.champion,
        g.opponent_puuid, g.opponent_game_name, g.opponent_tag_line,
        g.opponent_champion, g.matchup_status, g.win, g.kills, g.deaths,
        g.assists, g.cs, g.opponent_kills, g.opponent_deaths, g.opponent_assists
      FROM tracked_games g
      UNION ALL
      SELECT g.match_id, g.opponent_puuid,
        COALESCE(NULLIF(g.opponent_game_name,''),json_extract(participant.value,'$.riotIdGameName'),''),
        COALESCE(NULLIF(g.opponent_tag_line,''),json_extract(participant.value,'$.riotIdTagline'),''),
        COALESCE(rp.rank_tier,''), COALESCE(rp.rank_division,''),
        g.opponent_champion, g.player_puuid, g.player_game_name,
        g.player_tag_line, g.champion, g.matchup_status, 1-g.win,
        COALESCE(g.opponent_kills,json_extract(participant.value,'$.kills')),
        COALESCE(g.opponent_deaths,json_extract(participant.value,'$.deaths')),
        COALESCE(g.opponent_assists,json_extract(participant.value,'$.assists')),
        COALESCE(json_extract(participant.value,'$.totalMinionsKilled'),0)
          + COALESCE(json_extract(participant.value,'$.neutralMinionsKilled'),0),
        g.kills, g.deaths, g.assists
      FROM tracked_games g
      JOIN matches cached ON cached.match_id=g.match_id
      JOIN json_each(cached.raw_json,'$.info.participants') participant
        ON json_extract(participant.value,'$.puuid')=g.opponent_puuid
      LEFT JOIN players rp ON rp.puuid=g.opponent_puuid
      WHERE g.opponent_puuid<>'' AND g.opponent_champion<>''
        AND NOT EXISTS (SELECT 1 FROM tracked_games own
          WHERE own.match_id=g.match_id AND own.player_puuid=g.opponent_puuid)
    )`
	joins := ` FROM search_games g JOIN matches m ON m.match_id=g.match_id
      LEFT JOIN players p ON p.puuid=g.player_puuid`
	conditions := ` WHERE ` + strings.Join(where, " AND ")
	var matching, wins int
	if err := s.DB.QueryRow(perspectives+` SELECT COUNT(*), COALESCE(SUM(g.win),0)`+joins+conditions, args...).Scan(&matching, &wins); err != nil {
		return nil, 0, 0, 0, 0, err
	}
	query := `SELECT m.game_creation, m.platform, g.player_game_name, g.player_tag_line, COALESCE(p.game_name,''), COALESCE(p.tag_line,''),
      g.player_rank_tier, g.player_rank_division, g.champion, g.opponent_champion, g.opponent_game_name,
      g.opponent_tag_line, g.matchup_status, g.win, g.kills, g.deaths, g.assists, g.cs,
	  g.opponent_kills,g.opponent_deaths,g.opponent_assists,m.game_version, m.match_id,
      json_extract(detail.value,'$.summoner1Id'), json_extract(detail.value,'$.summoner2Id'),
      json_extract(detail.value,'$.perks.styles[0].selections[0].perk'),
      json_extract(detail.value,'$.perks.styles[1].selections[0].perk'),
      json_extract(detail.value,'$.perks.styles[1].selections[1].perk')` + joins + `
      LEFT JOIN json_each(m.raw_json,'$.info.participants') detail
        ON json_extract(detail.value,'$.puuid')=g.player_puuid` + conditions + ` ORDER BY m.game_creation DESC, m.match_id DESC LIMIT ?`
	rows, err := s.DB.Query(perspectives+query, append(args, f.Limit)...)
	if err != nil {
		return nil, 0, 0, 0, 0, err
	}
	defer rows.Close()
	var out []Result
	for rows.Next() {
		var creation int64
		var platform, fg, ft, pg, pt, tier, division, champion, opp, og, ot, status, version, id string
		var win, kills, deaths, assists, cs int
		var okills, odeaths, oassists sql.NullInt64
		var spell1, spell2, keystone, secondary1, secondary2 sql.NullInt64
		if err := rows.Scan(&creation, &platform, &fg, &ft, &pg, &pt, &tier, &division, &champion, &opp, &og, &ot, &status, &win, &kills, &deaths, &assists, &cs, &okills, &odeaths, &oassists, &version, &id, &spell1, &spell2, &keystone, &secondary1, &secondary2); err != nil {
			return nil, 0, 0, 0, 0, err
		}
		if fg == "" {
			fg, ft = pg, pt
		}
		if opp == "" {
			opp = "?"
		}
		oppID := "?"
		if og != "" {
			oppID = og + "#" + ot
		}
		patch := displayPatch(version)
		rank := strings.TrimSpace(tier + " " + division)
		if rank == "" {
			rank = "unknown"
		}
		oppKDA := "?"
		if okills.Valid && odeaths.Valid && oassists.Valid {
			oppKDA = fmt.Sprintf("%d/%d/%d", okills.Int64, odeaths.Int64, oassists.Int64)
		}
		out = append(out, Result{Date: time.UnixMilli(creation).Local(), Region: platformLabel(platform), PlayerID: fg + "#" + ft, Champion: champion,
			Rank: rank, Opponent: opp, OpponentID: oppID, OpponentKDA: oppKDA,
			Status: status, Win: win == 1, KDA: fmt.Sprintf("%d/%d/%d", kills, deaths, assists),
			CS: cs, Patch: patch, MatchID: id, Spells: loadoutPair(spellName, spell1, spell2),
			Keystone: runeName(keystone), SecondaryRunes: loadoutPair(runeName, secondary1, secondary2)})
	}
	return out, players, total, matching, wins, rows.Err()
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

func PrintResults(results []Result, players, total, matching, wins int) {
	title := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("63"))
	muted := lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	green := lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	red := lipgloss.NewStyle().Foreground(lipgloss.Color("203"))
	fmt.Println(title.Render("◆ MATCHUP LOOKUP"))
	fmt.Printf("Players %d  ·  Stored %d  ·  Matching %d  ·  %s  ·  %s\n\n",
		players, total, matching, green.Render(fmt.Sprintf("Wins %d", wins)), red.Render(fmt.Sprintf("Losses %d", matching-wins)))
	if len(results) == 0 {
		fmt.Println(muted.Render("No games found. Try -days 0 or a broader rank filter."))
		return
	}
	for i, r := range results {
		printResult(i+1, r)
		if i+1 < len(results) {
			fmt.Println()
		}
	}
}

func printResult(number int, r Result) {
	title := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("63"))
	green := lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	red := lipgloss.NewStyle().Foreground(lipgloss.Color("203"))
	resultStyle := red
	status := "LOSS"
	if r.Win {
		resultStyle = green
		status = "WIN"
	}
	fmt.Printf("%d. %s  %s vs %s  ·  %s  ·  %s  ·  patch %s\n",
		number, resultStyle.Bold(true).Render(status), r.Champion, r.Opponent, r.Date.Format("02 Jan 15:04"), r.Region, r.Patch)
	fmt.Printf("   Player Riot ID: %s\n", title.Render(r.PlayerID))
	fmt.Printf("   Rank %s  ·  K/D/A %s  ·  CS %d\n", r.Rank, r.KDA, r.CS)
	fmt.Printf("   Opponent Riot ID: %s  ·  K/D/A %s\n", r.OpponentID, r.OpponentKDA)
	fmt.Printf("   Spells: %s  ·  Keystone: %s  ·  Secondary: %s\n", r.Spells, r.Keystone, r.SecondaryRunes)
	fmt.Printf("   Match ID: %s", r.MatchID)
	if r.Status != "confirmed" {
		fmt.Printf("  ·  position %s", r.Status)
	}
	fmt.Println()
}
