package config

import (
	"errors"
	"os"
	"strconv"
	"time"
)

type Config struct {
	Addr            string
	DatabaseURL     string
	JWTSecret       string
	AccessTTL       time.Duration
	RefreshTTL      time.Duration
	AdminUsername   string
	AdminPassword   string
	MigrationsDir   string
	AllowedOrigin   string
	SecureCookies   bool
	FixturesEnabled bool
	RealAdapters    bool
	ShutdownTimeout time.Duration
}

func Load() (Config, error) {
	cfg := Config{
		Addr:            env("SPECTER_ADDR", ":8080"),
		DatabaseURL:     env("SPECTER_DATABASE_URL", "postgres://specter:specter@localhost:5432/specter?sslmode=disable"),
		JWTSecret:       os.Getenv("SPECTER_JWT_SECRET"),
		AccessTTL:       duration("SPECTER_ACCESS_TTL", 15*time.Minute),
		RefreshTTL:      duration("SPECTER_REFRESH_TTL", 7*24*time.Hour),
		AdminUsername:   env("SPECTER_ADMIN_USERNAME", "admin"),
		AdminPassword:   os.Getenv("SPECTER_ADMIN_PASSWORD"),
		MigrationsDir:   env("SPECTER_MIGRATIONS_DIR", "migrations"),
		AllowedOrigin:   env("SPECTER_ALLOWED_ORIGIN", "http://localhost:3000"),
		SecureCookies:   boolean("SPECTER_SECURE_COOKIES", false),
		FixturesEnabled: boolean("SPECTER_ENABLE_FIXTURES", false),
		RealAdapters:    boolean("SPECTER_ENABLE_REAL_ADAPTERS", false),
		ShutdownTimeout: duration("SPECTER_SHUTDOWN_TIMEOUT", 10*time.Second),
	}
	if len(cfg.JWTSecret) < 32 {
		return Config{}, errors.New("SPECTER_JWT_SECRET must contain at least 32 characters")
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
