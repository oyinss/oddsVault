package config

import (
	"strings"
	"testing"
	"time"
)

func TestLoadDefaults(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://localhost/oddsvault")
	t.Setenv("APP_ENV", "development")
	t.Setenv("LOG_LEVEL", "debug")

	cfg, err := Load("test-service")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.HTTPAddr != ":8080" {
		t.Fatalf("expected default addr :8080, got %q", cfg.HTTPAddr)
	}
	if cfg.JobInterval != 60*time.Second {
		t.Fatalf("expected 60s job interval, got %s", cfg.JobInterval)
	}
	if cfg.ServiceName != "test-service" {
		t.Fatalf("unexpected service name %q", cfg.ServiceName)
	}
}

func TestLoadRequiresDatabaseURL(t *testing.T) {
	t.Setenv("DATABASE_URL", "")
	if _, err := Load("test-service"); err == nil {
		t.Fatal("expected an error when DATABASE_URL is empty")
	}
}

func TestLoadRejectsUnknownValues(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://localhost/oddsvault")

	t.Setenv("APP_ENV", "staging")
	if _, err := Load("test-service"); err == nil {
		t.Fatal("expected an error for an unknown APP_ENV")
	}

	t.Setenv("APP_ENV", "development")
	t.Setenv("LOG_LEVEL", "verbose")
	if _, err := Load("test-service"); err == nil {
		t.Fatal("expected an error for an unknown LOG_LEVEL")
	}
}

func TestDurationAcceptsBareSeconds(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://localhost/oddsvault")
	t.Setenv("JOB_INTERVAL", "15")

	cfg, err := Load("test-service")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.JobInterval != 15*time.Second {
		t.Fatalf("expected 15s, got %s", cfg.JobInterval)
	}
	if !strings.Contains(cfg.DatabaseURL, "oddsvault") {
		t.Fatalf("database URL not preserved: %q", cfg.DatabaseURL)
	}
}
