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

type insightFactor struct {
	Label    string  `json:"label"`
	Bets     int     `json:"bets"`
	Staked   float64 `json:"staked"`
	Returned float64 `json:"returned"`
	PL       float64 `json:"profit_loss"`
	Won      int     `json:"won"`
	Lost     int     `json:"lost"`
	ROI      float64 `json:"roi"`
}

type insightFactorGroup struct {
	Group string          `json:"group"`
	Note  string          `json:"note"`
	Rows  []insightFactor `json:"rows"`
}

type insightCalibration struct {
	Bucket  string  `json:"bucket"`
	Samples int     `json:"samples"`
	Bias    float64 `json:"bias"`
	MAE     float64 `json:"mae"`
}

type insightTiming struct {
	Bucket         string  `json:"bucket"`
	Legs           int     `json:"legs"`
	Won            int     `json:"won"`
	Lost           int     `json:"lost"`
	Rate           float64 `json:"rate"`
	AvgMinutesLeft float64 `json:"avg_minutes_left"`
}

type insightBettor struct {
	Label    string  `json:"label"`
	Bets     int     `json:"bets"`
	Staked   float64 `json:"staked"`
	Returned float64 `json:"returned"`
	PL       float64 `json:"profit_loss"`
	Won      int     `json:"won"`
	Lost     int     `json:"lost"`
	ROI      float64 `json:"roi"`
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

	// Slip-level factors answer questions a leg table cannot: does bet type,
	// time of day or slip size change the outcome? Each row carries its bet
	// count so a thin bucket is visible as thin.
	factors := make([]insightFactorGroup, 0, 4)
	loadFactors := func(group, note, query string) {
		rows, err := s.pool.Query(r.Context(), query)
		if err != nil {
			return
		}
		defer rows.Close()
		out := make([]insightFactor, 0, 8)
		for rows.Next() {
			var item insightFactor
			if err := rows.Scan(&item.Label, &item.Bets, &item.Staked,
				&item.Returned, &item.PL, &item.Won, &item.Lost); err != nil {
				continue
			}
			if item.Staked > 0 {
				item.ROI = item.PL / item.Staked
			}
			out = append(out, item)
		}
		factors = append(factors, insightFactorGroup{Group: group, Note: note, Rows: out})
	}

	loadFactors("bet_type", "Singles against accumulators.",
		`SELECT bet_type, count(*)::int, COALESCE(sum(stake),0)::float8,
		        COALESCE(sum(actual_return),0)::float8, COALESCE(sum(profit_loss),0)::float8,
		        count(*) FILTER (WHERE status = 'won')::int,
		        count(*) FILTER (WHERE status = 'lost')::int
		 FROM personal_bets GROUP BY 1 ORDER BY count(*) DESC`)

	loadFactors("hour", "When the slip was placed, in your timezone.",
		`SELECT to_char(placed_at AT TIME ZONE 'Africa/Lagos', 'HH24') || ':00',
		        count(*)::int, COALESCE(sum(stake),0)::float8,
		        COALESCE(sum(actual_return),0)::float8, COALESCE(sum(profit_loss),0)::float8,
		        count(*) FILTER (WHERE status = 'won')::int,
		        count(*) FILTER (WHERE status = 'lost')::int
		 FROM personal_bets GROUP BY 1 ORDER BY 1`)

	loadFactors("slip_size", "How many legs a slip carried.",
		`SELECT CASE WHEN n = 1 THEN '1 leg' WHEN n = 2 THEN '2 legs'
		             WHEN n <= 4 THEN '3-4 legs' ELSE '5+ legs' END,
		        count(*)::int, COALESCE(sum(stake),0)::float8,
		        COALESCE(sum(actual_return),0)::float8, COALESCE(sum(profit_loss),0)::float8,
		        count(*) FILTER (WHERE status = 'won')::int,
		        count(*) FILTER (WHERE status = 'lost')::int
		 FROM (SELECT b.id, b.stake, b.actual_return, b.profit_loss, b.status,
		              count(l.id) AS n
		       FROM personal_bets b LEFT JOIN personal_bet_legs l ON l.personal_bet_id = b.id
		       GROUP BY b.id) slips
		 GROUP BY 1 ORDER BY 2 DESC`)

	// Fixtures are the closest honest proxy for team form: corner bets are not
	// attributable to one team, but the pairing is exact.
	fixtures := make([]insightRow, 0, 20)
	if rows, err := s.pool.Query(r.Context(), `
		SELECT ht.name || ' v ' || at.name, count(*)::int,
		       count(*) FILTER (WHERE l.leg_status = 'won')::int,
		       count(*) FILTER (WHERE l.leg_status = 'lost')::int,
		       count(*) FILTER (WHERE l.verified_status IS NOT NULL)::int
		FROM personal_bet_legs l
		JOIN event_participants hp ON hp.event_id = l.event_id AND hp.role = 'home'
		JOIN teams ht ON ht.id = hp.team_id
		JOIN event_participants ap ON ap.event_id = l.event_id AND ap.role = 'away'
		JOIN teams at ON at.id = ap.team_id
		GROUP BY 1
		HAVING count(*) >= 1
		ORDER BY count(*) DESC
		LIMIT 20`); err == nil {
		defer rows.Close()
		for rows.Next() {
			var item insightRow
			if err := rows.Scan(&item.Label, &item.Legs, &item.Won, &item.Lost, &item.Verified); err != nil {
				continue
			}
			item.Rate = rate(item.Won, item.Lost)
			fixtures = append(fixtures, item)
		}
	}

	// How wrong were the projections? One snapshot per event per minute bucket
	// keeps the poll frequency from weighting the result.
	calibration := make([]insightCalibration, 0, 8)
	if rows, err := s.pool.Query(r.Context(), `
		WITH finals AS (
			SELECT s.event_id, sum(s.corners)::float8 AS final
			FROM team_match_stats s
			JOIN events e ON e.id = s.event_id AND e.status = 'finished'
			GROUP BY 1
			HAVING count(*) >= 2
		),
		snaps AS (
			SELECT cs.event_id, cs.projected_total,
			       CASE WHEN cs.minute < 20 THEN '15-19'
			            WHEN cs.minute < 30 THEN '20-29'
			            WHEN cs.minute < 45 THEN '30-44'
			            WHEN cs.minute < 60 THEN '45-59'
			            WHEN cs.minute < 75 THEN '60-74'
			            ELSE '75+' END AS bucket,
			       row_number() OVER (
			           PARTITION BY cs.event_id,
			           CASE WHEN cs.minute < 20 THEN '15-19'
			                WHEN cs.minute < 30 THEN '20-29'
			                WHEN cs.minute < 45 THEN '30-44'
			                WHEN cs.minute < 60 THEN '45-59'
			                WHEN cs.minute < 75 THEN '60-74'
			                ELSE '75+' END
			           ORDER BY cs.captured_at
			       ) AS rn
			FROM corner_snapshots cs
			WHERE cs.projected_total IS NOT NULL
		)
		SELECT snaps.bucket, count(*)::int,
		       avg(snaps.projected_total - finals.final)::float8,
		       avg(abs(snaps.projected_total - finals.final))::float8
		FROM snaps
		JOIN finals ON finals.event_id = snaps.event_id
		WHERE snaps.rn = 1
		GROUP BY snaps.bucket
		ORDER BY snaps.bucket`); err == nil {
		defer rows.Close()
		for rows.Next() {
			var item insightCalibration
			if err := rows.Scan(&item.Bucket, &item.Samples, &item.Bias, &item.MAE); err != nil {
				continue
			}
			calibration = append(calibration, item)
		}
	}

	// When were the legs placed? Minute is derived from the stored kickoff, so
	// only legs linked to an event can answer it.
	timing := make([]insightTiming, 0, 8)
	if rows, err := s.pool.Query(r.Context(), `
		WITH placed AS (
			SELECT l.leg_status,
			       (EXTRACT(EPOCH FROM (b.placed_at - e.start_time)) / 60)::float8 AS minute
			FROM personal_bet_legs l
			JOIN personal_bets b ON b.id = l.personal_bet_id
			JOIN events e ON e.id = l.event_id
			WHERE b.placed_at IS NOT NULL AND e.start_time IS NOT NULL
			  AND l.leg_status IN ('won', 'lost')
		)
		SELECT CASE WHEN minute < 0 THEN 'pre-match'
		            WHEN minute < 15 THEN '0-14'
		            WHEN minute < 30 THEN '15-29'
		            WHEN minute < 45 THEN '30-44'
		            WHEN minute < 60 THEN '45-59'
		            WHEN minute < 75 THEN '60-74'
		            ELSE '75+' END AS bucket,
		       count(*)::int,
		       count(*) FILTER (WHERE leg_status = 'won')::int,
		       count(*) FILTER (WHERE leg_status = 'lost')::int,
		       avg(GREATEST(90 - minute, 0))::float8
		FROM placed
		GROUP BY 1
		ORDER BY 1`); err == nil {
		defer rows.Close()
		for rows.Next() {
			var item insightTiming
			if err := rows.Scan(&item.Bucket, &item.Legs, &item.Won, &item.Lost,
				&item.AvgMinutesLeft); err != nil {
				continue
			}
			item.Rate = rate(item.Won, item.Lost)
			timing = append(timing, item)
		}
	}

	bettors := make([]insightBettor, 0, 4)
	if rows, err := s.pool.Query(r.Context(), `
		SELECT COALESCE(placed_by, 'unknown'), count(*)::int,
		       COALESCE(sum(stake), 0)::float8,
		       COALESCE(sum(actual_return), 0)::float8,
		       COALESCE(sum(profit_loss), 0)::float8,
		       count(*) FILTER (WHERE status = 'won')::int,
		       count(*) FILTER (WHERE status = 'lost')::int
		FROM personal_bets
		GROUP BY 1
		ORDER BY 2 DESC`); err == nil {
		defer rows.Close()
		for rows.Next() {
			var item insightBettor
			if err := rows.Scan(&item.Label, &item.Bets, &item.Staked,
				&item.Returned, &item.PL, &item.Won, &item.Lost); err != nil {
				continue
			}
			if item.Staked > 0 {
				item.ROI = item.PL / item.Staked
			}
			bettors = append(bettors, item)
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"min_sample":         minSample,
		"overall":            overall,
		"lines":              lines,
		"competitions":       competitions,
		"odds_buckets":       odds,
		"stakes":             stakes,
		"disagreements":      disagreements,
		"factors":            factors,
		"fixtures":           fixtures,
		"corner_calibration": calibration,
		"bet_timing":         timing,
		"bettors":            bettors,
	})
}
