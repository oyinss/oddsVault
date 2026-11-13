// Package dedupe merges live events that describe the same fixture.
//
// Two providers can name the same league differently, so the same match can be
// imported twice. The keeper is the copy with a real score (ESPN preferred),
// provider links and corner counts move onto it, and the other is deleted only
// when no odds or bets reference it.
package dedupe

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/oyins/oddsvault/backend/internal/normalize"
)

// Result reports what one pass did.
type Result struct {
	Found   int
	Merged  int
	Skipped int
}

type eventRow struct {
	ID      int64
	Home    string
	Away    string
	Kickoff time.Time
	Score   bool
	Trusted bool
}

// Run merges duplicates when apply is true, otherwise it only reports them.
func Run(ctx context.Context, pool *pgxpool.Pool, apply bool) (Result, error) {
	var result Result

	groups, err := duplicates(ctx, pool)
	if err != nil {
		return result, err
	}

	for _, group := range groups {
		keeper, rest := choose(group)
		for _, drop := range rest {
			blocked, err := referenced(ctx, pool, drop.ID)
			if err != nil {
				return result, err
			}
			if blocked {
				result.Skipped++
				continue
			}
			result.Found++
			if !apply {
				continue
			}
			if err := merge(ctx, pool, keeper.ID, drop.ID); err != nil {
				return result, fmt.Errorf("merge %d into %d: %w", drop.ID, keeper.ID, err)
			}
			result.Merged++
		}
	}
	return result, nil
}

func duplicates(ctx context.Context, pool *pgxpool.Pool) ([][]eventRow, error) {
	rows, err := pool.Query(ctx, `
		SELECT e.id, ht.name, at.name,
		       COALESCE(e.start_time, e.start_date::timestamptz),
		       (l.home_score IS NOT NULL AND l.away_score IS NOT NULL),
		       EXISTS (
		           SELECT 1 FROM event_provider_ids p
		           WHERE p.event_id = e.id AND p.provider IN ('espn', 'fotmob')
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
		if err := rows.Scan(&item.ID, &item.Home, &item.Away, &item.Kickoff, &item.Score, &item.Trusted); err != nil {
			return nil, err
		}
		all = append(all, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	groups := make(map[string][]eventRow)
	for _, item := range all {
		key := teamKey(item.Home) + "|" + teamKey(item.Away) + "|" +
			item.Kickoff.UTC().Format("2006-01-02")
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

// teamAliases folds provider spellings that name the same side.
var teamAliases = map[string]string{
	"republic of ireland":          "ireland",
	"cote d ivoire":                "ivory coast",
	"korea republic":               "south korea",
	"ir iran":                      "iran",
	"usa":                          "united states",
	"uae":                          "united arab emirates",
	"bosnia herzegovina":           "bosnia and herzegovina",
	"macedonia":                    "north macedonia",
	"czech republic":               "czechia",
	"congo dr":                     "dr congo",
	"democratic republic of congo": "dr congo",
	"bosnia and herzegovina":       "bosnia and herzegovina",
}

// teamKey normalizes a name for duplicate grouping. It folds accents and
// punctuation, then known country spellings, but keeps age and gender markers
// because those are different fixtures.
func teamKey(name string) string {
	_, key := normalize.Resolve(name)
	for alias, canonical := range teamAliases {
		if key == alias {
			return canonical
		}
		if strings.HasPrefix(key, alias+" ") {
			return canonical + key[len(alias):]
		}
	}
	return key
}

func choose(group []eventRow) (eventRow, []eventRow) {
	// A fixture whose teams came from ESPN or FotMob is the better keeper: the
	// bookmaker spells sides differently and would orphan corner rows.
	keeper := group[0]
	for _, item := range group[1:] {
		switch {
		case item.Trusted && !keeper.Trusted:
			keeper = item
		case item.Trusted == keeper.Trusted && item.Score && !keeper.Score:
			keeper = item
		case item.Trusted == keeper.Trusted && item.Score == keeper.Score && item.ID < keeper.ID:
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

	if _, err := tx.Exec(ctx,
		`UPDATE event_provider_ids SET event_id = $1 WHERE event_id = $2`, keepID, dropID); err != nil {
		return fmt.Errorf("move provider ids: %w", err)
	}

	// Corners move by role, not by team id: the two events can hold different
	// team rows for the same side.
	homeKeep, awayKeep, err := participantTeams(ctx, tx, keepID)
	if err != nil {
		return err
	}
	homeDrop, awayDrop, err := participantTeams(ctx, tx, dropID)
	if err != nil {
		return err
	}
	pairs := [][2]int64{}
	if homeKeep != 0 && homeDrop != 0 {
		pairs = append(pairs, [2]int64{homeDrop, homeKeep})
	}
	if awayKeep != 0 && awayDrop != 0 {
		pairs = append(pairs, [2]int64{awayDrop, awayKeep})
	}
	for _, pair := range pairs {
		if _, err := tx.Exec(ctx, `
			INSERT INTO team_match_stats (event_id, team_id, corners, provider, captured_at)
			SELECT $1, $2, corners, provider, captured_at
			FROM team_match_stats WHERE event_id = $3 AND team_id = $4
			ON CONFLICT (event_id, team_id) DO UPDATE SET
				corners = COALESCE(team_match_stats.corners, EXCLUDED.corners),
				provider = CASE WHEN team_match_stats.corners IS NULL
				                THEN EXCLUDED.provider ELSE team_match_stats.provider END,
				captured_at = GREATEST(team_match_stats.captured_at, EXCLUDED.captured_at)`,
			keepID, pair[1], dropID, pair[0]); err != nil {
			return fmt.Errorf("move corners: %w", err)
		}
	}

	if _, err := tx.Exec(ctx, `DELETE FROM events WHERE id = $1`, dropID); err != nil {
		return fmt.Errorf("delete duplicate: %w", err)
	}
	return tx.Commit(ctx)
}

func participantTeams(ctx context.Context, tx pgx.Tx, eventID int64) (home, away int64, err error) {
	var homeID, awayID *int64
	if err := tx.QueryRow(ctx, `
		SELECT max(team_id) FILTER (WHERE role = 'home'),
		       max(team_id) FILTER (WHERE role = 'away')
		FROM event_participants WHERE event_id = $1`, eventID).Scan(&homeID, &awayID); err != nil {
		return 0, 0, fmt.Errorf("read participants: %w", err)
	}
	if homeID != nil {
		home = *homeID
	}
	if awayID != nil {
		away = *awayID
	}
	return home, away, nil
}
