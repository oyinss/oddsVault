// Model bet resolution.
//
// A model pick is frozen at generation time, but its outcome is not a matter of
// opinion: every leg points at the market selection it priced, and that
// selection is settled by the deterministic engine. This file only combines
// those leg outcomes into the bet's status.
package settlement

import (
	"context"
	"fmt"
	"log/slog"
)

// legTally counts the settled states of a model bet's legs.
type legTally struct {
	legs       int
	hits       int
	misses     int
	voids      int
	unresolved int
}

// modelBetStatus maps leg outcomes to a bet status.
//
// A void or push leg is removed from the bet, so "all remaining legs hit" is a
// win. Anything still waiting on data keeps the whole bet pending rather than
// guessing.
func modelBetStatus(t legTally) string {
	switch {
	case t.legs == 0:
		return "unknown"
	case t.unresolved > 0:
		return "pending"
	case t.misses > 0:
		return "lost"
	case t.voids == t.legs:
		return "void"
	case t.hits > 0:
		return "won"
	default:
		return "unknown"
	}
}

// ModelStats summarizes one model-bet resolution pass.
type ModelStats struct {
	Checked int
	Changed int
}

// SettleModelBets updates the status of frozen model bets whose legs have all
// been settled. A bet that already left 'pending' is never rewritten.
func (e *Engine) SettleModelBets(ctx context.Context) (ModelStats, error) {
	var stats ModelStats

	rows, err := e.pool.Query(ctx, `
		SELECT mb.id,
		       count(ml.id)::int,
		       count(*) FILTER (WHERE ss.status = 'hit')::int,
		       count(*) FILTER (WHERE ss.status = 'miss')::int,
		       count(*) FILTER (WHERE ss.status IN ('void', 'push'))::int,
		       count(*) FILTER (
		           WHERE ss.status IS NULL
		              OR ss.status IN ('pending', 'needs_data', 'not_gradable')
		       )::int
		FROM model_bets mb
		JOIN model_bet_legs ml ON ml.model_bet_id = mb.id
		LEFT JOIN selection_settlements ss ON ss.selection_id = ml.selection_id
		WHERE mb.status = 'pending'
		GROUP BY mb.id`)
	if err != nil {
		return stats, fmt.Errorf("model bet candidates: %w", err)
	}

	type decision struct {
		id     int64
		status string
	}
	var decisions []decision

	for rows.Next() {
		var id int64
		var tally legTally
		if err := rows.Scan(&id, &tally.legs, &tally.hits, &tally.misses,
			&tally.voids, &tally.unresolved); err != nil {
			rows.Close()
			return stats, err
		}
		stats.Checked++
		if status := modelBetStatus(tally); status != "pending" {
			decisions = append(decisions, decision{id: id, status: status})
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return stats, err
	}

	for _, d := range decisions {
		tag, err := e.pool.Exec(ctx, `
			UPDATE model_bets
			SET status = $2, resolved_at = now()
			WHERE id = $1 AND status = 'pending'`, d.id, d.status)
		if err != nil {
			return stats, fmt.Errorf("resolve model bet %d: %w", d.id, err)
		}
		if tag.RowsAffected() > 0 {
			stats.Changed++
		}
	}

	if stats.Changed > 0 && e.log != nil {
		e.log.Info("model bets resolved",
			slog.Int("checked", stats.Checked),
			slog.Int("changed", stats.Changed))
	}
	return stats, nil
}
