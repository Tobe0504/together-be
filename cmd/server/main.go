package main

import (
	"context"
	"log"
	"net/http"

	"github.com/tobenna/together/server/internal/api"
	"github.com/tobenna/together/server/internal/config"
	"github.com/tobenna/together/server/internal/db"
)

func main() {
	cfg := config.Load()

	ctx := context.Background()
	sqlDB, err := db.Open(ctx, cfg.DBTarget())
	if err != nil {
		log.Fatalf("open database: %v", err)
	}
	defer sqlDB.Close()

	store := db.NewStore(sqlDB)
	server := api.NewServer(cfg, store)

	addr := cfg.Bind + ":" + cfg.Port
	dbLabel := cfg.DBPath
	if cfg.DatabaseURL != "" {
		dbLabel = "remote libsql"
	}
	log.Printf("together server listening on %s (mode=%s, db=%s)", addr, cfg.Mode, dbLabel)
	if cfg.Mode == "local" && cfg.Bind == "0.0.0.0" {
		log.Printf("local mode: bound to all interfaces — guests on the same network can reach this server directly, no internet required")
	}
	if err := http.ListenAndServe(addr, server.Router()); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
