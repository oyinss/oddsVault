// Package livescorebet reads LiveScoreBet's public sportsbook gateway.
//
// The gateway returns pre-match events with their markets and decimal odds in
// one response, so this is a server-side collector rather than a browser
// capture. Kickoff times arrive as wall-clock strings with no timezone, which
// is why they are kept verbatim instead of being converted.
package livescorebet

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/oyins/oddsvault/backend/internal/providers"
)

const (
	baseURL   = "https://gateway-ng.livescorebet.com/sportsbook/gateway"
	userAgent = "OddsVault/0.1 (research)"
)

// Client fetches LiveScoreBet markets.
type Client struct {
	http *providers.Client
}

// New returns a LiveScoreBet adapter.
func New(client *providers.Client) *Client { return &Client{http: client} }

// Event is one bookmaker event with its raw markets.
type Event struct {
	ProviderEventID string
	Name            string
	StartTimeLocal  string
	State           string
	Home            string
	Away            string
	Markets         []Market
}

// Market is one bookmaker market.
type Market struct {
	Name       string
	PeriodType string
	Selections []Selection
}

// Selection is one priced selection.
type Selection struct {
	Name string
	Odds float64
	Kind string
}

type payload struct {
	Events struct {
		Categories []struct {
			Events []struct {
				ID           string `json:"id"`
				Name         string `json:"name"`
				StartTime    string `json:"startTime"`
				State        string `json:"state"`
				Participants []struct {
					Name      string `json:"name"`
					VenueRole string `json:"venueRole"`
				} `json:"participants"`
				Markets []struct {
					Name       string `json:"name"`
					PeriodType string `json:"periodType"`
					Selections []struct {
						Name string  `json:"name"`
						Odds float64 `json:"odds"`
						Kind string  `json:"kind"`
					} `json:"selections"`
				} `json:"markets"`
			} `json:"events"`
		} `json:"categories"`
	} `json:"events"`
}

// FetchCompetitionEvents downloads one competition's pre-match card.
func (c *Client) FetchCompetitionEvents(ctx context.Context, categoryID string) ([]Event, error) {
	url := fmt.Sprintf("%s/v3/view/events/matches?categoryid=%s&interval=ALL&clientid=web&lang=en-ng",
		baseURL, categoryID)

	res, err := c.http.Get(ctx, providers.Request{
		Provider: "livescorebet",
		CacheKey: "matches/" + categoryID,
		URL:      url,
		MaxAge:   60 * time.Second,
		Headers: map[string]string{
			"Accept":     "application/json",
			"Origin":     "https://www.livescorebet.com",
			"Referer":    "https://www.livescorebet.com/ng/",
			"User-Agent": userAgent,
		},
	})
	if err != nil {
		return nil, err
	}

	var doc payload
	if err := json.Unmarshal(res.Body, &doc); err != nil {
		return nil, fmt.Errorf("livescorebet: decode matches: %w", err)
	}

	var out []Event
	for _, category := range doc.Events.Categories {
		for _, raw := range category.Events {
			event := Event{
				ProviderEventID: raw.ID,
				Name:            raw.Name,
				StartTimeLocal:  raw.StartTime,
				State:           raw.State,
			}
			for _, participant := range raw.Participants {
				switch strings.ToLower(participant.VenueRole) {
				case "home":
					event.Home = participant.Name
				case "away":
					event.Away = participant.Name
				}
			}
			for _, market := range raw.Markets {
				m := Market{Name: market.Name, PeriodType: market.PeriodType}
				for _, selection := range market.Selections {
					m.Selections = append(m.Selections, Selection{
						Name: selection.Name, Odds: selection.Odds, Kind: selection.Kind,
					})
				}
				event.Markets = append(event.Markets, m)
			}
			if event.Home != "" && event.Away != "" {
				out = append(out, event)
			}
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("livescorebet: no events for competition %s", categoryID)
	}
	return out, nil
}

// Quotes converts supported pre-match markets into normalized observations.
// Unsupported markets (period variants, 1UP, same-game combos) are skipped
// rather than guessed at.
func Quotes(event Event) []providers.OddsQuote {
	var quotes []providers.OddsQuote

	add := func(marketType string, line *float64, selection string, odds float64) {
		if odds <= 1 {
			return // 0 means unavailable in this feed
		}
		quotes = append(quotes, providers.OddsQuote{
			Bookmaker:  "livescorebet",
			MarketType: marketType,
			Line:       line,
			Selection:  selection,
			Decimal:    odds,
		})
	}

	for _, market := range event.Markets {
		if market.PeriodType != "FT" {
			continue
		}
		switch market.Name {
		case "Full Time":
			for _, selection := range market.Selections {
				switch {
				case strings.EqualFold(selection.Name, event.Home):
					add("match_result", nil, "home", selection.Odds)
				case strings.EqualFold(selection.Name, "Draw"):
					add("match_result", nil, "draw", selection.Odds)
				case strings.EqualFold(selection.Name, event.Away):
					add("match_result", nil, "away", selection.Odds)
				}
			}

		case "Double Chance":
			for _, selection := range market.Selections {
				selectionName := doubleChance(selection.Name, event.Home, event.Away)
				if selectionName != "" {
					add("double_chance", nil, selectionName, selection.Odds)
				}
			}

		case "Both Teams To Score":
			for _, selection := range market.Selections {
				switch strings.ToLower(selection.Name) {
				case "yes":
					add("both_teams_to_score", nil, "yes", selection.Odds)
				case "no":
					add("both_teams_to_score", nil, "no", selection.Odds)
				}
			}

		case "Total Goals":
			for _, selection := range market.Selections {
				side, line, ok := parseTotal(selection.Name)
				if !ok {
					continue
				}
				add("total_goals", &line, side, selection.Odds)
			}
		}
	}
	return quotes
}

// doubleChance maps provider wording to our canonical selection.
func doubleChance(name, home, away string) string {
	switch {
	case name == home+" or Draw":
		return "home_or_draw"
	case name == home+" or "+away:
		return "home_or_away"
	case name == "Draw or "+away:
		return "draw_or_away"
	}
	return ""
}

func parseTotal(name string) (string, float64, bool) {
	for _, prefix := range []struct {
		label string
		side  string
	}{{"Over ", "over"}, {"Under ", "under"}} {
		if strings.HasPrefix(name, prefix.label) {
			line, err := strconv.ParseFloat(strings.TrimSpace(strings.TrimPrefix(name, prefix.label)), 64)
			if err != nil || line <= 0 {
				return "", 0, false
			}
			return prefix.side, line, true
		}
	}
	return "", 0, false
}
