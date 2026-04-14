package main

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

type Sample struct {
	ServerID      string `json:"server_id"`
	TimestampMS   int64  `json:"ts_ms"`
	OnlinePlayers *int   `json:"online_players"`
	MaxPlayers    *int   `json:"max_players"`
	LatencyMS     *int64 `json:"latency_ms"`
	IsOnline      bool   `json:"is_online"`
	MOTD          string `json:"motd,omitempty"`
	Version       string `json:"version,omitempty"`
	Favicon       string `json:"favicon,omitempty"`
	Error         string `json:"error,omitempty"`
}

func OpenDatabase(path string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}

	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(0)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, err := db.ExecContext(ctx, `
PRAGMA journal_mode=WAL;
PRAGMA synchronous=NORMAL;
PRAGMA temp_store=MEMORY;
PRAGMA foreign_keys=ON;
`); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("init pragmas: %w", err)
	}

	if _, err := db.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS servers (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    address TEXT NOT NULL,
    refresh_seconds INTEGER NOT NULL
);
CREATE TABLE IF NOT EXISTS samples (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    server_id TEXT NOT NULL,
    ts_ms INTEGER NOT NULL,
    online_players INTEGER,
    max_players INTEGER,
    latency_ms INTEGER,
    is_online INTEGER NOT NULL,
    error TEXT,
    FOREIGN KEY (server_id) REFERENCES servers(id)
);
CREATE INDEX IF NOT EXISTS idx_samples_server_ts ON samples(server_id, ts_ms DESC);
CREATE INDEX IF NOT EXISTS idx_samples_ts ON samples(ts_ms DESC);
`); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("init schema: %w", err)
	}

	return db, nil
}

func UpsertServerConfig(ctx context.Context, db *sql.DB, s ServerConfig) error {
	_, err := db.ExecContext(ctx, `
INSERT INTO servers (id, name, address, refresh_seconds)
VALUES (?, ?, ?, ?)
ON CONFLICT(id) DO UPDATE SET
    name=excluded.name,
    address=excluded.address,
    refresh_seconds=excluded.refresh_seconds;
`, s.ID, s.Name, s.Address, s.RefreshSeconds)
	if err != nil {
		return fmt.Errorf("upsert server %s: %w", s.ID, err)
	}
	return nil
}

func InsertSample(ctx context.Context, stmt *sql.Stmt, sample Sample) error {
	isOnline := 0
	if sample.IsOnline {
		isOnline = 1
	}
	_, err := stmt.ExecContext(
		ctx,
		sample.ServerID,
		sample.TimestampMS,
		sample.OnlinePlayers,
		sample.MaxPlayers,
		sample.LatencyMS,
		isOnline,
		sample.Error,
	)
	if err != nil {
		return fmt.Errorf("insert sample for %s: %w", sample.ServerID, err)
	}
	return nil
}
