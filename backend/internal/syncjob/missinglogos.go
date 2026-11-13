package syncjob

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/oyins/oddsvault/backend/internal/normalize"
	"github.com/oyins/oddsvault/backend/internal/providers/thesportsdb"
)

// RunMissingLogos attaches badges to teams that are playing now but have none.
//
// Live discovery creates teams from provider spellings, and some of those
// competitions carry no artwork. A readable row needs a badge or a flag, so the
// gap is filled from TheSportsDB by exact name match only: a wrong logo is worse
// than initials.
func (f *Football) RunMissingLogos(ctx context.Context) error {
	// A league named "UAE. Pro League" can still get its flag from the prefix.
	if updated, err := f.importer.BackfillCompetitionCountries(ctx); err == nil && updated > 0 {
		f.log.Info("competition countries backfilled", "updated", updated)
	}

	gaps, err := f.importer.TeamsMissingLogo(ctx, "football", 25)
	if err != nil {
		return err
	}
	if len(gaps) == 0 {
		return nil
	}

	adapter := thesportsdb.New(f.http)
	attached := 0
	var errs []error

	for _, gap := range gaps {
		_, key := normalize.Resolve(gap.Name)
		if key == "" {
			continue
		}

		results, err := adapter.Search(ctx, gap.Name)
		time.Sleep(1200 * time.Millisecond)
		if err != nil {
			// The free tier throttles hard. Stop the pass rather than burn the
			// remaining lookups on 429s; the next run picks up where this left
			// off.
			if strings.Contains(err.Error(), "429") {
				errs = append(errs, fmt.Errorf("logo search stopped after rate limit: %w", err))
				break
			}
			errs = append(errs, fmt.Errorf("logo search %q: %w", gap.Name, err))
			continue
		}

		var badge string
		for _, team := range results {
			if normalize.Key(team.Name) == key && team.Badge != "" {
				badge = team.Badge
				break
			}
		}
		if badge == "" {
			continue // initials beat a wrong badge
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
