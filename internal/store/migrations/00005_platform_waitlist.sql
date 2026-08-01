-- +goose Up
CREATE TABLE IF NOT EXISTS platform_waitlist (
	chat_id INTEGER NOT NULL,
	platform TEXT NOT NULL,
	joined_at TEXT NOT NULL,
	PRIMARY KEY (chat_id, platform)
);

-- +goose Down
DROP TABLE IF EXISTS platform_waitlist;
