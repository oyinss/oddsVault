package api

import (
	"net/http"
	"time"
)

type healthCounts struct {
	Events        int64 `json:"events"`
	EventsToday   int64 `json:"events_today"`
	Results       int64 `json:"results"`
	OddsSnapshots int64 `json:"odds_snapshots"`
	OddsLastHour  int64 `json:"odds_last_hour"`
	LiveStates    int64 `json:"live_states"`
	PersonalBets  int64 `json:"personal_bets"`
}

type importRun struct {
	Provider string  `json:"provider"`
	JobType  string  `json:"job_type"`
	Status   string  `json:"status"`
	Started  string  `json:"started_at"`
	Finished *string `json:"finished_at"`
	Received int     `json:"records_received"`
	Inserted int     `json:"records_inserted"`
	Updated  int     `json:"records_updated"`
	Skipped  int     `json:"records_skipped"`
	Error    *string `json:"error_message"`
}

type feedState struct {
	Provider    string `json:"provider"`
	JobType     string `json:"job_type"`
	LastSuccess string `json:"last_success"`
	AgeSeconds  int64  `json:"age_seconds"`
}

type collectorState struct {
	Bookmaker   string  `json:"bookmaker"`
	Kind        string  `json:"source_kind"`
	Status      string  `json:"status"`
	CompletedAt *string `json:"completed_at"`
	Offers      int     `json:"offer_count"`
	AgeSeconds  *int64  `json:"age_seconds"`
}

func (s *Server) handleAdminHealth(w http.ResponseWriter, r *http.Request) {
	if s.pool == nil {
		writeError(w, r, http.StatusServiceUnavailable, "not_ready", "database is not configured")
		return
	}

	start := time.Now()
	pingErr := s.pool.Ping(r.Context())
	latency := time.Since(start).Milliseconds()

	var counts healthCounts
	if pingErr == nil {
		if err := s.pool.QueryRow(r.Context(), `
			SELECT
				(SELECT count(*) FROM events),
				(SELECT count(*) FROM events WHERE start_date = CURRENT_DATE),
				(SELECT count(*) FROM event_results),
				(SELECT count(*) FROM odds_snapshots),
				(SELECT count(*) FROM odds_snapshots WHERE captured_at > now() - interval '1 hour'),
				(SELECT count(*) FROM event_live_states),
				(SELECT count(*) FROM personal_bets)`).Scan(
			&counts.Events, &counts.EventsToday, &counts.Results, &counts.OddsSnapshots,
			&counts.OddsLastHour, &counts.LiveStates, &counts.PersonalBets); err != nil {
			s.log.ErrorContext(r.Context(), "health counts", "error", err.Error())
		}
	}

	runs := make([]importRun, 0, 15)
	if rows, err := s.pool.Query(r.Context(), `
		SELECT provider, job_type, status, started_at, finished_at,
		       records_received, records_inserted, records_updated, records_skipped, error_message
		FROM import_runs ORDER BY id DESC LIMIT 15`); err == nil {
		defer rows.Close()
		for rows.Next() {
			var item importRun
			var started time.Time
			var finished *time.Time
			if err := rows.Scan(&item.Provider, &item.JobType, &item.Status, &started,
				&finished, &item.Received, &item.Inserted, &item.Updated, &item.Skipped,
				&item.Error); err != nil {
				continue
			}
			item.Started = started.UTC().Format(time.RFC3339)
			if finished != nil {
				v := finished.UTC().Format(time.RFC3339)
				item.Finished = &v
			}
			runs = append(runs, item)
		}
	}

	now := time.Now().UTC()
	feeds := make([]feedState, 0, 8)
	if rows, err := s.pool.Query(r.Context(), `
		SELECT provider, job_type, max(finished_at)
		FROM import_runs
		WHERE status = 'success' AND finished_at IS NOT NULL
		GROUP BY provider, job_type
		ORDER BY provider, job_type`); err == nil {
		defer rows.Close()
		for rows.Next() {
			var item feedState
			var last time.Time
			if err := rows.Scan(&item.Provider, &item.JobType, &last); err != nil {
				continue
			}
			item.LastSuccess = last.UTC().Format(time.RFC3339)
			item.AgeSeconds = int64(now.Sub(last).Seconds())
			feeds = append(feeds, item)
		}
	}

	collectors := make([]collectorState, 0, 4)
	if rows, err := s.pool.Query(r.Context(), `
		SELECT DISTINCT ON (sb.bookmaker_id)
		       b.slug, sb.source_kind, sb.status, sb.completed_at, sb.offer_count
		FROM snapshot_batches sb
		JOIN bookmakers b ON b.id = sb.bookmaker_id
		ORDER BY sb.bookmaker_id, sb.started_at DESC`); err == nil {
		defer rows.Close()
		for rows.Next() {
			var item collectorState
			var completed *time.Time
			if err := rows.Scan(&item.Bookmaker, &item.Kind, &item.Status, &completed, &item.Offers); err != nil {
				continue
			}
			if completed != nil {
				v := completed.UTC().Format(time.RFC3339)
				item.CompletedAt = &v
				age := int64(now.Sub(*completed).Seconds())
				item.AgeSeconds = &age
			}
			collectors = append(collectors, item)
		}
	}

	var unmatchedBookmakerEvents, unresolvedEvents, unresolvedLegs int64
	_ = s.pool.QueryRow(r.Context(), `
		SELECT
			(SELECT count(*) FROM bookmaker_events WHERE matched_event_id IS NULL),
			(SELECT count(*) FROM events WHERE identity_status = 'unresolved'),
			(SELECT count(*) FROM personal_bet_legs WHERE event_id IS NULL)`).Scan(
		&unmatchedBookmakerEvents, &unresolvedEvents, &unresolvedLegs)

	writeJSON(w, http.StatusOK, map[string]any{
		"generated_at": now.Format(time.RFC3339),
		"database": map[string]any{
			"reachable":  pingErr == nil,
			"latency_ms": latency,
		},
		"counts":      counts,
		"import_runs": runs,
		"feeds":       feeds,
		"collectors":  collectors,
		"unmatched": map[string]int64{
			"bookmaker_events": unmatchedBookmakerEvents,
			"events":           unresolvedEvents,
			"bet_legs":         unresolvedLegs,
		},
	})
}
