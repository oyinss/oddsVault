package api

import (
	"net/http"
	"time"

	"github.com/oyins/oddsvault/backend/internal/normalize"
	"github.com/oyins/oddsvault/backend/internal/store"
)

type betLegResponse struct {
	LegNumber      int           `json:"leg_number"`
	Competition    *string       `json:"competition"`
	EventRaw       string        `json:"event_raw"`
	MarketRaw      *string       `json:"market_raw"`
	SelectionRaw   *string       `json:"selection_raw"`
	MarketFamily   *string       `json:"market_family"`
	MarketSide     *string       `json:"market_side"`
	MarketLine     *float64      `json:"market_line"`
	OddsTaken      *float64      `json:"odds_taken"`
	LegStatus      string        `json:"leg_status"`
	IdentityStatus string        `json:"identity_status"`
	ResolutionNote *string       `json:"resolution_note"`
	EventID        *int64        `json:"event_id"`
	HomeTeam       *teamResponse `json:"home_team"`
	AwayTeam       *teamResponse `json:"away_team"`
}

type betResponse struct {
	ID              int64            `json:"id"`
	ExternalID      string           `json:"external_id"`
	Bookmaker       string           `json:"bookmaker"`
	Currency        string           `json:"currency"`
	PlacedAt        *string          `json:"placed_at"`
	PlacedAtRaw     *string          `json:"placed_at_raw"`
	BetType         *string          `json:"bet_type"`
	Stake           *float64         `json:"stake"`
	CombinedOdds    *float64         `json:"combined_odds"`
	PotentialReturn *float64         `json:"potential_return"`
	Status          string           `json:"status"`
	ActualReturn    *float64         `json:"actual_return"`
	ProfitLoss      *float64         `json:"profit_loss"`
	Source          string           `json:"source"`
	Legs            []betLegResponse `json:"legs"`
}

type betSummary struct {
	Bets       int     `json:"bets"`
	Staked     float64 `json:"staked"`
	Returned   float64 `json:"returned"`
	ProfitLoss float64 `json:"profit_loss"`
	Won        int     `json:"won"`
	Lost       int     `json:"lost"`
	Pending    int     `json:"pending"`
}

func (s *Server) handleMyBets(w http.ResponseWriter, r *http.Request) {
	if s.pool == nil {
		writeError(w, r, http.StatusServiceUnavailable, "not_ready", "database is not configured")
		return
	}

	var summary betSummary
	if err := s.pool.QueryRow(r.Context(), `
		SELECT count(*),
		       COALESCE(sum(stake), 0)::float8,
		       COALESCE(sum(actual_return), 0)::float8,
		       COALESCE(sum(profit_loss), 0)::float8,
		       count(*) FILTER (WHERE status = 'won'),
		       count(*) FILTER (WHERE status = 'lost'),
		       count(*) FILTER (WHERE status = 'pending')
		FROM personal_bets`).Scan(
		&summary.Bets, &summary.Staked, &summary.Returned, &summary.ProfitLoss,
		&summary.Won, &summary.Lost, &summary.Pending); err != nil {
		s.log.ErrorContext(r.Context(), "query bet summary", "error", err.Error())
		writeError(w, r, http.StatusInternalServerError, "query_failed", "could not load bets")
		return
	}

	rows, err := s.pool.Query(r.Context(), `
		SELECT pb.id, pb.external_id, b.name, pb.currency, pb.placed_at, pb.placed_at_raw, pb.bet_type,
		       pb.stake::float8, pb.combined_odds::float8, pb.potential_return::float8,
		       pb.status, pb.actual_return::float8, pb.profit_loss::float8, pb.source
		FROM personal_bets pb
		JOIN bookmakers b ON b.id = pb.bookmaker_id
		ORDER BY pb.placed_at DESC NULLS LAST, pb.id DESC`)
	if err != nil {
		s.log.ErrorContext(r.Context(), "query bets", "error", err.Error())
		writeError(w, r, http.StatusInternalServerError, "query_failed", "could not load bets")
		return
	}
	defer rows.Close()

	bets := make([]betResponse, 0, summary.Bets)
	index := make(map[int64]int, summary.Bets)
	for rows.Next() {
		var item betResponse
		var placedAt *time.Time
		if err := rows.Scan(
			&item.ID, &item.ExternalID, &item.Bookmaker, &item.Currency, &placedAt,
			&item.PlacedAtRaw, &item.BetType, &item.Stake, &item.CombinedOdds, &item.PotentialReturn,
			&item.Status, &item.ActualReturn, &item.ProfitLoss, &item.Source); err != nil {
			s.log.ErrorContext(r.Context(), "scan bet", "error", err.Error())
			writeError(w, r, http.StatusInternalServerError, "query_failed", "could not load bets")
			return
		}
		if placedAt != nil {
			formatted := placedAt.UTC().Format(time.RFC3339)
			item.PlacedAt = &formatted
		}
		index[item.ID] = len(bets)
		bets = append(bets, item)
	}
	if err := rows.Err(); err != nil {
		s.log.ErrorContext(r.Context(), "iterate bets", "error", err.Error())
		writeError(w, r, http.StatusInternalServerError, "query_failed", "could not load bets")
		return
	}

	legRows, err := s.pool.Query(r.Context(), `
		SELECT personal_bet_id, leg_number, competition_raw, event_raw, market_raw,
		       selection_raw, market_family, market_side, market_line::float8,
		       odds_taken::float8, leg_status, identity_status, resolution_note, event_id
		FROM personal_bet_legs
		ORDER BY personal_bet_id, leg_number`)
	if err != nil {
		s.log.ErrorContext(r.Context(), "query bet legs", "error", err.Error())
		writeError(w, r, http.StatusInternalServerError, "query_failed", "could not load bets")
		return
	}
	defer legRows.Close()

	for legRows.Next() {
		var betID int64
		var item betLegResponse
		if err := legRows.Scan(&betID, &item.LegNumber, &item.Competition, &item.EventRaw,
			&item.MarketRaw, &item.SelectionRaw, &item.MarketFamily, &item.MarketSide,
			&item.MarketLine, &item.OddsTaken, &item.LegStatus, &item.IdentityStatus,
			&item.ResolutionNote, &item.EventID); err != nil {
			s.log.ErrorContext(r.Context(), "scan bet leg", "error", err.Error())
			writeError(w, r, http.StatusInternalServerError, "query_failed", "could not load bets")
			return
		}
		if pos, ok := index[betID]; ok {
			bets[pos].Legs = append(bets[pos].Legs, item)
		}
	}
	if err := legRows.Err(); err != nil {
		s.log.ErrorContext(r.Context(), "iterate bet legs", "error", err.Error())
		writeError(w, r, http.StatusInternalServerError, "query_failed", "could not load bets")
		return
	}

	teams := make(map[string]teamInfo)
	teamRows, err := s.pool.Query(r.Context(), `
		SELECT t.id, t.name_key, t.name, t.slug, (ma.id IS NOT NULL)
		FROM teams t
		JOIN sports sp ON sp.id = t.sport_id AND sp.slug = 'football'
		LEFT JOIN media_assets ma ON ma.entity_type = 'team' AND ma.entity_id = t.id
		                         AND ma.asset_type = 'team_logo' AND ma.active`)
	if err == nil {
		defer teamRows.Close()
		for teamRows.Next() {
			var info teamInfo
			var key string
			if err := teamRows.Scan(&info.ID, &key, &info.Name, &info.Slug, &info.Crest); err == nil {
				teams[key] = info
			}
		}
	}

	for i := range bets {
		for j := range bets[i].Legs {
			home, away, _ := store.ParseLegEvent(bets[i].Legs[j].EventRaw)
			bets[i].Legs[j].HomeTeam = lookupTeam(teams, home)
			bets[i].Legs[j].AwayTeam = lookupTeam(teams, away)
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"summary": summary,
		"bets":    bets,
		"count":   len(bets),
	})
}

type teamInfo struct {
	ID    int64
	Name  string
	Slug  string
	Crest bool
}

func lookupTeam(teams map[string]teamInfo, name string) *teamResponse {
	if name == "" {
		return nil
	}
	_, key := normalize.Resolve(name)
	info, ok := teams[key]
	if !ok {
		return nil
	}
	team := teamResponse{ID: info.ID, Name: info.Name, Slug: info.Slug}
	if info.Crest {
		path := store.CrestEndpointPath(info.ID)
		team.CrestURL = &path
	}
	return &team
}
