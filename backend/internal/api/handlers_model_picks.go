package api

import (
	"net/http"
	"time"
)

type modelPick struct {
	ID           int64     `json:"id"`
	RunID        int64     `json:"run_id"`
	EventID      int64     `json:"event_id"`
	Sport        string    `json:"sport"`
	Competition  string    `json:"competition"`
	HomeName     string    `json:"home_name"`
	AwayName     string    `json:"away_name"`
	Kickoff      time.Time `json:"kickoff"`
	MarketType   string    `json:"market_type"`
	Selection    string    `json:"selection"`
	Odds         float64   `json:"odds"`
	Status       string    `json:"status"`
	LockedAt     time.Time `json:"locked_at"`
	Samples      int       `json:"samples"`
	HitRate      float64   `json:"hit_rate"`
	Implied      float64   `json:"implied"`
	Edge         float64   `json:"edge"`
	StrategySlug string    `json:"strategy_slug"`
	Version      int       `json:"strategy_version"`
	CutoffAt     time.Time `json:"data_cutoff_at"`
}

type modelPicksResponse struct {
	Picks    []modelPick `json:"picks"`
	Cutoff   *time.Time  `json:"data_cutoff_at"`
	RunID    *int64      `json:"run_id"`
	Strategy string      `json:"strategy"`
	Count    int         `json:"count"`
}

// handleModelPicks returns the most recent frozen pick set. It never recomputes
// or re-ranks: what was written at generation time is what is served.
func (s *Server) handleModelPicks(w http.ResponseWriter, r *http.Request) {
	if s.pool == nil {
		writeError(w, r, http.StatusServiceUnavailable, "not_ready", "database is not configured")
		return
	}

	var out = modelPicksResponse{Picks: []modelPick{}, Strategy: "match-result-edge"}

	var runID int64
	var cutoff time.Time
	err := s.pool.QueryRow(r.Context(), `
		SELECT mr.id, mr.data_cutoff_at
		FROM model_runs mr
		JOIN strategy_versions sv ON sv.id = mr.strategy_version_id
		JOIN strategy_definitions sd ON sd.id = sv.strategy_id
		WHERE sd.slug = $1
		ORDER BY mr.generated_at DESC
		LIMIT 1`, out.Strategy).Scan(&runID, &cutoff)
	if err != nil {
		// No run yet is a normal, honest state.
		writeJSON(w, http.StatusOK, out)
		return
	}
	out.RunID = &runID
	out.Cutoff = &cutoff

	// Every open pick stays visible until its match is resolved, even after a
	// newer run. Resolved picks are shown for the latest run only, so this is a
	// live recommendation list plus the most recent track record, never a
	// rewritten history.
	rows, err := s.pool.Query(r.Context(), `
		SELECT mb.id, mb.run_id, mb.event_id, sp.slug,
		       COALESCE(c.name, ''), ht.name, at.name,
		       COALESCE(e.start_time, e.start_date::timestamptz),
		       ml.market_type, ml.selection, ml.odds::float8, mb.status, mb.locked_at,
		       COALESCE((ml.evidence->>'samples')::int, 0),
		       COALESCE((ml.evidence->>'hit_rate')::float8, 0),
		       COALESCE((ml.evidence->>'implied')::float8, 0),
		       COALESCE((ml.evidence->>'edge')::float8, 0),
		       sd.slug, sv.version, mr.data_cutoff_at
		FROM model_bets mb
		JOIN model_bet_legs ml ON ml.model_bet_id = mb.id
		JOIN model_runs mr ON mr.id = mb.run_id
		JOIN strategy_versions sv ON sv.id = mr.strategy_version_id
		JOIN strategy_definitions sd ON sd.id = sv.strategy_id
		JOIN events e ON e.id = ml.event_id
		JOIN sports sp ON sp.id = e.sport_id
		LEFT JOIN competitions c ON c.id = e.competition_id
		JOIN event_participants hp ON hp.event_id = e.id AND hp.role = 'home'
		JOIN teams ht ON ht.id = hp.team_id
		JOIN event_participants ap ON ap.event_id = e.id AND ap.role = 'away'
		JOIN teams at ON at.id = ap.team_id
		WHERE mb.run_id = $1 OR mb.status = 'pending'
		ORDER BY COALESCE(e.start_time, e.start_date::timestamptz) ASC, ml.odds ASC, mb.id ASC`, runID)
	if err != nil {
		s.log.ErrorContext(r.Context(), "model picks", "error", err.Error())
		writeError(w, r, http.StatusInternalServerError, "query_failed", "could not load model picks")
		return
	}
	defer rows.Close()

	seen := make(map[int64]int)
	for rows.Next() {
		var p modelPick
		if err := rows.Scan(&p.ID, &p.RunID, &p.EventID, &p.Sport, &p.Competition,
			&p.HomeName, &p.AwayName, &p.Kickoff, &p.MarketType, &p.Selection,
			&p.Odds, &p.Status, &p.LockedAt, &p.Samples, &p.HitRate, &p.Implied,
			&p.Edge, &p.StrategySlug, &p.Version, &p.CutoffAt); err != nil {
			s.log.ErrorContext(r.Context(), "model picks scan", "error", err.Error())
			writeError(w, r, http.StatusInternalServerError, "query_failed", "could not load model picks")
			return
		}
		// One row per fixture: the newest run wins.
		if at, ok := seen[p.EventID]; ok {
			if p.RunID > out.Picks[at].RunID {
				out.Picks[at] = p
			}
			continue
		}
		seen[p.EventID] = len(out.Picks)
		out.Picks = append(out.Picks, p)
	}
	if err := rows.Err(); err != nil {
		s.log.ErrorContext(r.Context(), "model picks rows", "error", err.Error())
		writeError(w, r, http.StatusInternalServerError, "query_failed", "could not load model picks")
		return
	}

	out.Count = len(out.Picks)
	writeJSON(w, http.StatusOK, out)
}
