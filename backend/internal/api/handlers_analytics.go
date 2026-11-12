package api

import (
	"math"
	"net/http"
	"strconv"
	"time"
)

type movementPoint struct {
	At   string  `json:"at"`
	Odds float64 `json:"odds"`
}

type movementSelection struct {
	Selection string          `json:"selection"`
	Opening   float64         `json:"opening"`
	Current   float64         `json:"current"`
	High      float64         `json:"high"`
	Low       float64         `json:"low"`
	Samples   int             `json:"samples"`
	FirstAt   string          `json:"first_at"`
	LastAt    string          `json:"last_at"`
	Series    []movementPoint `json:"series"`
}

type movementMarket struct {
	Type       string              `json:"type"`
	Line       *float64            `json:"line"`
	Selections []movementSelection `json:"selections"`
}

func (s *Server) handleEventMovement(w http.ResponseWriter, r *http.Request) {
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
		SELECT m.id, mt.slug, m.line, se.id, se.selection, o.decimal_odds::float8, o.captured_at
		FROM odds_snapshots o
		JOIN market_selections se ON se.id = o.selection_id
		JOIN markets m ON m.id = se.market_id
		JOIN market_types mt ON mt.id = m.market_type_id
		WHERE o.event_id = $1 AND o.availability_status = 'available'
		ORDER BY m.id, se.id, o.captured_at`, id)
	if err != nil {
		s.log.ErrorContext(r.Context(), "query movement", "error", err.Error())
		writeError(w, r, http.StatusInternalServerError, "query_failed", "could not load movement")
		return
	}
	defer rows.Close()

	markets := make([]movementMarket, 0, 4)
	marketIndex := make(map[int64]int)
	selectionIndex := make(map[int64][2]int) // marketID -> [marketPos, selectionPos]

	for rows.Next() {
		var marketID, selectionID int64
		var marketType, selection string
		var line *float64
		var odds float64
		var capturedAt time.Time

		if err := rows.Scan(&marketID, &marketType, &line, &selectionID, &selection, &odds, &capturedAt); err != nil {
			s.log.ErrorContext(r.Context(), "scan movement", "error", err.Error())
			writeError(w, r, http.StatusInternalServerError, "query_failed", "could not load movement")
			return
		}

		mPos, ok := marketIndex[marketID]
		if !ok {
			markets = append(markets, movementMarket{Type: marketType, Line: line})
			mPos = len(markets) - 1
			marketIndex[marketID] = mPos
		}

		sPos, ok := selectionIndex[selectionID]
		if !ok {
			at := capturedAt.UTC().Format(time.RFC3339)
			markets[mPos].Selections = append(markets[mPos].Selections, movementSelection{
				Selection: selection,
				Opening:   odds,
				Current:   odds,
				High:      odds,
				Low:       odds,
				FirstAt:   at,
				LastAt:    at,
			})
			sPos = [2]int{mPos, len(markets[mPos].Selections) - 1}
			selectionIndex[selectionID] = sPos
		}

		item := &markets[sPos[0]].Selections[sPos[1]]
		item.Current = odds
		item.LastAt = capturedAt.UTC().Format(time.RFC3339)
		if odds > item.High {
			item.High = odds
		}
		if odds < item.Low {
			item.Low = odds
		}
		item.Samples++
		// Bound the series so one long-running selection cannot bloat the page.
		if len(item.Series) < 120 {
			item.Series = append(item.Series, movementPoint{
				At:   capturedAt.UTC().Format(time.RFC3339),
				Odds: odds,
			})
		}
	}
	if err := rows.Err(); err != nil {
		s.log.ErrorContext(r.Context(), "iterate movement", "error", err.Error())
		writeError(w, r, http.StatusInternalServerError, "query_failed", "could not load movement")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"event_id": id,
		"markets":  markets,
	})
}

type oddsBucket struct {
	Low      float64 `json:"low"`
	High     float64 `json:"high"`
	Samples  int     `json:"samples"`
	Hits     int     `json:"hits"`
	Misses   int     `json:"misses"`
	Excluded int     `json:"excluded"`
	HitRate  float64 `json:"hit_rate"`
	AvgOdds  float64 `json:"avg_odds"`
}

func (s *Server) handleOddsRanges(w http.ResponseWriter, r *http.Request) {
	if s.pool == nil {
		writeError(w, r, http.StatusServiceUnavailable, "not_ready", "database is not configured")
		return
	}

	market := r.URL.Query().Get("market")
	if market == "" {
		market = "match_result"
	}
	width := 0.1
	if raw := r.URL.Query().Get("bucket"); raw != "" {
		parsed, err := strconv.ParseFloat(raw, 64)
		if err != nil || (parsed != 0.05 && parsed != 0.1) {
			writeError(w, r, http.StatusBadRequest, "invalid_bucket", "bucket must be 0.05 or 0.1")
			return
		}
		width = parsed
	}

	var line *float64
	if raw := r.URL.Query().Get("line"); raw != "" {
		parsed, err := strconv.ParseFloat(raw, 64)
		if err != nil {
			writeError(w, r, http.StatusBadRequest, "invalid_line", "line must be a number")
			return
		}
		line = &parsed
	}

	rows, err := s.pool.Query(r.Context(), `
		WITH priced AS (
			SELECT se.id, latest.decimal_odds::float8 AS odds, st.status
			FROM market_selections se
			JOIN markets m ON m.id = se.market_id
			JOIN market_types mt ON mt.id = m.market_type_id
			JOIN selection_settlements st ON st.selection_id = se.id
			JOIN LATERAL (
				SELECT decimal_odds FROM odds_snapshots o
				WHERE o.selection_id = se.id
				ORDER BY o.captured_at DESC
				LIMIT 1
			) latest ON TRUE
			WHERE mt.slug = $1 AND ($2::numeric IS NULL OR m.line = $2)
		)
		SELECT floor(odds / $3) * $3 AS low,
		       count(*)::int,
		       count(*) FILTER (WHERE status = 'hit')::int,
		       count(*) FILTER (WHERE status = 'miss')::int,
		       count(*) FILTER (WHERE status IN ('push', 'void', 'needs_data', 'pending', 'not_gradable'))::int,
		       avg(odds)::float8
		FROM priced
		GROUP BY low
		ORDER BY low`, market, line, width)
	if err != nil {
		s.log.ErrorContext(r.Context(), "query odds ranges", "error", err.Error())
		writeError(w, r, http.StatusInternalServerError, "query_failed", "could not load analytics")
		return
	}
	defer rows.Close()

	buckets := make([]oddsBucket, 0, 12)
	total := oddsBucket{}
	for rows.Next() {
		var b oddsBucket
		if err := rows.Scan(&b.Low, &b.Samples, &b.Hits, &b.Misses, &b.Excluded, &b.AvgOdds); err != nil {
			s.log.ErrorContext(r.Context(), "scan odds range", "error", err.Error())
			writeError(w, r, http.StatusInternalServerError, "query_failed", "could not load analytics")
			return
		}
		b.Low = math.Round(b.Low*1000) / 1000
		b.High = math.Round((b.Low+width)*1000) / 1000
		if settled := b.Hits + b.Misses; settled > 0 {
			b.HitRate = float64(b.Hits) / float64(settled)
		}
		total.Samples += b.Samples
		total.Hits += b.Hits
		total.Misses += b.Misses
		total.Excluded += b.Excluded
		buckets = append(buckets, b)
	}
	if total.Hits+total.Misses > 0 {
		total.HitRate = float64(total.Hits) / float64(total.Hits+total.Misses)
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"market":   market,
		"line":     line,
		"bucket":   width,
		"settled":  total.Hits + total.Misses,
		"excluded": total.Excluded,
		"total":    total,
		"buckets":  buckets,
	})
}
