package espn

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
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
	spec, ok := leagues[competitionSlug]
	if !ok {
		return nil, fmt.Errorf("espn: no league code for %q", competitionSlug)
	}

	res, err := c.http.Get(ctx, providers.Request{
		Provider: "espn",
		CacheKey: competitionSlug + "/teams",
		URL:      fmt.Sprintf("%s/%s/teams", baseURL, spec.Path),
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

// TeamSearchResult is a club found by ESPN's public search.
type TeamSearchResult struct {
	Name    string
	LogoURL string
}

type searchPayload struct {
	Items []struct {
		DisplayName string `json:"displayName"`
		Name        string `json:"name"`
		Logos       []struct {
			Href string   `json:"href"`
			Rel  []string `json:"rel"`
		} `json:"logos"`
	} `json:"items"`
}

// SearchTeam looks a club up by name so a missing badge can be filled. The
// caller decides whether the name is an exact match.
func (c *Client) SearchTeam(ctx context.Context, query string) ([]TeamSearchResult, error) {
	requestURL := fmt.Sprintf(
		"https://site.api.espn.com/apis/common/v3/search?query=%s&type=team&sport=soccer&limit=10",
		url.QueryEscape(query))
	res, err := c.http.Get(ctx, providers.Request{
		Provider: "espn",
		CacheKey: "search/" + query,
		URL:      requestURL,
		MaxAge:   24 * time.Hour,
		Headers:  map[string]string{"Accept": "application/json"},
	})
	if err != nil {
		return nil, err
	}

	var doc searchPayload
	if err := json.Unmarshal(res.Body, &doc); err != nil {
		return nil, fmt.Errorf("espn: decode search: %w", err)
	}

	out := make([]TeamSearchResult, 0, len(doc.Items))
	for _, item := range doc.Items {
		name := firstNonEmpty(item.DisplayName, item.Name)
		if name == "" {
			continue
		}
		logo := ""
		for _, candidate := range item.Logos {
			logo = candidate.Href
			for _, rel := range candidate.Rel {
				if rel == "default" {
					break
				}
			}
			break
		}
		out = append(out, TeamSearchResult{Name: name, LogoURL: logo})
	}
	return out, nil
}
