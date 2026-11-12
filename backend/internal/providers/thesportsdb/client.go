// Package thesportsdb looks up club badges by name.
//
// It is used only for teams that have no ESPN asset: lower leagues, reserve and
// cup sides that appear in bet history. Matching is strict (normalized names
// must be equal) so a senior badge is never attached to a youth team.
package thesportsdb

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"time"

	"github.com/oyins/oddsvault/backend/internal/providers"
)

const baseURL = "https://www.thesportsdb.com/api/v1/json"

// Client queries TheSportsDB.
type Client struct {
	http *providers.Client
	key  string
}

// New returns a TheSportsDB adapter.
func New(client *providers.Client) *Client {
	key := os.Getenv("THESPORTSDB_KEY")
	if key == "" {
		key = "123"
	}
	return &Client{http: client, key: key}
}

// Team is a search result.
type Team struct {
	Name   string
	Badge  string
	League string
}

type payload struct {
	Teams []struct {
		Name      string `json:"strTeam"`
		Sport     string `json:"strSport"`
		Badge     string `json:"strBadge"`
		League    string `json:"strLeague"`
		TeamBadge string `json:"strTeamBadge"`
	} `json:"teams"`
}

// Search returns soccer teams matching a name.
func (c *Client) Search(ctx context.Context, name string) ([]Team, error) {
	endpoint := fmt.Sprintf("%s/%s/searchteams.php?t=%s", baseURL, c.key, url.QueryEscape(name))
	res, err := c.http.Get(ctx, providers.Request{
		Provider: "thesportsdb",
		CacheKey: "searchteams/" + name,
		URL:      endpoint,
		MaxAge:   7 * 24 * time.Hour,
	})
	if err != nil {
		return nil, err
	}

	var doc payload
	if err := json.Unmarshal(res.Body, &doc); err != nil {
		return nil, fmt.Errorf("thesportsdb: decode search: %w", err)
	}

	out := make([]Team, 0, len(doc.Teams))
	for _, team := range doc.Teams {
		if team.Sport != "Soccer" {
			continue
		}
		badge := team.Badge
		if badge == "" {
			badge = team.TeamBadge
		}
		if badge == "" {
			continue
		}
		out = append(out, Team{Name: team.Name, Badge: badge, League: team.League})
	}
	return out, nil
}
