package syncjob

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"github.com/oyins/oddsvault/backend/internal/providers/fotmob"
	"github.com/oyins/oddsvault/backend/internal/store"
)

// RunAttackBackfill stores goal minutes and attacking stats for finished
// matches. Corner counts alone cannot explain why a game produced corners;
// the goal timeline and shots can.
func (f *Football) RunAttackBackfill(ctx context.Context) error {
	candidates, err := f.importer.EventsNeedingExtras(ctx, 40)
	if err != nil {
		return err
	}
	if len(candidates) == 0 {
		return nil
	}

	runID, err := f.importer.StartRun(ctx, "fotmob", "backfill_attack_stats")
	if err != nil {
		return err
	}

	adapter := fotmob.New(f.http)
	stats := store.ImportStats{}
	var errs []error
	goals := 0

	for _, candidate := range candidates {
		id, convErr := strconv.ParseInt(candidate.ProviderEventID, 10, 64)
		if convErr != nil {
			continue
		}
		extra, err := adapter.FetchExtra(ctx, id)
		if err != nil {
			errs = append(errs, fmt.Errorf("extras event %d: %w", candidate.EventID, err))
			continue
		}

		storeGoals := make([]store.Goal, 0, len(extra.Goals))
		for _, goal := range extra.Goals {
			storeGoals = append(storeGoals, store.Goal{
				Minute: goal.Minute, Home: goal.Home,
				HomeScore: goal.HomeScore, AwayScore: goal.AwayScore,
				Player: goal.Player, OwnGoal: goal.OwnGoal,
			})
		}
		home := store.AttackLine{
			Shots: extra.Home.Shots, ShotsOnTarget: extra.Home.ShotsOnTarget,
			BigChances: extra.Home.BigChances, Passes: extra.Home.Passes,
			XG: extra.Home.XG, Possession: extra.Home.Possession,
		}
		away := store.AttackLine{
			Shots: extra.Away.Shots, ShotsOnTarget: extra.Away.ShotsOnTarget,
			BigChances: extra.Away.BigChances, Passes: extra.Away.Passes,
			XG: extra.Away.XG, Possession: extra.Away.Possession,
		}

		if err := f.importer.SaveMatchExtras(ctx, candidate.EventID,
			candidate.HomeTeamID, candidate.AwayTeamID, "fotmob", storeGoals, home, away); err != nil {
			errs = append(errs, err)
			continue
		}
		stats.Updated++
		goals += len(storeGoals)
	}

	if err := f.importer.FinishRun(ctx, runID, stats, errors.Join(errs...)); err != nil {
		f.log.Error("failed to record import run", "error", err.Error())
	}

	f.log.Info("attack stats backfilled",
		"events", stats.Updated, "goals", goals, "candidates", len(candidates), "errors", len(errs))
	return errors.Join(errs...)
}
