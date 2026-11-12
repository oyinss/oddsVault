package normalize

import "testing"

func TestTeamKeysMatchAcrossProviders(t *testing.T) {
	pairs := [][2]string{
		{"Arsenal FC", "Arsenal"},
		{"Manchester United FC", "Man United"},
		{"Manchester City FC", "Man City"},
		{"Newcastle United FC", "Newcastle"},
		{"Nottingham Forest FC", "Nott'm Forest"},
		{"Tottenham Hotspur FC", "Tottenham"},
		{"Brighton & Hove Albion FC", "Brighton"},
		{"AFC Bournemouth", "Bournemouth"},
		{"Leeds United FC", "Leeds"},
		{"Ipswich Town FC", "Ipswich"},
		{"Hull City AFC", "Hull"},
		{"Coventry City FC", "Coventry"},
		{"Wolverhampton Wanderers FC", "Wolves"},
		{"West Ham United FC", "West Ham"},
		{"Atlético Madrid", "Ath Madrid"},
		{"Athletic Club", "Ath Bilbao"},
		{"Real Sociedad", "Sociedad"},
		{"Málaga CF", "Malaga"},
		{"FC Köln", "FC Koln"},
		{"AC Milan", "Milan"},
		{"Inter Milan", "Inter"},
		{"AS Roma", "Roma"},
		{"Paris Saint-Germain FC", "Paris SG"},
		{"Olympique de Marseille", "Marseille"},
	}
	for _, pair := range pairs {
		_, a := Resolve(pair[0])
		_, b := Resolve(pair[1])
		if a != b {
			t.Errorf("%q key %q != %q key %q", pair[0], a, pair[1], b)
		}
		if a == "" {
			t.Errorf("%q resolved to an empty key", pair[0])
		}
	}
}

func TestKeyStripsClubTokensAndAccents(t *testing.T) {
	tests := map[string]string{
		"Arsenal FC":                "arsenal",
		"1. FC Union Berlin":        "union berlin",
		"TSG 1899 Hoffenheim":       "hoffenheim",
		"Brighton & Hove Albion FC": "brighton and hove albion",
		"  Real   Madrid  ":         "real madrid",
		"Deportivo Alavés":          "deportivo alaves",
	}
	for input, want := range tests {
		if got := Key(input); got != want {
			t.Errorf("Key(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestResolveUsesCanonicalDisplay(t *testing.T) {
	display, key := Resolve("Nott'm Forest")
	if display != "Nottingham Forest" {
		t.Errorf("display = %q", display)
	}
	if key != "nottingham forest" {
		t.Errorf("key = %q", key)
	}
	if got := Display("Arsenal FC"); got != "Arsenal" {
		t.Errorf("Display = %q", got)
	}
}

func TestSlug(t *testing.T) {
	if got := Slug("brighton and hove albion"); got != "brighton-and-hove-albion" {
		t.Errorf("slug = %q", got)
	}
	if got := Slug("manchester united"); got != "manchester-united" {
		t.Errorf("slug = %q", got)
	}
}

func TestAliasesAreUnambiguous(t *testing.T) {
	// Building the map panics on conflicting aliases; this also documents the
	// invariant that one provider spelling cannot mean two clubs.
	for key, canonical := range aliases() {
		if key == "" || canonical == "" {
			t.Fatalf("empty alias entry: %q -> %q", key, canonical)
		}
	}
}
