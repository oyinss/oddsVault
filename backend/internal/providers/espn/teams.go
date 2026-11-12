package espn

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/oyins/oddsvault/backend/internal/providers"
)

// Team is an ESPN club with its badge URL.
type Team struct {
	ProviderID string
	Name       string
	LogoURL    string
}

type teamsPayload struct {
	Sports []struct {
		Leagues []struct {
			Teams []struct {
				Team struct {
					ID          string `json:"id"`
					DisplayName string `json:"displayName"`
					Logos       []struct {
						Href string `json:"href"`
					} `json:"logos"`
				} `json:"team"`
			} `json:"teams"`
		} `json:"leagues"`
	} `json:"sports"`
}

// FetchTeams lists a competition's clubs and their badges.
func (c *Client) FetchTeams(ctx context.Context, competitionSlug string) ([]Team, error) {
	league, ok := leagues[competitionSlug]
	if !ok {
		return nil, fmt.Errorf("espn: no league code for %q", competitionSlug)
	}

	res, err := c.http.Get(ctx, providers.Request{
		Provider: "espn",
		CacheKey: league + "/teams",
		URL:      fmt.Sprintf("%s/%s/teams", baseURL, league),
		MaxAge:   24 * time.Hour,
		Headers:  map[string]string{"Accept": "application/json"},
	})
	if err != nil {
		return nil, err
	}

	var doc teamsPayload
	if err := json.Unmarshal(res.Body, &doc); err != nil {
		return nil, fmt.Errorf("espn: decode teams: %w", err)
	}

	var teams []Team
	for _, sport := range doc.Sports {
		for _, leagueDoc := range sport.Leagues {
			for _, entry := range leagueDoc.Teams {
				logo := ""
				if len(entry.Team.Logos) > 0 {
					logo = entry.Team.Logos[0].Href
				}
				if entry.Team.DisplayName == "" {
					continue
				}
				teams = append(teams, Team{
					ProviderID: entry.Team.ID,
					Name:       entry.Team.DisplayName,
					LogoURL:    logo,
				})
			}
		}
	}
	return teams, nil
}
