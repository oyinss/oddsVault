// Package onexbet reads 1xBet's public live football feed.
//
// It is the bookmaker the user bets with, and the only accessible source of
// live corner counts for the leagues they actually play. The feed is
// undocumented: one request per poll returns every live event with its clock,
// score and statistics, which keeps the load on their gateway low.
//
// Market prices live behind per-event calls and are not part of this adapter
// yet. Nothing here invents a price.
package onexbet

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/oyins/oddsvault/backend/internal/providers"
)

const liveFeedURL = "https://1xbet.ng/service-api/LiveFeed/Get1x2_VZip"

// Client fetches 1xBet live data.
type Client struct {
	http *providers.Client
}

// New returns a 1xBet adapter.
func New(client *providers.Client) *Client { return &Client{http: client} }

// Event is one live football match with the data the feed publishes.
type Event struct {
	ProviderEventID string
	LeagueID        int64
	LeagueName      string
	CountryName     string
	Home            string
	Away            string
	StartUTC        time.Time
	Status          providers.Status
	Clock           string
	HomeScore       *int
	AwayScore       *int
	HomeCorners     *int
	AwayCorners     *int
	HomeLogoURL     string
	AwayLogoURL     string
}

type payload struct {
	Value []struct {
		ID       int64    `json:"I"`
		Name     int64    `json:"N"`
		League   string   `json:"L"`
		Country  string   `json:"CN"`
		LeagueID int64    `json:"LI"`
		Home     string   `json:"O1"`
		Away     string   `json:"O2"`
		HomeIMG  []string `json:"O1IMG"`
		AwayIMG  []string `json:"O2IMG"`
		Start    int64    `json:"S"`
		SC       struct {
			FS struct {
				S1 *int `json:"S1"`
				S2 *int `json:"S2"`
			} `json:"FS"`
			PS []struct {
				Value struct {
					S1 *int `json:"S1"`
					S2 *int `json:"S2"`
				} `json:"Value"`
			} `json:"PS"`
			CPS string `json:"CPS"`
			SLS string `json:"SLS"`
			TS  int64  `json:"TS"`
			ST  []struct {
				Key   int `json:"Key"`
				Value []struct {
					ID int    `json:"ID"`
					N  string `json:"N"`
					S1 string `json:"S1"`
					S2 string `json:"S2"`
				} `json:"Value"`
			} `json:"ST"`
		} `json:"SC"`
	} `json:"Value"`
}

// FetchLive returns every live football event in the feed.
func (c *Client) FetchLive(ctx context.Context) ([]Event, error) {
	res, err := c.http.Get(ctx, providers.Request{
		Provider: "onexbet",
		CacheKey: "live/football",
		URL:      liveFeedURL + "?sports=1&count=300&lng=en&mode=4&country=190&getEmpty=true",
		MaxAge:   60 * time.Second,
		Headers: map[string]string{
			"Accept":           "application/json",
			"Referer":          "https://1xbet.ng/en/live/football",
			"X-Requested-With": "XMLHttpRequest",
		},
	})
	if err != nil {
		return nil, err
	}

	var doc payload
	if err := json.Unmarshal(res.Body, &doc); err != nil {
		return nil, fmt.Errorf("1xbet: decode live feed: %w", err)
	}

	out := make([]Event, 0, len(doc.Value))
	for _, raw := range doc.Value {
		if raw.Home == "" || raw.Away == "" {
			continue
		}
		event := Event{
			ProviderEventID: strconv.FormatInt(raw.ID, 10),
			LeagueID:        raw.LeagueID,
			LeagueName:      strings.TrimSpace(raw.League),
			CountryName:     strings.TrimSpace(raw.Country),
			Home:            strings.TrimSpace(raw.Home),
			Away:            strings.TrimSpace(raw.Away),
			Clock:           strings.TrimSpace(raw.SC.SLS),
			HomeScore:       raw.SC.FS.S1,
			AwayScore:       raw.SC.FS.S2,
		}
		if raw.Start > 0 {
			event.StartUTC = time.Unix(raw.Start, 0).UTC()
		}
		event.HomeLogoURL = teamLogoURL(raw.HomeIMG)
		event.AwayLogoURL = teamLogoURL(raw.AwayIMG)

		// Some events report the score per period only.
		if event.HomeScore == nil && event.AwayScore == nil {
			event.HomeScore, event.AwayScore = periodScore(raw.SC.PS)
		}

		switch {
		case strings.Contains(strings.ToLower(raw.SC.CPS), "finish"):
			event.Status = providers.StatusFinished
		case raw.SC.TS > 0 || raw.SC.SLS != "":
			event.Status = providers.StatusLive
		default:
			event.Status = providers.StatusScheduled
		}

		for _, block := range raw.SC.ST {
			for _, stat := range block.Value {
				if stat.ID != 70 { // 1xBet's corner statistic id
					continue
				}
				event.HomeCorners = parseStat(stat.S1)
				event.AwayCorners = parseStat(stat.S2)
			}
		}

		if event.Status == providers.StatusLive {
			out = append(out, event)
		}
	}
	return out, nil
}

// periodScore sums the reported periods, which is the current score while a
// match is in progress.
func periodScore(periods []struct {
	Value struct {
		S1 *int `json:"S1"`
		S2 *int `json:"S2"`
	} `json:"Value"`
}) (*int, *int) {
	home, away := 0, 0
	seen := false
	for _, period := range periods {
		if period.Value.S1 == nil && period.Value.S2 == nil {
			continue
		}
		if period.Value.S1 != nil {
			home += *period.Value.S1
		}
		if period.Value.S2 != nil {
			away += *period.Value.S2
		}
		seen = true
	}
	if !seen {
		return nil, nil
	}
	return &home, &away
}

// teamLogoURL builds the public badge URL from the filename in the feed.
func teamLogoURL(files []string) string {
	if len(files) == 0 || strings.TrimSpace(files[0]) == "" {
		return ""
	}
	return "https://1xbet.ng/sfiles/logo_teams/" + strings.TrimSpace(files[0])
}

func parseStat(raw string) *int {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value < 0 {
		return nil
	}
	return &value
}
