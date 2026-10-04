-- name: ListChampions :many
-- Catalog champions plus any champion seen in stored matches but missing
-- from the catalog (e.g. a release newer than the cached Data Dragon copy).
SELECT id, display_name FROM champions
UNION
SELECT DISTINCT champion, champion FROM match_participants
WHERE champion <> '' AND champion NOT IN (SELECT id FROM champions)
ORDER BY 2 COLLATE NOCASE;

-- name: UpsertChampion :exec
INSERT INTO champions (id, display_name) VALUES (?, ?)
ON CONFLICT (id) DO UPDATE SET display_name = excluded.display_name;
