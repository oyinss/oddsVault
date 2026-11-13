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
	live := flag.Bool("live", false, "poll live scores once and exit")
	fixtures := flag.Bool("fixtures", false, "rebuild multi-sport fixtures once and exit")
	corners := flag.Bool("corners", false, "refresh live corner counts once and exit")
	allLive := flag.Bool("alllive", false, "import every live soccer fixture once and exit")
	fotmob := flag.Bool("fotmob", false, "refresh live corners from FotMob once and exit")
	onexbetFlag := flag.Bool("onexbet", false, "import 1xBet live football once and exit")
	logos := flag.Bool("logos", false, "resolve missing team badges once and exit")
	flag.Parse()

	var err error
	switch {
	case *logos:
		err = runLogosOnce()
	case *onexbetFlag:
		err = runOnexbetOnce()
	case *fotmob:
		err = runFotmobOnce()
	case *allLive:
		err = runAllLiveOnce()
	case *corners:
		err = runLiveCornersOnce()
	case *fixtures:
		err = runMultiSportOnce()
	case *live:
		err = runLiveOnce()
	case *once:
		err = runOnce()
	default:
		err = worker.Boot("oddsvault-sports-sync", buildJobs)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "sports-sync: %v\n", err)
		os.Exit(1)
	}
}

// runLiveOnce refreshes live state only. Useful after adding a competition, and
// as a manual check that a provider is still answering.
func runLiveOnce() error {
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
	return football.RunLive(ctx)
}

// runMultiSportOnce rebuilds the non-football fixture list only.
func runMultiSportOnce() error {
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
	return football.RunMultiSportFixtures(ctx)
}

// runLogosOnce resolves missing badges for teams playing right now.
func runLogosOnce() error {
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
	return football.RunMissingLogos(ctx)
}

// runOnexbetOnce imports the live football feed from 1xBet once.
func runOnexbetOnce() error {
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
	return football.RunOnexbetLive(ctx)
}

// runFotmobOnce refreshes live corner counts from FotMob only.
func runFotmobOnce() error {
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
	return football.RunFotmobLive(ctx)
}

// runAllLiveOnce imports the live fixtures found on ESPN's all-soccer board.
func runAllLiveOnce() error {
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
	return football.RunAllLive(ctx)
}

// runLiveCornersOnce refreshes corner counts for matches in progress only.
func runLiveCornersOnce() error {
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
	return football.RunLiveCorners(ctx)
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
	if err := football.RunCrests(ctx); err != nil {
		return err
	}
	return football.RunStats(ctx)
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
			Name:  "sync_multi_sport_fixtures",
			Every: syncInterval,
			Run:   football.RunMultiSportFixtures,
		},
		{
			Name:  "sync_all_live_soccer",
			Every: 2 * time.Minute,
			Run:   football.RunAllLive,
		},
		{
			Name:  "sync_missing_logos",
			Every: 10 * time.Minute,
			Run:   football.RunMissingLogos,
		},
		{
			Name:  "sync_onexbet_live",
			Every: 3 * time.Minute,
			Run:   football.RunOnexbetLive,
		},
		{
			Name:  "sync_fotmob_corners",
			Every: 2 * time.Minute,
			Run:   football.RunFotmobLive,
		},
		{
			Name:  "sync_live_corners",
			Every: 90 * time.Second,
			Run:   football.RunLiveCorners,
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
		{
			Name:  "sync_match_stats",
			Every: 30 * time.Minute,
			Run:   football.RunStats,
		},
	}, nil
}
