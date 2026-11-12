package footballdata

import (
	"os"
	"testing"

	"github.com/oyins/oddsvault/backend/internal/providers"
)

func TestParseSeason(t *testing.T) {
	body, err := os.ReadFile("testdata/E0.csv")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}

	fixtures, err := ParseSeason(body, "premier-league", "2026/27", "2627", "E0")
	if err != nil {
		t.Fatalf("ParseSeason: %v", err)
	}
	if len(fixtures) != 3 {
		t.Fatalf("expected 3 fixtures, got %d", len(fixtures))
	}

	first := fixtures[0]
	if first.Status != providers.StatusFinished {
		t.Errorf("status = %q", first.Status)
	}
	if first.HomeScore == nil || *first.HomeScore != 3 || first.AwayScore == nil || *first.AwayScore != 0 {
		t.Errorf("score = %v-%v", first.HomeScore, first.AwayScore)
	}
	if first.HomeScoreHT == nil || *first.HomeScoreHT != 2 || first.AwayScoreHT == nil || *first.AwayScoreHT != 0 {
		t.Errorf("half-time score missing")
	}
	if first.StartDate.Format("2006-01-02") != "2026-08-21" {
		t.Errorf("date = %s", first.StartDate)
	}

	unplayed := fixtures[2]
	if unplayed.Finished() || unplayed.HomeScore != nil {
		t.Errorf("unplayed row must not carry a score")
	}
	if unplayed.Status != providers.StatusScheduled {
		t.Errorf("unplayed status = %q", unplayed.Status)
	}
}

func TestParseSeasonRejectsMissingColumns(t *testing.T) {
	if _, err := ParseSeason([]byte("Div,Date\nE0,21/08/2026\n"), "premier-league", "2026/27", "2627", "E0"); err == nil {
		t.Fatal("expected an error for missing required columns")
	}
}
