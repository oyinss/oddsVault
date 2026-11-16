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

type statPeriod struct {
	Stats []statGroup `json:"stats"`
}

type detailsPayload struct {
	Content struct {
		Stats struct {
			Periods map[string]statPeriod `json:"Periods"`
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

	// A finished match publishes "All", "FirstHalf" and "SecondHalf". Reading
	// whichever period a Go map yields first would silently record first-half
	// corners as the match total, so "All" is required before any fallback.
	periods := doc.Content.Stats.Periods
	if all, ok := periods["All"]; ok {
		if corners, found := cornersInPeriod(all); found {
			return corners, nil
		}
	}
	// Some in-play payloads only publish the current period. Sum the halves
	// rather than presenting one half as the whole match.
	var home, away int
	var sawHalf bool
	for _, key := range []string{"FirstHalf", "SecondHalf"} {
		period, ok := periods[key]
		if !ok {
			continue
		}
		corners, found := cornersInPeriod(period)
		if !found || corners.Home == nil || corners.Away == nil {
			return Corners{}, nil
		}
		home += *corners.Home
		away += *corners.Away
		sawHalf = true
	}
	if sawHalf {
		return Corners{Home: &home, Away: &away}, nil
	}
	return Corners{}, nil
}

func cornersInPeriod(period statPeriod) (Corners, bool) {
	for _, group := range period.Stats {
		for _, stat := range group.Stats {
			if !strings.EqualFold(stat.Title, "Corners") || len(stat.Stats) < 2 {
				continue
			}
			return Corners{Home: toInt(stat.Stats[0]), Away: toInt(stat.Stats[1])}, true
		}
	}
	return Corners{}, false
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

// GoalEvent is one goal with the score before it, so a match can be replayed.
type GoalEvent struct {
	Minute    int
	Home      bool
	HomeScore *int
	AwayScore *int
	Player    string
	OwnGoal   bool
	Shootout  bool
}

// AttackStats is the pressure behind the corners. Nils mean the provider did
// not publish the stat.
type AttackStats struct {
	Shots         *int
	ShotsOnTarget *int
	BigChances    *int
	Passes        *int
	XG            *float64
	Possession    *float64
}

// Extra bundles the goal timeline and both teams' attacking stats.
type Extra struct {
	Goals []GoalEvent
	Home  AttackStats
	Away  AttackStats
}

type extraPayload struct {
	Content struct {
		MatchFacts struct {
			Events struct {
				Events []struct {
					Type      string `json:"type"`
					Time      int    `json:"time"`
					IsHome    *bool  `json:"isHome"`
					HomeScore *int   `json:"homeScore"`
					AwayScore *int   `json:"awayScore"`
					OwnGoal   *bool  `json:"ownGoal"`
					Shootout  bool   `json:"isPenaltyShootoutEvent"`
					Player    struct {
						Name string `json:"name"`
					} `json:"player"`
				} `json:"events"`
			} `json:"events"`
		} `json:"matchFacts"`
		Stats struct {
			Periods map[string]statPeriod `json:"Periods"`
		} `json:"stats"`
	} `json:"content"`
}

// FetchExtra reads goals and attacking stats for one finished match.
func (c *Client) FetchExtra(ctx context.Context, matchID int64) (Extra, error) {
	res, err := c.http.Get(ctx, providers.Request{
		Provider: "fotmob",
		CacheKey: fmt.Sprintf("match/%d", matchID),
		URL:      fmt.Sprintf("%s/matchDetails?matchId=%d", baseURL, matchID),
		MaxAge:   45 * time.Second,
		Headers:  map[string]string{"Accept": "application/json"},
	})
	if err != nil {
		return Extra{}, err
	}

	var doc extraPayload
	if err := json.Unmarshal(res.Body, &doc); err != nil {
		return Extra{}, fmt.Errorf("fotmob: decode extras %d: %w", matchID, err)
	}

	var out Extra
	for _, event := range doc.Content.MatchFacts.Events.Events {
		if event.Type != "Goal" || event.IsHome == nil || event.Shootout {
			continue
		}
		if event.Time <= 0 || event.Time > 130 {
			continue
		}
		out.Goals = append(out.Goals, GoalEvent{
			Minute:    event.Time,
			Home:      *event.IsHome,
			HomeScore: event.HomeScore,
			AwayScore: event.AwayScore,
			Player:    event.Player.Name,
			OwnGoal:   event.OwnGoal != nil && *event.OwnGoal,
			Shootout:  event.Shootout,
		})
	}

	all, ok := doc.Content.Stats.Periods["All"]
	if !ok {
		return out, nil
	}
	for _, group := range all.Stats {
		for _, stat := range group.Stats {
			if len(stat.Stats) < 2 {
				continue
			}
			switch strings.ToLower(stat.Title) {
			case "total shots":
				out.Home.Shots = toInt(stat.Stats[0])
				out.Away.Shots = toInt(stat.Stats[1])
			case "shots on target":
				out.Home.ShotsOnTarget = toInt(stat.Stats[0])
				out.Away.ShotsOnTarget = toInt(stat.Stats[1])
			case "big chances":
				out.Home.BigChances = toInt(stat.Stats[0])
				out.Away.BigChances = toInt(stat.Stats[1])
			case "passes":
				out.Home.Passes = toInt(stat.Stats[0])
				out.Away.Passes = toInt(stat.Stats[1])
			case "expected goals (xg)":
				out.Home.XG = toFloat(stat.Stats[0])
				out.Away.XG = toFloat(stat.Stats[1])
			case "possession":
				out.Home.Possession = toFloat(stat.Stats[0])
				out.Away.Possession = toFloat(stat.Stats[1])
			}
		}
	}
	return out, nil
}

func toFloat(value any) *float64 {
	switch typed := value.(type) {
	case float64:
		return &typed
	case string:
		parsed, err := strconv.ParseFloat(strings.TrimSpace(typed), 64)
		if err != nil {
			return nil
		}
		return &parsed
	default:
		return nil
	}
}
