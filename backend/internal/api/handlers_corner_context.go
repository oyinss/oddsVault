package api

import (
	"context"
	"net/http"
	"strconv"
	"time"
)

type formMatch struct {
	EventID        int64  `json:"event_id"`
	Date           string `json:"date"`
	Opponent       string `json:"opponent"`
	Home           bool   `json:"home"`
	CornersFor     int    `json:"corners_for"`
	CornersAgainst int    `json:"corners_against"`
	Total          int    `json:"total_corners"`
}

type teamForm struct {
	Team       teamResponse       `json:"team"`
	Matches    []formMatch        `json:"matches"`
	Sample     int                `json:"sample"`
	AvgFor     float64            `json:"avg_for"`
	AvgAgainst float64            `json:"avg_against"`
	AvgTotal   float64            `json:"avg_total"`
	OverRates  map[string]float64 `json:"over_rates"`
}

type cornerContext struct {
	Event        eventResponse     `json:"event"`
	Line         float64           `json:"line"`
	Metrics      *liveCornerMatch  `json:"metrics"`
	HomeForm     teamForm          `json:"home_form"`
	AwayForm     teamForm          `json:"away_form"`
	Points       []cornerPoint     `json:"points"`
	BetHistory   *cornerBetHistory `json:"bet_history,omitempty"`
	CoverageNote string            `json:"coverage_note"`
}

// eventCompetitionID reads the league id behind one event.
func (s *Server) eventCompetitionID(ctx context.Context, eventID int64) (int64, error) {
	var id int64
	err := s.pool.QueryRow(ctx,
		`SELECT competition_id FROM events WHERE id = $1`, eventID).Scan(&id)
	return id, err
}

// handleCornerContext returns the recent corner record for both teams plus the
// timeline recorded while this match has been live. The sample is small by
// design: five matches, labelled as such.
func (s *Server) handleCornerContext(w http.ResponseWriter, r *http.Request) {
	if s.pool == nil {
		writeError(w, r, http.StatusServiceUnavailable, "not_ready", "database is not configured")
		return
	}
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		writeError(w, r, http.StatusBadRequest, "invalid_id", "event id must be a positive integer")
		return
	}

	line := 10.0
	if raw := r.URL.Query().Get("line"); raw != "" {
		if parsed, err := strconv.ParseFloat(raw, 64); err == nil && parsed > 0 && parsed <= 40 {
			line = parsed
		}
	}

	var out cornerContext
	out.Line = line

	row := s.pool.QueryRow(r.Context(), eventSelect+` WHERE e.id = $1`, id)
	event, err := scanEvent(row)
	if err != nil {
		writeError(w, r, http.StatusNotFound, "not_found", "event not found")
		return
	}
	out.Event = event

	metrics, err := s.cornerMatch(r.Context(), id, line)
	if err != nil {
		s.log.ErrorContext(r.Context(), "corner metrics", "error", err.Error())
	} else if metrics != nil {
		out.Metrics = metrics
	}

	homeForm, err := s.teamCornerForm(r.Context(), event.Home.ID)
	if err != nil {
		s.log.ErrorContext(r.Context(), "home corner form", "error", err.Error())
	}
	awayForm, err := s.teamCornerForm(r.Context(), event.Away.ID)
	if err != nil {
		s.log.ErrorContext(r.Context(), "away corner form", "error", err.Error())
	}
	homeForm.Team = event.Home
	awayForm.Team = event.Away
	out.HomeForm = homeForm
	out.AwayForm = awayForm

	points := make([]cornerPoint, 0, 32)
	if rows, err := s.pool.Query(r.Context(), `
		SELECT minute, total_corners, captured_at
		FROM corner_snapshots WHERE event_id = $1
		ORDER BY captured_at ASC`, id); err == nil {
		defer rows.Close()
		for rows.Next() {
			var point cornerPoint
			var captured time.Time
			if err := rows.Scan(&point.Minute, &point.Total, &captured); err != nil {
				continue
			}
			point.CapturedAt = captured.UTC().Format(time.RFC3339)
			points = append(points, point)
		}
	}
	out.Points = points

	// The user's own record in this spot, from the bet journal. It never mixes
	// accumulator money into a single match: only one-leg slips carry P/L here.
	history := &cornerBetHistory{}
	if competitionID, err := s.eventCompetitionID(r.Context(), id); err == nil {
		if record, err := s.cornerBetsByCompetition(r.Context(), competitionID); err == nil {
			// Legs whose fixture never resolved can still be attributed to a
			// league when their competition text is an exact identity match.
			if leagueOnly, err := s.leagueOnlyCornerLegs(r.Context()); err == nil {
				for _, leg := range leagueOnly[competitionID] {
					addLeagueLeg(record, leg)
				}
			}
			history.Competition = record
		} else {
			s.log.ErrorContext(r.Context(), "competition bet history", "error", err.Error())
		}
	}
	if record, err := s.cornerBetsByTeam(r.Context(), event.Home.ID); err == nil {
		history.Home = record
	} else {
		s.log.ErrorContext(r.Context(), "home bet history", "error", err.Error())
	}
	if record, err := s.cornerBetsByTeam(r.Context(), event.Away.ID); err == nil {
		history.Away = record
	} else {
		s.log.ErrorContext(r.Context(), "away bet history", "error", err.Error())
	}
	if journal, err := s.cornerJournalCounts(r.Context()); err == nil {
		if leagueOnly, legErr := s.leagueOnlyCornerLegs(r.Context()); legErr == nil {
			for _, legs := range leagueOnly {
				journal.LeagueLegs += len(legs)
			}
		}
		history.Journal = journal
	} else {
		s.log.ErrorContext(r.Context(), "journal corner counts", "error", err.Error())
	}
	history.Note = "Counts are corner-total legs from your own slips that are linked to these matches. P/L covers one-leg slips only: an accumulator's profit cannot be attributed to one match."
	out.BetHistory = history

	out.CoverageNote = "Recent form counts only finished matches with corner statistics stored. A team with no rows has no corner data in our coverage, not zero corners."
	writeJSON(w, http.StatusOK, out)
}

// teamCornerForm reads the last five finished matches with corners for a team.
func (s *Server) teamCornerForm(ctx context.Context, teamID int64) (teamForm, error) {
	form := teamForm{Matches: []formMatch{}, OverRates: map[string]float64{}}

	rows, err := s.pool.Query(ctx, `
		SELECT e.id, e.start_date, opp.name, (ep.role = 'home'),
		       s.corners, os.corners
		FROM event_participants ep
		JOIN events e ON e.id = ep.event_id AND e.status = 'finished'
		JOIN team_match_stats s ON s.event_id = e.id AND s.team_id = ep.team_id
		JOIN event_participants ope ON ope.event_id = e.id AND ope.role <> ep.role
		JOIN team_match_stats os ON os.event_id = e.id AND os.team_id = ope.team_id
		JOIN teams opp ON opp.id = ope.team_id
		WHERE ep.team_id = $1
		  AND s.corners IS NOT NULL AND os.corners IS NOT NULL
		ORDER BY e.start_date DESC, e.id DESC
		LIMIT 5`, teamID)
	if err != nil {
		return form, err
	}
	defer rows.Close()

	lines := []float64{8.5, 9.5, 10.5, 11.5}
	over := map[float64]int{}
	overAll := map[float64]int{}

	var sumFor, sumAgainst, sumTotal float64
	for rows.Next() {
		var item formMatch
		var day time.Time
		if err := rows.Scan(&item.EventID, &day, &item.Opponent, &item.Home,
			&item.CornersFor, &item.CornersAgainst); err != nil {
			return form, err
		}
		item.Date = day.Format("2006-01-02")
		item.Total = item.CornersFor + item.CornersAgainst
		form.Matches = append(form.Matches, item)
		sumFor += float64(item.CornersFor)
		sumAgainst += float64(item.CornersAgainst)
		sumTotal += float64(item.Total)
		for _, line := range lines {
			overAll[line]++
			if float64(item.Total) > line {
				over[line]++
			}
		}
	}
	if err := rows.Err(); err != nil {
		return form, err
	}

	form.Sample = len(form.Matches)
	if form.Sample > 0 {
		n := float64(form.Sample)
		form.AvgFor = round2(sumFor / n)
		form.AvgAgainst = round2(sumAgainst / n)
		form.AvgTotal = round2(sumTotal / n)
	}
	for _, line := range lines {
		if overAll[line] > 0 {
			form.OverRates[strconv.FormatFloat(line, 'f', 1, 64)] =
				round2(float64(over[line]) / float64(overAll[line]))
		}
	}
	return form, nil
}
