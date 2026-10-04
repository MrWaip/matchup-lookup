-- Searchable fields move out of the match JSON into columns, and the full
-- Riot response is kept gzip-compressed for later extraction. Only tracked
-- players and their lane opponents get participant rows; everyone else in
-- the match is available from raw_gz.
-- 00003 fills the new columns; 00004 drops the old ones.

-- +goose Up
CREATE TABLE match_participants (
    match_id TEXT NOT NULL REFERENCES matches(match_id),
    puuid TEXT NOT NULL,
    riot_id_game_name TEXT NOT NULL,
    riot_id_tagline TEXT NOT NULL,
    champion TEXT NOT NULL,
    team_id INTEGER NOT NULL,
    team_position TEXT NOT NULL,
    individual_position TEXT NOT NULL,
    lane TEXT NOT NULL,
    win BOOLEAN NOT NULL,
    kills INTEGER NOT NULL,
    deaths INTEGER NOT NULL,
    assists INTEGER NOT NULL,
    cs INTEGER NOT NULL,
    gold INTEGER NOT NULL,
    item0 INTEGER NOT NULL,
    item1 INTEGER NOT NULL,
    item2 INTEGER NOT NULL,
    item3 INTEGER NOT NULL,
    item4 INTEGER NOT NULL,
    item5 INTEGER NOT NULL,
    item6 INTEGER NOT NULL,
    -- NULL when the stored match predates loadout collection; see MissingLoadoutMatches.
    summoner1_id INTEGER,
    summoner2_id INTEGER,
    keystone INTEGER,
    secondary_rune1 INTEGER,
    secondary_rune2 INTEGER,
    PRIMARY KEY (match_id, puuid)
) WITHOUT ROWID;

ALTER TABLE matches ADD COLUMN patch_major INTEGER NOT NULL DEFAULT 0;
ALTER TABLE matches ADD COLUMN patch_minor INTEGER NOT NULL DEFAULT 0;
ALTER TABLE matches ADD COLUMN raw_gz BLOB NOT NULL DEFAULT x'';

-- +goose Down
ALTER TABLE matches DROP COLUMN raw_gz;
ALTER TABLE matches DROP COLUMN patch_minor;
ALTER TABLE matches DROP COLUMN patch_major;
DROP TABLE match_participants;
