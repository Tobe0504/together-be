package config

import (
	"flag"
	"os"
	"strings"
)

type Config struct {
	Mode             string
	Bind             string
	Port             string
	DBPath           string
	DatabaseURL      string
	JWTSecret        string
	LiveKitURL       string
	LiveKitAPIKey    string
	LiveKitAPISecret string
	CORSOrigins      []string
	LANAddr          string
	StaticDir        string
}

func Load() Config {
	mode := flag.String("mode", envOr("TOGETHER_MODE", "online"), "online|local — bind-address convenience only")
	bind := flag.String("bind", "", "override bind address (defaults: 127.0.0.1 for online, 0.0.0.0 for local)")
	port := flag.String("port", envOr("PORT", "8080"), "HTTP port")
	dbPath := flag.String("db", envOr("TOGETHER_DB_PATH", "together.db"), "path to sqlite database file")
	databaseURL := flag.String("database-url", envOr("DATABASE_URL", ""), "libsql:// URL for a remote Turso database (overrides --db)")
	lanAddr := flag.String("lan-addr", envOr("TOGETHER_LAN_ADDR", ""), "host:port guests reach this server at on the LAN (containers must set this)")
	staticDir := flag.String("static", envOr("TOGETHER_STATIC_DIR", ""), "path to built frontend static assets (serves them for local mode)")
	flag.Parse()

	b := *bind
	if b == "" {
		if *mode == "local" {
			b = "0.0.0.0"
		} else {
			b = "127.0.0.1"
		}
	}

	return Config{
		Mode:             *mode,
		Bind:             b,
		Port:             *port,
		DBPath:           *dbPath,
		DatabaseURL:      *databaseURL,
		JWTSecret:        envOr("JWT_SECRET", "dev-insecure-secret-change-me"),
		LiveKitURL:       envOr("LIVEKIT_URL", "ws://127.0.0.1:7880"),
		LiveKitAPIKey:    envOr("LIVEKIT_API_KEY", "devkey"),
		LiveKitAPISecret: envOr("LIVEKIT_API_SECRET", "devsecret1234567890devsecret"),
		CORSOrigins:      splitAndTrim(envOr("CORS_ORIGIN", "http://localhost:3000")),
		LANAddr:          *lanAddr,
		StaticDir:        *staticDir,
	}
}

func splitAndTrim(csv string) []string {
	parts := strings.Split(csv, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if v := strings.TrimSpace(p); v != "" {
			out = append(out, v)
		}
	}
	return out
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func (c Config) DBTarget() string {
	if c.DatabaseURL != "" {
		return c.DatabaseURL
	}
	return c.DBPath
}
