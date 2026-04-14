package main

import (
	"context"
	"database/sql"
	"errors"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"
)

func main() {
	fs := flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	configPath := fs.String("config", "config.json", "path to config file")
	_ = fs.Parse(cleanArgs(os.Args[1:]))

	cfg, err := LoadConfig(*configPath)
	if err != nil && *configPath == "config.json" {
		var pathErr *os.PathError
		if errors.As(err, &pathErr) && errors.Is(pathErr.Err, os.ErrNotExist) {
			cfg, err = LoadConfig(filepath.Clean(filepath.Join("..", "config.json")))
		}
	}
	if err != nil {
		log.Fatalf("config error: %v", err)
	}

	if err := os.MkdirAll(filepath.Dir(cfg.DatabasePath), 0o755); err != nil {
		log.Fatalf("create db directory: %v", err)
	}

	db, err := OpenDatabase(cfg.DatabasePath)
	if err != nil {
		log.Fatalf("database error: %v", err)
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	if err := syncServerConfig(ctx, db, cfg); err != nil {
		cancel()
		log.Fatalf("sync server config: %v", err)
	}
	cancel()

	rootCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	poller, err := NewPoller(rootCtx, cfg, db)
	if err != nil {
		log.Fatalf("poller init error: %v", err)
	}
	defer poller.Close()
	poller.Start(rootCtx)

	api := NewAPI(cfg, db, poller)
	httpServer := &http.Server{
		Addr:              cfg.ListenAddr,
		Handler:           api.Routes(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		<-rootCtx.Done()
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer shutdownCancel()
		_ = httpServer.Shutdown(shutdownCtx)
	}()

	log.Printf("minestats backend listening on %s", cfg.ListenAddr)
	if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("http server error: %v", err)
	}
}

func cleanArgs(args []string) []string {
	if len(args) > 0 && args[0] == "--" {
		return args[1:]
	}
	return args
}

func syncServerConfig(ctx context.Context, db *sql.DB, cfg Config) error {
	for _, s := range cfg.Servers {
		if err := UpsertServerConfig(ctx, db, s); err != nil {
			return err
		}
	}
	return nil
}
