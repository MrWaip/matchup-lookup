-- name: ListRiotKeys :many
SELECT id, label, value, added_at, rejected_at, rejection FROM riot_keys ORDER BY id;

-- name: AddRiotKey :exec
-- Re-adding a known key renames it and clears its rejection.
INSERT INTO riot_keys (label, value, added_at) VALUES (?, ?, ?)
ON CONFLICT (value) DO UPDATE SET label = excluded.label, rejected_at = 0, rejection = '';

-- name: DeleteRiotKey :exec
DELETE FROM riot_keys WHERE id = ?;

-- name: RejectRiotKey :exec
UPDATE riot_keys SET rejected_at = ?, rejection = ? WHERE id = ?;
