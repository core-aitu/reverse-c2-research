package main

import (
	"context"
	"crypto/tls"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/research/specter-reimplementation/internal/auth"
	"github.com/research/specter-reimplementation/internal/config"
	"github.com/research/specter-reimplementation/internal/events"
	"github.com/research/specter-reimplementation/internal/httpapi"
	"github.com/research/specter-reimplementation/internal/objectstore"
	"github.com/research/specter-reimplementation/internal/store"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	cfg, err := config.Load()
	if err != nil {
		logger.Error("configuration rejected", "error", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	database, err := connect(ctx, cfg.DatabaseURL, 30*time.Second)
	if err != nil {
		logger.Error("database connection failed", "error", err)
		os.Exit(1)
	}
	defer database.Close()
	if err := database.Migrate(ctx, cfg.MigrationsDir); err != nil {
		logger.Error("migration failed", "error", err)
		os.Exit(1)
	}
	if cfg.S3Endpoint != "" {
		objects, err := objectstore.NewS3(ctx, objectstore.S3Config{
			Endpoint: cfg.S3Endpoint, AccessKey: cfg.S3AccessKey, SecretKey: cfg.S3SecretKey,
			Region: cfg.S3Region, Bucket: cfg.S3Bucket, Prefix: cfg.S3Prefix, AutoCreate: cfg.S3AutoCreate,
		})
		if err != nil {
			logger.Error("object storage connection failed", "error", err)
			os.Exit(1)
		}
		database.AttachObjectStore(objects)
	}
	if err := database.EnsureAdmin(ctx, cfg.AdminUsername, cfg.AdminPassword); err != nil {
		logger.Error("admin bootstrap failed", "error", err)
		os.Exit(1)
	}
	if err := database.SeedDocumentation(ctx); err != nil {
		logger.Error("documentation seed failed", "error", err)
		os.Exit(1)
	}

	authService := auth.New(database, cfg.JWTSecret, cfg.AccessTTL, cfg.RefreshTTL)
	hub := events.NewHub()
	server := &http.Server{
		Addr:              cfg.Addr,
		Handler:           httpapi.New(cfg, database, authService, hub, logger),
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	var agentServer *http.Server
	go func() {
		logger.Info("server listening", "address", cfg.Addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("server stopped unexpectedly", "error", err)
			stop()
		}
	}()
	if cfg.AgentTLSCert != "" {
		agentServer = &http.Server{
			Addr:              cfg.AgentAddr,
			Handler:           server.Handler,
			ReadHeaderTimeout: 5 * time.Second,
			IdleTimeout:       60 * time.Second,
			TLSConfig:         &tls.Config{MinVersion: tls.VersionTLS13},
		}
		go func() {
			logger.Info("agent TLS server listening", "address", cfg.AgentAddr)
			if err := agentServer.ListenAndServeTLS(cfg.AgentTLSCert, cfg.AgentTLSKey); err != nil && err != http.ErrServerClosed {
				logger.Error("agent TLS server stopped unexpectedly", "error", err)
				stop()
			}
		}()
	}

	<-ctx.Done()
	shutdown, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()
	if err := server.Shutdown(shutdown); err != nil {
		logger.Error("graceful shutdown failed", "error", err)
	}
	if agentServer != nil {
		if err := agentServer.Shutdown(shutdown); err != nil {
			logger.Error("agent TLS graceful shutdown failed", "error", err)
		}
	}
}

func connect(ctx context.Context, databaseURL string, timeout time.Duration) (*store.Store, error) {
	deadline := time.Now().Add(timeout)
	var lastErr error
	for time.Now().Before(deadline) {
		database, err := store.Open(ctx, databaseURL)
		if err == nil {
			return database, nil
		}
		lastErr = err
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(time.Second):
		}
	}
	return nil, lastErr
}
