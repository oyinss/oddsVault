// Command bookmaker-collector captures live bookmaker odds.
//
// Phase 2 uses LiveScoreBet's public gateway, which returns markets and decimal
// odds server-side. SportyBet blocks server requests and stays on the capture
// path until a browser importer exists.
//
// Run continuously:  bookmaker-collector
// Run once and exit: bookmaker-collector -once
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

const captureInterval = 5 * time.Minute

func main() {
	once := flag.Bool("once", false, "run one capture and exit")
	flag.Parse()

	var err error
	if *once {
		err = runOnce()
	} else {
		err = worker.Boot("oddsvault-bookmaker-collector", buildJobs)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "bookmaker-collector: %v\n", err)
		os.Exit(1)
	}
}

func runOnce() error {
	cfg, err := config.Load("oddsvault-bookmaker-collector")
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

	odds := syncjob.NewOdds(
		providers.NewClient(pool, logger, "OddsVault/0.1 (research)"),
		store.NewImporter(pool, logger),
		logger,
	)
	return odds.Run(ctx)
}

func buildJobs(_ context.Context, pool *pgxpool.Pool, log *slog.Logger, _ config.Config) ([]worker.Job, error) {
	odds := syncjob.NewOdds(
		providers.NewClient(pool, log, "OddsVault/0.1 (research)"),
		store.NewImporter(pool, log),
		log,
	)
	return []worker.Job{
		{
			Name:  "capture_bookmaker_odds",
			Every: captureInterval,
			Run:   odds.Run,
		},
	}, nil
}
