package store

import (
	"context"
	"fmt"
)

// ExtraCandidate is a finished match still missing its goal timeline and
// attacking stats.
type ExtraCandidate struct {
	EventID         int64
	ProviderEventID string
	HomeTeamID      int64
	AwayTeamID      int64
}

// EventsNeedingExtras lists finished matches with complete corner counts and a
// FotMob id that have no attacking stats yet.
func (im *Importer) EventsNeedingExtras(ctx context.Context, limit int) ([]ExtraCandidate, error) {
	rows, err := im.pool.Query(ctx, `
		SELECT e.id, p.provider_event_id, hp.team_id, ap.team_id
		FROM events e
		JOIN event_provider_ids p ON p.event_id = e.id AND p.provider = 'fotmob'
		JOIN event_participants hp ON hp.event_id = e.id AND hp.role = 'home'
		JOIN event_participants ap ON ap.event_id = e.id AND ap.role = 'away'
		JOIN team_match_stats hs ON hs.event_id = e.id AND hs.team_id = hp.team_id AND hs.corners IS NOT NULL
		JOIN team_match_stats as_ ON as_.event_id = e.id AND as_.team_id = ap.team_id AND as_.corners IS NOT NULL
		WHERE e.status = 'finished'
		  AND NOT EXISTS (SELECT 1 FROM team_attack_stats t WHERE t.event_id = e.id)
		ORDER BY e.start_date DESC
		LIMIT $1`, limit)
	if err != nil {
		return nil, fmt.Errorf("list events needing extras: %w", err)
	}
	defer rows.Close()

	var out []ExtraCandidate
	for rows.Next() {
		var item ExtraCandidate
		if err := rows.Scan(&item.EventID, &item.ProviderEventID,
			&item.HomeTeamID, &item.AwayTeamID); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

// Goal is one goal from a provider timeline.
type Goal struct {
	Minute    int
	Home      bool
	HomeScore *int
	AwayScore *int
	Player    string
	OwnGoal   bool
}

// AttackLine is one team's attacking output. Pointers keep "not published"
// separate from a real zero.
type AttackLine struct {
	Shots         *int
	ShotsOnTarget *int
	BigChances    *int
	Passes        *int
	XG            *float64
	Possession    *float64
}

// SaveMatchExtras replaces the goal timeline and upserts both teams' attacking
// stats for one event.
func (im *Importer) SaveMatchExtras(ctx context.Context, eventID, homeTeamID, awayTeamID int64,
	provider string, goals []Goal, home, away AttackLine) error {
	tx, err := im.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(context.WithoutCancel(ctx)) }()

	if _, err := tx.Exec(ctx, `DELETE FROM match_goal_events WHERE event_id = $1`, eventID); err != nil {
		return fmt.Errorf("clear goals: %w", err)
	}
	for _, goal := range goals {
		if _, err := tx.Exec(ctx, `
			INSERT INTO match_goal_events
				(event_id, minute, is_home, home_score_before, away_score_before, player, own_goal, provider)
			VALUES ($1, $2, $3, $4, $5, NULLIF($6, ''), $7, $8)
			ON CONFLICT (event_id, minute, is_home, player) DO NOTHING`,
			eventID, goal.Minute, goal.Home, goal.HomeScore, goal.AwayScore, goal.Player, goal.OwnGoal, provider); err != nil {
			return fmt.Errorf("save goal: %w", err)
		}
	}

	for _, row := range []struct {
		teamID int64
		line   AttackLine
	}{{homeTeamID, home}, {awayTeamID, away}} {
		if _, err := tx.Exec(ctx, `
			INSERT INTO team_attack_stats
				(event_id, team_id, shots, shots_on_target, big_chances, passes, xg, possession, provider, captured_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, now())
			ON CONFLICT (event_id, team_id) DO UPDATE SET
				shots = EXCLUDED.shots,
				shots_on_target = EXCLUDED.shots_on_target,
				big_chances = EXCLUDED.big_chances,
				passes = EXCLUDED.passes,
				xg = EXCLUDED.xg,
				possession = EXCLUDED.possession,
				provider = EXCLUDED.provider,
				captured_at = now()`,
			eventID, row.teamID, row.line.Shots, row.line.ShotsOnTarget,
			row.line.BigChances, row.line.Passes, row.line.XG, row.line.Possession, provider); err != nil {
			return fmt.Errorf("save attack stats: %w", err)
		}
	}

	return tx.Commit(ctx)
}
