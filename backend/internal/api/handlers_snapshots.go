package api

import (
	"net/http"
	"strconv"
	"strings"
	"time"
)

type snapshotRow struct {
	CapturedAt      string   `json:"captured_at"`
	DecimalOdds     float64  `json:"decimal_odds"`
	PriceKind       string   `json:"price_kind"`
	Bookmaker       string   `json:"bookmaker"`
	BookmakerName   string   `json:"bookmaker_name"`
	EventID         int64    `json:"event_id"`
	Competition     string   `json:"competition"`
	CompetitionName string   `json:"competition_name"`
	Home            string   `json:"home"`
	Away            string   `json:"away"`
	StartDate       *string  `json:"start_date"`
	Market          string   `json:"market"`
	Line            *float64 `json:"line"`
	Selection       string   `json:"selection"`
	Settlement      *string  `json:"settlement_status"`
}

func (s *Server) handleSnapshots(w http.ResponseWriter, r *http.Request) {
	if s.pool == nil {
		writeError(w, r, http.StatusServiceUnavailable, "not_ready", "database is not configured")
		return
	}

	q := r.URL.Query()
	limit := 100
	if raw := q.Get("limit"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 1 || parsed > 300 {
			writeError(w, r, http.StatusBadRequest, "invalid_limit", "limit must be between 1 and 300")
			return
		}
		limit = parsed
	}

	conditions := []string{"1 = 1"}
	args := []any{}
	add := func(condition string, value any) {
		args = append(args, value)
		conditions = append(conditions, strings.Replace(condition, "?", "$"+strconv.Itoa(len(args)), 1))
	}

	if v := q.Get("competition"); v != "" {
		add("c.slug = ?", v)
	}
	if v := q.Get("bookmaker"); v != "" {
		add("b.slug = ?", v)
	}
	if v := q.Get("market"); v != "" {
		add("mt.slug = ?", v)
	}
	if v := q.Get("kind"); v == "live" || v == "closing" {
		add("o.price_kind = ?", v)
	}

	args = append(args, limit)
	query := `
		SELECT o.captured_at, o.decimal_odds::float8, o.price_kind,
		       b.slug, b.name,
		       e.id, c.slug, c.name, ht.name, at.name, e.start_date,
		       mt.slug, m.line, se.selection, st.status
		FROM odds_snapshots o
		JOIN bookmakers b ON b.id = o.bookmaker_id
		JOIN markets m ON m.id = o.market_id
		JOIN market_types mt ON mt.id = m.market_type_id
		JOIN market_selections se ON se.id = o.selection_id
		JOIN events e ON e.id = m.event_id
		JOIN competitions c ON c.id = e.competition_id
		JOIN event_participants hp ON hp.event_id = e.id AND hp.role = 'home'
		JOIN teams ht ON ht.id = hp.team_id
		JOIN event_participants ap ON ap.event_id = e.id AND ap.role = 'away'
		JOIN teams at ON at.id = ap.team_id
		LEFT JOIN selection_settlements st ON st.selection_id = se.id
		WHERE ` + strings.Join(conditions, " AND ") + `
		ORDER BY o.captured_at DESC
		LIMIT $` + strconv.Itoa(len(args))

	rows, err := s.pool.Query(r.Context(), query, args...)
	if err != nil {
		s.log.ErrorContext(r.Context(), "query snapshots", "error", err.Error())
		writeError(w, r, http.StatusInternalServerError, "query_failed", "could not load snapshots")
		return
	}
	defer rows.Close()

	items := make([]snapshotRow, 0, limit)
	for rows.Next() {
		var item snapshotRow
		var capturedAt time.Time
		var startDate *time.Time
		if err := rows.Scan(
			&capturedAt, &item.DecimalOdds, &item.PriceKind,
			&item.Bookmaker, &item.BookmakerName,
			&item.EventID, &item.Competition, &item.CompetitionName,
			&item.Home, &item.Away, &startDate,
			&item.Market, &item.Line, &item.Selection, &item.Settlement); err != nil {
			s.log.ErrorContext(r.Context(), "scan snapshot", "error", err.Error())
			writeError(w, r, http.StatusInternalServerError, "query_failed", "could not load snapshots")
			return
		}
		item.CapturedAt = capturedAt.UTC().Format(time.RFC3339)
		if startDate != nil {
			v := startDate.Format("2006-01-02")
			item.StartDate = &v
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		s.log.ErrorContext(r.Context(), "iterate snapshots", "error", err.Error())
		writeError(w, r, http.StatusInternalServerError, "query_failed", "could not load snapshots")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"snapshots": items,
		"count":     len(items),
	})
}
