package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"datablox/internal/config"
	"datablox/internal/discord"
	"datablox/internal/roblox"
	"datablox/internal/scheduler"
	"datablox/internal/service"
	"datablox/internal/store/sqlite"
	"datablox/internal/web"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		fatal(err)
	}

	log := newLogger(cfg.LogLevel)

	st, err := sqlite.Open(cfg.DatabasePath)
	if err != nil {
		fatal(err)
	}
	defer st.Close()
	log.Info("database ready", "path", cfg.DatabasePath)

	rc := roblox.New(cfg.RobloxTimeout)
	svc := service.New(rc, st)

	bot, err := discord.New(cfg, log, svc)
	if err != nil {
		fatal(err)
	}

	refresh := scheduler.New(log, svc, cfg.RefreshInterval, true)
	defer refresh.Stop()

	// Web dashboard (Bloxlink-like) — skip when running alongside `task web` to avoid :8003 conflict and lost OAuth state
	if os.Getenv("DISABLE_BOT_WEB") != "1" {
		webSrv, err := web.New(cfg, st, log, bot.Session())
		if err != nil {
			log.Warn("web dashboard failed to start", "err", err)
		} else {
			go func() {
				if err := webSrv.Start(); err != nil {
					log.Error("web dashboard error", "err", err)
				}
			}()
		}
	} else {
		log.Info("web dashboard disabled in bot (DISABLE_BOT_WEB=1) — use task web for HMR")
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := bot.Run(ctx); err != nil {
		fatal(err)
	}
	log.Info("bot stopped")
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

func fatal(err error) {
	slog.Error("fatal", "err", err)
	os.Exit(1)
}
