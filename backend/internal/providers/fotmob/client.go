// Package fotmob reads FotMob's public match API.
//
// It is a statistics source: live scores exist in several providers, but corner
// counts are published here for competitions where ESPN returns an empty
// statistics block. The API is undocumented and can change; every failure is
// reported, never smoothed over.
package fotmob

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/oyins/oddsvault/backend/internal/providers"
)

const baseURL = "https://www.fotmob.com/api/data"

// Client fetches FotMob data.
type Client struct {
	http *providers.Client
}

// New returns a FotMob adapter.
func New(client *providers.Client) *Client { return &Client{http: client} }

// Match is one fixture from the daily board.
type Match struct {
	ID         int64
	LeagueID   int64
	LeagueName string
	Home       string
	Away       string
	HomeScore  *int
	AwayScore  *int
	Live       bool
	Finished   bool
	Detail     string
	StartUTC   time.Time
}

type matchesPayload struct {
	Leagues []struct {
		ID        int64  `json:"id"`
		PrimaryID int64  `json:"primaryId"`
		Name      string `json:"name"`
		Matches   []struct {
			ID   int64 `json:"id"`
			Home struct {
				Name  string `json:"name"`
				Score *int   `json:"score"`
			} `json:"home"`
			Away struct {
				Name  string `json:"name"`
				Score *int   `json:"score"`
			} `json:"away"`
			Status struct {
				UTC      string `json:"utcTime"`
				Started  bool   `json:"started"`
				Finished bool   `json:"finished"`
				Cancel   bool   `json:"cancelled"`
				Reason   any    `json:"reason"`
				ScoreStr string `json:"scoreStr"`
				LiveTime struct {
					Short string `json:"short"`
				} `json:"liveTime"`
			} `json:"status"`
		} `json:"matches"`
	} `json:"leagues"`
}

// FetchMatches returns every fixture FotMob lists for one date.
func (c *Client) FetchMatches(ctx context.Context, date time.Time) ([]Match, error) {
	day := date.UTC().Format("20060102")
	res, err := c.http.Get(ctx, providers.Request{
		Provider: "fotmob",
		CacheKey: "matches/" + day,
		URL:      fmt.Sprintf("%s/matches?date=%s", baseURL, day),
		MaxAge:   60 * time.Second,
		Headers:  map[string]string{"Accept": "application/json"},
	})
	if err != nil {
		return nil, err
	}

	var doc matchesPayload
	if err := json.Unmarshal(res.Body, &doc); err != nil {
		return nil, fmt.Errorf("fotmob: decode matches: %w", err)
	}

	out := make([]Match, 0, 64)
	for _, league := range doc.Leagues {
		for _, raw := range league.Matches {
			match := Match{
				ID:         raw.ID,
				LeagueID:   league.ID,
				LeagueName: league.Name,
				Home:       raw.Home.Name,
				Away:       raw.Away.Name,
				HomeScore:  raw.Home.Score,
				AwayScore:  raw.Away.Score,
				Live:       raw.Status.Started && !raw.Status.Finished && !raw.Status.Cancel,
				Finished:   raw.Status.Finished,
				Detail:     firstNonEmpty(raw.Status.LiveTime.Short, stringReason(raw.Status.Reason)),
			}
			if start, ok := parseTime(raw.Status.UTC); ok {
				match.StartUTC = start
			}
			if match.Home == "" || match.Away == "" {
				continue
			}
			out = append(out, match)
		}
	}
	return out, nil
}

// Corners is the corner count for one match. A nil pair means the provider has
// not published statistics yet.
type Corners struct {
	Home *int
	Away *int
}

type statEntry struct {
	Title string `json:"title"`
	Stats []any  `json:"stats"`
}

type statGroup struct {
	Title string      `json:"title"`
	Stats []statEntry `json:"stats"`
}

type detailsPayload struct {
	Content struct {
		Stats struct {
			Periods map[string]struct {
				Stats []statGroup `json:"stats"`
			} `json:"Periods"`
		} `json:"stats"`
	} `json:"content"`
}

// FetchCorners reads the current corner count from a match's statistics.
func (c *Client) FetchCorners(ctx context.Context, matchID int64) (Corners, error) {
	res, err := c.http.Get(ctx, providers.Request{
		Provider: "fotmob",
		CacheKey: fmt.Sprintf("match/%d", matchID),
		URL:      fmt.Sprintf("%s/matchDetails?matchId=%d", baseURL, matchID),
		MaxAge:   45 * time.Second,
		Headers:  map[string]string{"Accept": "application/json"},
	})
	if err != nil {
		return Corners{}, err
	}

	var doc detailsPayload
	if err := json.Unmarshal(res.Body, &doc); err != nil {
		return Corners{}, fmt.Errorf("fotmob: decode match %d: %w", matchID, err)
	}

	for _, period := range doc.Content.Stats.Periods {
		for _, group := range period.Stats {
			for _, stat := range group.Stats {
				if !strings.EqualFold(stat.Title, "Corners") || len(stat.Stats) < 2 {
					continue
				}
				return Corners{Home: toInt(stat.Stats[0]), Away: toInt(stat.Stats[1])}, nil
			}
		}
	}
	return Corners{}, nil
}

func toInt(value any) *int {
	switch typed := value.(type) {
	case float64:
		v := int(typed)
		return &v
	case string:
		parsed, err := strconv.Atoi(strings.TrimSpace(typed))
		if err != nil {
			return nil
		}
		return &parsed
	default:
		return nil
	}
}

func parseTime(raw string) (time.Time, bool) {
	if raw == "" {
		return time.Time{}, false
	}
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02T15:04:05.000Z"} {
		if parsed, err := time.Parse(layout, raw); err == nil {
			return parsed, true
		}
	}
	return time.Time{}, false
}

// stringReason accepts a reason that is either a plain string or an object,
// because FotMob mixes both shapes in the same response.
func stringReason(value any) string {
	if text, ok := value.(string); ok {
		return text
	}
	return ""
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

// TeamSuggestion is a team result from FotMob's search.
type TeamSuggestion struct {
	ID   int64
	Name string
}

type searchPayload []struct {
	Suggestions []struct {
		Type string `json:"type"`
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"suggestions"`
}

// SearchTeams looks a club up by name. It is used only to find artwork: the
// collective is matched by exact name afterwards, never by proximity.
func (c *Client) SearchTeams(ctx context.Context, name string) ([]TeamSuggestion, error) {
	res, err := c.http.Get(ctx, providers.Request{
		Provider: "fotmob",
		CacheKey: "search/" + name,
		URL:      fmt.Sprintf("%s/search/suggest?term=%s&lang=en", baseURL, url.QueryEscape(name)),
		MaxAge:   24 * time.Hour,
		Headers:  map[string]string{"Accept": "application/json"},
	})
	if err != nil {
		return nil, err
	}

	var doc searchPayload
	if err := json.Unmarshal(res.Body, &doc); err != nil {
		return nil, fmt.Errorf("fotmob: decode search: %w", err)
	}

	out := make([]TeamSuggestion, 0, 4)
	for _, group := range doc {
		for _, item := range group.Suggestions {
			if item.Type != "team" {
				continue
			}
			id, err := strconv.ParseInt(item.ID, 10, 64)
			if err != nil || id <= 0 {
				continue
			}
			out = append(out, TeamSuggestion{ID: id, Name: item.Name})
		}
	}
	return out, nil
}

// TeamLogoURL builds the badge URL for a FotMob team id.
func TeamLogoURL(id int64) string {
	return fmt.Sprintf("https://images.fotmob.com/image_resources/logo/teamlogo/%d_small.png", id)
}
