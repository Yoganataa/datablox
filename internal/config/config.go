package config

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	DiscordToken    string
	GuildID         string
	DatabasePath    string
	RobloxTimeout   time.Duration
	RefreshInterval time.Duration
	LogLevel        string
	WebPort         string
	WebURL          string
	DiscordClientID string
	DiscordSecret   string
	RobloxClientID  string
	RobloxSecret    string
	AdminDiscordIDs map[string]bool
}

func Load() (*Config, error) {
	loadDotEnv()

	cfg := &Config{
		DiscordToken:    os.Getenv("DISCORD_TOKEN"),
		GuildID:         os.Getenv("GUILD_ID"),
		DatabasePath:    getEnv("DATABASE_PATH", "./data/bot.db"),
		RobloxTimeout:   getDuration("ROBLOX_TIMEOUT", 10*time.Second),
		RefreshInterval: getDuration("REFRESH_INTERVAL", 30*time.Minute),
		LogLevel:        getEnv("LOG_LEVEL", "info"),
		WebPort:         getEnv("WEB_PORT", "8080"),
		WebURL:          getEnv("WEB_URL", "http://localhost:8080"),
		DiscordClientID: os.Getenv("DISCORD_CLIENT_ID"),
		DiscordSecret:   os.Getenv("DISCORD_CLIENT_SECRET"),
		RobloxClientID:  os.Getenv("ROBLOX_CLIENT_ID"),
		RobloxSecret:    os.Getenv("ROBLOX_CLIENT_SECRET"),
		AdminDiscordIDs: parseIDSet(os.Getenv("ADMIN_DISCORD_IDS")),
	}

	if cfg.DiscordToken == "" {
		return nil, errors.New("DISCORD_TOKEN is required (copy .env.example to .env and set it)")
	}

	if cfg.DatabasePath != ":memory:" {
		dir := filepath.Dir(cfg.DatabasePath)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, err
		}
	}

	return cfg, nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getDuration(key string, fallback time.Duration) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return fallback
	}
	return d
}

// loadDotEnv reads a simple KEY=VALUE .env file (no quoting/expansion support).
func loadDotEnv() {
	path := ".env"
	b, err := os.ReadFile(path)
	if err != nil {
		return
	}
	for _, line := range strings.Split(string(b), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		val = strings.TrimSpace(val)
		if val != "" && os.Getenv(key) == "" {
			os.Setenv(key, val)
		}
	}
}

func ParseInt(s string) (int64, error) {
	return strconv.ParseInt(strings.TrimSpace(s), 10, 64)
}

// parseIDSet parses a comma-separated list of Discord IDs into a lookup set.
func parseIDSet(s string) map[string]bool {
	set := make(map[string]bool)
	for _, id := range strings.Split(s, ",") {
		id = strings.TrimSpace(id)
		if id != "" {
			set[id] = true
		}
	}
	return set
}
