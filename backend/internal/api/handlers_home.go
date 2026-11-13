package api

import (
	"net/http"
	"time"
)

type homeSummary struct {
	Sport          string `json:"sport"`
	Date           string `json:"date"`
	Live           int    `json:"live"`
	Today          int    `json:"today"`
	TodayScheduled int    `json:"today_scheduled"`
	TodayFinished  int    `json:"today_finished"`
	Upcoming       int    `json:"upcoming"`
	Competitions   int    `json:"competitions"`
}

// handleHomeSummary returns real counts for the homepage sidebar. The caller
// supplies its local date so "today" matches the viewer, not the server.
func (s *Server) handleHomeSummary(w http.ResponseWriter, r *http.Request) {
	if s.pool == nil {
		writeError(w, r, http.StatusServiceUnavailable, "not_ready", "database is not configured")
		return
	}

	q := r.URL.Query()
	sport := q.Get("sport")
	if sport == "" {
		sport = "football"
	}

	day := time.Now().UTC()
	if raw := q.Get("date"); raw != "" {
		parsed, err := time.Parse("2006-01-02", raw)
		if err != nil {
			writeError(w, r, http.StatusBadRequest, "invalid_date", "date must be YYYY-MM-DD")
			return
		}
		day = parsed
	}

	var out homeSummary
	out.Sport = sport
	out.Date = day.Format("2006-01-02")

	err := s.pool.QueryRow(r.Context(), `
		SELECT
			count(*) FILTER (WHERE e.status = 'live')::int,
			count(*) FILTER (WHERE e.start_date = $2)::int,
			count(*) FILTER (WHERE e.start_date = $2 AND e.status = 'scheduled')::int,
			count(*) FILTER (WHERE e.start_date = $2 AND e.status = 'finished')::int,
			count(*) FILTER (WHERE e.start_date > $2)::int,
			count(DISTINCT e.competition_id) FILTER (
				WHERE e.start_date BETWEEN $2::date - 7 AND $2::date + 30
			)::int
		FROM events e
		JOIN sports sp ON sp.id = e.sport_id
		WHERE sp.slug = $1`, sport, day).Scan(
		&out.Live, &out.Today, &out.TodayScheduled, &out.TodayFinished,
		&out.Upcoming, &out.Competitions)
	if err != nil {
		s.log.ErrorContext(r.Context(), "home summary", "error", err.Error())
		writeError(w, r, http.StatusInternalServerError, "query_failed", "could not load summary")
		return
	}

	writeJSON(w, http.StatusOK, out)
}
