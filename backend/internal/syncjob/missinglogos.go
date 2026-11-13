package syncjob

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/oyins/oddsvault/backend/internal/normalize"
	"github.com/oyins/oddsvault/backend/internal/providers/espn"
	"github.com/oyins/oddsvault/backend/internal/providers/fotmob"
	"github.com/oyins/oddsvault/backend/internal/providers/thesportsdb"
)

// logoKey normalizes a name for badge matching, ignoring gender and youth
// markers because those sides share the club crest.
func logoKey(name string) string {
	lower := strings.ToLower(name)
	for _, marker := range []string{
		"(women)", "(woman)", "(w)", "women", "woman",
		"(u23)", "(u21)", "(u20)", "(u19)", "(u17)", "(u16)",
		"u23", "u21", "u20", "u19", "u17", "u16",
	} {
		lower = strings.ReplaceAll(lower, marker, " ")
	}
	_, key := normalize.Resolve(lower)
	return key
}

// baseTeamName strips the same markers for a provider search query.
func baseTeamName(name string) string {
	cleaned := name
	for _, marker := range []string{
		"(Women)", "(Woman)", "(W)", "Women", "Woman",
		"(U23)", "(U21)", "(U20)", "(U19)", "(U17)", "(U16)",
		"U23", "U21", "U20", "U19", "U17", "U16",
	} {
		cleaned = strings.ReplaceAll(cleaned, marker, " ")
	}
	return strings.Join(strings.Fields(cleaned), " ")
}

// RunMissingLogos attaches badges to teams that are playing now but have none.
//
// FotMob carries a broad club directory and is tried first; TheSportsDB is the
// second pass for what it misses. Both are exact-name matches only: a wrong
// badge is worse than a monogram.
func (f *Football) RunMissingLogos(ctx context.Context) error {
	// A league named "UAE. Pro League" can still get its flag from the prefix.
	if updated, err := f.importer.BackfillCompetitionCountries(ctx); err == nil && updated > 0 {
		f.log.Info("competition countries backfilled", "updated", updated)
	}

	gaps, err := f.importer.TeamsMissingLogo(ctx, "football", 80)
	if err != nil {
		return err
	}
	if len(gaps) == 0 {
		return nil
	}

	fotmobClient := fotmob.New(f.http)
	espnClient := espn.New(f.http)
	attached := 0
	remaining := make([]struct {
		ID   int64
		Name string
	}, 0, len(gaps))
	var errs []error

	for _, gap := range gaps {
		key := logoKey(gap.Name)
		if key == "" {
			continue
		}
		// Women's and youth sides usually carry the club's own crest, so the
		// search and the comparison drop those markers. The rest of the name
		// must still match exactly.
		query := baseTeamName(gap.Name)

		// ESPN's directory is the widest and has no harsh rate limit, so it is
		// tried first. FotMob then covers clubs ESPN lists under another name.
		var logo, provider string
		if results, err := espnClient.SearchTeam(ctx, query); err == nil {
			for _, team := range results {
				if logoKey(team.Name) == key && team.LogoURL != "" {
					logo = team.LogoURL
					provider = "espn"
					break
				}
			}
		}

		if logo == "" {
			suggestions, err := fotmobClient.SearchTeams(ctx, query)
			time.Sleep(150 * time.Millisecond)
			if err == nil {
				for _, team := range suggestions {
					if logoKey(team.Name) == key {
						logo = fotmob.TeamLogoURL(team.ID)
						provider = "fotmob"
						break
					}
				}
			}
		}

		if logo == "" {
			remaining = append(remaining, struct {
				ID   int64
				Name string
			}{gap.ID, gap.Name})
			continue
		}

		if err := f.importer.SetTeamLogo(ctx, gap.ID, provider, logo); err != nil {
			errs = append(errs, err)
			continue
		}
		attached++
	}

	// Second pass on a small budget: TheSportsDB's free tier throttles hard.
	theSportsDB := thesportsdb.New(f.http)
	attempts := 0
	for _, gap := range remaining {
		if attempts >= 8 {
			break
		}
		key := logoKey(gap.Name)
		if key == "" {
			continue
		}
		attempts++

		results, err := theSportsDB.Search(ctx, baseTeamName(gap.Name))
		time.Sleep(1500 * time.Millisecond)
		if err != nil {
			if strings.Contains(err.Error(), "429") {
				errs = append(errs, fmt.Errorf("logo search stopped after rate limit: %w", err))
				break
			}
			errs = append(errs, fmt.Errorf("logo search %q: %w", gap.Name, err))
			continue
		}

		var badge string
		for _, team := range results {
			if logoKey(team.Name) == key && team.Badge != "" {
				badge = team.Badge
				break
			}
		}
		if badge == "" {
			continue
		}
		if err := f.importer.SetTeamLogo(ctx, gap.ID, "thesportsdb", badge); err != nil {
			errs = append(errs, err)
			continue
		}
		attached++
	}

	if attached > 0 || len(errs) > 0 {
		f.log.Info("missing team logos refreshed",
			"checked", len(gaps), "attached", attached, "errors", len(errs))
	}
	return errors.Join(errs...)
}
