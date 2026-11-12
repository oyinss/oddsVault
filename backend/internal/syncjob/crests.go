package syncjob

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/oyins/oddsvault/backend/internal/competitions"
	"github.com/oyins/oddsvault/backend/internal/normalize"
	"github.com/oyins/oddsvault/backend/internal/providers/espn"
	"github.com/oyins/oddsvault/backend/internal/providers/thesportsdb"
)

// RunCrests attaches provider badge URLs to canonical teams.
func (f *Football) RunCrests(ctx context.Context) error {
	selected, err := competitions.Selected()
	if err != nil {
		return err
	}
	for _, slug := range competitions.ESPNOnly {
		if comp, err := competitions.BySlug(slug); err == nil {
			selected = append(selected, comp)
		}
	}

	adapter := espn.New(f.http)
	var errs []error
	attached := 0

	for _, comp := range selected {
		teams, err := adapter.FetchTeams(ctx, comp.Slug)
		if err != nil {
			errs = append(errs, fmt.Errorf("crests %s: %w", comp.Slug, err))
			continue
		}
		for _, team := range teams {
			ok, err := f.importer.UpsertLogoByName(ctx, "football", "espn", team.Name, team.LogoURL)
			if err != nil {
				errs = append(errs, err)
				continue
			}
			if ok {
				attached++
			}
		}
	}

	// Clubs that exist only in bet history (lower leagues, cup and reserve
	// sides) have no ESPN asset. Resolve them strictly by name.
	legAttached, legErrs := f.resolveBetLegCrests(ctx)
	attached += legAttached
	errs = append(errs, legErrs...)

	f.log.Info("team crests refreshed", "competition", "football", "attached", attached)
	return errors.Join(errs...)
}

func (f *Football) resolveBetLegCrests(ctx context.Context) (int, []error) {
	rows, err := f.importer.UnresolvedLegTeamNames(ctx)
	if err != nil {
		return 0, []error{err}
	}
	if len(rows) == 0 {
		return 0, nil
	}

	adapter := thesportsdb.New(f.http)
	attached := 0
	var errs []error

	for _, name := range rows {
		key := normalize.Key(name)
		if key == "" {
			continue
		}
		hasCrest, err := f.importer.TeamHasLogo(ctx, "football", key)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		if hasCrest {
			continue
		}

		queries := []string{name}
		if alt := normalize.Display(name); alt != "" && alt != name {
			queries = append(queries, alt)
		}

		var badge string
		for _, query := range queries {
			results, err := adapter.Search(ctx, query)
			if err != nil {
				errs = append(errs, fmt.Errorf("crest search %q: %w", query, err))
				time.Sleep(300 * time.Millisecond)
				continue
			}
			time.Sleep(300 * time.Millisecond)
			for _, team := range results {
				if normalize.Key(team.Name) == key {
					badge = team.Badge
					break
				}
			}
			if badge != "" {
				break
			}
		}
		if badge == "" {
			continue // no exact match: initials beat a wrong badge
		}

		teamID, err := f.importer.EnsureTeamByName(ctx, "football", name)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		if err := f.importer.SetTeamLogo(ctx, teamID, "thesportsdb", badge); err != nil {
			errs = append(errs, err)
			continue
		}
		attached++
	}
	return attached, errs
}
