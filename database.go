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
		`CREATE TABLE IF NOT EXISTS fiora_games (
            match_id TEXT NOT NULL REFERENCES matches(match_id),
            fiora_puuid TEXT NOT NULL REFERENCES players(puuid),
            fiora_game_name TEXT NOT NULL, fiora_tag_line TEXT NOT NULL,
            fiora_rank_tier TEXT NOT NULL, fiora_rank_division TEXT NOT NULL,
            opponent_puuid TEXT NOT NULL DEFAULT '', opponent_game_name TEXT NOT NULL DEFAULT '',
            opponent_tag_line TEXT NOT NULL DEFAULT '', opponent_champion TEXT NOT NULL DEFAULT '',
            fiora_position TEXT NOT NULL DEFAULT '', opponent_position TEXT NOT NULL DEFAULT '',
            matchup_status TEXT NOT NULL, win INTEGER NOT NULL, kills INTEGER NOT NULL,
            deaths INTEGER NOT NULL, assists INTEGER NOT NULL, cs INTEGER NOT NULL,
            gold INTEGER NOT NULL, item0 INTEGER NOT NULL, item1 INTEGER NOT NULL,
            item2 INTEGER NOT NULL, item3 INTEGER NOT NULL, item4 INTEGER NOT NULL,
            item5 INTEGER NOT NULL, item6 INTEGER NOT NULL,
            PRIMARY KEY (match_id, fiora_puuid))`,
		`CREATE TABLE IF NOT EXISTS match_checks (
            match_id TEXT NOT NULL REFERENCES matches(match_id),
            player_puuid TEXT NOT NULL REFERENCES players(puuid),
            PRIMARY KEY (match_id, player_puuid))`,
		`CREATE INDEX IF NOT EXISTS idx_matches_creation ON matches(game_creation DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_games_opponent ON fiora_games(opponent_champion)`,
	} {
		if _, err := db.Exec(q); err != nil {
			db.Close()
			return nil, err
		}
	}
	return &Store{DB: db}, nil
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

func (s *Store) SaveFioraGame(m Match, p Player, f Participant, o *Participant, status string) error {
	var opp Participant
	if o != nil {
		opp = *o
	}
	win := 0
	if f.Win {
		win = 1
	}
	_, err := s.DB.Exec(`INSERT OR IGNORE INTO fiora_games
      (match_id, fiora_puuid, fiora_game_name, fiora_tag_line, fiora_rank_tier, fiora_rank_division,
       opponent_puuid, opponent_game_name, opponent_tag_line, opponent_champion, fiora_position,
       opponent_position, matchup_status, win, kills, deaths, assists, cs, gold,
       item0, item1, item2, item3, item4, item5, item6)
      VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		m.Metadata.MatchID, p.PUUID, f.RiotIDGameName, f.RiotIDTagline, p.Tier, p.Division,
		opp.PUUID, opp.RiotIDGameName, opp.RiotIDTagline, opp.ChampionName,
		position(f), position(opp), status, win, f.Kills, f.Deaths, f.Assists,
		f.TotalMinionsKilled+f.NeutralMinionsKilled, f.GoldEarned,
		f.Item0, f.Item1, f.Item2, f.Item3, f.Item4, f.Item5, f.Item6)
	return err
}
