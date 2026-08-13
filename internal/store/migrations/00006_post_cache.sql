-- +goose Up
CREATE TABLE IF NOT EXISTS post_cache (
	post_url TEXT PRIMARY KEY,
	photo_count INTEGER NOT NULL DEFAULT 0,
	video_count INTEGER NOT NULL DEFAULT 0,
	post_text TEXT NOT NULL DEFAULT '',
	cached_at TEXT NOT NULL
);

-- +goose Down
DROP TABLE IF EXISTS post_cache;
