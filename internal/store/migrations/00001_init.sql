-- Schema of databases created before goose migrations (PRAGMA user_version=2).
-- IF NOT EXISTS lets those databases adopt goose without changes.

-- +goose Up
CREATE TABLE IF NOT EXISTS players (
    puuid TEXT PRIMARY KEY,
    game_name TEXT NOT NULL,
    tag_line TEXT NOT NULL,
    region TEXT NOT NULL,
    source TEXT NOT NULL DEFAULT 'seed',
    rank_tier TEXT NOT NULL DEFAULT '',
    rank_division TEXT NOT NULL DEFAULT '',
    league_points INTEGER NOT NULL DEFAULT 0,
    last_checked_at INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS matches (
    match_id TEXT PRIMARY KEY,
    platform TEXT NOT NULL,
    game_version TEXT NOT NULL,
    game_creation INTEGER NOT NULL,
    game_duration INTEGER NOT NULL,
    queue_id INTEGER NOT NULL,
    fetched_at INTEGER NOT NULL,
    raw_json TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS tracked_games (
    match_id TEXT NOT NULL REFERENCES matches(match_id),
    player_puuid TEXT NOT NULL REFERENCES players(puuid),
    player_game_name TEXT NOT NULL,
    player_tag_line TEXT NOT NULL,
    player_rank_tier TEXT NOT NULL,
    player_rank_division TEXT NOT NULL,
    champion TEXT NOT NULL,
    opponent_puuid TEXT NOT NULL DEFAULT '',
    opponent_game_name TEXT NOT NULL DEFAULT '',
    opponent_tag_line TEXT NOT NULL DEFAULT '',
    opponent_champion TEXT NOT NULL DEFAULT '',
    player_position TEXT NOT NULL DEFAULT '',
    opponent_position TEXT NOT NULL DEFAULT '',
    matchup_status TEXT NOT NULL,
    win INTEGER NOT NULL,
    kills INTEGER NOT NULL,
    deaths INTEGER NOT NULL,
    assists INTEGER NOT NULL,
    cs INTEGER NOT NULL,
    opponent_kills INTEGER,
    opponent_deaths INTEGER,
    opponent_assists INTEGER,
    gold INTEGER NOT NULL,
    item0 INTEGER NOT NULL,
    item1 INTEGER NOT NULL,
    item2 INTEGER NOT NULL,
    item3 INTEGER NOT NULL,
    item4 INTEGER NOT NULL,
    item5 INTEGER NOT NULL,
    item6 INTEGER NOT NULL,
    PRIMARY KEY (match_id, player_puuid)
);

CREATE TABLE IF NOT EXISTS match_checks (
    match_id TEXT NOT NULL REFERENCES matches(match_id),
    player_puuid TEXT NOT NULL REFERENCES players(puuid),
    PRIMARY KEY (match_id, player_puuid)
);

CREATE TABLE IF NOT EXISTS app_settings (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS champions (
    id TEXT PRIMARY KEY,
    display_name TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS player_champions (
    player_puuid TEXT NOT NULL REFERENCES players(puuid),
    champion TEXT NOT NULL,
    PRIMARY KEY (player_puuid, champion)
);

CREATE TABLE IF NOT EXISTS player_seeds (
    game_name TEXT NOT NULL COLLATE NOCASE,
    tag_line TEXT NOT NULL COLLATE NOCASE,
    region TEXT NOT NULL,
    champion TEXT NOT NULL DEFAULT '' COLLATE NOCASE,
    source TEXT NOT NULL DEFAULT 'seed',
    resolved_puuid TEXT NOT NULL DEFAULT '',
    PRIMARY KEY (region, game_name, tag_line, champion)
);

CREATE INDEX IF NOT EXISTS idx_matches_creation ON matches(game_creation DESC);
CREATE INDEX IF NOT EXISTS idx_games_champions ON tracked_games(champion, opponent_champion);

-- +goose Down
-- The initial schema is never rolled back.
