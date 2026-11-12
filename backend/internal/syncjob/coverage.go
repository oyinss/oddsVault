package syncjob

import (
	"context"
	"errors"
	"fmt"

	"github.com/oyins/oddsvault/backend/internal/competitions"
	"github.com/oyins/oddsvault/backend/internal/providers"
	"github.com/oyins/oddsvault/backend/internal/providers/espn"
)

// RunCoverage fills fixtures for competitions that only exist so personal-bet
// legs can be identified, then tries to link unresolved legs.
func (f *Football) RunCoverage(ctx context.Context) error {
	now := f.now().UTC()
	from := now.AddDate(0, 0, -10)
	to := now.AddDate(0, 0, 14)

	adapter := espn.New(f.http)
	var errs []error

	for _, slug := range competitions.ESPNOnly {
		var states []providers.LiveState
		for day := from; !day.After(to); day = day.AddDate(0, 0, 1) {
			batch, err := adapter.FetchScoreboard(ctx, slug, day)
			if err != nil {
				f.log.Warn("coverage scoreboard failed",
					"competition", slug, "date", day.Format("2006-01-02"), "error", err.Error())
				continue
			}
			states = append(states, batch...)
		}
		if len(states) == 0 {
			continue
		}
		stats, err := f.importer.ApplyLive(ctx, states)
		f.importer.LogLive(stats, err)
		if err != nil {
			errs = append(errs, fmt.Errorf("coverage %s: %w", slug, err))
		}
	}

	resolved, unresolved, err := f.importer.ResolveBetLegs(ctx)
	if err != nil {
		errs = append(errs, err)
	} else {
		f.log.Info("bet legs resolved", "resolved", resolved, "unresolved", unresolved)
	}
	return errors.Join(errs...)
}
