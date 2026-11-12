package openfootball

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/oyins/oddsvault/backend/internal/providers"
)

func TestParseSeason(t *testing.T) {
	body, err := os.ReadFile("testdata/en.1.json")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	var doc seasonDoc
	if err := json.Unmarshal(body, &doc); err != nil {
		t.Fatalf("decode fixture: %v", err)
	}

	fixtures, err := ParseSeason(&doc, "premier-league", "2026/27", "2026-27", "en.1.json")
	if err != nil {
		t.Fatalf("ParseSeason: %v", err)
	}
	if len(fixtures) != 3 {
		t.Fatalf("expected 3 fixtures, got %d", len(fixtures))
	}

	first := fixtures[0]
	if first.Status != providers.StatusFinished {
		t.Errorf("first status = %q", first.Status)
	}
	if first.HomeScore == nil || *first.HomeScore != 3 || first.AwayScore == nil || *first.AwayScore != 0 {
		t.Errorf("first score = %v-%v", first.HomeScore, first.AwayScore)
	}
	if first.HomeScoreHT == nil || *first.HomeScoreHT != 2 {
		t.Errorf("first half-time score missing")
	}
	if first.StartDate.Format("2006-01-02") != "2026-08-21" {
		t.Errorf("first start date = %s", first.StartDate)
	}
	if first.StartTimeLocal != "20:00" {
		t.Errorf("first local time = %q", first.StartTimeLocal)
	}
	if first.ProviderEventID == "" {
		t.Errorf("first provider event id is empty")
	}

	unplayed := fixtures[2]
	if unplayed.Status != providers.StatusScheduled {
		t.Errorf("unplayed status = %q", unplayed.Status)
	}
	if unplayed.HomeScore != nil || unplayed.AwayScore != nil {
		t.Errorf("unplayed match must not carry a score")
	}

	if fixtures[0].ProviderEventID == fixtures[1].ProviderEventID {
		t.Errorf("provider event ids must be distinct")
	}
}

func TestParseSeasonRejectsBadDates(t *testing.T) {
	doc := &seasonDoc{Matches: []match{{Date: "21/08/2026", Team1: "A FC", Team2: "B FC"}}}
	if _, err := ParseSeason(doc, "premier-league", "2026/27", "2026-27", "x.json"); err == nil {
		t.Fatal("expected an error for a non-ISO date")
	}
}

func TestParseSeasonAcceptsArrayScore(t *testing.T) {
	raw := []byte(`{"matches":[{"round":"Matchday 3","date":"2026-09-05","time":"15:00","team1":"A FC","team2":"B FC","score":[0,0]}]}`)
	var doc seasonDoc
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("decode: %v", err)
	}
	fixtures, err := ParseSeason(&doc, "premier-league", "2026/27", "2026-27", "x.json")
	if err != nil {
		t.Fatalf("ParseSeason: %v", err)
	}
	if fixtures[0].Status != providers.StatusFinished {
		t.Fatalf("status = %q", fixtures[0].Status)
	}
	if fixtures[0].HomeScore == nil || *fixtures[0].HomeScore != 0 {
		t.Fatal("array score was not read as a full-time score")
	}
}
