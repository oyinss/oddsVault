package syncjob

import (
	"context"
	"errors"
	"fmt"
	"sort"

	"github.com/oyins/oddsvault/backend/internal/competitions"
	"github.com/oyins/oddsvault/backend/internal/providers"
	"github.com/oyins/oddsvault/backend/internal/providers/espn"
)

// multiSportWindow is how far ahead the scoreboard poll builds a fixture list
// for sports ESPN only publishes week by week.
const multiSportWindow = 30

// RunMultiSportFixtures fills the schedule for competitions outside football.
//
// Basketball and American football release fixtures in short blocks, so the
// live poll alone would only ever hold the current week. This walks the
// scoreboard day by day and writes the same normalized events.
func (f *Football) RunMultiSportFixtures(ctx context.Context) error {
	selected, err := competitions.List(competitions.LiveEnabled)
	if err != nil {
		return err
	}

	now := f.now().UTC()
	from := now.AddDate(0, 0, -7)
	to := now.AddDate(0, 0, multiSportWindow)

	adapter := espn.New(f.http)
	var errs []error

	for _, comp := range selected {
		if comp.SportSlug() == "football" {
			continue
		}

		seen := make(map[string]struct{})
		var states []providers.LiveState
		for day := from; !day.After(to); day = day.AddDate(0, 0, 1) {
			batch, err := adapter.FetchScoreboard(ctx, comp.Slug, day)
			if err != nil {
				f.log.Warn("fixture scoreboard failed",
					"competition", comp.Slug, "date", day.Format("2006-01-02"), "error", err.Error())
				continue
			}
			for _, state := range batch {
				if _, ok := seen[state.ProviderEventID]; ok {
					continue
				}
				seen[state.ProviderEventID] = struct{}{}
				states = append(states, state)
			}
		}

		if len(states) == 0 {
			f.log.Info("fixture sync: provider returned nothing",
				"competition", comp.Slug)
			continue
		}

		// Oldest first keeps event creation in kickoff order.
		sort.Slice(states, func(i, j int) bool {
			return states[i].StartUTC.Before(states[j].StartUTC)
		})

		stats, err := f.importer.ApplyLive(ctx, states)
		f.importer.LogLive(stats, err)
		if err != nil {
			errs = append(errs, fmt.Errorf("fixtures %s: %w", comp.Slug, err))
			continue
		}
		f.log.Info("fixtures imported",
			"competition", comp.Slug,
			"received", stats.Received,
			"created", stats.Created,
			"matched", stats.Matched,
			"updated", stats.Updated)
	}
	return errors.Join(errs...)
}
