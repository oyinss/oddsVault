// Package espn reads ESPN's public scoreboards.
//
// It is the live-state provider: no API key, status and scores only. ESPN does
// provide real event ids, so matching one of its fixtures confirms an event's
// identity instead of leaving it provisional.
//
// The same response shape covers soccer, basketball and American football, so
// one adapter serves every supported sport. The all-soccer board is also used to
// discover live fixtures outside the seeded competition catalogue.
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

type scoreEvent struct {
	ID    string `json:"id"`
	Date  string `json:"date"`
	Links []struct {
		Href string `json:"href"`
	} `json:"links"`
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
}

type scoreboard struct {
	Leagues []struct {
		Name string `json:"name"`
	} `json:"leagues"`
	Events []scoreEvent `json:"events"`
}

// AllLiveEvent is one live fixture discovered on the all-soccer board, with the
// ESPN league code it belongs to.
type AllLiveEvent struct {
	LeagueCode string
	State      providers.LiveState
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
		state, ok := buildState(event, competitionSlug, spec.Sport)
		if !ok {
			continue
		}
		states = append(states, state)
	}
	return states, nil
}

// FetchAllLive returns every live soccer fixture ESPN lists right now, with the
// league code taken from the event link. It is a discovery call: the caller
// resolves or creates the competition before importing.
func (c *Client) FetchAllLive(ctx context.Context) ([]AllLiveEvent, error) {
	res, err := c.http.Get(ctx, providers.Request{
		Provider: "espn",
		CacheKey: "soccer/all/live",
		URL:      baseURL + "/soccer/all/scoreboard",
		MaxAge:   30 * time.Second,
		Headers:  map[string]string{"Accept": "application/json"},
	})
	if err != nil {
		return nil, err
	}

	var doc scoreboard
	if err := json.Unmarshal(res.Body, &doc); err != nil {
		return nil, fmt.Errorf("espn: decode all-soccer scoreboard: %w", err)
	}

	out := make([]AllLiveEvent, 0, 16)
	for _, event := range doc.Events {
		code := leagueCode(event)
		if code == "" {
			continue
		}
		state, ok := buildState(event, "", "football")
		if !ok || state.Status != providers.StatusLive {
			continue
		}
		out = append(out, AllLiveEvent{LeagueCode: code, State: state})
	}
	return out, nil
}

// FetchLeagueName reads a league's display name from its own scoreboard.
func (c *Client) FetchLeagueName(ctx context.Context, code string) (string, error) {
	res, err := c.http.Get(ctx, providers.Request{
		Provider: "espn",
		CacheKey: "soccer/" + code + "/name",
		URL:      fmt.Sprintf("%s/soccer/%s/scoreboard", baseURL, code),
		MaxAge:   24 * time.Hour,
		Headers:  map[string]string{"Accept": "application/json"},
	})
	if err != nil {
		return "", err
	}
	var doc scoreboard
	if err := json.Unmarshal(res.Body, &doc); err != nil {
		return "", fmt.Errorf("espn: decode league %s: %w", code, err)
	}
	if len(doc.Leagues) == 0 || strings.TrimSpace(doc.Leagues[0].Name) == "" {
		return "", fmt.Errorf("espn: league %s has no name", code)
	}
	return strings.TrimSpace(doc.Leagues[0].Name), nil
}

// leagueCode reads the ESPN league code out of an event's player-stats link,
// which is the only place the all-soccer board carries it.
func leagueCode(event scoreEvent) string {
	for _, link := range event.Links {
		index := strings.Index(link.Href, "/league/")
		if index < 0 {
			continue
		}
		code := link.Href[index+len("/league/"):]
		if cut := strings.IndexAny(code, "/?#"); cut >= 0 {
			code = code[:cut]
		}
		if code != "" {
			return code
		}
	}
	return ""
}

// buildState normalizes one scoreboard event. competitionSlug may be empty for
// discovery, where the caller fills it in after resolving the league.
func buildState(event scoreEvent, competitionSlug, sport string) (providers.LiveState, bool) {
	if len(event.Competitions) == 0 {
		return providers.LiveState{}, false
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
		return providers.LiveState{}, false
	}
	// Cup and neutral-site fixtures are published before the teams are known.
	if isPlaceholder(home) || isPlaceholder(away) {
		return providers.LiveState{}, false
	}

	start, ok := parseTime(event.Date)
	if !ok {
		return providers.LiveState{}, false
	}

	status := mapStatus(competition.Status.Type.State, competition.Status.Type.Completed)
	if status != providers.StatusLive && status != providers.StatusFinished {
		// ESPN publishes "0" before kickoff. A scheduled game has no score, and
		// zero must mean a reported zero.
		homeScore, awayScore = nil, nil
	}

	return providers.LiveState{
		ProviderEventID: event.ID,
		Sport:           sport,
		CompetitionSlug: competitionSlug,
		StartUTC:        start,
		Status:          status,
		Detail:          firstNonEmpty(competition.Status.Type.Detail, competition.Status.Type.Short),
		HomeName:        home,
		AwayName:        away,
		HomeScore:       homeScore,
		AwayScore:       awayScore,
	}, true
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
