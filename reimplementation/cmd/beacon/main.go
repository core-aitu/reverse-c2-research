package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/research/specter-reimplementation/internal/beacon"
)

var buildVersion = "dev"

func main() {
	configPath := flag.String("config", os.Getenv("SPECTER_BEACON_CONFIG"), "one-use JSON launch configuration")
	endpoint := flag.String("endpoint", env("SPECTER_BEACON_ENDPOINT", "https://127.0.0.1:8443"), "TLS gateway base URL")
	token := flag.String("enrollment-token", os.Getenv("SPECTER_BEACON_ENROLLMENT_TOKEN"), "single-use enrollment token")
	agentID := flag.String("agent-id", os.Getenv("SPECTER_BEACON_AGENT_ID"), "requested agent ID")
	name := flag.String("name", env("SPECTER_BEACON_NAME", "safe-agent"), "display name")
	state := flag.String("state", env("SPECTER_BEACON_STATE", "beacon-state.json"), "persistent state file")
	caBundle := flag.String("ca-bundle", os.Getenv("SPECTER_BEACON_CA_BUNDLE"), "optional PEM CA bundle")
	interval := flag.Duration("interval", envDuration("SPECTER_BEACON_INTERVAL", 5*time.Second), "check-in interval")
	jitter := flag.Duration("jitter", 500*time.Millisecond, "maximum bounded interval/backoff jitter")
	minBackoff := flag.Duration("min-backoff", 500*time.Millisecond, "initial transient-error backoff")
	maxBackoff := flag.Duration("max-backoff", time.Minute, "maximum transient-error backoff")
	connectTimeout := flag.Duration("connect-timeout", 5*time.Second, "DNS/connect/TLS timeout")
	requestTimeout := flag.Duration("request-timeout", 15*time.Second, "complete request timeout")
	rotationInterval := flag.Duration("identity-rotation", 24*time.Hour, "agent identity rotation interval")
	batch := flag.Uint("batch", 8, "bounded task batch size")
	parallel := flag.Int("parallel", 1, "bounded safe-worker parallelism")
	allowLoopback := flag.Bool("allow-insecure-loopback", false, "allow HTTP only for an explicit loopback laboratory")
	flag.Parse()
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	visited := map[string]bool{}
	flag.CommandLine.Visit(func(value *flag.Flag) { visited[value.Name] = true })
	if *configPath != "" {
		launch, err := beacon.LoadLaunchConfig(*configPath)
		if err != nil {
			logger.Error("launch configuration rejected", "error", err)
			os.Exit(2)
		}
		applyString(endpoint, launch.Endpoint, "endpoint", "SPECTER_BEACON_ENDPOINT", visited)
		applyString(token, launch.EnrollmentToken, "enrollment-token", "SPECTER_BEACON_ENROLLMENT_TOKEN", visited)
		applyString(agentID, launch.AgentID, "agent-id", "SPECTER_BEACON_AGENT_ID", visited)
		applyString(name, launch.Name, "name", "SPECTER_BEACON_NAME", visited)
		applyString(state, launch.StatePath, "state", "SPECTER_BEACON_STATE", visited)
		applyString(caBundle, launch.CABundle, "ca-bundle", "SPECTER_BEACON_CA_BUNDLE", visited)
		if launch.Interval != "" && !visited["interval"] && os.Getenv("SPECTER_BEACON_INTERVAL") == "" {
			*interval, _ = time.ParseDuration(launch.Interval)
		}
	}
	client, err := beacon.New(beacon.Config{Endpoint: *endpoint, EnrollmentToken: *token, AgentID: *agentID, Name: *name, StatePath: *state, BuildVersion: buildVersion, CABundlePath: *caBundle, Interval: *interval, Jitter: *jitter, MinBackoff: *minBackoff, MaxBackoff: *maxBackoff, ConnectTimeout: *connectTimeout, RequestTimeout: *requestTimeout, IdentityRotationInterval: *rotationInterval, BatchSize: uint32(*batch), Parallelism: *parallel, AllowInsecureLoopback: *allowLoopback})
	if err != nil {
		logger.Error("configuration rejected", "error", err)
		os.Exit(2)
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	if err := client.Run(ctx); err != nil && err != context.Canceled {
		logger.Error("beacon stopped", "error", err)
		os.Exit(1)
	}
	logger.Info("beacon stopped gracefully")
}

func applyString(target *string, value, flagName, environment string, visited map[string]bool) {
	if value != "" && !visited[flagName] && os.Getenv(environment) == "" {
		*target = value
	}
}

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func envDuration(key string, fallback time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		if parsed, err := time.ParseDuration(value); err == nil {
			return parsed
		}
	}
	return fallback
}
