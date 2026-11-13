package espn

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/oyins/oddsvault/backend/internal/providers"
)

// MatchStats is the subset of a match summary the platform verifies against.
type MatchStats struct {
	Completed   bool
	Detail      string
	HomeCorners *int
	AwayCorners *int
}

type summaryPayload struct {
	Header struct {
		Competitions []struct {
			Status struct {
				Type struct {
					Completed bool   `json:"completed"`
					Detail    string `json:"detail"`
				} `json:"type"`
			} `json:"status"`
			Competitors []struct {
				HomeAway string `json:"homeAway"`
				Team     struct {
					ID string `json:"id"`
				} `json:"team"`
			} `json:"competitors"`
		} `json:"competitions"`
	} `json:"header"`
	Boxscore struct {
		Teams []struct {
			Team struct {
				ID string `json:"id"`
			} `json:"team"`
			Statistics []struct {
				Name         string `json:"name"`
				DisplayValue string `json:"displayValue"`
			} `json:"statistics"`
		} `json:"teams"`
	} `json:"boxscore"`
}

// FetchMatchStats reads corner counts and completion state for one event.
func (c *Client) FetchMatchStats(ctx context.Context, competitionSlug, eventID string) (MatchStats, error) {
	// The caller passes an ESPN league code when the competition was discovered
	// outside the catalogue, and one of our slugs otherwise.
	path := ""
	if spec, ok := leagues[competitionSlug]; ok {
		path = spec.Path
	} else if competitionSlug != "" {
		path = "soccer/" + competitionSlug
	}
	if path == "" {
		return MatchStats{}, fmt.Errorf("espn: no league code for %q", competitionSlug)
	}

	res, err := c.http.Get(ctx, providers.Request{
		Provider: "espn",
		CacheKey: competitionSlug + "/summary/" + eventID,
		URL:      fmt.Sprintf("%s/%s/summary?event=%s", baseURL, path, eventID),
		MaxAge:   time.Hour,
		Headers:  map[string]string{"Accept": "application/json"},
	})
	if err != nil {
		return MatchStats{}, err
	}

	var doc summaryPayload
	if err := json.Unmarshal(res.Body, &doc); err != nil {
		return MatchStats{}, fmt.Errorf("espn: decode summary: %w", err)
	}
	if len(doc.Header.Competitions) == 0 {
		return MatchStats{}, fmt.Errorf("espn: summary has no competition")
	}

	competition := doc.Header.Competitions[0]
	stats := MatchStats{
		Completed: competition.Status.Type.Completed,
		Detail:    competition.Status.Type.Detail,
	}

	homeID, awayID := "", ""
	for _, team := range competition.Competitors {
		switch team.HomeAway {
		case "home":
			homeID = team.Team.ID
		case "away":
			awayID = team.Team.ID
		}
	}

	// Some competitions publish a placeholder statistics block where every
	// number is "0". That is an absent statistic, not a real zero, so it must
	// not reach the corner board. Possession or shots above zero proves the
	// block is real.
	realStats := false

	for _, team := range doc.Boxscore.Teams {
		var corners *int
		for _, stat := range team.Statistics {
			switch stat.Name {
			case "possessionPct", "totalShots":
				if value, err := strconv.ParseFloat(stat.DisplayValue, 64); err == nil && value > 0 {
					realStats = true
				}
			case "wonCorners":
				if value, err := strconv.Atoi(stat.DisplayValue); err == nil {
					v := value
					corners = &v
				}
			}
		}
		switch team.Team.ID {
		case homeID:
			stats.HomeCorners = corners
		case awayID:
			stats.AwayCorners = corners
		}
	}
	if !realStats {
		stats.HomeCorners = nil
		stats.AwayCorners = nil
	}
	return stats, nil
}
