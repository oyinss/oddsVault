// Command api serves the OddsVault HTTP API.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/oyins/oddsvault/backend/internal/api"
	"github.com/oyins/oddsvault/backend/internal/config"
	"github.com/oyins/oddsvault/backend/internal/database"
	"github.com/oyins/oddsvault/backend/internal/logging"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "api: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load("oddsvault-api")
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

	// A missing database must not prevent boot: /readyz reports it instead.
	pingCtx, cancelPing := context.WithTimeout(ctx, cfg.ConnectTimeout)
	if err := database.Ping(pingCtx, pool, cfg.ConnectTimeout); err != nil {
		logger.Warn("database not reachable at startup; /readyz will report not_ready",
			slog.String("error", err.Error()))
	}
	cancelPing()

	server := api.New(api.Options{
		Logger:             logger,
		Pool:               pool,
		Env:                cfg.Env,
		ServiceName:        cfg.ServiceName,
		RequestTimeout:     cfg.RequestTimeout,
		CORSAllowedOrigins: cfg.CORSAllowedOrigins,
		JournalToken:       cfg.JournalToken,
	})

	httpServer := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           server.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       cfg.ReadTimeout,
		WriteTimeout:      cfg.WriteTimeout,
		IdleTimeout:       cfg.IdleTimeout,
		ErrorLog:          slog.NewLogLogger(logger.Handler(), slog.LevelError),
	}

	serveErr := make(chan error, 1)
	go func() {
		logger.Info("api listening", slog.String("addr", cfg.HTTPAddr), slog.String("env", cfg.Env))
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serveErr <- err
		}
	}()

	select {
	case err := <-serveErr:
		return fmt.Errorf("serve: %w", err)
	case <-ctx.Done():
		logger.Info("shutdown signal received")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("graceful shutdown: %w", err)
	}
	logger.Info("api stopped")
	return nil
}
