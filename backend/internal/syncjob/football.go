// Package syncjob orchestrates provider fetches into the importer.
package syncjob

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/oyins/oddsvault/backend/internal/competitions"
	"github.com/oyins/oddsvault/backend/internal/providers"
	"github.com/oyins/oddsvault/backend/internal/providers/footballdata"
	"github.com/oyins/oddsvault/backend/internal/providers/openfootball"
	"github.com/oyins/oddsvault/backend/internal/store"
)

// Football syncs the enabled football competitions.
type Football struct {
	http     *providers.Client
	importer *store.Importer
	log      *slog.Logger
	now      func() time.Time
}

// NewFootball builds the football sync job.
func NewFootball(client *providers.Client, importer *store.Importer, log *slog.Logger) *Football {
	if log == nil {
		log = slog.Default()
	}
	return &Football{http: client, importer: importer, log: log, now: time.Now}
}

// Run syncs every enabled competition. A failure in one provider does not stop
// the others; all errors are returned together.
func (f *Football) Run(ctx context.Context) error {
	selected, err := competitions.Selected()
	if err != nil {
		return err
	}
	season := competitions.CurrentSeason(f.now().UTC())

	var errs []error
	for _, comp := range selected {
		if err := f.syncOpenFootball(ctx, comp, season); err != nil {
			errs = append(errs, err)
		}
		if err := f.syncFootballData(ctx, comp, season); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func (f *Football) syncOpenFootball(ctx context.Context, comp competitions.Competition, season competitions.Season) error {
	const provider = "openfootball"
	const jobType = "sync_football_fixtures"

	runID, err := f.importer.StartRun(ctx, provider, jobType)
	if err != nil {
		return err
	}

	stats, syncErr := func() (store.ImportStats, error) {
		adapter := openfootball.New(f.http)
		fixtures, err := adapter.FetchSeason(ctx, comp.Slug, season.Name, season.Directory, comp.OpenFootballFile, true)
		if err != nil {
			return store.ImportStats{}, err
		}
		return f.importer.ImportFixtures(ctx, comp, season, fixtures, true)
	}()

	if finishErr := f.importer.FinishRun(ctx, runID, stats, syncErr); finishErr != nil {
		f.log.Error("failed to record import run", slog.String("error", finishErr.Error()))
	}
	if syncErr != nil {
		return fmt.Errorf("%s %s: %w", provider, comp.Slug, syncErr)
	}
	f.log.Info("fixtures imported",
		slog.String("provider", provider),
		slog.String("competition", comp.Slug),
		slog.String("season", season.Name),
		slog.Int("received", stats.Received),
		slog.Int("inserted", stats.Inserted),
		slog.Int("updated", stats.Updated),
		slog.Int("skipped", stats.Skipped))
	return nil
}

func (f *Football) syncFootballData(ctx context.Context, comp competitions.Competition, season competitions.Season) error {
	const provider = "football-data"
	const jobType = "sync_football_results"

	runID, err := f.importer.StartRun(ctx, provider, jobType)
	if err != nil {
		return err
	}

	stats, syncErr := func() (store.ImportStats, error) {
		adapter := footballdata.New(f.http)
		fixtures, err := adapter.FetchSeason(ctx, comp.Slug, season.Name, season.Code, comp.FootballDataDivision, true)
		if err != nil {
			return store.ImportStats{}, err
		}
		return f.importer.ImportFixtures(ctx, comp, season, fixtures, true)
	}()

	if finishErr := f.importer.FinishRun(ctx, runID, stats, syncErr); finishErr != nil {
		f.log.Error("failed to record import run", slog.String("error", finishErr.Error()))
	}
	if syncErr != nil {
		return fmt.Errorf("%s %s: %w", provider, comp.Slug, syncErr)
	}
	f.log.Info("results imported",
		slog.String("provider", provider),
		slog.String("competition", comp.Slug),
		slog.String("season", season.Name),
		slog.Int("received", stats.Received),
		slog.Int("inserted", stats.Inserted),
		slog.Int("updated", stats.Updated),
		slog.Int("skipped", stats.Skipped))
	return nil
}
