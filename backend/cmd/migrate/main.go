// Command migrate applies the forward-only SQL migrations embedded in the
// binary. It is safe to run repeatedly and safe to run concurrently: migrations
// are serialised with a PostgreSQL advisory lock.
//
// Usage:
//
//	migrate            apply pending migrations
//	migrate -status    show applied and pending migrations
//	migrate -dry-run    show what would be applied
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"text/tabwriter"
	"time"

	"github.com/oyins/oddsvault/backend/internal/config"
	"github.com/oyins/oddsvault/backend/internal/database"
	"github.com/oyins/oddsvault/backend/internal/logging"
	"github.com/oyins/oddsvault/backend/migrations"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "migrate: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	showStatus := flag.Bool("status", false, "list applied and pending migrations")
	dryRun := flag.Bool("dry-run", false, "show pending migrations without applying them")
	flag.Parse()

	cfg, err := config.Load("oddsvault-migrate")
	if err != nil {
		return err
	}
	logger := logging.New(cfg.Env, cfg.LogLevel)
	slog.SetDefault(logger)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	migs, err := database.LoadMigrations(migrations.FS)
	if err != nil {
		return err
	}

	pool, err := database.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	connectCtx, cancel := context.WithTimeout(ctx, cfg.ConnectTimeout)
	defer cancel()
	if err := database.Ping(connectCtx, pool, cfg.ConnectTimeout); err != nil {
		return fmt.Errorf("database unreachable: %w", err)
	}

	applied, err := database.Applied(ctx, pool)
	if err != nil {
		return err
	}
	appliedSet := make(map[int64]database.AppliedMigration, len(applied))
	for _, m := range applied {
		appliedSet[m.Version] = m
	}

	if *showStatus {
		return printStatus(migs, appliedSet)
	}

	pending := 0
	for _, m := range migs {
		if _, ok := appliedSet[m.Version]; !ok {
			pending++
		}
	}
	if pending == 0 {
		fmt.Printf("database is up to date (%d migrations applied)\n", len(applied))
		return nil
	}
	if *dryRun {
		fmt.Printf("%d pending migration(s):\n", pending)
		for _, m := range migs {
			if _, ok := appliedSet[m.Version]; !ok {
				fmt.Printf("  %s\n", m.Filename)
			}
		}
		return nil
	}

	start := time.Now()
	appliedNow, err := database.Apply(ctx, pool, migs)
	if err != nil {
		return err
	}
	for _, m := range appliedNow {
		fmt.Printf("applied %s\n", m.Filename)
	}
	fmt.Printf("%d migration(s) applied in %s\n", len(appliedNow), time.Since(start).Round(time.Millisecond))
	return nil
}

func printStatus(migs []database.Migration, applied map[int64]database.AppliedMigration) error {
	w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "VERSION\tSTATUS\tNAME\tAPPLIED AT")
	for _, m := range migs {
		status := "pending"
		appliedAt := "-"
		if a, ok := applied[m.Version]; ok {
			status = "applied"
			appliedAt = a.AppliedAt.UTC().Format(time.RFC3339)
		}
		fmt.Fprintf(w, "%04d\t%s\t%s\t%s\n", m.Version, status, m.Name, appliedAt)
	}
	if err := w.Flush(); err != nil {
		return err
	}

	known := make(map[int64]struct{}, len(migs))
	for _, m := range migs {
		known[m.Version] = struct{}{}
	}
	for version, a := range applied {
		if _, ok := known[version]; !ok {
			fmt.Fprintf(os.Stderr,
				"warning: migration %04d (%s) is applied in the database but missing from this binary\n",
				version, a.Name)
		}
	}
	return nil
}
