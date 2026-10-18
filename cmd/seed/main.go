package main

import (
	"flag"
	"fmt"
	"log/slog"
	"os"

	"agent-desk/internal/bootstrap"
	"agent-desk/internal/pkg/config"
	"agent-desk/internal/pkg/logx"

	"github.com/mlogclub/simple/sqls"
)

func main() {
	configPath := flag.String("config", "config/config.yaml", "path to config file")
	reset := flag.Bool("reset", false, "force reset and reseed 138 CS sessions")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		slog.Error("load config failed", "error", err)
		os.Exit(1)
	}

	logx.Init(logx.Config{
		Level:     cfg.Logger.Level,
		Format:    cfg.Logger.Format,
		AddSource: cfg.Logger.AddSource,
	})

	db, err := bootstrap.InitDB(cfg.DB)
	if err != nil {
		slog.Error("init db failed", "error", err)
		os.Exit(1)
	}
	sqls.SetDB(db)

	if err := bootstrap.InitMigrations(); err != nil {
		slog.Error("init migrations failed", "error", err)
		os.Exit(1)
	}

	count, err := bootstrap.SeedCSSessions(*reset)
	if err != nil {
		slog.Error("seed failed", "error", err)
		os.Exit(1)
	}

	fmt.Printf("Successfully seeded %d CS sessions and related risk events.\n", count)
}
