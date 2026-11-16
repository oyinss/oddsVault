package api

import "context"

// attachHalfSplits fills the first-half corner count from the snapshots taken
// while the match was live. The second half is the current total minus that
// figure. A match with no snapshot at or before the 45th minute gets nothing:
// an unknown split is left empty rather than assumed.
func (s *Server) attachHalfSplits(ctx context.Context, matches []liveCornerMatch) {
	if len(matches) == 0 {
		return
	}
	ids := make([]int64, 0, len(matches))
	for _, match := range matches {
		ids = append(ids, match.EventID)
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
		WHERE ranked.rn = 1`, ids)
	if err != nil {
		s.log.ErrorContext(ctx, "corner half splits", "error", err.Error())
		return
	}
	defer rows.Close()

	firstHalf := make(map[int64]int, len(matches))
	for rows.Next() {
		var eventID int64
		var total int
		if err := rows.Scan(&eventID, &total); err != nil {
			s.log.ErrorContext(ctx, "scan corner half split", "error", err.Error())
			return
		}
		firstHalf[eventID] = total
	}
	if err := rows.Err(); err != nil {
		s.log.ErrorContext(ctx, "iterate corner half splits", "error", err.Error())
		return
	}

	for index := range matches {
		first, ok := firstHalf[matches[index].EventID]
		if !ok || matches[index].TotalCorners == nil {
			continue
		}
		second := *matches[index].TotalCorners - first
		if second < 0 {
			// The snapshot is ahead of the current count, so the split cannot
			// be trusted and stays unknown.
			continue
		}
		matches[index].FirstHalfCorners = ptrInt(first)
		matches[index].SecondHalfCorners = ptrInt(second)
	}
}

func ptrInt(value int) *int { return &value }
