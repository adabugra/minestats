package main

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
)

type Config struct {
	ListenAddr            string         `json:"listen_addr"`
	DatabasePath          string         `json:"database_path"`
	HistoryRetentionHours int            `json:"history_retention_hours"`
	Servers               []ServerConfig `json:"servers"`
}

type ServerConfig struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	Address        string `json:"address"`
	RefreshSeconds int    `json:"refresh_seconds"`
}

func LoadConfig(path string) (Config, error) {
	var cfg Config

	data, err := os.ReadFile(path)
	if err != nil {
		return cfg, fmt.Errorf("read config: %w", err)
	}

	if err := json.Unmarshal(data, &cfg); err != nil {
		return cfg, fmt.Errorf("parse config: %w", err)
	}

	if cfg.ListenAddr == "" {
		cfg.ListenAddr = ":8080"
	}
	if cfg.DatabasePath == "" {
		cfg.DatabasePath = "./backend/data/minestats.db"
	}
	if cfg.HistoryRetentionHours <= 0 {
		cfg.HistoryRetentionHours = 24 * 7
	}
	if len(cfg.Servers) == 0 {
		return cfg, fmt.Errorf("config.servers must not be empty")
	}

	seen := make(map[string]struct{}, len(cfg.Servers))
	for i := range cfg.Servers {
		s := &cfg.Servers[i]
		if s.ID == "" {
			return cfg, fmt.Errorf("servers[%d].id is required", i)
		}
		if _, exists := seen[s.ID]; exists {
			return cfg, fmt.Errorf("duplicate server id: %s", s.ID)
		}
		seen[s.ID] = struct{}{}
		if s.Name == "" {
			s.Name = s.ID
		}
		if s.Address == "" {
			return cfg, fmt.Errorf("servers[%d].address is required", i)
		}
		if !strings.Contains(s.Address, ":") {
			s.Address += ":25565"
		}
		if s.RefreshSeconds < 1 || s.RefreshSeconds > 3 {
			return cfg, fmt.Errorf("servers[%d].refresh_seconds must be between 1 and 3", i)
		}
	}

	sort.Slice(cfg.Servers, func(i, j int) bool {
		return cfg.Servers[i].Name < cfg.Servers[j].Name
	})

	return cfg, nil
}
