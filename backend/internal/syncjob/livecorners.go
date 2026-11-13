package syncjob

import (
	"context"
	"errors"
	"fmt"

	"github.com/oyins/oddsvault/backend/internal/providers/espn"
)

// RunLiveCorners refreshes corner counts for football matches in progress.
//
// The same ESPN summary that verifies finished matches also reports corners
// live. A live count is only stored when the provider actually published one:
// zero is a real zero, and an absent statistic stays absent.
func (f *Football) RunLiveCorners(ctx context.Context) error {
	candidates, err := f.importer.LiveCornerCandidates(ctx, 40)
	if err != nil {
		return err
	}
	if len(candidates) == 0 {
		return nil
	}

	adapter := espn.New(f.http)
	saved := 0
	var errs []error

	for _, candidate := range candidates {
		stats, err := adapter.FetchMatchStats(ctx, candidate.CompetitionSlug, candidate.ProviderEventID)
		if err != nil {
			errs = append(errs, fmt.Errorf("live corners event %d: %w", candidate.EventID, err))
			continue
		}
		if stats.HomeCorners == nil || stats.AwayCorners == nil {
			continue
		}
		if err := f.importer.SaveCorners(ctx, candidate.EventID, "espn",
			candidate.HomeTeamID, candidate.AwayTeamID,
			*stats.HomeCorners, *stats.AwayCorners); err != nil {
			errs = append(errs, err)
			continue
		}
		saved++
	}

	f.log.Info("live corners refreshed", "matches", len(candidates), "saved", saved)
	return errors.Join(errs...)
}
