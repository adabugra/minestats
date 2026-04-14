package main

import (
	"context"
	"database/sql"
	"sync"
	"time"
)

type Poller struct {
	cfg        Config
	db         *sql.DB
	insertStmt *sql.Stmt
	latestMu   sync.RWMutex
	latest     map[string]Sample
}

func NewPoller(ctx context.Context, cfg Config, db *sql.DB) (*Poller, error) {
	stmt, err := db.PrepareContext(ctx, `
INSERT INTO samples (server_id, ts_ms, online_players, max_players, latency_ms, is_online, error)
VALUES (?, ?, ?, ?, ?, ?, ?);`)
	if err != nil {
		return nil, err
	}

	p := &Poller{
		cfg:        cfg,
		db:         db,
		insertStmt: stmt,
		latest:     make(map[string]Sample, len(cfg.Servers)),
	}
	return p, nil
}

func (p *Poller) Close() error {
	if p.insertStmt != nil {
		return p.insertStmt.Close()
	}
	return nil
}

func (p *Poller) Start(ctx context.Context) {
	for _, server := range p.cfg.Servers {
		s := server
		go p.runServerLoop(ctx, s)
	}
	go p.retentionLoop(ctx)
}

func (p *Poller) runServerLoop(ctx context.Context, server ServerConfig) {
	interval := time.Duration(server.RefreshSeconds) * time.Second
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	p.pollOnce(ctx, server)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			p.pollOnce(ctx, server)
		}
	}
}

func (p *Poller) pollOnce(ctx context.Context, server ServerConfig) {
	now := time.Now().UnixMilli()

	pollCtx, cancel := context.WithTimeout(ctx, 2500*time.Millisecond)
	defer cancel()

	status, err := QueryMinecraftStatus(pollCtx, server.Address)
	sample := Sample{
		ServerID:    server.ID,
		TimestampMS: now,
	}
	if err != nil {
		sample.IsOnline = false
		sample.Error = err.Error()
	} else {
		online := status.PlayersOnline
		max := status.PlayersMax
		latency := status.LatencyMS
		sample.IsOnline = true
		sample.OnlinePlayers = &online
		sample.MaxPlayers = &max
		sample.LatencyMS = &latency
		sample.MOTD = status.MOTD
		sample.Version = status.Version
		sample.Favicon = status.Favicon
	}

	dbCtx, dbCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer dbCancel()
	if err := InsertSample(dbCtx, p.insertStmt, sample); err != nil {
		return
	}

	p.latestMu.Lock()
	p.latest[server.ID] = sample
	p.latestMu.Unlock()
}

func (p *Poller) retentionLoop(ctx context.Context) {
	if p.cfg.HistoryRetentionHours <= 0 {
		return
	}
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			cutoff := time.Now().Add(-time.Duration(p.cfg.HistoryRetentionHours) * time.Hour).UnixMilli()
			dbCtx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			_, _ = p.db.ExecContext(dbCtx, `DELETE FROM samples WHERE ts_ms < ?;`, cutoff)
			cancel()
		}
	}
}

func (p *Poller) Latest() map[string]Sample {
	p.latestMu.RLock()
	defer p.latestMu.RUnlock()
	out := make(map[string]Sample, len(p.latest))
	for k, v := range p.latest {
		out[k] = v
	}
	return out
}
