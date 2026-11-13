package syncjob

import (
	"context"
	"errors"
	"fmt"

	"github.com/oyins/oddsvault/backend/internal/providers"
	"github.com/oyins/oddsvault/backend/internal/providers/espn"
)

// RunAllLive discovers every live soccer fixture ESPN lists, resolves the
// competition for each league code, and imports the live state.
//
// The seeded catalogue stays the source of schedules, odds and settlement. This
// only fills the live board for competitions we do not follow day to day, so a
// match in progress is never invisible just because its league is not in our
// list.
func (f *Football) RunAllLive(ctx context.Context) error {
	adapter := espn.New(f.http)

	discovered, err := adapter.FetchAllLive(ctx)
	if err != nil {
		return fmt.Errorf("all-live discovery: %w", err)
	}
	if len(discovered) == 0 {
		f.log.Info("all-live discovery: no live fixtures")
		return nil
	}

	names := make(map[string]string)
	states := make([]providers.LiveState, 0, len(discovered))
	var errs []error

	for _, event := range discovered {
		name, ok := names[event.LeagueCode]
		if !ok {
			if fetched, err := adapter.FetchLeagueName(ctx, event.LeagueCode); err == nil {
				name = fetched
			} else {
				errs = append(errs, fmt.Errorf("league name %s: %w", event.LeagueCode, err))
			}
			names[event.LeagueCode] = name
		}

		slug, err := f.importer.EnsureCompetition(ctx, "football", event.LeagueCode, name)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		event.State.CompetitionSlug = slug
		states = append(states, event.State)
	}

	if len(states) == 0 {
		return errors.Join(errs...)
	}

	stats, applyErr := f.importer.ApplyLive(ctx, states)
	f.importer.LogLive(stats, applyErr)
	if applyErr != nil {
		errs = append(errs, applyErr)
	}

	f.log.Info("all-live discovery finished",
		"fixtures", len(discovered),
		"imported", stats.Received,
		"created", stats.Created,
		"updated", stats.Updated,
		"leagues", len(names))
	return errors.Join(errs...)
}
