// Package espn reads ESPN's public scoreboards.
//
// It is the live-state provider: no API key, status and scores only. ESPN does
// provide real event ids, so matching one of its fixtures confirms an event's
// identity instead of leaving it provisional.
//
// The same response shape covers soccer, basketball and American football, so
// one adapter serves every supported sport.
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

const baseURL = "https://site.api.espn.com/apis/site/v2/sports"

// league describes where one of our competitions lives on ESPN.
type league struct {
	// Path is the sport and league path after /sports.
	Path string
	// Sport is our canonical sport slug for that path.
	Sport string
}

// leagues maps our competition slugs to ESPN coordinates.
var leagues = map[string]league{
	"premier-league":    {"soccer/eng.1", "football"},
	"la-liga":           {"soccer/esp.1", "football"},
	"serie-a":           {"soccer/ita.1", "football"},
	"bundesliga":        {"soccer/ger.1", "football"},
	"ligue-1":           {"soccer/fra.1", "football"},
	"copa-del-rey":      {"soccer/esp.copa_del_rey", "football"},
	"nations-league":    {"soccer/uefa.nations", "football"},
	"champions-league":  {"soccer/uefa.champions", "football"},
	"argentina-primera": {"soccer/arg.1", "football"},
	"chile-primera":     {"soccer/chi.1", "football"},
	"paraguay-primera":  {"soccer/par.1", "football"},
	"uruguay-primera":   {"soccer/uru.1", "football"},
	"italy-serie-c":     {"soccer/ita.3", "football"},

	"nba": {"basketball/nba", "basketball"},
	"nfl": {"football/nfl", "american-football"},
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
	spec, ok := leagues[competitionSlug]
	if !ok {
		return nil, fmt.Errorf("espn: no league code for %q", competitionSlug)
	}
	day := date.UTC().Format("20060102")
	res, err := c.http.Get(ctx, providers.Request{
		Provider: "espn",
		CacheKey: competitionSlug + "/scoreboard/" + day,
		URL:      fmt.Sprintf("%s/%s/scoreboard?dates=%s", baseURL, spec.Path, day),
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
		// Cup and neutral-site fixtures are published before the teams are
		// known. "TBD" is not a team, and importing it would collapse every
		// unresolved fixture into one event.
		if isPlaceholder(home) || isPlaceholder(away) {
			continue
		}

		start, ok := parseTime(event.Date)
		if !ok {
			continue
		}

		status := mapStatus(competition.Status.Type.State, competition.Status.Type.Completed)
		if status != providers.StatusLive && status != providers.StatusFinished {
			// ESPN publishes "0" before kickoff. A scheduled game has no score,
			// and zero must mean a reported zero.
			homeScore, awayScore = nil, nil
		}

		states = append(states, providers.LiveState{
			ProviderEventID: event.ID,
			Sport:           spec.Sport,
			CompetitionSlug: competitionSlug,
			StartUTC:        start,
			Status:          status,
			Detail:          firstNonEmpty(competition.Status.Type.Detail, competition.Status.Type.Short),
			HomeName:        home,
			AwayName:        away,
			HomeScore:       homeScore,
			AwayScore:       awayScore,
		})
	}
	return states, nil
}

// isPlaceholder reports whether ESPN published a slot instead of a team.
func isPlaceholder(name string) bool {
	upper := strings.ToUpper(strings.TrimSpace(name))
	for _, prefix := range []string{"TBD", "TBA", "TBC", "WINNER", "LOSER"} {
		if strings.HasPrefix(upper, prefix) {
			return true
		}
	}
	return false
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
