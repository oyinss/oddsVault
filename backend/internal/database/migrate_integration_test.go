package database

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// TestApplyIntegration exercises the real runner against a disposable
// PostgreSQL/TimescaleDB database. It is skipped unless TEST_DATABASE_URL is
// set and never touches public tables: everything happens in a throwaway schema.
func TestApplyIntegration(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping database integration test")
	}

	ctx := context.Background()
	admin, err := Open(ctx, url)
	if err != nil {
		t.Fatalf("open admin pool: %v", err)
	}
	defer admin.Close()

	schema := fmt.Sprintf("mig_test_%d", time.Now().UnixNano())
	if _, err := admin.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatalf("create schema: %v", err)
	}
	t.Cleanup(func() {
		cleanup, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_, _ = admin.Exec(cleanup, "DROP SCHEMA "+schema+" CASCADE")
	})

	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		t.Fatalf("parse config: %v", err)
	}
	cfg.ConnConfig.RuntimeParams["search_path"] = schema
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatalf("open pool: %v", err)
	}
	defer pool.Close()

	fsys := fstest.MapFS{
		"0001_first.sql":  {Data: []byte("CREATE TABLE widgets (id bigint PRIMARY KEY);")},
		"0002_second.sql": {Data: []byte("ALTER TABLE widgets ADD COLUMN name text;")},
	}
	migs, err := LoadMigrations(fsys)
	if err != nil {
		t.Fatalf("load migrations: %v", err)
	}

	applied, err := Apply(ctx, pool, migs)
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if len(applied) != 2 {
		t.Fatalf("expected 2 applied migrations, got %d", len(applied))
	}

	// Re-running must be a no-op and must not error.
	again, err := Apply(ctx, pool, migs)
	if err != nil {
		t.Fatalf("second apply: %v", err)
	}
	if len(again) != 0 {
		t.Fatalf("expected no migrations on the second run, got %d", len(again))
	}

	// The migrated DDL really ran.
	var hasName bool
	if err := pool.QueryRow(ctx,
		`SELECT EXISTS (
			SELECT 1 FROM information_schema.columns
			WHERE table_schema = $1 AND table_name = 'widgets' AND column_name = 'name'
		)`, schema).Scan(&hasName); err != nil {
		t.Fatalf("inspect widgets: %v", err)
	}
	if !hasName {
		t.Fatal("expected widgets.name to exist")
	}

	// Editing an applied migration must be rejected.
	tampered := make([]Migration, len(migs))
	copy(tampered, migs)
	tampered[0].Checksum = strings.Repeat("0", 64)
	if _, err := Apply(ctx, pool, tampered); err == nil {
		t.Fatal("expected a checksum failure for an edited migration")
	}

	records, err := Applied(ctx, pool)
	if err != nil {
		t.Fatalf("list applied: %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("expected 2 recorded migrations, got %d", len(records))
	}
}
