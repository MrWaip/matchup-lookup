-- name: GetMatchRaw :one
SELECT raw_gz FROM matches WHERE match_id = ?;

-- name: UpsertMatch :exec
INSERT INTO matches
    (match_id, platform, game_version, patch_major, patch_minor,
     game_creation, game_duration, queue_id, fetched_at, raw_gz)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT (match_id) DO UPDATE SET
    raw_gz = excluded.raw_gz,
    fetched_at = excluded.fetched_at;

-- name: InsertParticipant :exec
INSERT OR IGNORE INTO match_participants
    (match_id, puuid, riot_id_game_name, riot_id_tagline, champion, team_id,
     team_position, individual_position, lane, win, kills, deaths, assists, cs, gold,
     item0, item1, item2, item3, item4, item5, item6,
     summoner1_id, summoner2_id, keystone, secondary_rune1, secondary_rune2)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: UpdateParticipantLoadout :exec
UPDATE match_participants
SET summoner1_id = ?, summoner2_id = ?, keystone = ?, secondary_rune1 = ?, secondary_rune2 = ?
WHERE match_id = ? AND puuid = ?;

-- name: MatchNeedsLoadout :one
SELECT EXISTS (
    SELECT 1 FROM match_participants
    WHERE match_id = ? AND (summoner1_id IS NULL OR keystone IS NULL)
);

-- name: ListMatchesMissingLoadout :many
SELECT m.match_id, m.platform FROM matches m
WHERE EXISTS (SELECT 1 FROM tracked_games g WHERE g.match_id = m.match_id)
  AND EXISTS (
      SELECT 1 FROM match_participants p
      WHERE p.match_id = m.match_id AND (p.summoner1_id IS NULL OR p.keystone IS NULL))
ORDER BY m.game_creation DESC;

-- name: IsChecked :one
SELECT EXISTS (SELECT 1 FROM match_checks WHERE match_id = ? AND player_puuid = ?);

-- name: MarkChecked :exec
INSERT OR IGNORE INTO match_checks (match_id, player_puuid) VALUES (?, ?);

-- name: InsertTrackedGame :exec
INSERT OR IGNORE INTO tracked_games
    (match_id, player_puuid, opponent_puuid, player_rank_tier, player_rank_division,
     player_position, opponent_position, matchup_status)
VALUES (?, ?, ?, ?, ?, ?, ?, ?);

-- name: CountTrackedGames :one
SELECT COUNT(*) FROM tracked_games;

-- name: LatestPatch :one
SELECT patch_major, patch_minor FROM matches
ORDER BY patch_major DESC, patch_minor DESC LIMIT 1;

-- Replays expire with each patch, so older matches are pruned. match_checks
-- keeps their IDs so the next update does not download them again.

-- name: DeleteTrackedGamesBeforePatch :exec
DELETE FROM tracked_games WHERE match_id IN (
    SELECT match_id FROM matches
    WHERE patch_major < @major OR (patch_major = @major AND patch_minor < @minor));

-- name: DeleteParticipantsBeforePatch :exec
DELETE FROM match_participants WHERE match_id IN (
    SELECT match_id FROM matches
    WHERE patch_major < @major OR (patch_major = @major AND patch_minor < @minor));

-- name: DeleteMatchesBeforePatch :execrows
DELETE FROM matches
WHERE patch_major < @major OR (patch_major = @major AND patch_minor < @minor);
