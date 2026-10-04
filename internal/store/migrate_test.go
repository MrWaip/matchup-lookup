package store

import (
	"context"
	"database/sql"
	"path/filepath"
	"strings"
	"testing"
)

// createV2Database builds a database as written by versions before goose:
// the 00001 schema with uncompressed match JSON and PRAGMA user_version=2.
func createV2Database(t *testing.T, path string, raw string) {
	t.Helper()
	schema, err := migrationFiles.ReadFile("migrations/00001_init.sql")
	if err != nil {
		t.Fatal(err)
	}
	up := strings.Split(string(schema), "-- +goose Down")[0]
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for _, stmt := range []string{up,
		`INSERT INTO players (puuid, game_name, tag_line, region) VALUES ('fiora', 'FioraPlayer', 'EUW', 'euw1')`,
		`INSERT INTO matches VALUES ('EUW1_1', 'EUW1', '16.19.700.1', 1, 1800, 420, 1, '` + raw + `')`,
		`INSERT INTO tracked_games (match_id, player_puuid, player_game_name, player_tag_line,
           player_rank_tier, player_rank_division, champion, opponent_puuid, opponent_champion,
           player_position, opponent_position, matchup_status, win, kills, deaths, assists, cs, gold,
           item0, item1, item2, item3, item4, item5, item6)
         VALUES ('EUW1_1', 'fiora', 'FioraPlayer', 'EUW', 'DIAMOND', 'I', 'Fiora', 'darius', 'Darius',
           'TOP', 'TOP', 'confirmed', 1, 8, 2, 5, 210, 0, 0, 0, 0, 0, 0, 0, 0)`,
		`INSERT INTO match_checks VALUES ('EUW1_1', 'fiora')`,
		`INSERT INTO app_settings VALUES ('riot_api_key', 'RGAPI-old-key')`,
		`PRAGMA user_version=2`,
	} {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("%s: %v", stmt[:min(len(stmt), 60)], err)
		}
	}
}

func TestOpenMigratesV2Database(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "matches.db")
	raw := `{"metadata":{"matchId":"EUW1_1"},"info":{"gameVersion":"16.19.700.1","participants":[` +
		`{"puuid":"fiora","riotIdGameName":"FioraPlayer","riotIdTagline":"EUW","championName":"Fiora","teamId":100,"win":true,"kills":8,"deaths":2,"assists":5,"totalMinionsKilled":200,"neutralMinionsKilled":10},` +
		`{"puuid":"darius","riotIdGameName":"DariusPlayer","riotIdTagline":"EUW","championName":"Darius","teamId":200,"kills":5,"deaths":4,"assists":2,"summoner1Id":4,"summoner2Id":14,` +
		`"perks":{"styles":[{"selections":[{"perk":8010}]},{"selections":[{"perk":8473},{"perk":8451}]}]}},` +
		`{"puuid":"other","championName":"LeeSin","teamId":200}]}}`
	createV2Database(t, path, raw)

	db, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctx := context.Background()
	q := New(db)

	backups, _ := filepath.Glob(path + ".pre-migration-*.db")
	if len(backups) != 1 {
		t.Fatalf("backups: %v", backups)
	}
	compressed, err := q.GetMatchRaw(ctx, "EUW1_1")
	if err != nil {
		t.Fatal(err)
	}
	restored, err := DecompressRaw(compressed)
	if err != nil || string(restored) != raw {
		t.Fatalf("raw JSON not preserved: %q %v", restored, err)
	}
	var participants int
	if err := db.QueryRow(`SELECT COUNT(*) FROM match_participants`).Scan(&participants); err != nil || participants != 2 {
		t.Fatalf("participants for player and opponent only: %d %v", participants, err)
	}
	if needs, err := q.MatchNeedsLoadout(ctx, "EUW1_1"); err != nil || !needs {
		t.Fatalf("player without stored loadout must be refreshed: %v %v", needs, err)
	}
	if checked, err := q.IsChecked(ctx, IsCheckedParams{MatchID: "EUW1_1", PlayerPuuid: "fiora"}); err != nil || !checked {
		t.Fatalf("match check lost: %v %v", checked, err)
	}
	rows, err := q.SearchGames(ctx, SearchGamesParams{Champion: "Fiora"})
	if err != nil || len(rows) != 1 {
		t.Fatalf("search after migration: %+v %v", rows, err)
	}
	r := rows[0]
	if !r.Win || r.Kills != 8 || r.Cs != 210 || r.RankTier != "DIAMOND" || r.OpponentChampion.String != "Darius" || r.OpponentKills.Int64 != 5 {
		t.Fatalf("migrated row: %+v", r)
	}
	opponentSide, err := q.SearchGames(ctx, SearchGamesParams{Champion: "Darius"})
	if err != nil || len(opponentSide) != 1 || opponentSide[0].Win || opponentSide[0].Keystone.Int64 != 8010 {
		t.Fatalf("opponent side after migration: %+v %v", opponentSide, err)
	}

	keys, err := q.ListRiotKeys(ctx)
	if err != nil || len(keys) != 1 || keys[0].Value != "RGAPI-old-key" {
		t.Fatalf("single key moved to riot_keys: %+v %v", keys, err)
	}

	// Reopening applies nothing and makes no further backup.
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	db, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if backups, _ := filepath.Glob(path + ".pre-migration-*.db"); len(backups) != 1 {
		t.Fatalf("unexpected backup on reopen: %v", backups)
	}
}

func TestOpenFreshDatabaseMakesNoBackup(t *testing.T) {
	path := filepath.Join(t.TempDir(), "matches.db")
	db, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if backups, _ := filepath.Glob(path + ".pre-migration-*.db"); len(backups) != 0 {
		t.Fatalf("fresh database backed up: %v", backups)
	}
}
