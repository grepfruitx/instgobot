-- +goose Up
CREATE TABLE IF NOT EXISTS cache_stats (
	platform TEXT PRIMARY KEY,
	hits INTEGER NOT NULL DEFAULT 0,
	misses INTEGER NOT NULL DEFAULT 0
);

-- +goose Down
DROP TABLE IF EXISTS cache_stats;
