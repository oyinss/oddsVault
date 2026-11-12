// Command sports-sync imports fixtures and results from public sports-data
// providers. Phase 1A covers football: OpenFootball for the schedule and
// football-data.co.uk for results.
//
// Run continuously:  sports-sync
// Run once and exit: sports-sync -once
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/oyins/oddsvault/backend/internal/config"
	"github.com/oyins/oddsvault/backend/internal/database"
	"github.com/oyins/oddsvault/backend/internal/logging"
	"github.com/oyins/oddsvault/backend/internal/providers"
	"github.com/oyins/oddsvault/backend/internal/store"
	"github.com/oyins/oddsvault/backend/internal/syncjob"
	"github.com/oyins/oddsvault/backend/internal/worker"
)

const syncInterval = 6 * time.Hour

func main() {
	once := flag.Bool("once", false, "run one sync and exit")
	flag.Parse()

	var err error
	if *once {
		err = runOnce()
	} else {
		err = worker.Boot("oddsvault-sports-sync", buildJobs)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "sports-sync: %v\n", err)
		os.Exit(1)
	}
}

func runOnce() error {
	cfg, err := config.Load("oddsvault-sports-sync")
	if err != nil {
		return err
	}
	logger := logging.New(cfg.Env, cfg.LogLevel)
	slog.SetDefault(logger)

	ctx, cancel := context.WithTimeout(context.Background(), cfg.JobTimeout)
	defer cancel()

	pool, err := database.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	if err := database.Ping(ctx, pool, cfg.ConnectTimeout); err != nil {
		return fmt.Errorf("database unreachable: %w", err)
	}

	football := syncjob.NewFootball(
		providers.NewClient(pool, logger, "OddsVault/0.1 (research)"),
		store.NewImporter(pool, logger),
		logger,
	)
	if err := football.Run(ctx); err != nil {
		return err
	}
	if err := football.RunCoverage(ctx); err != nil {
		return err
	}
	return football.RunCrests(ctx)
}

func buildJobs(_ context.Context, pool *pgxpool.Pool, log *slog.Logger, cfg config.Config) ([]worker.Job, error) {
	client := providers.NewClient(pool, log, "OddsVault/0.1 (research)")
	importer := store.NewImporter(pool, log)
	football := syncjob.NewFootball(client, importer, log)

	if cfg.RunOnStart {
		log.Info("worker runs jobs at startup", slog.String("reason", "JOB_RUN_ON_START"))
	}

	return []worker.Job{
		{
			Name:  "sync_football_fixtures",
			Every: syncInterval,
			Run:   football.Run,
		},
		{
			Name:  "sync_live_scores",
			Every: 2 * time.Minute,
			Run:   football.RunLive,
		},
		{
			Name:  "sync_espn_coverage",
			Every: 6 * time.Hour,
			Run:   football.RunCoverage,
		},
		{
			Name:  "sync_team_crests",
			Every: 24 * time.Hour,
			Run:   football.RunCrests,
		},
	}, nil
}
