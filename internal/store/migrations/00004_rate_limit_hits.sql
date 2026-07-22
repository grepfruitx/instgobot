-- +goose Up
CREATE TABLE IF NOT EXISTS rate_limit_hits (
	kind TEXT PRIMARY KEY,
	count INTEGER NOT NULL DEFAULT 0
);

-- +goose Down
DROP TABLE IF EXISTS rate_limit_hits;
