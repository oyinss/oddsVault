package syncjob

import (
	"context"
	"errors"
	"fmt"

	"github.com/oyins/oddsvault/backend/internal/providers/espn"
)

// RunStats fills provider match statistics for finished events, then verifies
// journal legs against them. Verification never changes the bookmaker's own
// verdict; it records the independent calculation alongside it.
func (f *Football) RunStats(ctx context.Context) error {
	candidates, err := f.importer.EventsNeedingStats(ctx, 25)
	if err != nil {
		return err
	}

	adapter := espn.New(f.http)
	saved := 0
	var errs []error

	for _, candidate := range candidates {
		stats, err := adapter.FetchMatchStats(ctx, candidate.CompetitionSlug, candidate.ProviderEventID)
		if err != nil {
			errs = append(errs, fmt.Errorf("stats event %d: %w", candidate.EventID, err))
			continue
		}
		if !stats.Completed || stats.HomeCorners == nil || stats.AwayCorners == nil {
			continue
		}
		if err := f.importer.SaveCorners(ctx, candidate.EventID, "espn",
			candidate.HomeTeamID, candidate.AwayTeamID, *stats.HomeCorners, *stats.AwayCorners); err != nil {
			errs = append(errs, err)
			continue
		}
		saved++
	}

	verified, mismatches, err := f.importer.VerifyCornerLegs(ctx)
	if err != nil {
		errs = append(errs, err)
	}

	f.log.Info("match stats refreshed",
		"events", saved,
		"candidates", len(candidates),
		"legs_verified", verified,
		"disagreements", mismatches)
	return errors.Join(errs...)
}
