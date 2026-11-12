// Package espn reads ESPN's public soccer scoreboard.
//
// It is the live-state provider: no API key, status and scores only. ESPN does
// provide real event ids, so matching one of its fixtures confirms an event's
// identity instead of leaving it provisional.
package espn

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/oyins/oddsvault/backend/internal/providers"
)

const baseURL = "https://site.api.espn.com/apis/site/v2/sports/soccer"

// leagues maps our competition slugs to ESPN league codes.
var leagues = map[string]string{
	"premier-league":    "eng.1",
	"la-liga":           "esp.1",
	"serie-a":           "ita.1",
	"bundesliga":        "ger.1",
	"ligue-1":           "fra.1",
	"copa-del-rey":      "esp.copa_del_rey",
	"nations-league":    "uefa.nations",
	"champions-league":  "uefa.champions",
	"argentina-primera": "arg.1",
	"chile-primera":     "chi.1",
	"paraguay-primera":  "par.1",
	"uruguay-primera":   "uru.1",
	"italy-serie-c":     "ita.3",
}

// Client fetches ESPN scoreboards.
type Client struct {
	http *providers.Client
}

// New returns an ESPN adapter.
func New(client *providers.Client) *Client { return &Client{http: client} }

type scoreboard struct {
	Events []struct {
		ID           string `json:"id"`
		Date         string `json:"date"`
		Competitions []struct {
			Status struct {
				Type struct {
					State     string `json:"state"`
					Completed bool   `json:"completed"`
					Detail    string `json:"detail"`
					Short     string `json:"shortDetail"`
				} `json:"type"`
			} `json:"status"`
			Competitors []struct {
				HomeAway string `json:"homeAway"`
				Score    string `json:"score"`
				Team     struct {
					DisplayName string `json:"displayName"`
				} `json:"team"`
			} `json:"competitors"`
		} `json:"competitions"`
	} `json:"events"`
}

// FetchScoreboard returns the fixtures ESPN lists for one calendar date (UTC).
func (c *Client) FetchScoreboard(ctx context.Context, competitionSlug string, date time.Time) ([]providers.LiveState, error) {
	league, ok := leagues[competitionSlug]
	if !ok {
		return nil, fmt.Errorf("espn: no league code for %q", competitionSlug)
	}
	day := date.UTC().Format("20060102")
	res, err := c.http.Get(ctx, providers.Request{
		Provider: "espn",
		CacheKey: league + "/scoreboard/" + day,
		URL:      fmt.Sprintf("%s/%s/scoreboard?dates=%s", baseURL, league, day),
		MaxAge:   60 * time.Second,
		Headers:  map[string]string{"Accept": "application/json"},
	})
	if err != nil {
		return nil, err
	}

	var doc scoreboard
	if err := json.Unmarshal(res.Body, &doc); err != nil {
		return nil, fmt.Errorf("espn: decode scoreboard: %w", err)
	}

	states := make([]providers.LiveState, 0, len(doc.Events))
	for _, event := range doc.Events {
		if len(event.Competitions) == 0 {
			continue
		}
		competition := event.Competitions[0]

		var home, away string
		var homeScore, awayScore *int
		for _, competitor := range competition.Competitors {
			switch competitor.HomeAway {
			case "home":
				home = competitor.Team.DisplayName
				homeScore = parseScore(competitor.Score)
			case "away":
				away = competitor.Team.DisplayName
				awayScore = parseScore(competitor.Score)
			}
		}
		if home == "" || away == "" {
			continue
		}

		start, ok := parseTime(event.Date)
		if !ok {
			continue
		}

		states = append(states, providers.LiveState{
			ProviderEventID: event.ID,
			CompetitionSlug: competitionSlug,
			StartUTC:        start,
			Status:          mapStatus(competition.Status.Type.State, competition.Status.Type.Completed),
			Detail:          firstNonEmpty(competition.Status.Type.Detail, competition.Status.Type.Short),
			HomeName:        home,
			AwayName:        away,
			HomeScore:       homeScore,
			AwayScore:       awayScore,
		})
	}
	return states, nil
}

// parseTime accepts ESPN's RFC3339 variants, which sometimes omit seconds.
func parseTime(raw string) (time.Time, bool) {
	for _, layout := range []string{time.RFC3339, "2006-01-02T15:04Z07:00"} {
		if parsed, err := time.Parse(layout, raw); err == nil {
			return parsed, true
		}
	}
	return time.Time{}, false
}

func mapStatus(state string, completed bool) providers.Status {
	if completed || state == "post" {
		return providers.StatusFinished
	}
	switch state {
	case "in":
		return providers.StatusLive
	case "pre":
		return providers.StatusScheduled
	case "postponed":
		return providers.StatusPostponed
	case "canceled", "cancelled":
		return providers.StatusCancelled
	default:
		return providers.StatusUnknown
	}
}

func parseScore(raw string) *int {
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

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
