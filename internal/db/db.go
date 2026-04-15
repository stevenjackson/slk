package db

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	_ "modernc.org/sqlite"
)

func Open() (*sql.DB, error) {
	path, err := dbPath()
	if err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}
	// WAL mode for concurrent reads; generous timeout for sync writes
	db.Exec("PRAGMA journal_mode=WAL")
	db.Exec("PRAGMA busy_timeout=30000")
	if err := migrate(db); err != nil {
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return db, nil
}

func dbPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	p := os.Getenv("SLK_DB")
	if p == "" {
		p = filepath.Join(home, ".slk", "slk.db")
	} else if strings.HasPrefix(p, "~/") {
		p = filepath.Join(home, p[2:])
	}
	if err := os.MkdirAll(filepath.Dir(p), 0700); err != nil {
		return "", fmt.Errorf("create db dir: %w", err)
	}
	return p, nil
}

func migrate(db *sql.DB) error {
	_, err := db.Exec(`
	CREATE TABLE IF NOT EXISTS channels (
		id            TEXT PRIMARY KEY,
		name          TEXT NOT NULL,
		last_synced_ts TEXT
	);

	CREATE TABLE IF NOT EXISTS users (
		id        TEXT PRIMARY KEY,
		name      TEXT,
		real_name TEXT
	);

	CREATE TABLE IF NOT EXISTS messages (
		ts           TEXT PRIMARY KEY,
		channel_id   TEXT NOT NULL,
		user_id      TEXT,
		text         TEXT,
		reply_count  INTEGER DEFAULT 0,
		replies_json TEXT,
		status       TEXT DEFAULT 'unread',
		synced_at    TEXT NOT NULL,
		FOREIGN KEY(channel_id) REFERENCES channels(id)
	);

	CREATE INDEX IF NOT EXISTS idx_messages_channel  ON messages(channel_id);
	CREATE INDEX IF NOT EXISTS idx_messages_status   ON messages(status);
	CREATE INDEX IF NOT EXISTS idx_messages_ts       ON messages(ts);

	CREATE TABLE IF NOT EXISTS config (
		key   TEXT PRIMARY KEY,
		value TEXT NOT NULL
	);
	`)
	return err
}
