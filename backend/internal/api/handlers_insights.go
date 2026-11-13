package api

import (
	"net/http"
	"strconv"
)

type insightLine struct {
	Family    string   `json:"family"`
	Side      string   `json:"side"`
	Line      *float64 `json:"line"`
	Legs      int      `json:"legs"`
	Won       int      `json:"won"`
	Lost      int      `json:"lost"`
	Rate      float64  `json:"rate"`
	Verified  int      `json:"verified"`
	Disagrees int      `json:"disagreements"`
}

type insightRow struct {
	Label    string  `json:"label"`
	Legs     int     `json:"legs"`
	Won      int     `json:"won"`
	Lost     int     `json:"lost"`
	Rate     float64 `json:"rate"`
	Verified int     `json:"verified"`
}

type insightBucket struct {
	Low  float64  `json:"low"`
	High *float64 `json:"high"`
	Legs int      `json:"legs"`
	Won  int      `json:"won"`
	Lost int      `json:"lost"`
	Rate float64  `json:"rate"`
}

type insightStake struct {
	Label  string  `json:"label"`
	Bets   int     `json:"bets"`
	Staked float64 `json:"staked"`
	Return float64 `json:"returned"`
	PL     float64 `json:"profit_loss"`
}

type insightDisagreement struct {
	LegID    int64    `json:"leg_id"`
	EventRaw string   `json:"event_raw"`
	Side     string   `json:"side"`
	Line     *float64 `json:"line"`
	Provider string   `json:"provider_status"`
	Verified string   `json:"verified_status"`
	Note     string   `json:"note"`
	EventID  *int64   `json:"event_id"`
}

func rate(won, lost int) float64 {
	if won+lost == 0 {
		return 0
	}
	return float64(won) / float64(won+lost)
}

func (s *Server) handleBetInsights(w http.ResponseWriter, r *http.Request) {
	if s.pool == nil {
		writeError(w, r, http.StatusServiceUnavailable, "not_ready", "database is not configured")
		return
	}

	minSample := 5
	if raw := r.URL.Query().Get("min_sample"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 0 || parsed > 1000 {
			writeError(w, r, http.StatusBadRequest, "invalid_min_sample", "min_sample must be 0-1000")
			return
		}
		minSample = parsed
	}

	overall := struct {
		Bets          int     `json:"bets"`
		Staked        float64 `json:"staked"`
		Returned      float64 `json:"returned"`
		ProfitLoss    float64 `json:"profit_loss"`
		ROI           float64 `json:"roi"`
		SettledLegs   int     `json:"settled_legs"`
		VerifiedLegs  int     `json:"verified_legs"`
		Disagreements int     `json:"disagreements"`
	}{}

	_ = s.pool.QueryRow(r.Context(), `
		SELECT count(*), COALESCE(sum(stake), 0)::float8,
		       COALESCE(sum(actual_return), 0)::float8,
		       COALESCE(sum(profit_loss), 0)::float8
		FROM personal_bets`).Scan(&overall.Bets, &overall.Staked, &overall.Returned, &overall.ProfitLoss)
	if overall.Staked > 0 {
		overall.ROI = overall.ProfitLoss / overall.Staked
	}

	_ = s.pool.QueryRow(r.Context(), `
		SELECT count(*) FILTER (WHERE leg_status IN ('won', 'lost')),
		       count(*) FILTER (WHERE verified_status IS NOT NULL),
		       count(*) FILTER (WHERE verified_status IN ('hit', 'miss', 'push')
		         AND ((leg_status = 'won' AND verified_status <> 'hit')
		           OR (leg_status = 'lost' AND verified_status <> 'miss')))
		FROM personal_bet_legs`).Scan(&overall.SettledLegs, &overall.VerifiedLegs, &overall.Disagreements)

	lines := make([]insightLine, 0, 20)
	if rows, err := s.pool.Query(r.Context(), `
		SELECT market_family, market_side, market_line::float8,
		       count(*)::int,
		       count(*) FILTER (WHERE leg_status = 'won')::int,
		       count(*) FILTER (WHERE leg_status = 'lost')::int,
		       count(*) FILTER (WHERE verified_status IS NOT NULL)::int,
		       count(*) FILTER (WHERE verified_status IN ('hit', 'miss', 'push')
		         AND ((leg_status = 'won' AND verified_status <> 'hit')
		           OR (leg_status = 'lost' AND verified_status <> 'miss')))::int
		FROM personal_bet_legs
		WHERE market_family IS NOT NULL AND market_family <> 'unknown'
		GROUP BY 1, 2, 3
		ORDER BY count(*) DESC`); err == nil {
		defer rows.Close()
		for rows.Next() {
			var item insightLine
			if err := rows.Scan(&item.Family, &item.Side, &item.Line, &item.Legs,
				&item.Won, &item.Lost, &item.Verified, &item.Disagrees); err != nil {
				continue
			}
			item.Rate = rate(item.Won, item.Lost)
			if item.Legs >= minSample {
				lines = append(lines, item)
			}
		}
	}

	competitions := make([]insightRow, 0, 20)
	if rows, err := s.pool.Query(r.Context(), `
		SELECT COALESCE(competition_raw, 'unknown'), count(*)::int,
		       count(*) FILTER (WHERE leg_status = 'won')::int,
		       count(*) FILTER (WHERE leg_status = 'lost')::int,
		       count(*) FILTER (WHERE verified_status IS NOT NULL)::int
		FROM personal_bet_legs
		GROUP BY 1
		ORDER BY count(*) DESC`); err == nil {
		defer rows.Close()
		for rows.Next() {
			var item insightRow
			if err := rows.Scan(&item.Label, &item.Legs, &item.Won, &item.Lost, &item.Verified); err != nil {
				continue
			}
			item.Rate = rate(item.Won, item.Lost)
			if item.Legs >= minSample {
				competitions = append(competitions, item)
			}
		}
	}

	odds := make([]insightBucket, 0, 12)
	if rows, err := s.pool.Query(r.Context(), `
		SELECT floor(odds_taken / 0.25) * 0.25 AS low, count(*)::int,
		       count(*) FILTER (WHERE leg_status = 'won')::int,
		       count(*) FILTER (WHERE leg_status = 'lost')::int
		FROM personal_bet_legs
		WHERE odds_taken IS NOT NULL
		GROUP BY low ORDER BY low`); err == nil {
		defer rows.Close()
		for rows.Next() {
			var item insightBucket
			if err := rows.Scan(&item.Low, &item.Legs, &item.Won, &item.Lost); err != nil {
				continue
			}
			high := item.Low + 0.25
			item.High = &high
			item.Rate = rate(item.Won, item.Lost)
			odds = append(odds, item)
		}
	}

	stakes := make([]insightStake, 0, 5)
	if rows, err := s.pool.Query(r.Context(), `
		SELECT CASE
		         WHEN stake < 5000 THEN 'under 5k'
		         WHEN stake < 10000 THEN '5k-10k'
		         WHEN stake < 20000 THEN '10k-20k'
		         WHEN stake < 50000 THEN '20k-50k'
		         ELSE '50k+'
		       END AS bucket,
		       count(*)::int, COALESCE(sum(stake), 0)::float8,
		       COALESCE(sum(actual_return), 0)::float8,
		       COALESCE(sum(profit_loss), 0)::float8
		FROM personal_bets
		GROUP BY 1`); err == nil {
		defer rows.Close()
		for rows.Next() {
			var item insightStake
			if err := rows.Scan(&item.Label, &item.Bets, &item.Staked, &item.Return, &item.PL); err != nil {
				continue
			}
			stakes = append(stakes, item)
		}
	}

	disagreements := make([]insightDisagreement, 0, 8)
	if rows, err := s.pool.Query(r.Context(), `
		SELECT l.id, l.event_raw, COALESCE(l.market_side, ''),
		       l.market_line::float8, l.leg_status, l.verified_status,
		       COALESCE(l.verification_note, ''), l.event_id
		FROM personal_bet_legs l
		WHERE l.verified_status IN ('hit', 'miss', 'push')
		  AND ((l.leg_status = 'won' AND l.verified_status <> 'hit')
		    OR (l.leg_status = 'lost' AND l.verified_status <> 'miss'))
		ORDER BY l.id DESC`); err == nil {
		defer rows.Close()
		for rows.Next() {
			var item insightDisagreement
			if err := rows.Scan(&item.LegID, &item.EventRaw, &item.Side, &item.Line,
				&item.Provider, &item.Verified, &item.Note, &item.EventID); err != nil {
				continue
			}
			disagreements = append(disagreements, item)
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"min_sample":    minSample,
		"overall":       overall,
		"lines":         lines,
		"competitions":  competitions,
		"odds_buckets":  odds,
		"stakes":        stakes,
		"disagreements": disagreements,
	})
}
