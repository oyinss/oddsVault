package api

import (
	"net/http"
	"sort"
	"strconv"
	"time"
)

type bookmakerPrice struct {
	Bookmaker     string  `json:"bookmaker"`
	BookmakerName string  `json:"bookmaker_name"`
	Odds          float64 `json:"odds"`
	PriceKind     string  `json:"price_kind"`
	CapturedAt    string  `json:"captured_at"`
}

type pricedSelection struct {
	Selection string           `json:"selection"`
	Prices    []bookmakerPrice `json:"prices"`
}

type pricedMarket struct {
	Type       string            `json:"type"`
	Line       *float64          `json:"line"`
	Selections []pricedSelection `json:"selections"`
}

// handleEventPrices returns the latest price per bookmaker for each selection,
// so the UI can compare books without inventing an "average" price.
func (s *Server) handleEventPrices(w http.ResponseWriter, r *http.Request) {
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
		SELECT DISTINCT ON (se.id, b.id)
		       m.id, mt.slug, m.line, se.id, se.selection,
		       b.slug, b.name, o.decimal_odds::float8, o.price_kind, o.captured_at
		FROM odds_snapshots o
		JOIN market_selections se ON se.id = o.selection_id
		JOIN markets m ON m.id = se.market_id
		JOIN market_types mt ON mt.id = m.market_type_id
		JOIN bookmakers b ON b.id = o.bookmaker_id
		WHERE m.event_id = $1
		ORDER BY se.id, b.id, o.captured_at DESC`, id)
	if err != nil {
		s.log.ErrorContext(r.Context(), "query event prices", "error", err.Error())
		writeError(w, r, http.StatusInternalServerError, "query_failed", "could not load prices")
		return
	}
	defer rows.Close()

	type book struct {
		Slug string `json:"slug"`
		Name string `json:"name"`
	}
	books := make(map[string]book)
	markets := make([]pricedMarket, 0, 4)
	marketIndex := make(map[int64]int)
	selectionIndex := make(map[int64][2]int)

	for rows.Next() {
		var marketID, selectionID int64
		var marketType, selection, bookSlug, bookName, priceKind string
		var line *float64
		var odds float64
		var capturedAt time.Time

		if err := rows.Scan(&marketID, &marketType, &line, &selectionID, &selection,
			&bookSlug, &bookName, &odds, &priceKind, &capturedAt); err != nil {
			s.log.ErrorContext(r.Context(), "scan event price", "error", err.Error())
			writeError(w, r, http.StatusInternalServerError, "query_failed", "could not load prices")
			return
		}
		books[bookSlug] = book{Slug: bookSlug, Name: bookName}

		mPos, ok := marketIndex[marketID]
		if !ok {
			markets = append(markets, pricedMarket{Type: marketType, Line: line})
			mPos = len(markets) - 1
			marketIndex[marketID] = mPos
		}
		sPos, ok := selectionIndex[selectionID]
		if !ok {
			markets[mPos].Selections = append(markets[mPos].Selections, pricedSelection{Selection: selection})
			sPos = [2]int{mPos, len(markets[mPos].Selections) - 1}
			selectionIndex[selectionID] = sPos
		}
		markets[sPos[0]].Selections[sPos[1]].Prices = append(
			markets[sPos[0]].Selections[sPos[1]].Prices,
			bookmakerPrice{
				Bookmaker:     bookSlug,
				BookmakerName: bookName,
				Odds:          odds,
				PriceKind:     priceKind,
				CapturedAt:    capturedAt.UTC().Format(time.RFC3339),
			})
	}
	if err := rows.Err(); err != nil {
		s.log.ErrorContext(r.Context(), "iterate event prices", "error", err.Error())
		writeError(w, r, http.StatusInternalServerError, "query_failed", "could not load prices")
		return
	}

	bookList := make([]book, 0, len(books))
	for _, b := range books {
		bookList = append(bookList, b)
	}
	sort.Slice(bookList, func(i, j int) bool { return bookList[i].Name < bookList[j].Name })

	writeJSON(w, http.StatusOK, map[string]any{
		"event_id":   id,
		"bookmakers": bookList,
		"markets":    markets,
	})
}
