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
	matched := 0

	for _, event := range discovered {
		// A fixture another provider already stored must not be created twice
		// just because the league name differs. Refresh its score instead.
		existing, findErr := f.importer.FindEventByPartialTeams(ctx, "football",
			event.State.HomeName, event.State.AwayName, event.State.StartUTC)
		if findErr != nil {
			errs = append(errs, findErr)
		} else if existing != nil {
			if err := f.importer.LinkProviderID(ctx, existing.EventID, "espn",
				event.State.ProviderEventID, event.State.StartUTC, string(event.State.Status)); err != nil {
				errs = append(errs, err)
			}
			if err := f.importer.RefreshLiveState(ctx, existing.EventID, "espn",
				event.State.Detail, event.State.HomeScore, event.State.AwayScore); err != nil {
				errs = append(errs, err)
			}
			matched++
			continue
		}

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
		"matched_existing", matched,
		"leagues", len(names))
	return errors.Join(errs...)
}
