package livescorebet

import "testing"

func TestQuotesMapsSupportedMarkets(t *testing.T) {
	event := Event{
		Home: "Manchester City",
		Away: "Fulham",
		Markets: []Market{
			{Name: "Full Time", PeriodType: "FT", Selections: []Selection{
				{Name: "Manchester City", Odds: 1.43},
				{Name: "Draw", Odds: 4.9},
				{Name: "Fulham", Odds: 6.5},
			}},
			{Name: "Double Chance", PeriodType: "FT", Selections: []Selection{
				{Name: "Manchester City or Draw", Odds: 1.08},
				{Name: "Manchester City or Fulham", Odds: 1.15},
				{Name: "Draw or Fulham", Odds: 2.7},
			}},
			{Name: "Both Teams To Score", PeriodType: "FT", Selections: []Selection{
				{Name: "Yes", Odds: 1.57},
				{Name: "No", Odds: 2.17},
			}},
			{Name: "Total Goals", PeriodType: "FT", Selections: []Selection{
				{Name: "Over 2.5", Odds: 1.44},
				{Name: "Under 2.5", Odds: 2.63},
				{Name: "Over 0.5", Odds: 0}, // unavailable in feed
			}},
			{Name: "Full Time - 1UP", PeriodType: "FT", Selections: []Selection{
				{Name: "Manchester City", Odds: 1.21},
			}},
			{Name: "Total Goals - 1st Half", PeriodType: "FH", Selections: []Selection{
				{Name: "Over 1.5", Odds: 2.12},
			}},
		},
	}

	quotes := Quotes(event)

	type key struct {
		market    string
		selection string
	}
	got := make(map[key]float64, len(quotes))
	for _, quote := range quotes {
		got[key{quote.MarketType, quote.Selection}] = quote.Decimal
		if quote.MarketType == "total_goals" && quote.Line == nil {
			t.Fatalf("total goals quote without a line: %+v", quote)
		}
	}

	want := map[key]float64{
		{"match_result", "home"}:          1.43,
		{"match_result", "draw"}:          4.9,
		{"match_result", "away"}:          6.5,
		{"double_chance", "home_or_draw"}: 1.08,
		{"double_chance", "home_or_away"}: 1.15,
		{"double_chance", "draw_or_away"}: 2.7,
		{"both_teams_to_score", "yes"}:    1.57,
		{"both_teams_to_score", "no"}:     2.17,
		{"total_goals", "over"}:           1.44,
		{"total_goals", "under"}:          2.63,
	}
	for wantKey, wantOdds := range want {
		odds, ok := got[wantKey]
		if !ok {
			t.Errorf("missing %v", wantKey)
			continue
		}
		if odds != wantOdds {
			t.Errorf("%v odds = %v, want %v", wantKey, odds, wantOdds)
		}
	}

	// 1UP and half-time markets must never be graded as full-time 1X2/totals.
	for _, quote := range quotes {
		if quote.Decimal == 1.21 || quote.Decimal == 2.12 {
			t.Fatalf("unsupported market leaked into quotes: %+v", quote)
		}
	}
	if len(quotes) != len(want) {
		t.Fatalf("expected %d quotes, got %d", len(want), len(quotes))
	}
}

func TestParseTotal(t *testing.T) {
	side, line, ok := parseTotal("Over 3.5")
	if !ok || side != "over" || line != 3.5 {
		t.Fatalf("parseTotal = %q %v %v", side, line, ok)
	}
	if _, _, ok := parseTotal("Exactly 2"); ok {
		t.Fatal("unexpected parse for an unsupported total label")
	}
}
