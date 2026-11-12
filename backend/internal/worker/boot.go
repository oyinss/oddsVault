package worker

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/oyins/oddsvault/backend/internal/config"
	"github.com/oyins/oddsvault/backend/internal/database"
	"github.com/oyins/oddsvault/backend/internal/logging"
)

// JobBuilder builds the jobs for one worker process. It may be nil, which
// means the binary boots and idles (used by binaries whose phase has not
// arrived yet).
type JobBuilder func(ctx context.Context, pool *pgxpool.Pool, logger *slog.Logger, cfg config.Config) ([]Job, error)

// Boot is the shared entrypoint for the background binaries.
func Boot(serviceName string, build JobBuilder) error {
	cfg, err := config.Load(serviceName)
	if err != nil {
		return err
	}

	logger := logging.New(cfg.Env, cfg.LogLevel)
	slog.SetDefault(logger)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := database.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	if err := database.Ping(ctx, pool, cfg.ConnectTimeout); err != nil {
		logger.Warn("database not reachable at startup",
			slog.String("service", serviceName),
			slog.String("error", err.Error()))
	}

	var jobs []Job
	if build != nil {
		jobs, err = build(ctx, pool, logger, cfg)
		if err != nil {
			return err
		}
	}

	return Run(ctx, Options{
		Name:        serviceName,
		Logger:      logger,
		Jobs:        jobs,
		JobTimeout:  cfg.JobTimeout,
		ShutdownMax: cfg.ShutdownTimeout,
		RunOnStart:  cfg.RunOnStart,
	})
}
