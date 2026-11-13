// Command dedupe-live merges live events that describe the same fixture.
//
// Two providers can name the same league differently, and a fixture imported
// before the cross-provider matcher existed can exist twice. This command keeps
// the event with the best score, moves provider links and corner counts onto it,
// and deletes the other only when nothing else references it.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/oyins/oddsvault/backend/internal/config"
	"github.com/oyins/oddsvault/backend/internal/database"
)

func main() {
	apply := flag.Bool("apply", false, "delete the duplicates; without it the command only reports")
	flag.Parse()

	cfg, err := config.Load("oddsvault-dedupe-live")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	pool, err := database.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer pool.Close()

	groups, err := duplicates(ctx, pool)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	merged := 0
	for _, group := range groups {
		keeper, rest := choose(group)
		for _, drop := range rest {
			blocked, err := referenced(ctx, pool, drop.ID)
			if err != nil {
				fmt.Fprintf(os.Stderr, "check %d: %v\n", drop.ID, err)
				continue
			}
			if blocked {
				fmt.Printf("skip %d (%s v %s): odds or bets reference it\n",
					drop.ID, drop.Home, drop.Away)
				continue
			}
			fmt.Printf("merge %d -> %d (%s v %s, %s)\n", drop.ID, keeper.ID, drop.Home, drop.Away, drop.Kickoff.Format(time.RFC3339))
			if *apply {
				if err := merge(ctx, pool, keeper.ID, drop.ID); err != nil {
					fmt.Fprintf(os.Stderr, "merge %d into %d: %v\n", drop.ID, keeper.ID, err)
					continue
				}
			}
			merged++
		}
	}

	fmt.Printf("%d duplicate event(s) %s\n", merged, map[bool]string{true: "merged", false: "found (dry run)"}[*apply])
}

type eventRow struct {
	ID      int64
	Home    string
	Away    string
	Kickoff time.Time
	Score   bool
	ESPN    bool
}

func duplicates(ctx context.Context, pool *pgxpool.Pool) ([][]eventRow, error) {
	rows, err := pool.Query(ctx, `
		SELECT e.id, ht.name, at.name,
		       COALESCE(e.start_time, e.start_date::timestamptz),
		       (l.home_score IS NOT NULL AND l.away_score IS NOT NULL),
		       EXISTS (
		           SELECT 1 FROM event_provider_ids p
		           WHERE p.event_id = e.id AND p.provider = 'espn'
		       )
		FROM events e
		JOIN event_participants hp ON hp.event_id = e.id AND hp.role = 'home'
		JOIN teams ht ON ht.id = hp.team_id
		JOIN event_participants ap ON ap.event_id = e.id AND ap.role = 'away'
		JOIN teams at ON at.id = ap.team_id
		LEFT JOIN event_live_states l ON l.event_id = e.id
		WHERE e.status = 'live'
		ORDER BY e.id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var all []eventRow
	for rows.Next() {
		var item eventRow
		if err := rows.Scan(&item.ID, &item.Home, &item.Away, &item.Kickoff, &item.Score, &item.ESPN); err != nil {
			return nil, err
		}
		all = append(all, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Group by the pairing and the calendar date.
	groups := make(map[string][]eventRow)
	for _, item := range all {
		key := item.Home + "|" + item.Away + "|" + item.Kickoff.UTC().Format("2006-01-02")
		groups[key] = append(groups[key], item)
	}

	var out [][]eventRow
	for _, group := range groups {
		if len(group) > 1 {
			out = append(out, group)
		}
	}
	return out, nil
}

func choose(group []eventRow) (eventRow, []eventRow) {
	keeper := group[0]
	for _, item := range group[1:] {
		switch {
		case item.Score && !keeper.Score:
			keeper = item
		case item.Score == keeper.Score && item.ESPN && !keeper.ESPN:
			keeper = item
		case item.Score == keeper.Score && item.ESPN == keeper.ESPN && item.ID < keeper.ID:
			keeper = item
		}
	}
	rest := make([]eventRow, 0, len(group)-1)
	for _, item := range group {
		if item.ID != keeper.ID {
			rest = append(rest, item)
		}
	}
	return keeper, rest
}

func referenced(ctx context.Context, pool *pgxpool.Pool, eventID int64) (bool, error) {
	var blocked bool
	err := pool.QueryRow(ctx, `
		SELECT EXISTS (SELECT 1 FROM markets WHERE event_id = $1)
		    OR EXISTS (SELECT 1 FROM personal_bet_legs WHERE event_id = $1)
		    OR EXISTS (SELECT 1 FROM model_bet_legs WHERE event_id = $1)`, eventID).Scan(&blocked)
	return blocked, err
}

func merge(ctx context.Context, pool *pgxpool.Pool, keepID, dropID int64) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(context.WithoutCancel(ctx)) }()

	// Provider links move across; the unique key is provider + provider id.
	if _, err := tx.Exec(ctx,
		`UPDATE event_provider_ids SET event_id = $1 WHERE event_id = $2`, keepID, dropID); err != nil {
		return fmt.Errorf("move provider ids: %w", err)
	}

	// Corners: keep what the keeper has, fill gaps from the duplicate.
	if _, err := tx.Exec(ctx, `
		INSERT INTO team_match_stats (event_id, team_id, corners, provider, captured_at)
		SELECT $1, team_id, corners, provider, captured_at
		FROM team_match_stats WHERE event_id = $2
		ON CONFLICT (event_id, team_id) DO UPDATE SET
			corners = COALESCE(team_match_stats.corners, EXCLUDED.corners),
			provider = CASE WHEN team_match_stats.corners IS NULL
			                THEN EXCLUDED.provider ELSE team_match_stats.provider END,
			captured_at = GREATEST(team_match_stats.captured_at, EXCLUDED.captured_at)`,
		keepID, dropID); err != nil {
		return fmt.Errorf("move corners: %w", err)
	}

	if _, err := tx.Exec(ctx, `DELETE FROM events WHERE id = $1`, dropID); err != nil {
		return fmt.Errorf("delete duplicate: %w", err)
	}
	return tx.Commit(ctx)
}
