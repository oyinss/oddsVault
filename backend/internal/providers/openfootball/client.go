// Package openfootball reads the public-domain football.json season files.
//
// The source has no ids, no timezone and no live state: fixtures carry a local
// date and time, and scores appear only once a match is finished. Provider keys
// are derived fingerprints, recorded as observations rather than authorities.
package openfootball

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/oyins/oddsvault/backend/internal/normalize"
	"github.com/oyins/oddsvault/backend/internal/providers"
)

const baseURL = "https://raw.githubusercontent.com/openfootball/football.json/master"

// Client fetches and normalizes OpenFootball seasons.
type Client struct {
	http *providers.Client
}

// New returns an OpenFootball adapter.
func New(client *providers.Client) *Client { return &Client{http: client} }

type seasonDoc struct {
	Name    string  `json:"name"`
	Matches []match `json:"matches"`
}

type match struct {
	Round string `json:"round"`
	Date  string `json:"date"`
	Time  string `json:"time"`
	Team1 string `json:"team1"`
	Team2 string `json:"team2"`
	Score *score `json:"score"`
}

// score accepts both documented shapes: {ht, ft} and a bare [home, away]
// array, which some seasons use for the full-time result.
type score struct {
	HT []*int `json:"ht"`
	FT []*int `json:"ft"`
}

func (s *score) UnmarshalJSON(data []byte) error {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return nil
	}
	if trimmed[0] == '[' {
		var ft []*int
		if err := json.Unmarshal(trimmed, &ft); err != nil {
			return err
		}
		s.FT = ft
		return nil
	}
	type plain score
	var decoded plain
	if err := json.Unmarshal(trimmed, &decoded); err != nil {
		return err
	}
	*s = score(decoded)
	return nil
}

// FetchSeason returns normalized fixtures for one competition season.
func (c *Client) FetchSeason(ctx context.Context, competitionSlug, seasonName, seasonDir, file string, current bool) ([]providers.Fixture, error) {
	maxAge := time.Duration(0) // completed seasons never change
	if current {
		maxAge = 6 * time.Hour
	}
	res, err := c.http.Get(ctx, providers.Request{
		Provider: "openfootball",
		CacheKey: seasonDir + "/" + file,
		URL:      fmt.Sprintf("%s/%s/%s", baseURL, seasonDir, file),
		MaxAge:   maxAge,
	})
	if err != nil {
		return nil, err
	}

	var doc seasonDoc
	if err := json.Unmarshal(res.Body, &doc); err != nil {
		return nil, fmt.Errorf("decode openfootball %s: %w", file, err)
	}
	return ParseSeason(&doc, competitionSlug, seasonName, seasonDir, file)
}

// ParseSeason converts a decoded season document into normalized fixtures.
// It is exported for fixture-based tests.
func ParseSeason(doc *seasonDoc, competitionSlug, seasonName, seasonDir, file string) ([]providers.Fixture, error) {
	if len(doc.Matches) == 0 {
		return nil, fmt.Errorf("openfootball %s: no matches in payload", file)
	}

	fixtures := make([]providers.Fixture, 0, len(doc.Matches))
	for i, m := range doc.Matches {
		startDate, err := time.Parse("2006-01-02", strings.TrimSpace(m.Date))
		if err != nil {
			return nil, fmt.Errorf("openfootball %s match %d: invalid date %q", file, i, m.Date)
		}
		if strings.TrimSpace(m.Team1) == "" || strings.TrimSpace(m.Team2) == "" {
			return nil, fmt.Errorf("openfootball %s match %d: missing team", file, i)
		}

		f := providers.Fixture{
			Provider:        "openfootball",
			CompetitionSlug: competitionSlug,
			SeasonName:      seasonName,
			Round:           strings.TrimSpace(m.Round),
			StartDate:       startDate,
			StartTimeLocal:  strings.TrimSpace(m.Time),
			Status:          providers.StatusScheduled,
			Home:            providers.TeamRef{Name: strings.TrimSpace(m.Team1)},
			Away:            providers.TeamRef{Name: strings.TrimSpace(m.Team2)},
		}
		f.ProviderEventID = fingerprint("of", seasonDir, file, m)

		if m.Score != nil {
			if len(m.Score.FT) == 2 && m.Score.FT[0] != nil && m.Score.FT[1] != nil {
				f.HomeScore = m.Score.FT[0]
				f.AwayScore = m.Score.FT[1]
				f.Status = providers.StatusFinished
			}
			if len(m.Score.HT) == 2 && m.Score.HT[0] != nil && m.Score.HT[1] != nil {
				f.HomeScoreHT = m.Score.HT[0]
				f.AwayScoreHT = m.Score.HT[1]
			}
		}
		fixtures = append(fixtures, f)
	}
	return fixtures, nil
}

func fingerprint(prefix, seasonDir, file string, m match) string {
	date := strings.TrimSpace(m.Date)
	return fmt.Sprintf("%s:%s:%s:%s:%s:%s",
		prefix, seasonDir, file, date, normalize.Key(m.Team1), normalize.Key(m.Team2))
}
