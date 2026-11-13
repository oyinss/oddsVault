package api

import (
	"net/http"
	"strconv"
	"strings"
)

// handleSearch finds fixtures by team or competition name across every sport.
// It searches what is actually stored: there is no fuzzy guessing of names that
// are not there, and a short query is rejected rather than scanning everything.
func (s *Server) handleSearch(w http.ResponseWriter, r *http.Request) {
	if s.pool == nil {
		writeError(w, r, http.StatusServiceUnavailable, "not_ready", "database is not configured")
		return
	}

	query := strings.TrimSpace(r.URL.Query().Get("q"))
	if len([]rune(query)) < 2 {
		writeError(w, r, http.StatusBadRequest, "invalid_query", "q must be at least two characters")
		return
	}

	limit := 30
	if raw := r.URL.Query().Get("limit"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 1 || parsed > 50 {
			writeError(w, r, http.StatusBadRequest, "invalid_limit", "limit must be between 1 and 50")
			return
		}
		limit = parsed
	}

	pattern := "%" + query + "%"
	sql := eventSelect + `
		WHERE (ht.name ILIKE $1 OR at.name ILIKE $1 OR c.name ILIKE $1)
		  AND e.start_date >= CURRENT_DATE - 7
		ORDER BY (e.status = 'live') DESC,
		         abs(e.start_date - CURRENT_DATE),
		         COALESCE(e.start_time, e.start_date::timestamptz) ASC
		LIMIT $2`

	s.writeEventList(w, r, sql, []any{pattern, limit}, limit)
}
