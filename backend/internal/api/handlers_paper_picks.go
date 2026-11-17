package api

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"net/http"
	"strconv"
	"time"

	"github.com/oyins/oddsvault/backend/internal/store"
)

// Paper picks are corner selections recorded by hand with no stake. They exist
// to collect decision data, so the board context at the moment of recording is
// frozen with the pick and settlement runs when the match publishes its final
// corner count.

type paperPickRequest struct {
	EventID  int64   `json:"event_id"`
	Provider string  `json:"provider"`
	Side     string  `json:"side"`
	Line     float64 `json:"line"`
	Odds     float64 `json:"odds"`
}

// paperPickCurrent is the match state now, so a pending pick can be followed
// instead of only remembered. to_line means "needed" for an over and "room
// left" for an under.
type paperPickCurrent struct {
	Live             bool     `json:"live"`
	Status           string   `json:"status"`
	HomeScore        *int     `json:"home_score"`
	AwayScore        *int     `json:"away_score"`
	Minute           *int     `json:"minute"`
	HomeCorners      *int     `json:"home_corners"`
	AwayCorners      *int     `json:"away_corners"`
	TotalCorners     *int     `json:"total_corners"`
	PacePerMinute    *float64 `json:"pace_per_minute"`
	ProjectedTotal   *float64 `json:"projected_total"`
	ProjectionLow    *float64 `json:"projection_low"`
	ProjectionHigh   *float64 `json:"projection_high"`
	RemainingMinutes *int     `json:"remaining_minutes"`
	ToLine           *int     `json:"to_line"`
	NeededPerMinute  *float64 `json:"needed_per_minute"`
	CapturedAt       *string  `json:"captured_at"`
}

type paperPickResponse struct {
	ID              int64             `json:"id"`
	EventID         int64             `json:"event_id"`
	Competition     string            `json:"competition"`
	Home            string            `json:"home"`
	Away            string            `json:"away"`
	HomeTeamID      int64             `json:"home_team_id"`
	AwayTeamID      int64             `json:"away_team_id"`
	CurrentStatus   string            `json:"event_status"`
	Kickoff         *string           `json:"kickoff"`
	Provider        string            `json:"provider"`
	ProviderSlug    string            `json:"provider_slug"`
	Side            string            `json:"side"`
	Line            float64           `json:"line"`
	Odds            float64           `json:"odds"`
	Minute          *int              `json:"minute"`
	HomeScore       *int              `json:"home_score_at_save"`
	AwayScore       *int              `json:"away_score_at_save"`
	TotalCorners    *int              `json:"total_corners"`
	HomeCorners     *int              `json:"home_corners"`
	AwayCorners     *int              `json:"away_corners"`
	PacePerMinute   *float64          `json:"pace_per_minute"`
	ProjectedTotal  *float64          `json:"projected_total"`
	ProjectionLow   *float64          `json:"projection_low"`
	ProjectionHigh  *float64          `json:"projection_high"`
	RecOverLine     *float64          `json:"rec_over_line"`
	RecUnderLine    *float64          `json:"rec_under_line"`
	Needed          *int              `json:"needed"`
	NeededPerMinute *float64          `json:"needed_per_minute"`
	ModelRisk       bool              `json:"model_risk"`
	EarlyPace       bool              `json:"early_pace"`
	EventStatus     string            `json:"event_status_at_save"`
	Status          string            `json:"status"`
	FinalTotal      *int              `json:"final_total"`
	SettlementRule  *string           `json:"settlement_rule"`
	CreatedAt       string            `json:"created_at"`
	SettledAt       *string           `json:"settled_at"`
	DataCutoffAt    string            `json:"data_cutoff_at"`
	RecoveredAt     *string           `json:"recovered_at,omitempty"`
	Current         *paperPickCurrent `json:"current,omitempty"`
}

func toPaperPickResponse(p store.PaperPick) paperPickResponse {
	out := paperPickResponse{
		ID: p.ID, EventID: p.EventID, Competition: p.Competition, Home: p.Home, Away: p.Away,
		HomeTeamID: p.HomeTeamID, AwayTeamID: p.AwayTeamID, CurrentStatus: p.CurrentStatus,
		Provider: p.Provider, ProviderSlug: p.ProviderSlug,
		Side: p.Side, Line: p.Line, Odds: p.Odds,
		Minute: p.Minute, HomeScore: p.HomeScore, AwayScore: p.AwayScore,
		TotalCorners: p.TotalCorners, HomeCorners: p.HomeCorners, AwayCorners: p.AwayCorners,
		PacePerMinute: p.PacePerMinute, ProjectedTotal: p.ProjectedTotal,
		ProjectionLow: p.ProjectionLow, ProjectionHigh: p.ProjectionHigh,
		RecOverLine: p.RecOverLine, RecUnderLine: p.RecUnderLine,
		Needed: p.Needed, NeededPerMinute: p.NeededPerMinute,
		ModelRisk: p.ModelRisk, EarlyPace: p.EarlyPace,
		EventStatus: p.EventStatus, Status: p.Status, FinalTotal: p.FinalTotal,
		SettlementRule: p.SettlementRule,
		CreatedAt:      p.CreatedAt.UTC().Format(time.RFC3339),
		DataCutoffAt:   p.DataCutoffAt.UTC().Format(time.RFC3339),
	}
	if p.RecoveredAt != nil {
		formatted := p.RecoveredAt.UTC().Format(time.RFC3339)
		out.RecoveredAt = &formatted
	}
	if p.Kickoff != nil {
		formatted := p.Kickoff.UTC().Format(time.RFC3339)
		out.Kickoff = &formatted
	}
	if p.SettledAt != nil {
		formatted := p.SettledAt.UTC().Format(time.RFC3339)
		out.SettledAt = &formatted
	}
	return out
}

// handleCreatePaperPick records one manual corner pick. The context comes from
// the same metrics the board shows, not from the request body.
func (s *Server) handleCreatePaperPick(w http.ResponseWriter, r *http.Request) {
	if s.pool == nil {
		writeError(w, r, http.StatusServiceUnavailable, "not_ready", "database is not configured")
		return
	}

	var body paperPickRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&body); err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid_body", "expected a JSON body")
		return
	}
	if body.EventID <= 0 {
		writeError(w, r, http.StatusBadRequest, "invalid_event", "event_id is required")
		return
	}
	if body.Provider == "" {
		writeError(w, r, http.StatusBadRequest, "invalid_provider", "choose the bookmaker that offered the price")
		return
	}
	var providerName string
	if err := s.pool.QueryRow(r.Context(),
		`SELECT name FROM bookmakers WHERE slug = $1 AND active`, body.Provider).Scan(&providerName); err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid_provider", "unknown bookmaker")
		return
	}

	if body.Side != "over" && body.Side != "under" {
		writeError(w, r, http.StatusBadRequest, "invalid_side", "side must be over or under")
		return
	}
	if body.Line <= 0 || body.Line > 40 {
		writeError(w, r, http.StatusBadRequest, "invalid_line", "line must be between 0 and 40")
		return
	}
	if body.Odds <= 1 || body.Odds > 1000 {
		writeError(w, r, http.StatusBadRequest, "invalid_odds", "odds must be greater than 1")
		return
	}

	event, err := s.cornerMatch(r.Context(), body.EventID, body.Line)
	if err != nil {
		s.log.ErrorContext(r.Context(), "paper pick event", "error", err.Error())
		writeError(w, r, http.StatusInternalServerError, "query_failed", "could not read the match")
		return
	}
	if event == nil {
		writeError(w, r, http.StatusNotFound, "not_found", "event not found")
		return
	}

	var eventStatus string
	if err := s.pool.QueryRow(r.Context(),
		`SELECT status FROM events WHERE id = $1`, body.EventID).Scan(&eventStatus); err != nil {
		writeError(w, r, http.StatusNotFound, "not_found", "event not found")
		return
	}

	importer := store.NewImporter(s.pool, s.log)
	pick, err := importer.CreatePaperPick(r.Context(), store.PaperPickInput{
		EventID:         body.EventID,
		CompetitionID:   event.CompetitionID,
		HomeTeamID:      event.Home.ID,
		AwayTeamID:      event.Away.ID,
		ProviderSlug:    body.Provider,
		Side:            body.Side,
		Line:            body.Line,
		Odds:            body.Odds,
		Minute:          event.Minute,
		HomeScore:       event.HomeScore,
		AwayScore:       event.AwayScore,
		HomeCorners:     event.HomeCorners,
		AwayCorners:     event.AwayCorners,
		TotalCorners:    event.TotalCorners,
		PacePerMinute:   event.CornersPerMin,
		ProjectedTotal:  event.ProjectedTotal,
		ProjectionLow:   event.ProjectionLow,
		ProjectionHigh:  event.ProjectionHigh,
		RecOverLine:     event.RecOverLine,
		RecUnderLine:    event.RecUnderLine,
		Needed:          event.Needed,
		NeededPerMinute: event.NeededPerMin,
		ModelRisk:       event.ModelRisk,
		EarlyPace:       event.EarlyPace,
		EventStatus:     eventStatus,
		DataCutoffAt:    time.Now().UTC(),
	})
	if err != nil {
		s.log.ErrorContext(r.Context(), "create paper pick", "error", err.Error())
		writeError(w, r, http.StatusInternalServerError, "save_failed", "could not save the pick")
		return
	}

	pick.Competition = event.Competition
	pick.Home = event.Home.Name
	pick.Away = event.Away.Name
	pick.Provider = providerName

	created := []paperPickResponse{toPaperPickResponse(pick)}
	s.attachPaperPickCurrent(r.Context(), created)
	writeJSON(w, http.StatusCreated, created[0])
}

// handlePaperPicks lists recent recordings with a small summary.
func (s *Server) handlePaperPicks(w http.ResponseWriter, r *http.Request) {
	if s.pool == nil {
		writeError(w, r, http.StatusServiceUnavailable, "not_ready", "database is not configured")
		return
	}

	limit := 200
	if raw := r.URL.Query().Get("limit"); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil {
			limit = parsed
		}
	}

	importer := store.NewImporter(s.pool, s.log)
	picks, err := importer.ListPaperPicks(r.Context(), limit)
	if err != nil {
		s.log.ErrorContext(r.Context(), "list paper picks", "error", err.Error())
		writeError(w, r, http.StatusInternalServerError, "query_failed", "could not read paper picks")
		return
	}

	summary := map[string]int{"pending": 0, "won": 0, "lost": 0, "push": 0, "void": 0}
	items := make([]paperPickResponse, 0, len(picks))
	for _, pick := range picks {
		summary[pick.Status]++
		items = append(items, toPaperPickResponse(pick))
	}

	s.attachPaperPickCurrent(r.Context(), items)

	writeJSON(w, http.StatusOK, map[string]any{
		"count":   len(items),
		"summary": summary,
		"picks":   items,
	})
}

// handleUpdatePaperPick corrects a recording that has not settled. The board
// context stays frozen; only the side, line, price and bookmaker change.
func (s *Server) handleUpdatePaperPick(w http.ResponseWriter, r *http.Request) {
	if s.pool == nil {
		writeError(w, r, http.StatusServiceUnavailable, "not_ready", "database is not configured")
		return
	}
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		writeError(w, r, http.StatusBadRequest, "invalid_id", "pick id must be a positive integer")
		return
	}

	var body paperPickRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&body); err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid_body", "expected a JSON body")
		return
	}
	if body.Provider == "" {
		writeError(w, r, http.StatusBadRequest, "invalid_provider", "choose the bookmaker that offered the price")
		return
	}
	var providerName string
	if err := s.pool.QueryRow(r.Context(),
		`SELECT name FROM bookmakers WHERE slug = $1 AND active`, body.Provider).Scan(&providerName); err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid_provider", "unknown bookmaker")
		return
	}
	if body.Side != "over" && body.Side != "under" {
		writeError(w, r, http.StatusBadRequest, "invalid_side", "side must be over or under")
		return
	}
	if body.Line <= 0 || body.Line > 40 {
		writeError(w, r, http.StatusBadRequest, "invalid_line", "line must be between 0 and 40")
		return
	}
	if body.Odds <= 1 || body.Odds > 1000 {
		writeError(w, r, http.StatusBadRequest, "invalid_odds", "odds must be greater than 1")
		return
	}

	importer := store.NewImporter(s.pool, s.log)
	pick, err := importer.UpdatePaperPick(r.Context(), id, body.Provider, body.Side, body.Line, body.Odds)
	if err != nil {
		if errors.Is(err, store.ErrPaperPickSettled) {
			writeError(w, r, http.StatusConflict, "already_settled",
				"this pick already settled, so its side and line are part of the record")
			return
		}
		s.log.ErrorContext(r.Context(), "update paper pick", "error", err.Error())
		writeError(w, r, http.StatusInternalServerError, "update_failed", "could not update the pick")
		return
	}
	pick.Provider = providerName

	items := []paperPickResponse{toPaperPickResponse(pick)}
	s.attachPaperPickCurrent(r.Context(), items)
	writeJSON(w, http.StatusOK, items[0])
}

// handleDeletePaperPick removes a mistyped recording.
func (s *Server) handleDeletePaperPick(w http.ResponseWriter, r *http.Request) {
	if s.pool == nil {
		writeError(w, r, http.StatusServiceUnavailable, "not_ready", "database is not configured")
		return
	}
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		writeError(w, r, http.StatusBadRequest, "invalid_id", "pick id must be a positive integer")
		return
	}
	importer := store.NewImporter(s.pool, s.log)
	found, err := importer.DeletePaperPick(r.Context(), id)
	if err != nil {
		s.log.ErrorContext(r.Context(), "delete paper pick", "error", err.Error())
		writeError(w, r, http.StatusInternalServerError, "delete_failed", "could not delete the pick")
		return
	}
	if !found {
		writeError(w, r, http.StatusNotFound, "not_found", "pick not found")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"deleted": true, "id": id})
}

// handlePaperPickProviders lists the bookmakers a pick can name, so the form
// offers a choice instead of a free-text field.
func (s *Server) handlePaperPickProviders(w http.ResponseWriter, r *http.Request) {
	if s.pool == nil {
		writeError(w, r, http.StatusServiceUnavailable, "not_ready", "database is not configured")
		return
	}
	rows, err := s.pool.Query(r.Context(),
		`SELECT slug, name FROM bookmakers WHERE active ORDER BY name`)
	if err != nil {
		s.log.ErrorContext(r.Context(), "paper pick providers", "error", err.Error())
		writeError(w, r, http.StatusInternalServerError, "query_failed", "could not read bookmakers")
		return
	}
	defer rows.Close()

	type provider struct {
		Slug string `json:"slug"`
		Name string `json:"name"`
	}
	out := make([]provider, 0, 8)
	for rows.Next() {
		var item provider
		if err := rows.Scan(&item.Slug, &item.Name); err != nil {
			writeError(w, r, http.StatusInternalServerError, "query_failed", "could not read bookmakers")
			return
		}
		out = append(out, item)
	}
	writeJSON(w, http.StatusOK, map[string]any{"providers": out})
}

// attachPaperPicks marks board rows that already carry a pending paper pick.
func (s *Server) attachPaperPicks(ctx context.Context, matches []liveCornerMatch) {
	if len(matches) == 0 {
		return
	}
	rows, err := s.pool.Query(ctx, `
		SELECT p.event_id, p.id, p.side, p.line, p.odds, b.name, b.slug
		FROM paper_picks p
		JOIN bookmakers b ON b.slug = p.provider_slug
		WHERE p.status = 'pending'`)
	if err != nil {
		s.log.ErrorContext(ctx, "board paper picks", "error", err.Error())
		return
	}
	defer rows.Close()

	byEvent := make(map[int64][]paperPickBadge)
	for rows.Next() {
		var eventID int64
		var badge paperPickBadge
		if err := rows.Scan(&eventID, &badge.ID, &badge.Side, &badge.Line, &badge.Odds,
			&badge.Provider, &badge.ProviderSlug); err != nil {
			s.log.ErrorContext(ctx, "scan board paper pick", "error", err.Error())
			return
		}
		byEvent[eventID] = append(byEvent[eventID], badge)
	}
	for index := range matches {
		if picks, ok := byEvent[matches[index].EventID]; ok {
			matches[index].PaperPicks = picks
		}
	}
}

// attachPaperPickCurrent fills the live/final state for a list of picks with one
// query for all events.
func (s *Server) attachPaperPickCurrent(ctx context.Context, picks []paperPickResponse) {
	if len(picks) == 0 {
		return
	}
	ids := make([]int64, 0, len(picks))
	seen := make(map[int64]bool, len(picks))
	for _, pick := range picks {
		if seen[pick.EventID] {
			continue
		}
		seen[pick.EventID] = true
		ids = append(ids, pick.EventID)
	}

	rows, err := s.pool.Query(ctx, `
		SELECT e.id, e.status, COALESCE(l.detail, ''), l.captured_at,
		       l.home_score, l.away_score, hs.corners, as_.corners
		FROM events e
		LEFT JOIN event_live_states l ON l.event_id = e.id
		LEFT JOIN event_participants hp ON hp.event_id = e.id AND hp.role = 'home'
		LEFT JOIN event_participants ap ON ap.event_id = e.id AND ap.role = 'away'
		LEFT JOIN team_match_stats hs ON hs.event_id = e.id AND hs.team_id = hp.team_id
		LEFT JOIN team_match_stats as_ ON as_.event_id = e.id AND as_.team_id = ap.team_id
		WHERE e.id = ANY($1)`, ids)
	if err != nil {
		s.log.ErrorContext(ctx, "paper pick current", "error", err.Error())
		return
	}
	defer rows.Close()

	type state struct {
		status     string
		detail     string
		capturedAt *time.Time
		homeScore  *int
		awayScore  *int
		home       *int
		away       *int
	}
	states := make(map[int64]state, len(ids))
	for rows.Next() {
		var id int64
		var item state
		if err := rows.Scan(&id, &item.status, &item.detail, &item.capturedAt,
			&item.homeScore, &item.awayScore, &item.home, &item.away); err != nil {
			s.log.ErrorContext(ctx, "scan paper pick current", "error", err.Error())
			return
		}
		states[id] = item
	}

	for index := range picks {
		pick := &picks[index]
		item, ok := states[pick.EventID]
		if !ok {
			continue
		}
		current := &paperPickCurrent{
			Live:        item.status == "live",
			Status:      item.status,
			HomeScore:   item.homeScore,
			AwayScore:   item.awayScore,
			HomeCorners: item.home,
			AwayCorners: item.away,
		}
		if item.capturedAt != nil {
			formatted := item.capturedAt.UTC().Format(time.RFC3339)
			current.CapturedAt = &formatted
		}
		if item.home != nil && item.away != nil {
			match := liveCornerMatch{
				Minute:      parseMinute(item.detail),
				HomeCorners: item.home,
				AwayCorners: item.away,
			}
			applyCornerMetrics(&match, pick.Line)
			current.Minute = match.Minute
			current.TotalCorners = match.TotalCorners
			current.PacePerMinute = match.CornersPerMin
			current.ProjectedTotal = match.ProjectedTotal
			current.ProjectionLow = match.ProjectionLow
			current.ProjectionHigh = match.ProjectionHigh
			current.RemainingMinutes = match.RemainingMins
			current.NeededPerMinute = match.NeededPerMin

			if current.TotalCorners != nil {
				total := *current.TotalCorners
				var toLine int
				if pick.Side == "under" {
					// Room left before the line is crossed.
					toLine = int(math.Floor(pick.Line)) - total
				} else {
					toLine = int(math.Ceil(pick.Line)) - total
				}
				if toLine < 0 {
					toLine = 0
				}
				current.ToLine = &toLine
			}
		}
		pick.Current = current
	}
}
