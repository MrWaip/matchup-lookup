package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

type Store struct{ DB *sql.DB }

func OpenStore(path string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	for _, q := range []string{
		"PRAGMA foreign_keys=ON",
		"PRAGMA busy_timeout=5000",
		`CREATE TABLE IF NOT EXISTS players (
            puuid TEXT PRIMARY KEY, game_name TEXT NOT NULL, tag_line TEXT NOT NULL,
            region TEXT NOT NULL, source TEXT NOT NULL DEFAULT 'seed',
            rank_tier TEXT NOT NULL DEFAULT '', rank_division TEXT NOT NULL DEFAULT '',
            league_points INTEGER NOT NULL DEFAULT 0, last_checked_at INTEGER NOT NULL DEFAULT 0)`,
		`CREATE TABLE IF NOT EXISTS matches (
            match_id TEXT PRIMARY KEY, platform TEXT NOT NULL, game_version TEXT NOT NULL,
            game_creation INTEGER NOT NULL, game_duration INTEGER NOT NULL,
            queue_id INTEGER NOT NULL, fetched_at INTEGER NOT NULL, raw_json TEXT NOT NULL)`,
		`CREATE TABLE IF NOT EXISTS tracked_games (
            match_id TEXT NOT NULL REFERENCES matches(match_id),
            player_puuid TEXT NOT NULL REFERENCES players(puuid),
            player_game_name TEXT NOT NULL, player_tag_line TEXT NOT NULL,
            player_rank_tier TEXT NOT NULL, player_rank_division TEXT NOT NULL,
            champion TEXT NOT NULL,
            opponent_puuid TEXT NOT NULL DEFAULT '', opponent_game_name TEXT NOT NULL DEFAULT '',
            opponent_tag_line TEXT NOT NULL DEFAULT '', opponent_champion TEXT NOT NULL DEFAULT '',
            player_position TEXT NOT NULL DEFAULT '', opponent_position TEXT NOT NULL DEFAULT '',
            matchup_status TEXT NOT NULL, win INTEGER NOT NULL, kills INTEGER NOT NULL,
            deaths INTEGER NOT NULL, assists INTEGER NOT NULL, cs INTEGER NOT NULL,
            opponent_kills INTEGER, opponent_deaths INTEGER, opponent_assists INTEGER,
            gold INTEGER NOT NULL, item0 INTEGER NOT NULL, item1 INTEGER NOT NULL,
            item2 INTEGER NOT NULL, item3 INTEGER NOT NULL, item4 INTEGER NOT NULL,
            item5 INTEGER NOT NULL, item6 INTEGER NOT NULL,
            PRIMARY KEY (match_id, player_puuid))`,
		`CREATE TABLE IF NOT EXISTS match_checks (
            match_id TEXT NOT NULL REFERENCES matches(match_id),
            player_puuid TEXT NOT NULL REFERENCES players(puuid),
            PRIMARY KEY (match_id, player_puuid))`,
		`CREATE TABLE IF NOT EXISTS app_settings (
            key TEXT PRIMARY KEY, value TEXT NOT NULL)`,
		`CREATE TABLE IF NOT EXISTS champions (
            id TEXT PRIMARY KEY, display_name TEXT NOT NULL)`,
		`CREATE INDEX IF NOT EXISTS idx_matches_creation ON matches(game_creation DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_games_champions ON tracked_games(champion, opponent_champion)`,
	} {
		if _, err := db.Exec(q); err != nil {
			db.Close()
			return nil, err
		}
	}
	// Keep and enrich matches from databases created by the initial Fiora-only prototype.
	var legacy int
	if err := db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='fiora_games'`).Scan(&legacy); err != nil {
		db.Close()
		return nil, err
	}
	var version int
	if err := db.QueryRow(`PRAGMA user_version`).Scan(&version); err != nil {
		db.Close()
		return nil, err
	}
	if legacy != 0 && version < 2 {
		_, err := db.Exec(`INSERT OR IGNORE INTO tracked_games
          (match_id,player_puuid,player_game_name,player_tag_line,player_rank_tier,player_rank_division,champion,
           opponent_puuid,opponent_game_name,opponent_tag_line,opponent_champion,player_position,opponent_position,
           matchup_status,win,kills,deaths,assists,cs,gold,item0,item1,item2,item3,item4,item5,item6)
          SELECT match_id,fiora_puuid,fiora_game_name,fiora_tag_line,fiora_rank_tier,fiora_rank_division,'Fiora',
           opponent_puuid,opponent_game_name,opponent_tag_line,opponent_champion,fiora_position,opponent_position,
           matchup_status,win,kills,deaths,assists,cs,gold,item0,item1,item2,item3,item4,item5,item6 FROM fiora_games`)
		if err != nil {
			db.Close()
			return nil, err
		}
		if err := backfillLegacy(&Store{DB: db}); err != nil {
			db.Close()
			return nil, err
		}
	}
	if _, err := db.Exec(`PRAGMA user_version=2`); err != nil {
		db.Close()
		return nil, err
	}
	if err := os.Chmod(path, 0600); err != nil {
		db.Close()
		return nil, err
	}
	return &Store{DB: db}, nil
}

func (s *Store) RiotKey() (string, error) {
	var key string
	err := s.DB.QueryRow(`SELECT value FROM app_settings WHERE key='riot_api_key'`).Scan(&key)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return key, err
}

func (s *Store) SetRiotKey(key string) error {
	if key == "" {
		return fmt.Errorf("Riot API key cannot be empty")
	}
	_, err := s.DB.Exec(`INSERT INTO app_settings(key,value) VALUES('riot_api_key',?)
      ON CONFLICT(key) DO UPDATE SET value=excluded.value`, key)
	return err
}

func backfillLegacy(s *Store) error {
	players, err := s.ListPlayers()
	if err != nil {
		return err
	}
	byPUUID := make(map[string]Player, len(players))
	for _, p := range players {
		byPUUID[p.PUUID] = p
	}
	rows, err := s.DB.Query(`SELECT match_id FROM matches`)
	if err != nil {
		return err
	}
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()
	for _, id := range ids {
		m, _, err := s.CachedMatch(id)
		if err != nil {
			return err
		}
		for _, participant := range m.Info.Participants {
			p, tracked := byPUUID[participant.PUUID]
			if !tracked || m.Info.QueueID != 420 {
				continue
			}
			o, status := opponent(m, participant)
			if err := s.SaveTrackedGame(m, p, participant, o, status); err != nil {
				return err
			}
			if err := s.MarkChecked(id, p.PUUID); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *Store) Close() error { return s.DB.Close() }

func (s *Store) PlayerBySeed(seed Seed) (Player, bool, error) {
	var p Player
	err := s.DB.QueryRow(`SELECT puuid, game_name, tag_line, region, rank_tier, rank_division, league_points
      FROM players WHERE game_name=? COLLATE NOCASE AND tag_line=? COLLATE NOCASE AND region=?`, seed.GameName, seed.TagLine, seed.Region).
		Scan(&p.PUUID, &p.GameName, &p.TagLine, &p.Region, &p.Tier, &p.Division, &p.LP)
	if err == sql.ErrNoRows {
		return Player{}, false, nil
	}
	return p, err == nil, err
}

func (s *Store) UpsertPlayer(seed Seed, a Account) error {
	source := seed.Source
	if source == "" {
		source = "seed"
	}
	_, err := s.DB.Exec(`INSERT INTO players(puuid, game_name, tag_line, region, source)
      VALUES(?,?,?,?,?) ON CONFLICT(puuid) DO UPDATE SET game_name=excluded.game_name,
      tag_line=excluded.tag_line, region=excluded.region, source=excluded.source`,
		a.PUUID, a.GameName, a.TagLine, seed.Region, source)
	return err
}

func (s *Store) UpdateRank(puuid string, rank LeagueEntry) error {
	_, err := s.DB.Exec(`UPDATE players SET rank_tier=?, rank_division=?, league_points=?,
      last_checked_at=? WHERE puuid=?`, rank.Tier, rank.Rank, rank.LeaguePoints, time.Now().Unix(), puuid)
	return err
}

func (s *Store) LoadPlayer(puuid string) (Player, error) {
	var p Player
	err := s.DB.QueryRow(`SELECT puuid, game_name, tag_line, region, rank_tier, rank_division, league_points
      FROM players WHERE puuid=?`, puuid).
		Scan(&p.PUUID, &p.GameName, &p.TagLine, &p.Region, &p.Tier, &p.Division, &p.LP)
	return p, err
}

func (s *Store) ListPlayers() ([]Player, error) {
	rows, err := s.DB.Query(`SELECT puuid, game_name, tag_line, region, rank_tier, rank_division, league_points
      FROM players ORDER BY game_name COLLATE NOCASE, tag_line COLLATE NOCASE`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var players []Player
	for rows.Next() {
		var p Player
		if err := rows.Scan(&p.PUUID, &p.GameName, &p.TagLine, &p.Region, &p.Tier, &p.Division, &p.LP); err != nil {
			return nil, err
		}
		players = append(players, p)
	}
	return players, rows.Err()
}

func (s *Store) CachedMatch(id string) (Match, bool, error) {
	var raw string
	err := s.DB.QueryRow(`SELECT raw_json FROM matches WHERE match_id=?`, id).Scan(&raw)
	if err == sql.ErrNoRows {
		return Match{}, false, nil
	}
	if err != nil {
		return Match{}, false, err
	}
	var m Match
	err = json.Unmarshal([]byte(raw), &m)
	return m, err == nil, err
}

func (s *Store) SaveMatch(m Match) error {
	if m.Metadata.MatchID == "" {
		return fmt.Errorf("match has no ID")
	}
	raw, err := json.Marshal(m)
	if err != nil {
		return err
	}
	_, err = s.DB.Exec(`INSERT OR IGNORE INTO matches
      (match_id, platform, game_version, game_creation, game_duration, queue_id, fetched_at, raw_json)
      VALUES(?,?,?,?,?,?,?,?)`, m.Metadata.MatchID, m.Info.PlatformID, m.Info.GameVersion,
		m.Info.GameCreation, m.Info.GameDuration, m.Info.QueueID, time.Now().Unix(), string(raw))
	return err
}

func (s *Store) GameExists(matchID, puuid string) (bool, error) {
	var one int
	err := s.DB.QueryRow(`SELECT 1 FROM match_checks WHERE match_id=? AND player_puuid=?`, matchID, puuid).Scan(&one)
	if err == sql.ErrNoRows {
		return false, nil
	}
	return err == nil, err
}

func (s *Store) MarkChecked(matchID, puuid string) error {
	_, err := s.DB.Exec(`INSERT OR IGNORE INTO match_checks(match_id,player_puuid) VALUES(?,?)`, matchID, puuid)
	return err
}

func (s *Store) SaveTrackedGame(m Match, p Player, f Participant, o *Participant, status string) error {
	var opp Participant
	if o != nil {
		opp = *o
	}
	win := 0
	if f.Win {
		win = 1
	}
	var okills, odeaths, oassists any
	if o != nil {
		okills, odeaths, oassists = o.Kills, o.Deaths, o.Assists
	}
	_, err := s.DB.Exec(`INSERT INTO tracked_games
      (match_id, player_puuid, player_game_name, player_tag_line, player_rank_tier, player_rank_division, champion,
       opponent_puuid, opponent_game_name, opponent_tag_line, opponent_champion, player_position,
       opponent_position, matchup_status, win, kills, deaths, assists, cs,
       opponent_kills, opponent_deaths, opponent_assists, gold,
       item0, item1, item2, item3, item4, item5, item6)
      VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)
      ON CONFLICT(match_id,player_puuid) DO UPDATE SET
      opponent_kills=COALESCE(tracked_games.opponent_kills,excluded.opponent_kills),
      opponent_deaths=COALESCE(tracked_games.opponent_deaths,excluded.opponent_deaths),
      opponent_assists=COALESCE(tracked_games.opponent_assists,excluded.opponent_assists)`,
		m.Metadata.MatchID, p.PUUID, f.RiotIDGameName, f.RiotIDTagline, p.Tier, p.Division, f.ChampionName,
		opp.PUUID, opp.RiotIDGameName, opp.RiotIDTagline, opp.ChampionName,
		position(f), position(opp), status, win, f.Kills, f.Deaths, f.Assists,
		f.TotalMinionsKilled+f.NeutralMinionsKilled, okills, odeaths, oassists, f.GoldEarned,
		f.Item0, f.Item1, f.Item2, f.Item3, f.Item4, f.Item5, f.Item6)
	return err
}
