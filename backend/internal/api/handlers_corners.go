package api

import (
	"math"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

type cornerTeam struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	CrestURL string `json:"crest_url"`
}

type liveCornerMatch struct {
	EventID        int64      `json:"event_id"`
	Competition    string     `json:"competition"`
	Home           cornerTeam `json:"home"`
	Away           cornerTeam `json:"away"`
	HomeScore      *int       `json:"home_score"`
	AwayScore      *int       `json:"away_score"`
	Clock          string     `json:"clock"`
	Minute         *int       `json:"minute"`
	HomeCorners    *int       `json:"home_corners"`
	AwayCorners    *int       `json:"away_corners"`
	TotalCorners   *int       `json:"total_corners"`
	ProjectedTotal *float64   `json:"projected_total"`
	CornersPerMin  *float64   `json:"corners_per_minute"`
	RemainingMins  *int       `json:"remaining_minutes"`
	Needed         *int       `json:"needed"`
	NeededPerMin   *float64   `json:"needed_per_minute"`
	CapturedAt     *string    `json:"captured_at"`
	Pace           string     `json:"pace"`
}

// minutePattern only accepts a clock written with a minute mark, so a
// descriptive provider string like "Fri, November 13th at 12:00 PM EST" cannot
// be mistaken for the 13th minute.
var minutePattern = regexp.MustCompile(`(\d{1,3})(?:\+\d{1,2})?\s*['\x{2019}]`)

// parseMinute reads the elapsed minute from a provider clock. Only a clock with
// a minute mark is trusted: "HT" is treated as 45, anything else stays unknown.
func parseMinute(detail string) *int {
	trimmed := strings.TrimSpace(detail)
	if trimmed == "HT" || trimmed == "Half Time" {
		v := 45
		return &v
	}
	match := minutePattern.FindStringSubmatch(trimmed)
	if len(match) < 2 {
		return nil
	}
	value, err := strconv.Atoi(match[1])
	if err != nil || value < 0 || value > 120 {
		return nil
	}
	return &value
}

func round2(value float64) float64 {
	return math.Round(value*100) / 100
}

// handleLiveCorners returns corner counts and pace for football matches in
// progress. The projection is a straight extrapolation, labelled as such; it is
// not a prediction and no probability is invented.
func (s *Server) handleLiveCorners(w http.ResponseWriter, r *http.Request) {
	if s.pool == nil {
		writeError(w, r, http.StatusServiceUnavailable, "not_ready", "database is not configured")
		return
	}

	line := 10.0
	if raw := r.URL.Query().Get("line"); raw != "" {
		parsed, err := strconv.ParseFloat(raw, 64)
		if err != nil || parsed < 0 || parsed > 40 {
			writeError(w, r, http.StatusBadRequest, "invalid_line", "line must be between 0 and 40")
			return
		}
		line = parsed
	}

	rows, err := s.pool.Query(r.Context(), `
		SELECT e.id, c.name,
		       ht.id, ht.name, at.id, at.name,
		       l.home_score, l.away_score,
		       COALESCE(l.detail, ''), l.captured_at,
		       hs.corners, as_.corners
		FROM events e
		JOIN competitions c ON c.id = e.competition_id
		JOIN sports sp ON sp.id = e.sport_id
		JOIN event_participants hp ON hp.event_id = e.id AND hp.role = 'home'
		JOIN teams ht ON ht.id = hp.team_id
		JOIN event_participants ap ON ap.event_id = e.id AND ap.role = 'away'
		JOIN teams at ON at.id = ap.team_id
		LEFT JOIN event_live_states l ON l.event_id = e.id
		LEFT JOIN team_match_stats hs ON hs.event_id = e.id AND hs.team_id = hp.team_id
		LEFT JOIN team_match_stats as_ ON as_.event_id = e.id AND as_.team_id = ap.team_id
		WHERE e.status = 'live' AND sp.slug = 'football'
		ORDER BY COALESCE(e.start_time, e.start_date::timestamptz) ASC`)
	if err != nil {
		s.log.ErrorContext(r.Context(), "live corners", "error", err.Error())
		writeError(w, r, http.StatusInternalServerError, "query_failed", "could not load live corners")
		return
	}
	defer rows.Close()

	matches := make([]liveCornerMatch, 0, 16)
	for rows.Next() {
		var m liveCornerMatch
		var clock string
		var capturedAt *time.Time
		if err := rows.Scan(&m.EventID, &m.Competition,
			&m.Home.ID, &m.Home.Name, &m.Away.ID, &m.Away.Name,
			&m.HomeScore, &m.AwayScore,
			&clock, &capturedAt, &m.HomeCorners, &m.AwayCorners); err != nil {
			s.log.ErrorContext(r.Context(), "scan live corner", "error", err.Error())
			writeError(w, r, http.StatusInternalServerError, "query_failed", "could not load live corners")
			return
		}

		m.Clock = clock
		m.Minute = parseMinute(clock)
		m.Home.CrestURL = crestPath(m.Home.ID)
		m.Away.CrestURL = crestPath(m.Away.ID)
		if capturedAt != nil {
			formatted := capturedAt.UTC().Format(time.RFC3339)
			m.CapturedAt = &formatted
		}

		applyCornerMetrics(&m, line)
		matches = append(matches, m)
	}
	if err := rows.Err(); err != nil {
		s.log.ErrorContext(r.Context(), "iterate live corners", "error", err.Error())
		writeError(w, r, http.StatusInternalServerError, "query_failed", "could not load live corners")
		return
	}

	// Most projected corners first; matches without a projection go last.
	sort.SliceStable(matches, func(i, j int) bool {
		left, right := matches[i].ProjectedTotal, matches[j].ProjectedTotal
		switch {
		case left == nil && right == nil:
			return matches[i].EventID < matches[j].EventID
		case left == nil:
			return false
		case right == nil:
			return true
		default:
			return *left > *right
		}
	})

	writeJSON(w, http.StatusOK, map[string]any{
		"generated_at": time.Now().UTC().Format(time.RFC3339),
		"line":         line,
		"count":        len(matches),
		"matches":      matches,
	})
}

func ptrFloat(v float64) *float64 { return &v }

// applyCornerMetrics derives pace and projection from the counts already
// reported. It never estimates before the 15th minute: an early rate is noise,
// and the UI must not present it as a signal.
func applyCornerMetrics(m *liveCornerMatch, line float64) {
	m.Pace = "unknown"
	if m.HomeCorners == nil || m.AwayCorners == nil {
		return
	}

	total := *m.HomeCorners + *m.AwayCorners
	m.TotalCorners = &total
	if m.Minute == nil || *m.Minute < 15 {
		return
	}

	elapsed := float64(*m.Minute)
	perMinute := float64(total) / elapsed
	m.CornersPerMin = ptrFloat(round2(perMinute))
	m.ProjectedTotal = ptrFloat(round2(perMinute * 90))

	remaining := 90 - *m.Minute
	if remaining < 0 {
		remaining = 0
	}
	m.RemainingMins = &remaining

	needed := int(math.Ceil(line)) - total
	if needed < 0 {
		needed = 0
	}
	m.Needed = &needed
	if remaining == 0 {
		return
	}

	m.NeededPerMin = ptrFloat(round2(float64(needed) / float64(remaining)))
	// Pace compares the current rate with the rate required to clear the line.
	switch {
	case perMinute > *m.NeededPerMin*1.2:
		m.Pace = "above"
	case perMinute < *m.NeededPerMin*0.8:
		m.Pace = "below"
	default:
		m.Pace = "on_pace"
	}
}
