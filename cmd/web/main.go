package main

import (
	"log/slog"
	"os"

	"datablox/internal/config"
	"datablox/internal/store/sqlite"
	"datablox/internal/web"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("config", "err", err)
		os.Exit(1)
	}
	log := newLogger(cfg.LogLevel)

	st, err := sqlite.Open(cfg.DatabasePath)
	if err != nil {
		slog.Error("db", "err", err)
		os.Exit(1)
	}
	defer st.Close()
	log.Info("database ready (web HMR)", "path", cfg.DatabasePath)

	// Web-only: discord=nil -> verify sync jadi no-op, dashboard tetap jalan
	// Bot runs separately without HMR to avoid flooding the gateway/rate-limit
	srv, err := web.New(cfg, st, log, nil)
	if err != nil {
		slog.Error("web init", "err", err)
		os.Exit(1)
	}
	log.Info("web HMR listening", "url", cfg.WebURL, "proxy", "templ proxy -> localhost:"+cfg.WebPort)
	if err := srv.Start(); err != nil {
		slog.Error("web", "err", err)
		os.Exit(1)
	}
}

func newLogger(level string) *slog.Logger {
	var lvl slog.Level
	switch level {
	case "debug":
		lvl = slog.LevelDebug
	case "warn":
		lvl = slog.LevelWarn
	case "error":
		lvl = slog.LevelError
	default:
		lvl = slog.LevelInfo
	}
	return slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: lvl}))
}
