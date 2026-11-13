// Package competitions holds the canonical competition catalogue and the
// provider coordinates needed to fetch each one.
//
// Slugs must match the rows seeded in migrations; a mismatch is a boot error.
package competitions

import (
	"fmt"
	"time"
)

// Competition describes one league/tournament and where its data comes from.
type Competition struct {
	Slug                 string
	Name                 string
	CountrySlug          string
	Sport                string
	Timezone             string
	FootballDataDivision string
	OpenFootballFile     string
	FotmobLeagueID       int
	ESPNCode             string
}

// All is the canonical catalogue. Keep in sync with
// 0006_seed_football_reference.sql.
var All = []Competition{
	{
		Slug: "premier-league", Name: "Premier League", CountrySlug: "england",
		Timezone: "Europe/London", FootballDataDivision: "E0", OpenFootballFile: "en.1.json", FotmobLeagueID: 47,
	},
	{
		Slug: "la-liga", Name: "La Liga", CountrySlug: "spain",
		Timezone: "Europe/Madrid", FootballDataDivision: "SP1", OpenFootballFile: "es.1.json", FotmobLeagueID: 87,
	},
	{
		Slug: "serie-a", Name: "Serie A", CountrySlug: "italy",
		Timezone: "Europe/Rome", FootballDataDivision: "I1", OpenFootballFile: "it.1.json", FotmobLeagueID: 55,
	},
	{
		Slug: "bundesliga", Name: "Bundesliga", CountrySlug: "germany",
		Timezone: "Europe/Berlin", FootballDataDivision: "D1", OpenFootballFile: "de.1.json", FotmobLeagueID: 54,
	},
	{
		Slug: "ligue-1", Name: "Ligue 1", CountrySlug: "france",
		Timezone: "Europe/Paris", FootballDataDivision: "F1", OpenFootballFile: "fr.1.json",
		FotmobLeagueID: 53, ESPNCode: "fra.1",
	},
	{
		// Cup coverage exists only to identify bets placed on it. It is not in
		// Enabled, so it never joins the league sync.
		Slug: "copa-del-rey", Name: "Copa del Rey", CountrySlug: "spain",
		Timezone: "Europe/Madrid", ESPNCode: "esp.copa_del_rey",
	},
	{
		Slug: "nations-league", Name: "UEFA Nations League",
		Timezone: "UTC", ESPNCode: "uefa.nations",
	},
	{
		Slug: "champions-league", Name: "UEFA Champions League",
		Timezone: "UTC", ESPNCode: "uefa.champions",
	},
	{
		Slug: "argentina-primera", Name: "Argentine Primera División",
		Timezone: "America/Argentina/Buenos_Aires", ESPNCode: "arg.1",
	},
	{
		Slug: "chile-primera", Name: "Chilean Primera División",
		Timezone: "America/Santiago", ESPNCode: "chi.1",
	},
	{
		Slug: "paraguay-primera", Name: "Paraguayan Primera División",
		Timezone: "America/Asuncion", ESPNCode: "par.1",
	},
	{
		Slug: "uruguay-primera", Name: "Uruguayan Primera División",
		Timezone: "America/Montevideo", ESPNCode: "uru.1",
	},
	{
		Slug: "italy-serie-c", Name: "Italian Serie C",
		Timezone: "Europe/Rome", ESPNCode: "ita.3",
	},
	{
		Slug: "nba", Name: "NBA", CountrySlug: "usa", Sport: "basketball",
		Timezone: "America/New_York", ESPNCode: "basketball/nba",
	},
	{
		Slug: "nfl", Name: "NFL", CountrySlug: "usa", Sport: "american-football",
		Timezone: "America/New_York", ESPNCode: "football/nfl",
	},
}

// SportSlug returns the canonical sport for a competition. An unset sport
// means football, the catalogue's original scope.
func (c Competition) SportSlug() string {
	if c.Sport == "" {
		return "football"
	}
	return c.Sport
}

// ESPNOnly lists competitions that only the live provider covers. They exist
// so bookmaker/ bet-history entries can be identified against real fixtures.
var ESPNOnly = []string{"copa-del-rey", "nations-league", "champions-league", "argentina-primera", "chile-primera", "paraguay-primera", "uruguay-primera", "italy-serie-c", "ligue-1"}

// Enabled lists the competitions the league sync covers. Expand deliberately: each new
// league needs alias coverage before it is turned on.
var Enabled = []string{"premier-league"}

// LiveEnabled lists the competitions whose live state is polled every couple of
// minutes. The other ESPN-only competitions still refresh through coverage, but
// at a slower cadence.
var LiveEnabled = []string{"premier-league", "nba", "nfl"}

// BySlug returns a competition or an error.
func BySlug(slug string) (Competition, error) {
	for _, c := range All {
		if c.Slug == slug {
			return c, nil
		}
	}
	return Competition{}, fmt.Errorf("unknown competition %q", slug)
}

// Selected returns the enabled competitions.
func Selected() ([]Competition, error) {
	return List(Enabled)
}

// List returns the named competitions in order.
func List(slugs []string) ([]Competition, error) {
	out := make([]Competition, 0, len(slugs))
	for _, slug := range slugs {
		c, err := BySlug(slug)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, nil
}

// Season identifies one competition season across providers.
type Season struct {
	Name      string // 2026/27
	Code      string // 2627    (football-data.co.uk path)
	Directory string // 2026-27 (openfootball path)
	StartYear int
}

// CurrentSeason derives the season containing now. European leagues start in
// July; anything before that belongs to the previous season.
func CurrentSeason(now time.Time) Season {
	start := now.Year()
	if now.Month() < time.July {
		start--
	}
	return SeasonFor(start)
}

// SeasonFor builds the three provider spellings for a season start year.
func SeasonFor(startYear int) Season {
	end := startYear + 1
	return Season{
		Name:      fmt.Sprintf("%d/%02d", startYear, end%100),
		Code:      fmt.Sprintf("%02d%02d", startYear%100, end%100),
		Directory: fmt.Sprintf("%d-%02d", startYear, end%100),
		StartYear: startYear,
	}
}
