package api

import "context"

// firstHalfTotals returns the match corner total at the break for each event we
// captured a snapshot for. The value is the highest total seen at or before the
// 45th minute, which is what a half-time scoreboard would have shown.
func (s *Server) firstHalfTotals(ctx context.Context, eventIDs []int64) map[int64]int {
	if len(eventIDs) == 0 {
		return nil
	}
	rows, err := s.pool.Query(ctx, `
		WITH ranked AS (
			SELECT event_id, total_corners,
			       row_number() OVER (
			           PARTITION BY event_id
			           ORDER BY minute DESC, captured_at DESC
			       ) AS rn
			FROM corner_snapshots
			WHERE event_id = ANY($1::bigint[])
			  AND minute IS NOT NULL
			  AND minute <= 45
		),
		-- Only claim a split for a match we followed past the break, otherwise
		-- a corner scored in first-half stoppage time would be counted as second
		-- half simply because our last snapshot was before it.
		second_half_seen AS (
			SELECT DISTINCT event_id
			FROM corner_snapshots
			WHERE event_id = ANY($1::bigint[])
			  AND minute IS NOT NULL
			  AND minute > 45
		)
		SELECT ranked.event_id, ranked.total_corners
		FROM ranked
		JOIN second_half_seen USING (event_id)
		WHERE ranked.rn = 1`, eventIDs)
	if err != nil {
		s.log.ErrorContext(ctx, "first half totals", "error", err.Error())
		return nil
	}
	defer rows.Close()

	out := make(map[int64]int, len(eventIDs))
	for rows.Next() {
		var eventID int64
		var total int
		if err := rows.Scan(&eventID, &total); err != nil {
			s.log.ErrorContext(ctx, "scan first half total", "error", err.Error())
			return out
		}
		out[eventID] = total
	}
	return out
}
