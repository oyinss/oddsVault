package espn

import (
	"testing"

	"github.com/oyins/oddsvault/backend/internal/providers"
)

func TestParseTimeAcceptsMinutePrecision(t *testing.T) {
	// ESPN omits seconds in scoreboard dates; RFC3339 alone rejects them.
	parsed, ok := parseTime("2026-11-07T12:30Z")
	if !ok {
		t.Fatal("expected minute-precision timestamp to parse")
	}
	if parsed.UTC().Format("15:04") != "12:30" {
		t.Fatalf("parsed %s", parsed)
	}
	if _, ok := parseTime("not-a-time"); ok {
		t.Fatal("expected garbage to be rejected")
	}
}

func TestMapStatus(t *testing.T) {
	tests := []struct {
		state     string
		completed bool
		want      providers.Status
	}{
		{"post", true, providers.StatusFinished},
		{"in", false, providers.StatusLive},
		{"pre", false, providers.StatusScheduled},
		{"postponed", false, providers.StatusPostponed},
		{"something", false, providers.StatusUnknown},
	}
	for _, tc := range tests {
		if got := mapStatus(tc.state, tc.completed); got != tc.want {
			t.Errorf("mapStatus(%q,%v) = %q, want %q", tc.state, tc.completed, got, tc.want)
		}
	}
}

func TestParseScore(t *testing.T) {
	if got := parseScore("3"); got == nil || *got != 3 {
		t.Fatalf("parseScore(3) = %v", got)
	}
	if got := parseScore("0"); got == nil || *got != 0 {
		t.Fatal("zero must be preserved, not treated as missing")
	}
	if got := parseScore(""); got != nil {
		t.Fatal("empty score must be nil")
	}
}
