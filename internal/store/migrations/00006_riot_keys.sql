-- Several Riot API keys, used in order as fallbacks (e.g. a personal key and
-- a 24-hour development key). Riot's policies forbid combining keys to raise
-- rate limits, so only one key is in use at a time. rejected_at marks a key
-- Riot answered with 401/403 (expired or revoked); it is skipped until
-- re-added.

-- +goose Up
CREATE TABLE riot_keys (
    id INTEGER PRIMARY KEY,
    label TEXT NOT NULL,
    value TEXT NOT NULL UNIQUE,
    added_at INTEGER NOT NULL,
    rejected_at INTEGER NOT NULL DEFAULT 0,
    rejection TEXT NOT NULL DEFAULT ''
);

INSERT INTO riot_keys (label, value, added_at)
SELECT 'Key', value, CAST(strftime('%s', 'now') AS INTEGER)
FROM app_settings WHERE key = 'riot_api_key' AND value <> '';

DELETE FROM app_settings WHERE key = 'riot_api_key';

-- +goose Down
INSERT OR REPLACE INTO app_settings (key, value)
SELECT 'riot_api_key', value FROM riot_keys ORDER BY id LIMIT 1;
DROP TABLE riot_keys;
