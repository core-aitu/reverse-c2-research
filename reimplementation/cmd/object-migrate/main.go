package main

import (
	"context"
	"log/slog"
	"os"

	"github.com/research/specter-reimplementation/internal/config"
	"github.com/research/specter-reimplementation/internal/objectstore"
	"github.com/research/specter-reimplementation/internal/store"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	cfg, err := config.Load()
	if err != nil {
		logger.Error("configuration rejected", "error", err)
		os.Exit(2)
	}
	if cfg.S3Endpoint == "" {
		logger.Error("object storage is not configured")
		os.Exit(2)
	}
	ctx := context.Background()
	database, err := store.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		logger.Error("database connection failed", "error", err)
		os.Exit(1)
	}
	defer database.Close()
	if err = database.Migrate(ctx, cfg.MigrationsDir); err != nil {
		logger.Error("migration failed", "error", err)
		os.Exit(1)
	}
	objects, err := objectstore.NewS3(ctx, objectstore.S3Config{
		Endpoint: cfg.S3Endpoint, AccessKey: cfg.S3AccessKey, SecretKey: cfg.S3SecretKey,
		Region: cfg.S3Region, Bucket: cfg.S3Bucket, Prefix: cfg.S3Prefix, AutoCreate: cfg.S3AutoCreate,
	})
	if err != nil {
		logger.Error("object storage connection failed", "error", err)
		os.Exit(1)
	}
	database.AttachObjectStore(objects)
	total := 0
	for {
		count, err := database.MigrateLegacyObjects(ctx, 100)
		if err != nil {
			logger.Error("object migration failed", "error", err, "migrated", total)
			os.Exit(1)
		}
		total += count
		if count == 0 {
			break
		}
	}
	logger.Info("object migration complete", "migrated", total)
}
