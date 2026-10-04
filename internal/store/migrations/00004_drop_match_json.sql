-- tracked_games now keeps only what describes the tracked player's point of
-- view; match stats come from match_participants. match_checks loses its
-- foreign keys so checks survive pruning of old-patch matches.

-- +goose Up
CREATE TABLE tracked_games_v2 (
    match_id TEXT NOT NULL REFERENCES matches(match_id),
    player_puuid TEXT NOT NULL REFERENCES players(puuid),
    opponent_puuid TEXT NOT NULL DEFAULT '',
    player_rank_tier TEXT NOT NULL,
    player_rank_division TEXT NOT NULL,
    player_position TEXT NOT NULL,
    opponent_position TEXT NOT NULL,
    matchup_status TEXT NOT NULL,
    PRIMARY KEY (match_id, player_puuid)
) WITHOUT ROWID;

INSERT INTO tracked_games_v2
    (match_id, player_puuid, opponent_puuid, player_rank_tier, player_rank_division,
     player_position, opponent_position, matchup_status)
SELECT match_id, player_puuid, opponent_puuid, player_rank_tier, player_rank_division,
       player_position, opponent_position, matchup_status
FROM tracked_games;

DROP INDEX idx_games_champions;
DROP TABLE tracked_games;
ALTER TABLE tracked_games_v2 RENAME TO tracked_games;

CREATE TABLE match_checks_v2 (
    match_id TEXT NOT NULL,
    player_puuid TEXT NOT NULL,
    PRIMARY KEY (match_id, player_puuid)
) WITHOUT ROWID;

INSERT INTO match_checks_v2 (match_id, player_puuid)
SELECT match_id, player_puuid FROM match_checks;

DROP TABLE match_checks;
ALTER TABLE match_checks_v2 RENAME TO match_checks;

ALTER TABLE matches DROP COLUMN raw_json;

CREATE INDEX idx_matches_patch ON matches(patch_major, patch_minor);
CREATE INDEX idx_participants_champion ON match_participants(champion);

-- +goose Down
-- Dropped columns cannot be restored; restore the pre-migration backup instead.
