package api

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

type competitionResponse struct {
	Slug        string  `json:"slug"`
	Name        string  `json:"name"`
	Country     *string `json:"country"`
	Season      *string `json:"current_season"`
	SeasonStart *string `json:"season_started_on,omitempty"`
}

func (s *Server) handleCompetitions(w http.ResponseWriter, r *http.Request) {
	if s.pool == nil {
		writeError(w, r, http.StatusServiceUnavailable, "not_ready", "database is not configured")
		return
	}

	rows, err := s.pool.Query(r.Context(), `
		SELECT c.slug, c.name, co.name, se.name, se.start_date
		FROM competitions c
		JOIN sports sp ON sp.id = c.sport_id AND sp.slug = 'football'
		LEFT JOIN countries co ON co.id = c.country_id
		LEFT JOIN seasons se ON se.competition_id = c.id AND se.is_current
		ORDER BY c.name`)
	if err != nil {
		s.log.ErrorContext(r.Context(), "query competitions", "error", err.Error())
		writeError(w, r, http.StatusInternalServerError, "query_failed", "could not load competitions")
		return
	}
	defer rows.Close()

	items := make([]competitionResponse, 0, 8)
	for rows.Next() {
		var item competitionResponse
		var seasonStart *time.Time
		if err := rows.Scan(&item.Slug, &item.Name, &item.Country, &item.Season, &seasonStart); err != nil {
			s.log.ErrorContext(r.Context(), "scan competition", "error", err.Error())
			writeError(w, r, http.StatusInternalServerError, "query_failed", "could not load competitions")
			return
		}
		if seasonStart != nil {
			formatted := seasonStart.Format("2006-01-02")
			item.SeasonStart = &formatted
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		s.log.ErrorContext(r.Context(), "iterate competitions", "error", err.Error())
		writeError(w, r, http.StatusInternalServerError, "query_failed", "could not load competitions")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"competitions": items, "count": len(items)})
}

type teamResponse struct {
	ID       int64   `json:"id"`
	Name     string  `json:"name"`
	Slug     string  `json:"slug"`
	CrestURL *string `json:"crest_url"`
}

type resultResponse struct {
	HomeScore   *int    `json:"home_score"`
	AwayScore   *int    `json:"away_score"`
	HomeScoreHT *int    `json:"home_score_ht"`
	AwayScoreHT *int    `json:"away_score_ht"`
	Provider    string  `json:"provider"`
	CapturedAt  *string `json:"captured_at"`
}

type liveResponse struct {
	Status     string  `json:"status"`
	HomeScore  *int    `json:"home_score"`
	AwayScore  *int    `json:"away_score"`
	Detail     *string `json:"detail"`
	Provider   string  `json:"provider"`
	CapturedAt string  `json:"captured_at"`
}

type eventResponse struct {
	ID              int64           `json:"id"`
	Competition     string          `json:"competition"`
	CompetitionName string          `json:"competition_name"`
	Season          *string         `json:"season"`
	StartDate       *string         `json:"start_date"`
	StartTime       *string         `json:"start_time"`
	StartTimeLocal  *string         `json:"start_time_local"`
	Status          string          `json:"status"`
	Round           *string         `json:"round"`
	IdentityStatus  string          `json:"identity_status"`
	Home            teamResponse    `json:"home"`
	Away            teamResponse    `json:"away"`
	Result          *resultResponse `json:"result"`
	Live            *liveResponse   `json:"live"`
	UpdatedAt       string          `json:"updated_at"`
}

const eventSelect = `
	SELECT e.id, c.slug, c.name, se.name, e.start_date, e.start_time, e.start_time_local,
	       e.status, e.round_label, e.identity_status,
	       ht.id, ht.name, ht.slug, at.id, at.name, at.slug,
	       r.home_score, r.away_score, r.home_score_ht, r.away_score_ht,
	       r.provider, r.captured_at, e.updated_at,
	       l.status, l.home_score, l.away_score, l.detail, l.provider, l.captured_at,
	       hma.id, ama.id
	FROM events e
	JOIN competitions c ON c.id = e.competition_id
	LEFT JOIN seasons se ON se.id = e.season_id
	JOIN event_participants hp ON hp.event_id = e.id AND hp.role = 'home'
	JOIN teams ht ON ht.id = hp.team_id
	JOIN event_participants ap ON ap.event_id = e.id AND ap.role = 'away'
	JOIN teams at ON at.id = ap.team_id
	LEFT JOIN event_results r ON r.event_id = e.id
	LEFT JOIN event_live_states l ON l.event_id = e.id
	LEFT JOIN media_assets hma ON hma.entity_type = 'team' AND hma.entity_id = ht.id
	                          AND hma.asset_type = 'team_logo' AND hma.active
	LEFT JOIN media_assets ama ON ama.entity_type = 'team' AND ama.entity_id = at.id
	                          AND ama.asset_type = 'team_logo' AND ama.active`

func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	if s.pool == nil {
		writeError(w, r, http.StatusServiceUnavailable, "not_ready", "database is not configured")
		return
	}
	q := r.URL.Query()

	limit := 50
	if raw := q.Get("limit"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 1 || parsed > 200 {
			writeError(w, r, http.StatusBadRequest, "invalid_limit", "limit must be between 1 and 200")
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

	if slug := q.Get("competition"); slug != "" {
		add("c.slug = ?", slug)
	}
	if status := q.Get("status"); status != "" {
		switch status {
		case "scheduled", "live", "finished", "postponed", "cancelled", "abandoned", "unknown":
			add("e.status = ?", status)
		default:
			writeError(w, r, http.StatusBadRequest, "invalid_status", "unknown status filter")
			return
		}
	}
	if from := q.Get("from"); from != "" {
		date, err := time.Parse("2006-01-02", from)
		if err != nil {
			writeError(w, r, http.StatusBadRequest, "invalid_date", "from must be YYYY-MM-DD")
			return
		}
		add("e.start_date >= ?", date)
	}
	if to := q.Get("to"); to != "" {
		date, err := time.Parse("2006-01-02", to)
		if err != nil {
			writeError(w, r, http.StatusBadRequest, "invalid_date", "to must be YYYY-MM-DD")
			return
		}
		add("e.start_date <= ?", date)
	}

	orderBy := "abs(e.start_date - CURRENT_DATE), e.start_date ASC"
	switch q.Get("status") {
	case "finished":
		orderBy = "e.start_date DESC NULLS LAST, e.id DESC"
	case "scheduled":
		orderBy = "e.start_date ASC NULLS LAST, e.id ASC"
	}

	args = append(args, limit)
	query := eventSelect + `
		WHERE ` + strings.Join(conditions, " AND ") + `
		ORDER BY ` + orderBy + `
		LIMIT $` + strconv.Itoa(len(args))

	s.writeEventList(w, r, query, args, limit)
}

// handleLive returns events in progress or kicking off today.
func (s *Server) handleLive(w http.ResponseWriter, r *http.Request) {
	if s.pool == nil {
		writeError(w, r, http.StatusServiceUnavailable, "not_ready", "database is not configured")
		return
	}
	query := eventSelect + `
		WHERE e.status = 'live'
		   OR e.start_date BETWEEN CURRENT_DATE - 3 AND CURRENT_DATE + 7
		ORDER BY (e.status = 'live') DESC,
		         COALESCE(e.start_time, e.start_date::timestamptz) ASC
		LIMIT 60`
	s.writeEventList(w, r, query, nil, 60)
}

func (s *Server) writeEventList(w http.ResponseWriter, r *http.Request, query string, args []any, capacity int) {
	rows, err := s.pool.Query(r.Context(), query, args...)
	if err != nil {
		s.log.ErrorContext(r.Context(), "query events", "error", err.Error())
		writeError(w, r, http.StatusInternalServerError, "query_failed", "could not load events")
		return
	}
	defer rows.Close()

	events := make([]eventResponse, 0, capacity)
	for rows.Next() {
		item, err := scanEvent(rows)
		if err != nil {
			s.log.ErrorContext(r.Context(), "scan event", "error", err.Error())
			writeError(w, r, http.StatusInternalServerError, "query_failed", "could not load events")
			return
		}
		events = append(events, item)
	}
	if err := rows.Err(); err != nil {
		s.log.ErrorContext(r.Context(), "iterate events", "error", err.Error())
		writeError(w, r, http.StatusInternalServerError, "query_failed", "could not load events")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"events": events, "count": len(events)})
}

func (s *Server) handleEvent(w http.ResponseWriter, r *http.Request) {
	if s.pool == nil {
		writeError(w, r, http.StatusServiceUnavailable, "not_ready", "database is not configured")
		return
	}
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		writeError(w, r, http.StatusBadRequest, "invalid_id", "event id must be a positive integer")
		return
	}

	row := s.pool.QueryRow(r.Context(), eventSelect+` WHERE e.id = $1`, id)
	item, err := scanEvent(row)
	if err != nil {
		if isNoRows(err) {
			writeError(w, r, http.StatusNotFound, "not_found", "event not found")
			return
		}
		s.log.ErrorContext(r.Context(), "query event", "error", err.Error())
		writeError(w, r, http.StatusInternalServerError, "query_failed", "could not load event")
		return
	}

	links, err := s.eventProviderLinks(r, id)
	if err != nil {
		s.log.ErrorContext(r.Context(), "query event links", "error", err.Error())
	}
	markets, err := s.eventMarkets(r, id)
	if err != nil {
		s.log.ErrorContext(r.Context(), "query event markets", "error", err.Error())
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"event":          item,
		"provider_links": links,
		"markets":        markets,
	})
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanEvent(row rowScanner) (eventResponse, error) {
	var item eventResponse
	var startDate *time.Time
	var startTime *time.Time
	var season, startTimeLocal, round *string
	var homeScore, awayScore, homeHT, awayHT *int
	var resultProvider *string
	var resultCaptured *time.Time
	var updatedAt time.Time
	var liveStatus, liveProvider *string
	var liveHome, liveAway *int
	var liveDetail *string
	var liveCaptured *time.Time
	var homeCrest, awayCrest *int64

	err := row.Scan(
		&item.ID, &item.Competition, &item.CompetitionName, &season,
		&startDate, &startTime, &startTimeLocal, &item.Status, &round, &item.IdentityStatus,
		&item.Home.ID, &item.Home.Name, &item.Home.Slug,
		&item.Away.ID, &item.Away.Name, &item.Away.Slug,
		&homeScore, &awayScore, &homeHT, &awayHT,
		&resultProvider, &resultCaptured, &updatedAt,
		&liveStatus, &liveHome, &liveAway, &liveDetail, &liveProvider, &liveCaptured,
		&homeCrest, &awayCrest,
	)
	if err != nil {
		return eventResponse{}, err
	}

	if startDate != nil {
		v := startDate.Format("2006-01-02")
		item.StartDate = &v
	}
	if startTime != nil {
		v := startTime.UTC().Format(time.RFC3339)
		item.StartTime = &v
	}
	item.Season = season
	item.StartTimeLocal = startTimeLocal
	item.Round = round
	item.UpdatedAt = updatedAt.UTC().Format(time.RFC3339)

	if resultProvider != nil {
		result := &resultResponse{
			HomeScore: homeScore, AwayScore: awayScore,
			HomeScoreHT: homeHT, AwayScoreHT: awayHT,
			Provider: *resultProvider,
		}
		if resultCaptured != nil {
			v := resultCaptured.UTC().Format(time.RFC3339)
			result.CapturedAt = &v
		}
		item.Result = result
	}

	if homeCrest != nil {
		path := "/api/v1/teams/" + strconv.FormatInt(item.Home.ID, 10) + "/crest"
		item.Home.CrestURL = &path
	}
	if awayCrest != nil {
		path := "/api/v1/teams/" + strconv.FormatInt(item.Away.ID, 10) + "/crest"
		item.Away.CrestURL = &path
	}

	if liveStatus != nil && liveProvider != nil && liveCaptured != nil {
		item.Live = &liveResponse{
			Status:     *liveStatus,
			HomeScore:  liveHome,
			AwayScore:  liveAway,
			Detail:     liveDetail,
			Provider:   *liveProvider,
			CapturedAt: liveCaptured.UTC().Format(time.RFC3339),
		}
	}
	return item, nil
}

func (s *Server) eventProviderLinks(r *http.Request, eventID int64) ([]map[string]any, error) {
	rows, err := s.pool.Query(r.Context(), `
		SELECT provider, provider_event_id, match_method, linked_at
		FROM event_provider_ids
		WHERE event_id = $1
		ORDER BY provider`, eventID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	links := make([]map[string]any, 0, 2)
	for rows.Next() {
		var provider, providerID string
		var method *string
		var linkedAt time.Time
		if err := rows.Scan(&provider, &providerID, &method, &linkedAt); err != nil {
			return nil, err
		}
		links = append(links, map[string]any{
			"provider":          provider,
			"provider_event_id": providerID,
			"match_method":      method,
			"linked_at":         linkedAt.UTC().Format(time.RFC3339),
		})
	}
	return links, rows.Err()
}

type selectionResponse struct {
	ID               int64   `json:"id"`
	Selection        string  `json:"selection"`
	DecimalOdds      *string `json:"decimal_odds"`
	Bookmaker        *string `json:"bookmaker"`
	PriceKind        *string `json:"price_kind"`
	OddsCapturedAt   *string `json:"odds_captured_at"`
	SettlementStatus *string `json:"settlement_status"`
	SettlementReason *string `json:"settlement_reason"`
	SettlementActual *string `json:"settlement_actual"`
}

type marketResponse struct {
	ID         int64               `json:"id"`
	Type       string              `json:"type"`
	Period     string              `json:"period"`
	Line       *string             `json:"line"`
	Selections []selectionResponse `json:"selections"`
}

func (s *Server) eventMarkets(r *http.Request, eventID int64) ([]marketResponse, error) {
	rows, err := s.pool.Query(r.Context(), `
		SELECT m.id, mt.slug, m.period, m.line,
		       se.id, se.selection,
		       latest.decimal_odds, latest.bookmaker, latest.price_kind, latest.captured_at,
		       st.status, st.reason_code, st.actual_value
		FROM markets m
		JOIN market_types mt ON mt.id = m.market_type_id
		JOIN market_selections se ON se.market_id = m.id
		LEFT JOIN LATERAL (
			SELECT o.decimal_odds, b.name AS bookmaker, o.price_kind, o.captured_at
			FROM odds_snapshots o
			JOIN bookmakers b ON b.id = o.bookmaker_id
			WHERE o.selection_id = se.id
			ORDER BY o.captured_at DESC
			LIMIT 1
		) latest ON TRUE
		LEFT JOIN selection_settlements st ON st.selection_id = se.id
		WHERE m.event_id = $1
		ORDER BY mt.slug, m.line NULLS FIRST, se.selection`, eventID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	markets := make([]marketResponse, 0, 4)
	index := make(map[int64]int)
	for rows.Next() {
		var marketID int64
		var marketType, period string
		var line *string
		var selection selectionResponse
		var odds *string
		var bookmaker *string
		var kind *string
		var captured *time.Time

		if err := rows.Scan(&marketID, &marketType, &period, &line,
			&selection.ID, &selection.Selection,
			&odds, &bookmaker, &kind, &captured,
			&selection.SettlementStatus, &selection.SettlementReason, &selection.SettlementActual); err != nil {
			return nil, err
		}
		selection.DecimalOdds = odds
		selection.Bookmaker = bookmaker
		selection.PriceKind = kind
		if captured != nil {
			v := captured.UTC().Format(time.RFC3339)
			selection.OddsCapturedAt = &v
		}

		pos, ok := index[marketID]
		if !ok {
			markets = append(markets, marketResponse{
				ID: marketID, Type: marketType, Period: period, Line: line,
			})
			pos = len(markets) - 1
			index[marketID] = pos
		}
		markets[pos].Selections = append(markets[pos].Selections, selection)
	}
	return markets, rows.Err()
}

func isNoRows(err error) bool { return errors.Is(err, pgx.ErrNoRows) }
