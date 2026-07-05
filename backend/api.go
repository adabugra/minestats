package main

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"
	"time"
)

type API struct {
	cfg    Config
	db     *sql.DB
	poller *Poller
}

func NewAPI(cfg Config, db *sql.DB, poller *Poller) *API {
	return &API{cfg: cfg, db: db, poller: poller}
}

func (a *API) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/health", a.handleHealth)
	mux.HandleFunc("/api/servers", a.handleServers)
	mux.HandleFunc("/api/history", a.handleHistory)
	return withCORS(mux)
}

func (a *API) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (a *API) handleServers(w http.ResponseWriter, r *http.Request) {
	type serverInfo struct {
		ID             string  `json:"id"`
		Name           string  `json:"name"`
		Address        string  `json:"address"`
		RefreshSeconds int     `json:"refresh_seconds"`
		LastSample     *Sample `json:"last_sample"`
	}

	latest := a.poller.Latest()
	servers := make([]serverInfo, 0, len(a.cfg.Servers))
	for _, s := range a.cfg.Servers {
		entry := serverInfo{ID: s.ID, Name: s.Name, Address: s.Address, RefreshSeconds: s.RefreshSeconds}
		if sample, ok := latest[s.ID]; ok {
			sCopy := sample
			entry.LastSample = &sCopy
		}
		servers = append(servers, entry)
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"generated_at": time.Now().UnixMilli(),
		"servers":      servers,
	})
}

func (a *API) handleHistory(w http.ResponseWriter, r *http.Request) {
	minutes := 60
	allHistory := false
	if q := r.URL.Query().Get("minutes"); q != "" {
		if q == "all" || q == "0" {
			allHistory = true
			if a.cfg.HistoryRetentionHours > 0 {
				minutes = a.cfg.HistoryRetentionHours * 60
			}
		} else {
			v, err := strconv.Atoi(q)
			if err == nil && v >= 1 && v <= 525600 {
				minutes = v
			}
		}
	}

	bucketMS := int64(0)
	if q := r.URL.Query().Get("bucket_seconds"); q != "" {
		v, err := strconv.Atoi(q)
		if err == nil && v >= 0 && v <= 86400 {
			bucketMS = int64(v) * 1000
		}
	}

	from := time.Now().Add(-time.Duration(minutes) * time.Minute).UnixMilli()
	if allHistory && a.cfg.HistoryRetentionHours <= 0 {
		from = 0
	}

	query := `
SELECT server_id, ts_ms, online_players, is_online
FROM samples
WHERE ts_ms >= ?
ORDER BY ts_ms ASC;`
	args := []any{from}
	if bucketMS > 0 {
		query = `
WITH bucketed AS (
	SELECT
		server_id,
		((ts_ms / ?) * ?) AS bucket_ts,
		MAX(ts_ms) AS max_ts
	FROM samples
	WHERE ts_ms >= ?
	GROUP BY server_id, bucket_ts
)
SELECT
	b.server_id,
	b.bucket_ts,
	s.online_players,
	s.is_online
FROM bucketed b
JOIN samples s
	ON s.server_id = b.server_id
	AND s.ts_ms = b.max_ts
ORDER BY b.bucket_ts ASC;`
		args = []any{bucketMS, bucketMS, from}
	}

	rows, err := a.db.QueryContext(r.Context(), query, args...)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "query failed"})
		return
	}
	defer rows.Close()

	series := make(map[string][][2]any, len(a.cfg.Servers))
	for _, s := range a.cfg.Servers {
		series[s.ID] = make([][2]any, 0, minutes*60)
	}

	for rows.Next() {
		var serverID string
		var ts int64
		var onlinePlayers sql.NullInt64
		var isOnline int
		if err := rows.Scan(&serverID, &ts, &onlinePlayers, &isOnline); err != nil {
			continue
		}
		value := any(nil)
		if isOnline == 1 && onlinePlayers.Valid {
			value = onlinePlayers.Int64
		}
		series[serverID] = append(series[serverID], [2]any{ts, value})
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"from":        from,
		"to":          time.Now().UnixMilli(),
		"minutes":     minutes,
		"bucket_ms":   bucketMS,
		"all_history": allHistory,
		"series":      series,
	})
}

func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		w.Header().Set("Access-Control-Allow-Methods", "GET,OPTIONS")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
