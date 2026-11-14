package api

import (
	"net/http"
	"strconv"
	"time"
)

type cornerPoint struct {
	Minute     *int   `json:"minute"`
	Total      int    `json:"total_corners"`
	CapturedAt string `json:"captured_at"`
}

// handleEventCornerHistory returns the snapshot series recorded while a match
// was live. Providers publish only the running total, so this is the closest we
// can get to when each corner arrived: one point per poll where the count moved.
func (s *Server) handleEventCornerHistory(w http.ResponseWriter, r *http.Request) {
	if s.pool == nil {
		writeError(w, r, http.StatusServiceUnavailable, "not_ready", "database is not configured")
		return
	}
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		writeError(w, r, http.StatusBadRequest, "invalid_id", "event id must be a positive integer")
		return
	}

	rows, err := s.pool.Query(r.Context(), `
		SELECT minute, total_corners, captured_at
		FROM corner_snapshots
		WHERE event_id = $1
		ORDER BY captured_at ASC`, id)
	if err != nil {
		s.log.ErrorContext(r.Context(), "corner history", "error", err.Error())
		writeError(w, r, http.StatusInternalServerError, "query_failed", "could not load corner history")
		return
	}
	defer rows.Close()

	points := make([]cornerPoint, 0, 32)
	for rows.Next() {
		var point cornerPoint
		var captured time.Time
		if err := rows.Scan(&point.Minute, &point.Total, &captured); err != nil {
			writeError(w, r, http.StatusInternalServerError, "query_failed", "could not load corner history")
			return
		}
		point.CapturedAt = captured.UTC().Format(time.RFC3339)
		points = append(points, point)
	}
	if err := rows.Err(); err != nil {
		writeError(w, r, http.StatusInternalServerError, "query_failed", "could not load corner history")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"event_id": id,
		"points":   points,
		"count":    len(points),
		"note":     "One point per poll where the corner count changed; provider totals do not include per-corner timestamps.",
	})
}
