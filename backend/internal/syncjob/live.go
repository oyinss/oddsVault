package syncjob

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/oyins/oddsvault/backend/internal/competitions"
	"github.com/oyins/oddsvault/backend/internal/providers"
	"github.com/oyins/oddsvault/backend/internal/providers/espn"
)

// RunLive fetches live/finished state for every live-enabled competition from
// ESPN, across sports. It runs every few minutes and only covers the days that
// still have unfinished events.
func (f *Football) RunLive(ctx context.Context) error {
	selected, err := competitions.List(competitions.LiveEnabled)
	if err != nil {
		return err
	}

	now := f.now().UTC()
	// Only query days that still have unfinished events: a moved fixture or a
	// delayed result can sit several days away from its scheduled date.
	from := now.AddDate(0, 0, -7)
	to := now.AddDate(0, 0, 7)
	dates, err := f.importer.UnfinishedEventDates(ctx, from, to)
	if err != nil {
		return err
	}
	if len(dates) == 0 {
		f.log.Info("live sync skipped: no unfinished events in range",
			"from", from.Format("2006-01-02"), "to", to.Format("2006-01-02"))
		return nil
	}

	// A fixture can be moved by a day or two; widen each stored date by one day
	// on either side so the provider's actual date is still queried.
	daySet := make(map[string]time.Time)
	for _, d := range dates {
		for offset := -1; offset <= 1; offset++ {
			day := d.AddDate(0, 0, offset)
			daySet[day.Format("2006-01-02")] = day
		}
	}
	days := make([]time.Time, 0, len(daySet))
	for _, day := range daySet {
		days = append(days, day)
	}
	sort.Slice(days, func(i, j int) bool { return days[i].Before(days[j]) })

	adapter := espn.New(f.http)
	var states []providers.LiveState
	seen := make(map[string]struct{})

	for _, comp := range selected {
		for _, day := range days {
			batch, err := adapter.FetchScoreboard(ctx, comp.Slug, day)
			if err != nil {
				f.log.Warn("live scoreboard fetch failed",
					"competition", comp.Slug, "date", day.Format("2006-01-02"), "error", err.Error())
				continue
			}
			for _, state := range batch {
				key := comp.Slug + ":" + state.ProviderEventID
				if _, ok := seen[key]; ok {
					continue
				}
				seen[key] = struct{}{}
				states = append(states, state)
			}
		}
	}

	if len(states) == 0 {
		f.log.Info("live sync: provider returned no fixtures in window")
		return nil
	}

	stats, syncErr := f.importer.ApplyLive(ctx, states)
	f.importer.LogLive(stats, syncErr)
	if syncErr != nil {
		return fmt.Errorf("live sync: %w", syncErr)
	}
	return nil
}
