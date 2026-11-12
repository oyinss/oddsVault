// Package footballdata reads football-data.co.uk season CSVs.
//
// The CSVs contain played matches only: final score, half-time score and
// closing odds. They are the result source; the fixture schedule comes from
// OpenFootball. Kickoff times are venue-local with no timezone, so only the
// calendar date is used for identity.
package footballdata

import (
	"bytes"
	"context"
	"encoding/csv"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/oyins/oddsvault/backend/internal/normalize"
	"github.com/oyins/oddsvault/backend/internal/providers"
)

const baseURL = "https://www.football-data.co.uk/mmz4281"

// Client fetches and normalizes football-data.co.uk CSVs.
type Client struct {
	http *providers.Client
}

// New returns a football-data.co.uk adapter.
func New(client *providers.Client) *Client { return &Client{http: client} }

// FetchSeason returns normalized fixtures/results for one division.
func (c *Client) FetchSeason(ctx context.Context, competitionSlug, seasonName, seasonCode, division string, current bool) ([]providers.Fixture, error) {
	maxAge := time.Duration(0)
	if current {
		maxAge = 6 * time.Hour
	}
	res, err := c.http.Get(ctx, providers.Request{
		Provider: "football-data",
		CacheKey: seasonCode + "/" + division + ".csv",
		URL:      fmt.Sprintf("%s/%s/%s.csv", baseURL, seasonCode, division),
		MaxAge:   maxAge,
	})
	if err != nil {
		return nil, err
	}

	fixtures, err := ParseSeason(res.Body, competitionSlug, seasonName, seasonCode, division)
	if err != nil {
		return nil, err
	}
	if len(fixtures) == 0 {
		return nil, fmt.Errorf("football-data %s/%s: no rows in payload", seasonCode, division)
	}
	return fixtures, nil
}

// ParseSeason parses a raw CSV body. It is exported for fixture-based tests.
func ParseSeason(body []byte, competitionSlug, seasonName, seasonCode, division string) ([]providers.Fixture, error) {
	body = bytes.TrimPrefix(body, []byte{0xEF, 0xBB, 0xBF})

	reader := csv.NewReader(bytes.NewReader(body))
	reader.FieldsPerRecord = -1
	reader.LazyQuotes = true

	rows, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("parse football-data CSV: %w", err)
	}
	if len(rows) < 2 {
		return nil, fmt.Errorf("football-data CSV has no data rows")
	}

	col := make(map[string]int, len(rows[0]))
	for i, name := range rows[0] {
		col[strings.TrimSpace(name)] = i
	}
	for _, required := range []string{"Date", "HomeTeam", "AwayTeam", "FTHG", "FTAG"} {
		if _, ok := col[required]; !ok {
			return nil, fmt.Errorf("football-data CSV missing column %q", required)
		}
	}

	get := func(row []string, name string) string {
		i, ok := col[name]
		if !ok || i >= len(row) {
			return ""
		}
		return strings.TrimSpace(row[i])
	}

	fixtures := make([]providers.Fixture, 0, len(rows)-1)
	for lineNo, row := range rows[1:] {
		if len(row) == 0 || strings.TrimSpace(row[0]) == "" {
			continue
		}
		date, err := time.Parse("02/01/2006", get(row, "Date"))
		if err != nil {
			return nil, fmt.Errorf("football-data row %d: invalid date %q", lineNo+2, get(row, "Date"))
		}
		home, away := get(row, "HomeTeam"), get(row, "AwayTeam")
		if home == "" || away == "" {
			return nil, fmt.Errorf("football-data row %d: missing team", lineNo+2)
		}

		f := providers.Fixture{
			Provider:        "football-data",
			CompetitionSlug: competitionSlug,
			SeasonName:      seasonName,
			StartDate:       date,
			StartTimeLocal:  get(row, "Time"),
			Status:          providers.StatusScheduled,
			Home:            providers.TeamRef{Name: home},
			Away:            providers.TeamRef{Name: away},
			HomeScore:       intOrNil(get(row, "FTHG")),
			AwayScore:       intOrNil(get(row, "FTAG")),
			HomeScoreHT:     intOrNil(get(row, "HTHG")),
			AwayScoreHT:     intOrNil(get(row, "HTAG")),
		}
		if f.Finished() {
			f.Status = providers.StatusFinished
		}
		f.Odds = closingOdds(row, get)
		f.ProviderEventID = fmt.Sprintf("fd:%s:%s:%s:%s:%s",
			seasonCode, division, date.Format("2006-01-02"),
			normalize.Key(home), normalize.Key(away))
		fixtures = append(fixtures, f)
	}
	return fixtures, nil
}

// closingOdds extracts Bet365 closing prices: 1X2 and over/under 2.5.
// Closing columns are preferred; pre-match columns are the fallback.
func closingOdds(row []string, get func([]string, string) string) []providers.OddsQuote {
	pick := func(closing, fallback string) string {
		if v := get(row, closing); v != "" {
			return v
		}
		return get(row, fallback)
	}

	var quotes []providers.OddsQuote
	add := func(selection, raw string) {
		value, err := strconv.ParseFloat(raw, 64)
		if err != nil || value <= 1 {
			return
		}
		quotes = append(quotes, providers.OddsQuote{
			Bookmaker: "bet365", MarketType: "match_result",
			Selection: selection, Decimal: value,
		})
	}
	add("home", pick("B365CH", "B365H"))
	add("draw", pick("B365CD", "B365D"))
	add("away", pick("B365CA", "B365A"))

	addTotal := func(selection, raw string) {
		value, err := strconv.ParseFloat(raw, 64)
		if err != nil || value <= 1 {
			return
		}
		line := 2.5
		quotes = append(quotes, providers.OddsQuote{
			Bookmaker: "bet365", MarketType: "total_goals",
			Line: &line, Selection: selection, Decimal: value,
		})
	}
	addTotal("over", pick("B365C>2.5", "B365>2.5"))
	addTotal("under", pick("B365C<2.5", "B365<2.5"))

	return quotes
}

func intOrNil(raw string) *int {
	if raw == "" {
		return nil
	}
	v, err := strconv.Atoi(raw)
	if err != nil || v < 0 {
		return nil
	}
	return &v
}
