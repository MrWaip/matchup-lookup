-- Reclaim the space freed by moving match JSON to compressed blobs.

-- +goose NO TRANSACTION
-- +goose Up
VACUUM;

-- +goose Down
