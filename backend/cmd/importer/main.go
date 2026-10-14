package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/slatt-oo/beijing-geo-map/backend/internal/config"
	"github.com/slatt-oo/beijing-geo-map/backend/internal/importer"
	"github.com/slatt-oo/beijing-geo-map/backend/migrations"
)

// 正常快照约有 4100 个 POI；少于这个数量说明快照损坏，继续导入会把 poi 表同步删空。
const minSnapshotPOIs = 1000

func main() {
	if err := run(); err != nil {
		slog.Error("import failed", "err", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: cfg.LogLevel})))

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := migrations.Up(cfg.DatabaseURL); err != nil {
		return err
	}
	slog.Info("migrations applied")

	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("create database pool: %w", err)
	}
	defer pool.Close()

	start := time.Now()
	stats, err := importer.Run(ctx, pool, importer.Options{DataDir: cfg.DataDir, MinPOIs: minSnapshotPOIs})
	if err != nil {
		return err
	}
	slog.Info("import finished",
		"districts", stats.Districts,
		"pois", stats.POIs,
		"pois_with_district", stats.POIsWithDistrict,
		"deleted_pois", stats.DeletedPOIs,
		"pois_by_category", stats.POIsByCategory,
		"elapsed_ms", time.Since(start).Milliseconds(),
	)
	return nil
}
