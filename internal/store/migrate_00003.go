package store

import (
	"context"
	"database/sql"
)

// extractMatchJSON is migration 00003: it fills match_participants for tracked
// players and their opponents, and the patch columns, from the JSON stored
// before 00002, and compresses that JSON
// into raw_gz. Migration 00004 then drops the uncompressed column.
// Loadout columns stay NULL when the old JSON lacks them, which makes the
// next update refresh those matches from Riot.
func extractMatchJSON(ctx context.Context, tx *sql.Tx) error {
	_, err := tx.ExecContext(ctx, `INSERT OR IGNORE INTO match_participants
      (match_id, puuid, riot_id_game_name, riot_id_tagline, champion, team_id,
       team_position, individual_position, lane, win, kills, deaths, assists, cs, gold,
       item0, item1, item2, item3, item4, item5, item6,
       summoner1_id, summoner2_id, keystone, secondary_rune1, secondary_rune2)
      SELECT m.match_id,
        json_extract(p.value, '$.puuid'),
        COALESCE(json_extract(p.value, '$.riotIdGameName'), ''),
        COALESCE(json_extract(p.value, '$.riotIdTagline'), ''),
        COALESCE(json_extract(p.value, '$.championName'), ''),
        COALESCE(json_extract(p.value, '$.teamId'), 0),
        COALESCE(json_extract(p.value, '$.teamPosition'), ''),
        COALESCE(json_extract(p.value, '$.individualPosition'), ''),
        COALESCE(json_extract(p.value, '$.lane'), ''),
        COALESCE(json_extract(p.value, '$.win'), 0),
        COALESCE(json_extract(p.value, '$.kills'), 0),
        COALESCE(json_extract(p.value, '$.deaths'), 0),
        COALESCE(json_extract(p.value, '$.assists'), 0),
        COALESCE(json_extract(p.value, '$.totalMinionsKilled'), 0)
          + COALESCE(json_extract(p.value, '$.neutralMinionsKilled'), 0),
        COALESCE(json_extract(p.value, '$.goldEarned'), 0),
        COALESCE(json_extract(p.value, '$.item0'), 0),
        COALESCE(json_extract(p.value, '$.item1'), 0),
        COALESCE(json_extract(p.value, '$.item2'), 0),
        COALESCE(json_extract(p.value, '$.item3'), 0),
        COALESCE(json_extract(p.value, '$.item4'), 0),
        COALESCE(json_extract(p.value, '$.item5'), 0),
        COALESCE(json_extract(p.value, '$.item6'), 0),
        json_extract(p.value, '$.summoner1Id'),
        json_extract(p.value, '$.summoner2Id'),
        json_extract(p.value, '$.perks.styles[0].selections[0].perk'),
        json_extract(p.value, '$.perks.styles[1].selections[0].perk'),
        json_extract(p.value, '$.perks.styles[1].selections[1].perk')
      FROM matches m, json_each(m.raw_json, '$.info.participants') p
      WHERE EXISTS (SELECT 1 FROM tracked_games g WHERE g.match_id = m.match_id
        AND json_extract(p.value, '$.puuid') IN (g.player_puuid, g.opponent_puuid))`)
	if err != nil {
		return err
	}
	rows, err := tx.QueryContext(ctx, `SELECT match_id, game_version, raw_json FROM matches`)
	if err != nil {
		return err
	}
	type pending struct {
		id, version string
		raw         []byte
	}
	var matches []pending
	for rows.Next() {
		var m pending
		if err := rows.Scan(&m.id, &m.version, &m.raw); err != nil {
			rows.Close()
			return err
		}
		matches = append(matches, m)
	}
	if err := rows.Close(); err != nil {
		return err
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for _, m := range matches {
		compressed, err := CompressRaw(m.raw)
		if err != nil {
			return err
		}
		major, minor := ParsePatch(m.version)
		if _, err := tx.ExecContext(ctx, `UPDATE matches SET raw_gz = ?, patch_major = ?, patch_minor = ? WHERE match_id = ?`,
			compressed, major, minor, m.id); err != nil {
			return err
		}
	}
	return nil
}
