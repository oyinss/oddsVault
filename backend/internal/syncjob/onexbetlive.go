package syncjob

import (
	"context"
	"errors"
	"fmt"

	"github.com/oyins/oddsvault/backend/internal/providers"
	"github.com/oyins/oddsvault/backend/internal/providers/onexbet"
)

// RunOnexbetLive imports live football from 1xBet's feed and keeps its corner
// counts current.
//
// This is the only accessible source for several leagues the user plays, so it
// also creates the competition and the event when the pairing is not stored.
// A pairing that already exists is refreshed, not duplicated.
func (f *Football) RunOnexbetLive(ctx context.Context) error {
	adapter := onexbet.New(f.http)

	events, err := adapter.FetchLive(ctx)
	if err != nil {
		return fmt.Errorf("1xbet live: %w", err)
	}
	if len(events) == 0 {
		return nil
	}

	var errs []error

	// Pass 1: import fixtures we do not already have.
	states := make([]providers.LiveState, 0, len(events))
	existing := make(map[string]bool, len(events))

	for _, event := range events {
		stored, err := f.importer.FindEventByPartialTeams(ctx, "football", event.Home, event.Away, event.StartUTC)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		if stored != nil {
			// Even when the fixture is already stored, this keeps the
			// competition's country current for the flag shown on a row.
			if _, err := f.importer.EnsureCompetitionByOnexbet(ctx, "football",
				event.LeagueID, event.LeagueName, event.CountryName); err != nil {
				errs = append(errs, err)
			}
			existing[event.ProviderEventID] = true
			continue
		}

		slug, err := f.importer.EnsureCompetitionByOnexbet(ctx, "football", event.LeagueID, event.LeagueName, event.CountryName)
		if err != nil {
			errs = append(errs, err)
			continue
		}

		states = append(states, providers.LiveState{
			Provider:        "onexbet",
			ProviderEventID: event.ProviderEventID,
			Sport:           "football",
			CompetitionSlug: slug,
			StartUTC:        event.StartUTC,
			Status:          event.Status,
			Detail:          event.Clock,
			HomeName:        event.Home,
			AwayName:        event.Away,
			HomeScore:       event.HomeScore,
			AwayScore:       event.AwayScore,
		})
	}

	if len(states) > 0 {
		stats, applyErr := f.importer.ApplyLive(ctx, states)
		f.importer.LogLive(stats, applyErr)
		if applyErr != nil {
			errs = append(errs, applyErr)
		}
	}

	// Pass 2: clock, score and corners for events this provider owns.
	refreshed := 0
	corners := 0
	for _, event := range events {
		stored, err := f.importer.FindEventByPartialTeams(ctx, "football", event.Home, event.Away, event.StartUTC)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		if stored == nil {
			continue
		}
		linked, err := f.importer.EventLinkedToProvider(ctx, stored.EventID, "onexbet")
		if err != nil {
			errs = append(errs, err)
			continue
		}
		if !linked {
			// An ESPN or FotMob event keeps its own provider's state.
			continue
		}

		if err := f.importer.RefreshLiveState(ctx, stored.EventID, "onexbet",
			event.Clock, event.HomeScore, event.AwayScore); err != nil {
			errs = append(errs, err)
			continue
		}
		refreshed++

		// 1xBet has artwork for nearly every club it prices, including the
		// lower divisions no other provider carries.
		if event.HomeLogoURL != "" {
			if err := f.importer.SetTeamLogoIfMissing(ctx, stored.HomeTeamID, "onexbet", event.HomeLogoURL); err != nil {
				errs = append(errs, err)
			}
		}
		if event.AwayLogoURL != "" {
			if err := f.importer.SetTeamLogoIfMissing(ctx, stored.AwayTeamID, "onexbet", event.AwayLogoURL); err != nil {
				errs = append(errs, err)
			}
		}
		if err := f.importer.SetCompetitionLogoIfMissing(ctx, stored.CompetitionSlug, "onexbet", event.LeagueLogoURL); err != nil {
			errs = append(errs, err)
		}

		if event.HomeCorners != nil && event.AwayCorners != nil {
			if err := f.importer.SaveCorners(ctx, stored.EventID, "onexbet",
				stored.HomeTeamID, stored.AwayTeamID,
				event.HomeCorners, event.AwayCorners); err != nil {
				errs = append(errs, err)
				continue
			}
			corners++
			_ = f.importer.RecordCornerSnapshot(ctx, stored.EventID,
				parseClockMinute(event.Clock), *event.HomeCorners, *event.AwayCorners)
		}
	}

	f.log.Info("1xbet live refreshed",
		"live", len(events),
		"discovered", len(states),
		"existing", len(existing),
		"refreshed", refreshed,
		"corners", corners)
	return errors.Join(errs...)
}
