-- name: GetPlayer :one
SELECT puuid, game_name, tag_line, region, rank_tier, rank_division, league_points
FROM players WHERE puuid = ?;

-- name: FindPlayerByRiotID :one
SELECT puuid, game_name, tag_line, region, rank_tier, rank_division, league_points
FROM players
WHERE game_name = @game_name COLLATE NOCASE AND tag_line = @tag_line COLLATE NOCASE AND region = @region;

-- name: ListPlayers :many
SELECT puuid, game_name, tag_line, region, rank_tier, rank_division, league_points
FROM players ORDER BY game_name COLLATE NOCASE, tag_line COLLATE NOCASE;

-- name: ListActivePlayers :many
SELECT p.puuid, p.game_name, p.tag_line, p.region, p.rank_tier, p.rank_division, p.league_points
FROM players p LEFT JOIN player_controls c
  ON c.region = p.region AND c.game_name = p.game_name COLLATE NOCASE AND c.tag_line = p.tag_line COLLATE NOCASE
WHERE COALESCE(c.enabled, 1) = 1
ORDER BY p.game_name COLLATE NOCASE, p.tag_line COLLATE NOCASE;

-- name: CountPlayers :one
SELECT COUNT(*) FROM players;

-- name: UpsertPlayer :exec
INSERT INTO players (puuid, game_name, tag_line, region, source)
VALUES (?, ?, ?, ?, ?)
ON CONFLICT (puuid) DO UPDATE SET
    game_name = excluded.game_name,
    tag_line = excluded.tag_line,
    region = excluded.region,
    source = excluded.source;

-- name: UpdatePlayerRank :exec
UPDATE players
SET rank_tier = ?, rank_division = ?, league_points = ?, last_checked_at = ?
WHERE puuid = ?;

-- name: AddPlayerChampion :exec
INSERT OR IGNORE INTO player_champions (player_puuid, champion) VALUES (?, ?);

-- name: ListPlayerChampions :many
SELECT champion FROM player_champions
WHERE player_puuid = ? ORDER BY champion COLLATE NOCASE;

-- name: UpsertSeed :exec
INSERT INTO player_seeds (game_name, tag_line, region, champion, source, resolved_puuid)
VALUES (?, ?, ?, ?, ?, ?)
ON CONFLICT (region, game_name, tag_line, champion) DO UPDATE SET
    source = excluded.source,
    resolved_puuid = CASE WHEN player_seeds.resolved_puuid = ''
        THEN excluded.resolved_puuid ELSE player_seeds.resolved_puuid END;

-- name: ListPendingSeeds :many
SELECT game_name, tag_line, region, source FROM player_seeds
WHERE resolved_puuid = '' AND NOT EXISTS (
    SELECT 1 FROM player_controls c WHERE c.region = player_seeds.region
    AND c.game_name = player_seeds.game_name COLLATE NOCASE
    AND c.tag_line = player_seeds.tag_line COLLATE NOCASE AND c.enabled = 0)
GROUP BY region, game_name, tag_line
ORDER BY region, game_name;

-- name: ListAllPendingSeeds :many
SELECT game_name, tag_line, region, source FROM player_seeds
WHERE resolved_puuid = ''
GROUP BY region, game_name, tag_line
ORDER BY region, game_name;

-- name: GetPlayerControl :one
SELECT enabled, tags FROM player_controls
WHERE region = ? AND game_name = ? COLLATE NOCASE AND tag_line = ? COLLATE NOCASE;

-- name: SavePlayerControl :exec
INSERT INTO player_controls (region, game_name, tag_line, enabled, tags)
VALUES (?, ?, ?, ?, ?)
ON CONFLICT (region, game_name, tag_line) DO UPDATE SET enabled = excluded.enabled, tags = excluded.tags;

-- name: DeletePlayerControl :exec
DELETE FROM player_controls WHERE region = ? AND game_name = ? COLLATE NOCASE AND tag_line = ? COLLATE NOCASE;

-- name: DeleteSeedIdentity :exec
DELETE FROM player_seeds WHERE region = ? AND game_name = ? COLLATE NOCASE AND tag_line = ? COLLATE NOCASE;

-- name: DeletePlayerChampions :exec
DELETE FROM player_champions WHERE player_puuid = ?;

-- name: DeletePlayerGames :exec
DELETE FROM tracked_games WHERE player_puuid = ?;

-- name: DeletePlayerChecks :exec
DELETE FROM match_checks WHERE player_puuid = ?;

-- name: DeletePlayer :exec
DELETE FROM players WHERE puuid = ?;

-- name: ListSeedChampions :many
SELECT champion FROM player_seeds
WHERE region = ? AND game_name = ? AND tag_line = ?;

-- name: ResolveSeed :exec
UPDATE player_seeds SET resolved_puuid = @puuid
WHERE region = @region AND game_name = @game_name AND tag_line = @tag_line;
