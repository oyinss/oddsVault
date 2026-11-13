package syncjob

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"github.com/oyins/oddsvault/backend/internal/providers"
	"github.com/oyins/oddsvault/backend/internal/providers/fotmob"
)

// RunFotmobLive discovers live fixtures FotMob carries that ESPN did not list,
// then fills corner counts from FotMob for every live match it can match.
//
// Discovery only creates a competition and an event for a pairing that is not
// already stored, so a fixture ESPN already imported is never duplicated. A
// fixture imported here is marked provisional: the team-name match is weaker
// than an ESPN id.
func (f *Football) RunFotmobLive(ctx context.Context) error {
	adapter := fotmob.New(f.http)

	matches, err := adapter.FetchMatches(ctx, f.now().UTC())
	if err != nil {
		return fmt.Errorf("fotmob matches: %w", err)
	}

	live := make([]fotmob.Match, 0, len(matches))
	for _, match := range matches {
		if match.Live {
			live = append(live, match)
		}
	}
	if len(live) == 0 {
		return nil
	}

	var errs []error

	// Pass 1: import fixtures we do not already have.
	states := make([]providers.LiveState, 0, len(live))
	for _, match := range live {
		existing, err := f.importer.FindEventByTeamNames(ctx, "football", match.Home, match.Away, match.StartUTC)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		if existing != nil {
			// A fixture we discovered earlier keeps its clock and score fresh.
			// An ESPN-owned event is left alone: its state belongs to ESPN.
			linked, linkErr := f.importer.EventLinkedToProvider(ctx, existing.EventID, "fotmob")
			if linkErr != nil {
				errs = append(errs, linkErr)
				continue
			}
			if linked {
				if err := f.importer.RefreshLiveState(ctx, existing.EventID, "fotmob",
					match.Detail, match.HomeScore, match.AwayScore); err != nil {
					errs = append(errs, err)
				}
			}
			continue
		}

		slug, err := f.importer.EnsureCompetitionByFotmob(ctx, "football", match.LeagueID, match.LeagueName)
		if err != nil {
			errs = append(errs, err)
			continue
		}

		states = append(states, providers.LiveState{
			Provider:        "fotmob",
			ProviderEventID: strconv.FormatInt(match.ID, 10),
			Sport:           "football",
			CompetitionSlug: slug,
			StartUTC:        match.StartUTC,
			Status:          providers.StatusLive,
			Detail:          match.Detail,
			HomeName:        match.Home,
			AwayName:        match.Away,
			HomeScore:       match.HomeScore,
			AwayScore:       match.AwayScore,
		})
	}

	if len(states) > 0 {
		stats, applyErr := f.importer.ApplyLive(ctx, states)
		f.importer.LogLive(stats, applyErr)
		if applyErr != nil {
			errs = append(errs, applyErr)
		}
	}

	// Pass 2: fill corners for every live match that now has an event.
	saved := 0
	unmatched := 0
	for _, match := range live {
		event, err := f.importer.FindEventByTeamNames(ctx, "football", match.Home, match.Away, match.StartUTC)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		if event == nil {
			unmatched++
			continue
		}

		corners, err := adapter.FetchCorners(ctx, match.ID)
		if err != nil {
			errs = append(errs, fmt.Errorf("fotmob corners %d: %w", match.ID, err))
			continue
		}
		// A missing pair never overwrites a count another provider published.
		if corners.Home == nil || corners.Away == nil {
			continue
		}

		if err := f.importer.SaveCorners(ctx, event.EventID, "fotmob",
			event.HomeTeamID, event.AwayTeamID, corners.Home, corners.Away); err != nil {
			errs = append(errs, err)
			continue
		}
		saved++
	}

	f.log.Info("fotmob live refreshed",
		"live", len(live),
		"discovered", len(states),
		"corners", saved,
		"unmatched", unmatched)
	return errors.Join(errs...)
}
