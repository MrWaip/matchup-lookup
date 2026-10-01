package main

import (
	"bytes"
	"database/sql"
	"fmt"
	"strings"
	"text/tabwriter"
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
		args = append(args, f.Patch+".%")
	}
	if f.Player != "" {
		where = append(where, "(g.player_game_name=? COLLATE NOCASE OR p.game_name=? COLLATE NOCASE OR g.player_puuid=?)")
		args = append(args, f.Player, f.Player, f.Player)
	}
	base := ` FROM tracked_games g JOIN matches m ON m.match_id=g.match_id JOIN players p ON p.puuid=g.player_puuid WHERE ` + strings.Join(where, " AND ")
	var matching, wins int
	if err := s.DB.QueryRow(`SELECT COUNT(*), COALESCE(SUM(g.win),0)`+base, args...).Scan(&matching, &wins); err != nil {
		return nil, 0, 0, 0, 0, err
	}
	query := `SELECT m.game_creation, m.platform, g.player_game_name, g.player_tag_line, p.game_name, p.tag_line,
      g.player_rank_tier, g.player_rank_division, g.champion, g.opponent_champion, g.opponent_game_name,
      g.opponent_tag_line, g.matchup_status, g.win, g.kills, g.deaths, g.assists, g.cs,
	  g.opponent_kills,g.opponent_deaths,g.opponent_assists,m.game_version, m.match_id` + base + ` ORDER BY m.game_creation DESC, m.match_id DESC LIMIT ?`
	rows, err := s.DB.Query(query, append(args, f.Limit)...)
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
		if err := rows.Scan(&creation, &platform, &fg, &ft, &pg, &pt, &tier, &division, &champion, &opp, &og, &ot, &status, &win, &kills, &deaths, &assists, &cs, &okills, &odeaths, &oassists, &version, &id); err != nil {
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
		patch := version
		parts := strings.Split(version, ".")
		if len(parts) >= 2 {
			patch = parts[0] + "." + parts[1]
		}
		oppKDA := "?"
		if okills.Valid && odeaths.Valid && oassists.Valid {
			oppKDA = fmt.Sprintf("%d/%d/%d", okills.Int64, odeaths.Int64, oassists.Int64)
		}
		out = append(out, Result{Date: time.UnixMilli(creation).Local(), Region: platformLabel(platform), PlayerID: fg + "#" + ft, Champion: champion,
			Rank: strings.TrimSpace(tier + " " + division), Opponent: opp, OpponentID: oppID, OpponentKDA: oppKDA,
			Status: status, Win: win == 1, KDA: fmt.Sprintf("%d/%d/%d", kills, deaths, assists),
			CS: cs, Patch: patch, MatchID: id})
	}
	return out, players, total, matching, wins, rows.Err()
}

func PrintResults(results []Result, players, total, matching, wins int) {
	title := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("63"))
	muted := lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	green := lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	red := lipgloss.NewStyle().Foreground(lipgloss.Color("203"))
	fmt.Println(title.Render("◆ MATCHUP LOOKUP"))
	fmt.Printf("Players %d  ·  Stored %d  ·  Matching %d  ·  %s  ·  %s\n\n",
		players, total, matching, green.Render(fmt.Sprintf("Wins %d", wins)), red.Render(fmt.Sprintf("Losses %d", matching-wins)))
	var buf bytes.Buffer
	w := tabwriter.NewWriter(&buf, 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "DATE\tSERVER\tPLAYER RIOT ID\tCHAMPION\tRANK\tOPPONENT\tOPPONENT RIOT ID\tRESULT\tK/D/A\tOPP K/D/A\tCS\tPATCH\tMATCH ID\tPOSITION")
	for _, r := range results {
		result := "LOSS"
		if r.Win {
			result = "WIN"
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%d\t%s\t%s\t%s\n",
			r.Date.Format("2006-01-02 15:04"), r.Region, r.PlayerID, r.Champion, r.Rank, r.Opponent, r.OpponentID,
			result, r.KDA, r.OpponentKDA, r.CS, r.Patch, r.MatchID, r.Status)
	}
	w.Flush()
	lines := strings.Split(strings.TrimSuffix(buf.String(), "\n"), "\n")
	for i, line := range lines {
		if i == 0 {
			fmt.Println(muted.Render(line))
			continue
		}
		if results[i-1].Win {
			fmt.Println(green.Render(line))
		} else {
			fmt.Println(red.Render(line))
		}
	}
	if len(results) == 0 {
		fmt.Println(muted.Render("No games found. Try -days 0 or a broader rank filter."))
	}
}
