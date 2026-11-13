package api

import (
	"context"
	"net/http"
	"sort"
	"strconv"
	"time"
)

// boardMarkets is the set of markets the capture path actually produces. Asking
// for anything else is a client error, not an empty board.
var boardMarkets = map[string]bool{
	"match_result":        true,
	"moneyline":           true,
	"total_goals":         true,
	"both_teams_to_score": true,
	"double_chance":       true,
}

type boardPrice struct {
	Bookmaker     string  `json:"bookmaker"`
	BookmakerName string  `json:"bookmaker_name"`
	Odds          float64 `json:"odds"`
	PriceKind     string  `json:"price_kind"`
	CapturedAt    string  `json:"captured_at"`
}

type boardSelection struct {
	Selection string       `json:"selection"`
	Best      *boardPrice  `json:"best"`
	Opening   *boardPrice  `json:"opening"`
	Movement  *float64     `json:"movement"`
	Prices    []boardPrice `json:"prices"`
}

type boardLine struct {
	Line       *float64         `json:"line"`
	Selections []boardSelection `json:"selections"`
}

type boardTeam struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	CrestURL string `json:"crest_url"`
}

type boardEvent struct {
	EventID            int64       `json:"event_id"`
	Sport              string      `json:"sport"`
	Competition        string      `json:"competition"`
	CompetitionSlug    string      `json:"competition_slug"`
	CompetitionCountry *string     `json:"competition_country"`
	Home               boardTeam   `json:"home"`
	Away               boardTeam   `json:"away"`
	Kickoff            string      `json:"kickoff"`
	Status             string      `json:"status"`
	Lines              []boardLine `json:"lines"`
	LastCapturedAt     *string     `json:"last_captured_at"`
}

// handleOddsBoard returns the newest captured price per bookmaker for one
// market across the next fixtures, with the opening price and the movement
// between them. Nothing is averaged: the best price is the best captured one.
func (s *Server) handleOddsBoard(w http.ResponseWriter, r *http.Request) {
	if s.pool == nil {
		writeError(w, r, http.StatusServiceUnavailable, "not_ready", "database is not configured")
		return
	}

	q := r.URL.Query()
	sport := q.Get("sport")
	if sport == "" {
		sport = "football"
	}
	market := q.Get("market")
	if market == "" {
		if sport == "football" {
			market = "match_result"
		} else {
			market = "moneyline"
		}
	}
	if !boardMarkets[market] {
		writeError(w, r, http.StatusBadRequest, "invalid_market", "market is not supported")
		return
	}

	limit := 20
	if raw := q.Get("limit"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 1 || parsed > 50 {
			writeError(w, r, http.StatusBadRequest, "invalid_limit", "limit must be between 1 and 50")
			return
		}
		limit = parsed
	}

	events, ids, err := s.boardEvents(r.Context(), sport, q.Get("competition"), market, limit)
	if err != nil {
		s.log.ErrorContext(r.Context(), "odds board events", "error", err.Error())
		writeError(w, r, http.StatusInternalServerError, "query_failed", "could not load odds board")
		return
	}

	if len(ids) > 0 {
		if err := s.attachBoardPrices(r.Context(), events, ids, market); err != nil {
			s.log.ErrorContext(r.Context(), "odds board prices", "error", err.Error())
			writeError(w, r, http.StatusInternalServerError, "query_failed", "could not load prices")
			return
		}
	}

	out := make([]boardEvent, 0, len(events))
	for _, event := range events {
		if len(event.Lines) > 0 {
			out = append(out, event)
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"sport":        sport,
		"market":       market,
		"generated_at": time.Now().UTC().Format(time.RFC3339),
		"count":        len(out),
		"events":       out,
	})
}

// boardEvents loads the next events that actually carry the requested market,
// so the board never lists a fixture it cannot price.
func (s *Server) boardEvents(ctx context.Context, sport, competition, market string, limit int) ([]boardEvent, []int64, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT e.id, sp.slug, c.name, c.slug, co.iso2,
		       ht.id, ht.name, at.id, at.name,
		       COALESCE(e.start_time, e.start_date::timestamptz) AS kickoff,
		       e.status,
		       EXISTS (
		           SELECT 1 FROM media_assets ma
		           WHERE ma.entity_type = 'team' AND ma.entity_id = ht.id
		             AND ma.asset_type = 'team_logo' AND ma.active
		       ) AS home_has_logo,
		       EXISTS (
		           SELECT 1 FROM media_assets ma
		           WHERE ma.entity_type = 'team' AND ma.entity_id = at.id
		             AND ma.asset_type = 'team_logo' AND ma.active
		       ) AS away_has_logo
		FROM events e
		JOIN sports sp ON sp.id = e.sport_id
		JOIN competitions c ON c.id = e.competition_id
		LEFT JOIN countries co ON co.id = c.country_id
		JOIN event_participants hp ON hp.event_id = e.id AND hp.role = 'home'
		JOIN teams ht ON ht.id = hp.team_id
		JOIN event_participants ap ON ap.event_id = e.id AND ap.role = 'away'
		JOIN teams at ON at.id = ap.team_id
		WHERE sp.slug = $1
		  AND ($2 = '' OR c.slug = $2)
		  AND e.status = 'scheduled'
		  AND COALESCE(e.start_time, e.start_date::timestamptz) > now() - interval '3 hours'
		  AND EXISTS (
		      SELECT 1 FROM markets m
		      JOIN market_types mt ON mt.id = m.market_type_id
		      WHERE m.event_id = e.id AND mt.slug = $3
		  )
		ORDER BY kickoff ASC
		LIMIT $4`, sport, competition, market, limit)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	events := make([]boardEvent, 0, limit)
	ids := make([]int64, 0, limit)
	for rows.Next() {
		var e boardEvent
		var kickoff time.Time
		var homeHasLogo, awayHasLogo bool
		if err := rows.Scan(&e.EventID, &e.Sport, &e.Competition, &e.CompetitionSlug, &e.CompetitionCountry,
			&e.Home.ID, &e.Home.Name, &e.Away.ID, &e.Away.Name,
			&kickoff, &e.Status, &homeHasLogo, &awayHasLogo); err != nil {
			return nil, nil, err
		}
		e.Kickoff = kickoff.UTC().Format(time.RFC3339)
		if homeHasLogo {
			e.Home.CrestURL = crestPath(e.Home.ID)
		}
		if awayHasLogo {
			e.Away.CrestURL = crestPath(e.Away.ID)
		}
		events = append(events, e)
		ids = append(ids, e.EventID)
	}
	return events, ids, rows.Err()
}

// attachBoardPrices fills each event's lines from the newest and oldest price
// per selection and bookmaker.
//
// The order is deliberate: rows arrive newest price descending, so the first
// row for a selection carries the best available price.
func (s *Server) attachBoardPrices(ctx context.Context, events []boardEvent, ids []int64, market string) error {
	rows, err := s.pool.Query(ctx, `
		SELECT m.event_id, m.id, m.line, se.selection,
		       b.slug, b.name, latest.decimal_odds::float8, latest.price_kind, latest.captured_at,
		       first.decimal_odds::float8, first.captured_at
		FROM markets m
		JOIN market_types mt ON mt.id = m.market_type_id AND mt.slug = $2
		JOIN market_selections se ON se.market_id = m.id
		JOIN LATERAL (
		    SELECT o.decimal_odds, o.price_kind, o.captured_at, o.bookmaker_id
		    FROM odds_snapshots o
		    WHERE o.selection_id = se.id
		    ORDER BY o.captured_at DESC
		    LIMIT 1
		) latest ON TRUE
		JOIN bookmakers b ON b.id = latest.bookmaker_id
		JOIN LATERAL (
		    SELECT o.decimal_odds, o.captured_at
		    FROM odds_snapshots o
		    WHERE o.selection_id = se.id
		    ORDER BY o.captured_at ASC
		    LIMIT 1
		) first ON TRUE
		WHERE m.event_id = ANY($1)
		ORDER BY m.event_id, m.line NULLS FIRST, m.id, se.id, latest.decimal_odds DESC`,
		ids, market)
	if err != nil {
		return err
	}
	defer rows.Close()

	eventIndex := make(map[int64]int, len(events))
	for i, event := range events {
		eventIndex[event.EventID] = i
	}

	// A market can carry several lines (totals), and each line has its own
	// selections, so both are part of the identity.
	type lineKey struct {
		eventID  int64
		marketID int64
		line     float64
		hasLine  bool
	}
	type selectionKey struct {
		lineKey
		selection string
	}

	lineIndex := make(map[lineKey][2]int)
	selectionIndex := make(map[selectionKey][2]int)

	for rows.Next() {
		var eventID, marketID int64
		var line *float64
		var selection, bookSlug, bookName, priceKind string
		var odds float64
		var capturedAt time.Time
		var opening float64
		var openingAt time.Time

		if err := rows.Scan(&eventID, &marketID, &line, &selection,
			&bookSlug, &bookName, &odds, &priceKind, &capturedAt,
			&opening, &openingAt); err != nil {
			return err
		}

		eventPos, ok := eventIndex[eventID]
		if !ok {
			continue
		}

		key := lineKey{eventID: eventID, marketID: marketID, hasLine: line != nil}
		if line != nil {
			key.line = *line
		}

		linePos, ok := lineIndex[key]
		if !ok {
			events[eventPos].Lines = append(events[eventPos].Lines, boardLine{Line: line})
			linePos = [2]int{eventPos, len(events[eventPos].Lines) - 1}
			lineIndex[key] = linePos
		}

		selectionKeyValue := selectionKey{lineKey: key, selection: selection}
		selPos, ok := selectionIndex[selectionKeyValue]
		if !ok {
			line := &events[linePos[0]].Lines[linePos[1]]
			line.Selections = append(line.Selections, boardSelection{
				Selection: selection,
				Prices:    []boardPrice{},
			})
			selPos = [2]int{linePos[0], len(line.Selections) - 1}
			selectionIndex[selectionKeyValue] = selPos
		}

		price := boardPrice{
			Bookmaker:     bookSlug,
			BookmakerName: bookName,
			Odds:          odds,
			PriceKind:     priceKind,
			CapturedAt:    capturedAt.UTC().Format(time.RFC3339),
		}

		lineRef := &events[selPos[0]].Lines[linePos[1]]
		sel := &lineRef.Selections[selPos[1]]
		sel.Prices = append(sel.Prices, price)
		if sel.Best == nil {
			best := price
			openingPrice := boardPrice{
				Bookmaker:     bookSlug,
				BookmakerName: bookName,
				Odds:          opening,
				PriceKind:     priceKind,
				CapturedAt:    openingAt.UTC().Format(time.RFC3339),
			}
			sel.Best = &best
			sel.Opening = &openingPrice
			if opening > 0 {
				move := (odds - opening) / opening
				sel.Movement = &move
			}
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}

	for i := range events {
		sort.SliceStable(events[i].Lines, func(a, b int) bool {
			left, right := events[i].Lines[a].Line, events[i].Lines[b].Line
			switch {
			case left == nil && right == nil:
				return false
			case left == nil:
				return true
			case right == nil:
				return false
			default:
				return *left < *right
			}
		})

		var newest time.Time
		for li := range events[i].Lines {
			for si := range events[i].Lines[li].Selections {
				sel := &events[i].Lines[li].Selections[si]
				for pi := range sel.Prices {
					at, err := time.Parse(time.RFC3339, sel.Prices[pi].CapturedAt)
					if err == nil && at.After(newest) {
						newest = at
					}
				}
			}
			// Reading order: home, draw, away; over before under.
			sort.SliceStable(events[i].Lines[li].Selections, func(a, b int) bool {
				left := events[i].Lines[li].Selections[a].Selection
				right := events[i].Lines[li].Selections[b].Selection
				return selectionRank(left) < selectionRank(right)
			})
		}
		if !newest.IsZero() {
			formatted := newest.UTC().Format(time.RFC3339)
			events[i].LastCapturedAt = &formatted
		}
	}
	return nil
}

// selectionRank keeps home, draw, away in reading order.
func selectionRank(selection string) int {
	switch selection {
	case "home", "yes", "over":
		return 0
	case "draw":
		return 1
	case "away", "no", "under":
		return 2
	case "home_or_draw":
		return 3
	case "home_or_away":
		return 4
	case "draw_or_away":
		return 5
	default:
		return 9
	}
}

// crestPath builds the public crest path for a team.
func crestPath(teamID int64) string {
	return "/api/v1/teams/" + strconv.FormatInt(teamID, 10) + "/crest"
}
