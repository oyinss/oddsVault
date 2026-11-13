// Package providers defines the normalized shapes that every external sports
// source is converted into before it can touch the database.
package providers

import "time"

// Status is the normalized event status.
type Status string

const (
	StatusScheduled Status = "scheduled"
	StatusLive      Status = "live"
	StatusFinished  Status = "finished"
	StatusPostponed Status = "postponed"
	StatusCancelled Status = "cancelled"
	StatusAbandoned Status = "abandoned"
	StatusUnknown   Status = "unknown"
)

// TeamRef is a provider's view of one team.
type TeamRef struct {
	Name string
}

// Fixture is one normalized event. Scores are pointers because 0 and "not
// reported" must stay distinguishable.
type Fixture struct {
	Provider        string
	ProviderEventID string
	CompetitionSlug string
	SeasonName      string
	Round           string

	// StartDate is the fixture's calendar date in the source's own local time.
	StartDate time.Time
	// StartTimeLocal keeps the source's wall-clock kickoff, e.g. "20:00".
	StartTimeLocal string

	Status      Status
	Home        TeamRef
	Away        TeamRef
	HomeScore   *int
	AwayScore   *int
	HomeScoreHT *int
	AwayScoreHT *int

	// Odds are optional closing prices supplied by the same source.
	Odds []OddsQuote
}

// OddsQuote is one price observed for one selection.
type OddsQuote struct {
	Bookmaker  string
	MarketType string
	Line       *float64
	Selection  string
	Decimal    float64
}

// LiveState is a provider's current view of one event.
type LiveState struct {
	// Provider names the source of this state, e.g. "espn" or "fotmob".
	Provider        string
	ProviderEventID string
	// Sport is the canonical sport slug (football, basketball, ...). Empty
	// means football, the provider's original scope.
	Sport           string
	CompetitionSlug string
	StartUTC        time.Time
	Status          Status
	Detail          string
	HomeName        string
	AwayName        string
	HomeScore       *int
	AwayScore       *int
}

// Int returns a pointer to v, for building optional score fields.
func Int(v int) *int { return &v }

// Finished reports whether both final scores were reported.
func (f Fixture) Finished() bool {
	return f.HomeScore != nil && f.AwayScore != nil
}
