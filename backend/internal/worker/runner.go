// Package worker provides the process skeleton shared by the background
// binaries (sports-sync, bookmaker-collector, settlement-worker,
// analytics-worker).
//
// A worker boots cleanly with zero jobs. That is intentional for Phase 0: the
// same binary shape is reused once provider adapters exist, and nothing is
// scheduled before there is real work to do.
package worker

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"os"
	"os/signal"
	"syscall"
	"time"
)

// Job is one periodic unit of work. Run must be idempotent and bounded by the
// context it receives.
type Job struct {
	Name  string
	Every time.Duration
	Run   func(ctx context.Context) error
}

// Options configures a worker process.
type Options struct {
	Name        string
	Logger      *slog.Logger
	Jobs        []Job
	JobTimeout  time.Duration
	ShutdownMax time.Duration
	// RunOnStart executes each job once immediately instead of waiting for the
	// first interval. Useful for development and manual verification.
	RunOnStart bool
}

// Run starts the worker and blocks until SIGINT/SIGTERM or the parent context
// is cancelled.
func Run(parent context.Context, opts Options) error {
	log := opts.Logger.With(slog.String("worker", opts.Name))
	if opts.JobTimeout <= 0 {
		opts.JobTimeout = 5 * time.Minute
	}
	if opts.ShutdownMax <= 0 {
		opts.ShutdownMax = 20 * time.Second
	}

	ctx, stop := signal.NotifyContext(parent, os.Interrupt, syscall.SIGTERM)
	defer stop()

	log.Info("worker starting", slog.Int("jobs", len(opts.Jobs)))

	if len(opts.Jobs) == 0 {
		log.Info("worker idle: no jobs configured; waiting for signal",
			slog.String("reason", "provider adapters and collectors land in later phases"))
		<-ctx.Done()
		log.Info("worker stopping")
		return nil
	}

	errCh := make(chan error, len(opts.Jobs))
	for _, job := range opts.Jobs {
		if job.Every <= 0 {
			return fmt.Errorf("job %q: interval must be positive", job.Name)
		}
		if job.Run == nil {
			return fmt.Errorf("job %q: run function is nil", job.Name)
		}
		go runJob(ctx, log, job, opts.JobTimeout, opts.RunOnStart, errCh)
	}

	var runErr error
	select {
	case <-ctx.Done():
		log.Info("worker stopping")
	case runErr = <-errCh:
		log.Error("worker job failed permanently", slog.String("error", runErr.Error()))
		stop()
	}

	shutdownDeadline := time.After(opts.ShutdownMax)
	select {
	case <-shutdownDeadline:
		log.Warn("worker shutdown timed out; exiting")
	case <-ctx.Done():
	}
	if errors.Is(runErr, context.Canceled) {
		return nil
	}
	return runErr
}

func runJob(ctx context.Context, log *slog.Logger, job Job, timeout time.Duration, runOnStart bool, errCh chan<- error) {
	jitter := time.Duration(rand.Int64N(max(int64(job.Every)/5, 1)))
	timer := time.NewTimer(jitter)
	defer timer.Stop()

	for {
		if runOnStart {
			runOnStart = false
		} else {
			select {
			case <-ctx.Done():
				return
			case <-timer.C:
			}
		}

		jobLog := log.With(slog.String("job", job.Name))
		jobCtx, cancel := context.WithTimeout(ctx, timeout)
		start := time.Now()
		err := job.Run(jobCtx)
		cancel()

		if ctx.Err() != nil {
			return
		}
		if err != nil {
			jobLog.Error("job failed",
				slog.Duration("duration", time.Since(start)),
				slog.String("error", err.Error()))
		} else {
			jobLog.Info("job finished", slog.Duration("duration", time.Since(start)))
		}

		// Jobs retry on the next tick; only unrecoverable errors stop the worker.
		var fatal *Fatal
		if errors.As(err, &fatal) {
			errCh <- fmt.Errorf("job %s: %w", job.Name, fatal.Err)
			return
		}

		next := job.Every
		jitter = time.Duration(rand.Int64N(max(int64(job.Every)/5, 1)))
		timer.Reset(next + jitter)
	}
}

// Fatal wraps an error that must stop the whole worker process.
type Fatal struct{ Err error }

func (f *Fatal) Error() string { return "fatal: " + f.Err.Error() }
func (f *Fatal) Unwrap() error { return f.Err }
