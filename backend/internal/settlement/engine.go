// Package settlement evaluates market selections against verified results.
//
// It is deterministic: same result + same rule version = same settlement. Every
// change is written as a new evaluation_version; earlier versions are kept.
package settlement

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// RuleVersion identifies the implemented rule set.
const RuleVersion = 1

// Engine settles selections.
type Engine struct {
	pool *pgxpool.Pool
	log  *slog.Logger
}

// New builds a settlement engine.
func New(pool *pgxpool.Pool, log *slog.Logger) *Engine {
	if log == nil {
		log = slog.Default()
	}
	return &Engine{pool: pool, log: log}
}

// Stats summarizes one settlement run.
type Stats struct {
	Evaluated  int
	Changed    int
	Present    int
	Hits       int
	Misses     int
	Pushes     int
	Ungradable int
}

type candidate struct {
	selectionID int64
	marketType  string
	line        *float64
	selection   string
	homeScore   int
	awayScore   int
}

// Run settles every selection whose event has a verified final score.
func (e *Engine) Run(ctx context.Context) (Stats, error) {
	var stats Stats

	rows, err := e.pool.Query(ctx, `
		SELECT se.id, mt.slug, m.line, se.selection, r.home_score, r.away_score
		FROM market_selections se
		JOIN markets m ON m.id = se.market_id
		JOIN market_types mt ON mt.id = m.market_type_id
		JOIN events e ON e.id = m.event_id
		JOIN event_results r ON r.event_id = e.id
		WHERE e.status = 'finished'
		  AND r.home_score IS NOT NULL
		  AND r.away_score IS NOT NULL
		ORDER BY se.id`)
	if err != nil {
		return stats, fmt.Errorf("query settleable selections: %w", err)
	}
	defer rows.Close()

	var candidates []candidate
	for rows.Next() {
		var c candidate
		if err := rows.Scan(&c.selectionID, &c.marketType, &c.line, &c.selection, &c.homeScore, &c.awayScore); err != nil {
			return stats, fmt.Errorf("scan settleable selection: %w", err)
		}
		candidates = append(candidates, c)
	}
	if err := rows.Err(); err != nil {
		return stats, fmt.Errorf("iterate settleable selections: %w", err)
	}
	if len(candidates) == 0 {
		return stats, nil
	}

	tx, err := e.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return stats, fmt.Errorf("begin settlement: %w", err)
	}
	defer func() { _ = tx.Rollback(context.WithoutCancel(ctx)) }()

	now := time.Now().UTC()
	for _, c := range candidates {
		stats.Evaluated++

		status, actual, detail, reason := evaluate(c)
		switch status {
		case "hit":
			stats.Hits++
		case "miss":
			stats.Misses++
		case "push":
			stats.Pushes++
		default:
			stats.Ungradable++
		}

		changed, err := upsertSettlement(ctx, tx, c.selectionID, status, actual, detail, reason, now)
		if err != nil {
			return stats, err
		}
		if changed {
			stats.Changed++
		} else {
			stats.Present++
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return stats, fmt.Errorf("commit settlement: %w", err)
	}
	e.log.Info("settlement run finished",
		slog.Int("evaluated", stats.Evaluated),
		slog.Int("changed", stats.Changed),
		slog.Int("hits", stats.Hits),
		slog.Int("misses", stats.Misses),
		slog.Int("pushes", stats.Pushes),
		slog.Int("ungradable", stats.Ungradable))
	return stats, nil
}

func upsertSettlement(ctx context.Context, tx pgx.Tx, selectionID int64, status, actual, detail, reason string, now time.Time) (bool, error) {
	var prevStatus, prevActual string
	var version int
	err := tx.QueryRow(ctx, `
		SELECT status, COALESCE(actual_value, ''), evaluation_version
		FROM selection_settlements WHERE selection_id = $1`, selectionID).
		Scan(&prevStatus, &prevActual, &version)
	if err != nil && err != pgx.ErrNoRows {
		return false, fmt.Errorf("read settlement %d: %w", selectionID, err)
	}

	changed := err == pgx.ErrNoRows || prevStatus != status || prevActual != actual
	if !changed {
		return false, nil
	}
	if err == nil {
		version++
	} else {
		version = 1
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO selection_settlements
			(selection_id, status, actual_value, result_detail, reason_code,
			 data_source, evaluated_at, rule_version, evaluation_version)
		VALUES ($1, $2, $3, $4, $5, 'event_results', $6, $7, $8)
		ON CONFLICT (selection_id) DO UPDATE SET
			status = EXCLUDED.status,
			actual_value = EXCLUDED.actual_value,
			result_detail = EXCLUDED.result_detail,
			reason_code = EXCLUDED.reason_code,
			data_source = EXCLUDED.data_source,
			evaluated_at = EXCLUDED.evaluated_at,
			rule_version = EXCLUDED.rule_version,
			evaluation_version = EXCLUDED.evaluation_version`,
		selectionID, status, actual, detail, reason, now, RuleVersion, version); err != nil {
		return false, fmt.Errorf("write settlement %d: %w", selectionID, err)
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO selection_settlement_versions
			(selection_id, status, actual_value, result_detail, reason_code,
			 data_source, evaluated_at, rule_version, evaluation_version)
		VALUES ($1, $2, $3, $4, $5, 'event_results', $6, $7, $8)
		ON CONFLICT (selection_id, evaluation_version) DO NOTHING`,
		selectionID, status, actual, detail, reason, now, RuleVersion, version); err != nil {
		return false, fmt.Errorf("write settlement version %d: %w", selectionID, err)
	}
	return true, nil
}

// evaluate returns the settlement for one selection.
func evaluate(c candidate) (status, actual, detail, reason string) {
	actual = fmt.Sprintf("%d-%d", c.homeScore, c.awayScore)

	switch c.marketType {
	case "match_result":
		winner := "draw"
		switch {
		case c.homeScore > c.awayScore:
			winner = "home"
		case c.awayScore > c.homeScore:
			winner = "away"
		}
		return hitOrMiss(winner == c.selection), actual, "match result " + winner, ""

	case "double_chance":
		ok := false
		switch c.selection {
		case "home_or_draw":
			ok = c.homeScore >= c.awayScore
		case "home_or_away":
			ok = c.homeScore != c.awayScore
		case "draw_or_away":
			ok = c.awayScore >= c.homeScore
		default:
			return "not_gradable", actual, "", "unsupported_selection"
		}
		return hitOrMiss(ok), actual, "double chance", ""

	case "draw_no_bet":
		if c.homeScore == c.awayScore {
			return "push", actual, "draw with stake returned", "push_rule"
		}
		if c.selection != "home" && c.selection != "away" {
			return "not_gradable", actual, "", "unsupported_selection"
		}
		winner := "home"
		if c.awayScore > c.homeScore {
			winner = "away"
		}
		return hitOrMiss(winner == c.selection), actual, "draw no bet", ""

	case "both_teams_to_score":
		both := c.homeScore > 0 && c.awayScore > 0
		switch c.selection {
		case "yes":
			return hitOrMiss(both), actual, "both teams scored", ""
		case "no":
			return hitOrMiss(!both), actual, "not both teams scored", ""
		default:
			return "not_gradable", actual, "", "unsupported_selection"
		}

	case "total_goals":
		if c.line == nil {
			return "not_gradable", actual, "", "missing_line"
		}
		total := float64(c.homeScore + c.awayScore)
		if total == *c.line {
			return "push", actual, fmt.Sprintf("total %.0f equals line %g", total, *c.line), "push_rule"
		}
		switch c.selection {
		case "over":
			return hitOrMiss(total > *c.line), actual, fmt.Sprintf("total %.0f", total), ""
		case "under":
			return hitOrMiss(total < *c.line), actual, fmt.Sprintf("total %.0f", total), ""
		default:
			return "not_gradable", actual, "", "unsupported_selection"
		}
	}

	return "not_gradable", actual, "", "unsupported_market"
}

func hitOrMiss(hit bool) string {
	if hit {
		return "hit"
	}
	return "miss"
}
