// Command settlement-worker evaluates market selections against verified
// results. Settlement rules are deterministic and versioned.
//
// Run continuously:  settlement-worker
// Run once and exit: settlement-worker -once
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
	"github.com/oyins/oddsvault/backend/internal/settlement"
	"github.com/oyins/oddsvault/backend/internal/worker"
)

func main() {
	once := flag.Bool("once", false, "run one settlement pass and exit")
	flag.Parse()

	var err error
	if *once {
		err = runOnce()
	} else {
		err = worker.Boot("oddsvault-settlement-worker", buildJobs)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "settlement-worker: %v\n", err)
		os.Exit(1)
	}
}

func runOnce() error {
	cfg, err := config.Load("oddsvault-settlement-worker")
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

	_, err = settlement.New(pool, logger).Run(ctx)
	return err
}

func buildJobs(_ context.Context, pool *pgxpool.Pool, log *slog.Logger, _ config.Config) ([]worker.Job, error) {
	engine := settlement.New(pool, log)
	return []worker.Job{
		{
			Name:  "settle_finished_events",
			Every: 10 * time.Minute,
			Run: func(ctx context.Context) error {
				_, err := engine.Run(ctx)
				return err
			},
		},
	}, nil
}
