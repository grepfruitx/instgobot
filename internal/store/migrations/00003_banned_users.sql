-- +goose Up
CREATE TABLE IF NOT EXISTS banned_users (
	chat_id INTEGER PRIMARY KEY,
	banned_at TEXT NOT NULL
);

-- +goose Down
DROP TABLE IF EXISTS banned_users;
