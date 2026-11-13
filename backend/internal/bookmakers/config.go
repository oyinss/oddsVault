// Package bookmakers maps bookmaker feeds to canonical competitions.
package bookmakers

// Feed describes one bookmaker competition feed.
type Feed struct {
	BookmakerSlug   string
	CompetitionSlug string
	// CategoryID is the provider's competition identifier.
	CategoryID string
}

// Feeds is the Phase 2 capture set. Add one line per bookmaker/competition;
// never guess a mapping that has not been checked against the live feed.
var Feeds = []Feed{
	{
		BookmakerSlug:   "livescorebet",
		CompetitionSlug: "premier-league",
		CategoryID:      "SBTC3_40253",
	},
	{
		BookmakerSlug:   "livescorebet",
		CompetitionSlug: "nations-league",
		CategoryID:      "SBTC3_110988",
	},
	{
		BookmakerSlug:   "livescorebet",
		CompetitionSlug: "champions-league",
		CategoryID:      "SBTC3_40685",
	},
	{
		BookmakerSlug:   "livescorebet",
		CompetitionSlug: "nba",
		CategoryID:      "SBTC3_42648",
	},
	{
		BookmakerSlug:   "livescorebet",
		CompetitionSlug: "nfl",
		CategoryID:      "SBTC3_88808",
	},
}
