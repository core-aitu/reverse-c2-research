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
	Addr                  string
	AgentAddr             string
	AgentTLSCert          string
	AgentTLSKey           string
	AgentPublicURL        string
	AgentCABundle         string
	DatabaseURL           string
	JWTSecret             string
	AccessTTL             time.Duration
	RefreshTTL            time.Duration
	AdminUsername         string
	AdminPassword         string
	MigrationsDir         string
	AllowedOrigin         string
	SecureCookies         bool
	FixturesEnabled       bool
	RealAdapters          bool
	WorkerPath            string
	TransferDir           string
	S3Endpoint            string
	S3AccessKey           string
	S3SecretKey           string
	S3Region              string
	S3Bucket              string
	S3Prefix              string
	S3AutoCreate          bool
	RequireSignedReleases bool
	ShutdownTimeout       time.Duration
}

func Load() (Config, error) {
	cfg := Config{
		Addr:                  env("SPECTER_ADDR", ":8080"),
		AgentAddr:             env("SPECTER_AGENT_ADDR", ":8443"),
		AgentTLSCert:          os.Getenv("SPECTER_AGENT_TLS_CERT"),
		AgentTLSKey:           os.Getenv("SPECTER_AGENT_TLS_KEY"),
		AgentPublicURL:        os.Getenv("SPECTER_AGENT_PUBLIC_URL"),
		AgentCABundle:         os.Getenv("SPECTER_AGENT_CA_BUNDLE"),
		DatabaseURL:           env("SPECTER_DATABASE_URL", "postgres://specter:specter@localhost:5432/specter?sslmode=disable"),
		JWTSecret:             os.Getenv("SPECTER_JWT_SECRET"),
		AccessTTL:             duration("SPECTER_ACCESS_TTL", 15*time.Minute),
		RefreshTTL:            duration("SPECTER_REFRESH_TTL", 7*24*time.Hour),
		AdminUsername:         env("SPECTER_ADMIN_USERNAME", "admin"),
		AdminPassword:         os.Getenv("SPECTER_ADMIN_PASSWORD"),
		MigrationsDir:         env("SPECTER_MIGRATIONS_DIR", "migrations"),
		AllowedOrigin:         env("SPECTER_ALLOWED_ORIGIN", "http://localhost:3000"),
		SecureCookies:         boolean("SPECTER_SECURE_COOKIES", false),
		FixturesEnabled:       boolean("SPECTER_ENABLE_FIXTURES", false),
		RealAdapters:          boolean("SPECTER_ENABLE_REAL_ADAPTERS", false),
		WorkerPath:            env("SPECTER_WORKER_PATH", "/app/specter-worker"),
		TransferDir:           env("SPECTER_TRANSFER_DIR", filepath.Join(os.TempDir(), "specter-transfers")),
		S3Endpoint:            os.Getenv("SPECTER_S3_ENDPOINT"),
		S3AccessKey:           os.Getenv("SPECTER_S3_ACCESS_KEY"),
		S3SecretKey:           os.Getenv("SPECTER_S3_SECRET_KEY"),
		S3Region:              env("SPECTER_S3_REGION", "us-east-1"),
		S3Bucket:              env("SPECTER_S3_BUCKET", "specter-objects"),
		S3Prefix:              strings.Trim(os.Getenv("SPECTER_S3_PREFIX"), "/"),
		S3AutoCreate:          boolean("SPECTER_S3_AUTO_CREATE", false),
		RequireSignedReleases: boolean("SPECTER_REQUIRE_SIGNED_RELEASES", false),
		ShutdownTimeout:       duration("SPECTER_SHUTDOWN_TIMEOUT", 10*time.Second),
	}
	if len(cfg.JWTSecret) < 32 {
		return Config{}, errors.New("SPECTER_JWT_SECRET must contain at least 32 characters")
	}
	if (cfg.AgentTLSCert == "") != (cfg.AgentTLSKey == "") {
		return Config{}, errors.New("SPECTER_AGENT_TLS_CERT and SPECTER_AGENT_TLS_KEY must be configured together")
	}
	if cfg.S3Endpoint != "" && (cfg.S3AccessKey == "" || cfg.S3SecretKey == "" || cfg.S3Bucket == "") {
		return Config{}, errors.New("S3 endpoint requires access key, secret key and bucket")
	}
	return cfg, nil
}

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func duration(key string, fallback time.Duration) time.Duration {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := time.ParseDuration(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func boolean(key string, fallback bool) bool {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fallback
	}
	return parsed
}
