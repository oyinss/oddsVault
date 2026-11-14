package syncjob

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"github.com/oyins/oddsvault/backend/internal/normalize"
	"github.com/oyins/oddsvault/backend/internal/providers"
	"github.com/oyins/oddsvault/backend/internal/providers/fotmob"
	"github.com/oyins/oddsvault/backend/internal/store"
)

// Corner form backfill: the match detail page shows each team's last finished
// matches with corner counts. Our own history only covers what we have been
// tracking live, which is a matter of days. This job reads FotMob's daily
// boards for the past weeks and stores corners for matches involving a team
// that is on the board now.
//
// Only matches of teams we display are imported, so the job stays small. A
// league FotMob does not cover simply keeps no history; that absence is
// reported, never filled with an estimate.
const (
	cornerFormLookbackDays = 21
	cornerFormMaxFetches   = 240
	cornerFormDayBatch     = 40
)

// RunCornerFormBackfill fills recent corner history for board teams.
func (f *Football) RunCornerFormBackfill(ctx context.Context) error {
	keys, err := f.importer.BoardTeamKeys(ctx)
	if err != nil {
		return err
	}
	if len(keys) == 0 {
		return nil
	}

	runID, err := f.importer.StartRun(ctx, "fotmob", "backfill_corner_form")
	if err != nil {
		return err
	}

	adapter := fotmob.New(f.http)
	var errs []error
	stats := store.ImportStats{}
	fetched := 0
	saved := 0

	for offset := 1; offset <= cornerFormLookbackDays; offset++ {
		if fetched >= cornerFormMaxFetches {
			break
		}
		day := f.now().UTC().AddDate(0, 0, -offset)
		matches, err := adapter.FetchMatches(ctx, day)
		if err != nil {
			errs = append(errs, fmt.Errorf("fotmob history %s: %w", day.Format("2006-01-02"), err))
			continue
		}

		var states []providers.LiveState
		var relevant []fotmob.Match

		for _, match := range matches {
			if len(relevant) >= cornerFormDayBatch {
				break
			}
			if !match.Finished || match.HomeScore == nil || match.AwayScore == nil || match.StartUTC.IsZero() {
				continue
			}
			_, homeMatch := boardTeamID(keys, match.Home)
			_, awayMatch := boardTeamID(keys, match.Away)
			if !homeMatch && !awayMatch {
				continue
			}
			if _, missed := f.formMisses.Load(match.ID); missed {
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
				Status:          providers.StatusFinished,
				Detail:          match.Detail,
				HomeName:        match.Home,
				AwayName:        match.Away,
				HomeScore:       match.HomeScore,
				AwayScore:       match.AwayScore,
			})
			relevant = append(relevant, match)
		}

		if len(states) == 0 {
			continue
		}

		importStats, applyErr := f.importer.ApplyLive(ctx, states)
		if applyErr != nil {
			errs = append(errs, applyErr)
		}
		stats.Received += importStats.Received
		stats.Inserted += importStats.Created
		stats.Updated += importStats.Matched + importStats.Updated
		stats.Skipped += importStats.Skipped

		providerIDs := make([]string, 0, len(relevant))
		for _, match := range relevant {
			providerIDs = append(providerIDs, strconv.FormatInt(match.ID, 10))
		}
		events, err := f.importer.EventsByProviderIDs(ctx, "fotmob", providerIDs)
		if err != nil {
			errs = append(errs, err)
			continue
		}

		type pendingMatch struct {
			match   fotmob.Match
			eventID int64
			homeID  int64
			awayID  int64
		}
		var pending []pendingMatch
		var ids []int64
		for _, match := range relevant {
			event, ok := events[strconv.FormatInt(match.ID, 10)]
			if !ok {
				continue
			}
			pending = append(pending, pendingMatch{
				match:   match,
				eventID: event.EventID,
				homeID:  event.HomeTeamID,
				awayID:  event.AwayTeamID,
			})
			ids = append(ids, event.EventID)
		}

		// Skip matches that already carry corner counts, so a second run does
		// not fetch them again.
		done, err := f.importer.EventsWithCornerStats(ctx, ids)
		if err != nil {
			errs = append(errs, err)
			continue
		}

		for _, item := range pending {
			if fetched >= cornerFormMaxFetches {
				break
			}
			if done[item.eventID] {
				continue
			}
			fetched++
			corners, err := adapter.FetchCorners(ctx, item.match.ID)
			if err != nil {
				errs = append(errs, fmt.Errorf("fotmob corners %d: %w", item.match.ID, err))
				continue
			}
			if corners.Home == nil || corners.Away == nil {
				// FotMob lists the fixture but publishes no statistics for this
				// competition. Remember it so the request cap is not spent on it
				// again during this run.
				f.formMisses.Store(item.match.ID, struct{}{})
				continue
			}
			if err := f.importer.SaveCorners(ctx, item.eventID, "fotmob",
				item.homeID, item.awayID, corners.Home, corners.Away); err != nil {
				errs = append(errs, err)
				continue
			}
			saved++
		}
	}

	if finishErr := f.importer.FinishRun(ctx, runID, stats, errors.Join(errs...)); finishErr != nil {
		f.log.Error("failed to record import run", "error", finishErr.Error())
	}

	f.log.Info("corner form backfill",
		"teams_on_board", len(keys),
		"matches_with_corners", saved,
		"corner_fetches", fetched,
		"errors", len(errs))
	return errors.Join(errs...)
}

// boardTeamID resolves a provider team name against the board's canonical and
// alias keys. Exact keys only: a near match is not evidence of identity.
func boardTeamID(keys map[string]int64, name string) (int64, bool) {
	_, resolved := normalize.Resolve(name)
	if id, ok := keys[resolved]; ok {
		return id, true
	}
	if id, ok := keys[normalize.Key(name)]; ok {
		return id, true
	}
	return 0, false
}
