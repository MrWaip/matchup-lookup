-- +goose Up
CREATE TABLE player_controls (
    region TEXT NOT NULL,
    game_name TEXT NOT NULL COLLATE NOCASE,
    tag_line TEXT NOT NULL COLLATE NOCASE,
    enabled INTEGER NOT NULL DEFAULT 1,
    tags TEXT NOT NULL DEFAULT '[]',
    PRIMARY KEY (region, game_name, tag_line)
);

-- +goose Down
DROP TABLE player_controls;
