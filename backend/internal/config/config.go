// Package config loads process configuration from the environment.
//
// Every binary in backend/cmd reads configuration through this package so that
// there is exactly one definition of what an environment variable means.
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Environment values.
const (
	EnvDevelopment = "development"
	EnvTest        = "test"
	EnvProduction  = "production"
)

// Config is the fully resolved process configuration.
type Config struct {
	// Env is one of EnvDevelopment, EnvTest, EnvProduction.
	Env string
	// ServiceName identifies this process in logs and metrics.
	ServiceName string
	// HTTPAddr is the listen address for cmd/api, e.g. ":8080".
	HTTPAddr string
	// DatabaseURL is the PostgreSQL/TimescaleDB connection string.
	DatabaseURL string
	// JournalToken, when set, is required by GET /api/v1/my-bets. The public
	// site injects it at the Pages proxy; a direct hit on the tunnel without
	// it is refused. Empty disables the check for local development.
	JournalToken string
	// LogLevel is debug, info, warn or error.
	LogLevel string
	// ShutdownTimeout bounds graceful shutdown.
	ShutdownTimeout time.Duration
	// ConnectTimeout bounds the initial database ping.
	ConnectTimeout time.Duration
	// ReadTimeout / WriteTimeout / IdleTimeout are HTTP server timeouts.
	ReadTimeout    time.Duration
	WriteTimeout   time.Duration
	IdleTimeout    time.Duration
	RequestTimeout time.Duration
	// CORSAllowedOrigins lists browser origins allowed to read the API
	// directly. Production serves the UI behind the same-origin Pages proxy,
	// so this stays empty there; local `next dev` sets it to :3000.
	CORSAllowedOrigins []string
	// JobInterval is the default tick interval for the worker binaries.
	JobInterval time.Duration
	// JobTimeout bounds a single job execution.
	JobTimeout time.Duration
	// RunOnStart executes every worker job once at boot.
	RunOnStart bool
}

// Load reads and validates configuration for the named service.
func Load(serviceName string) (Config, error) {
	cfg := Config{
		Env:                strings.ToLower(env("APP_ENV", EnvDevelopment)),
		ServiceName:        serviceName,
		HTTPAddr:           env("HTTP_ADDR", ":8080"),
		DatabaseURL:        os.Getenv("DATABASE_URL"),
		JournalToken:       os.Getenv("JOURNAL_TOKEN"),
		LogLevel:           strings.ToLower(env("LOG_LEVEL", "info")),
		ShutdownTimeout:    duration("SHUTDOWN_TIMEOUT", 20*time.Second),
		ConnectTimeout:     duration("DB_CONNECT_TIMEOUT", 10*time.Second),
		ReadTimeout:        duration("HTTP_READ_TIMEOUT", 15*time.Second),
		WriteTimeout:       duration("HTTP_WRITE_TIMEOUT", 30*time.Second),
		IdleTimeout:        duration("HTTP_IDLE_TIMEOUT", 60*time.Second),
		RequestTimeout:     duration("HTTP_REQUEST_TIMEOUT", 30*time.Second),
		CORSAllowedOrigins: csv("CORS_ALLOWED_ORIGINS"),
		JobInterval:        duration("JOB_INTERVAL", 60*time.Second),
		JobTimeout:         duration("JOB_TIMEOUT", 5*time.Minute),
		RunOnStart:         boolean("JOB_RUN_ON_START", false),
	}

	if cfg.DatabaseURL == "" {
		return Config{}, fmt.Errorf("DATABASE_URL is required (see .env.example)")
	}
	switch cfg.Env {
	case EnvDevelopment, EnvTest, EnvProduction:
	default:
		return Config{}, fmt.Errorf("APP_ENV must be development, test or production (got %q)", cfg.Env)
	}
	switch cfg.LogLevel {
	case "debug", "info", "warn", "error":
	default:
		return Config{}, fmt.Errorf("LOG_LEVEL must be debug, info, warn or error (got %q)", cfg.LogLevel)
	}
	if cfg.JobInterval < time.Second {
		return Config{}, fmt.Errorf("JOB_INTERVAL must be at least 1s")
	}
	if cfg.JobTimeout <= 0 {
		return Config{}, fmt.Errorf("JOB_TIMEOUT must be positive")
	}
	return cfg, nil
}

// IsProduction reports whether the process runs against production settings.
func (c Config) IsProduction() bool { return c.Env == EnvProduction }

func env(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && strings.TrimSpace(v) != "" {
		return strings.TrimSpace(v)
	}
	return fallback
}

// csv splits a comma-separated environment value, trimming blanks. An empty
// value yields nil so callers can treat "not configured" as "allow nothing".
func csv(key string) []string {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return nil
	}
	values := make([]string, 0, strings.Count(raw, ",")+1)
	for _, part := range strings.Split(raw, ",") {
		if value := strings.TrimSpace(part); value != "" {
			values = append(values, value)
		}
	}
	return values
}

func boolean(key string, fallback bool) bool {
	raw, ok := os.LookupEnv(key)
	if !ok {
		return fallback
	}
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return fallback
	}
}

func duration(key string, fallback time.Duration) time.Duration {
	raw, ok := os.LookupEnv(key)
	if !ok || strings.TrimSpace(raw) == "" {
		return fallback
	}
	raw = strings.TrimSpace(raw)
	if d, err := time.ParseDuration(raw); err == nil {
		return d
	}
	// Bare numbers are seconds, which matches how container envs are usually written.
	if secs, err := strconv.Atoi(raw); err == nil {
		return time.Duration(secs) * time.Second
	}
	return fallback
}
