package store

import (
	"context"
	"fmt"

	"github.com/oyins/oddsvault/backend/internal/normalize"
)

// UpsertLogoByName attaches a provider logo to the canonical team with the same
// name key. Unknown teams are skipped rather than created: logos never invent a
// club.
func (im *Importer) UpsertLogoByName(ctx context.Context, sportSlug, providerName, name, logoURL string) (bool, error) {
	_, key := normalize.Resolve(name)
	if key == "" || logoURL == "" {
		return false, nil
	}

	var teamID int64
	err := im.pool.QueryRow(ctx, `
		SELECT t.id FROM teams t
		JOIN sports s ON s.id = t.sport_id
		WHERE s.slug = $1 AND t.name_key = $2`, sportSlug, key).Scan(&teamID)
	if err != nil {
		return false, nil // no canonical team: nothing to attach to
	}

	if _, err := im.pool.Exec(ctx, `
		INSERT INTO media_assets
			(entity_type, entity_id, asset_type, source_provider, source_url, license_metadata)
		VALUES ('team', $1, 'team_logo', $2, $3, jsonb_build_object('provider', $2::text, 'terms', 'unverified'))
		ON CONFLICT (entity_type, entity_id, asset_type) DO UPDATE SET
			source_provider = EXCLUDED.source_provider,
			source_url = EXCLUDED.source_url,
			active = true,
			fetched_at = now()`,
		teamID, providerName, logoURL); err != nil {
		return false, fmt.Errorf("upsert team logo for %q: %w", name, err)
	}
	return true, nil
}

// CrestEndpointPath is the public path the UI uses for a team logo.
func CrestEndpointPath(teamID int64) string {
	return fmt.Sprintf("/api/v1/teams/%d/crest", teamID)
}

// EnsureTeamByName returns the canonical team for a name, creating a reference
// row when the team is known only from a bet slip. It never creates events.
func (im *Importer) EnsureTeamByName(ctx context.Context, sportSlug, name string) (int64, error) {
	display, key := normalize.Resolve(name)
	if key == "" {
		return 0, fmt.Errorf("team name %q has no usable key", name)
	}

	var teamID int64
	err := im.pool.QueryRow(ctx, `
		SELECT t.id FROM teams t
		JOIN sports s ON s.id = t.sport_id
		WHERE s.slug = $1 AND t.name_key = $2`, sportSlug, key).Scan(&teamID)
	if err == nil {
		return teamID, nil
	}

	var sportID int64
	if err := im.pool.QueryRow(ctx, `SELECT id FROM sports WHERE slug = $1`, sportSlug).Scan(&sportID); err != nil {
		return 0, fmt.Errorf("lookup sport: %w", err)
	}
	if err := im.pool.QueryRow(ctx, `
		INSERT INTO teams (sport_id, name, name_key, slug)
		VALUES ($1, $2, $3, $4)
		RETURNING id`, sportID, display, key, normalize.Slug(key)).Scan(&teamID); err != nil {
		return 0, fmt.Errorf("create team %q: %w", display, err)
	}
	return teamID, nil
}

// SetTeamLogo attaches a provider logo to a team.
func (im *Importer) SetTeamLogo(ctx context.Context, teamID int64, provider, logoURL string) error {
	_, err := im.pool.Exec(ctx, `
		INSERT INTO media_assets
			(entity_type, entity_id, asset_type, source_provider, source_url, license_metadata)
		VALUES ('team', $1, 'team_logo', $2, $3,
		        jsonb_build_object('provider', $2::text, 'terms', 'unverified'))
		ON CONFLICT (entity_type, entity_id, asset_type) DO UPDATE SET
			source_provider = EXCLUDED.source_provider,
			source_url = EXCLUDED.source_url,
			active = true,
			fetched_at = now()`,
		teamID, provider, logoURL)
	if err != nil {
		return fmt.Errorf("set team logo: %w", err)
	}
	return nil
}

// UnresolvedLegTeamNames lists distinct bet-leg sides that still need a crest.
func (im *Importer) UnresolvedLegTeamNames(ctx context.Context) ([]string, error) {
	rows, err := im.pool.Query(ctx, `
		SELECT DISTINCT name FROM (
			SELECT home_raw AS name FROM personal_bet_legs WHERE home_raw IS NOT NULL
			UNION
			SELECT away_raw FROM personal_bet_legs WHERE away_raw IS NOT NULL
		) names
		ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("list leg team names: %w", err)
	}
	defer rows.Close()

	var names []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		names = append(names, name)
	}
	return names, rows.Err()
}

// TeamHasLogo reports whether a canonical team already has an active logo.
func (im *Importer) TeamHasLogo(ctx context.Context, sportSlug, nameKey string) (bool, error) {
	var exists bool
	err := im.pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM teams t
			JOIN sports s ON s.id = t.sport_id
			JOIN media_assets ma ON ma.entity_type = 'team' AND ma.entity_id = t.id
			                    AND ma.asset_type = 'team_logo' AND ma.active
			WHERE s.slug = $1 AND t.name_key = $2
		)`, sportSlug, nameKey).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("check team logo: %w", err)
	}
	return exists, nil
}

// TeamLogoGap is a team that appears in a live event without a badge.
type TeamLogoGap struct {
	ID   int64
	Name string
}

// TeamsMissingLogo lists teams playing right now that have no active badge.
// The limit keeps third-party lookups bounded on a busy slate.
func (im *Importer) TeamsMissingLogo(ctx context.Context, sportSlug string, limit int) ([]TeamLogoGap, error) {
	rows, err := im.pool.Query(ctx, `
		SELECT t.id, t.name
		FROM teams t
		JOIN sports s ON s.id = t.sport_id
		LEFT JOIN media_assets ma ON ma.entity_type = 'team' AND ma.entity_id = t.id
		                          AND ma.asset_type = 'team_logo' AND ma.active
		WHERE s.slug = $1 AND ma.id IS NULL
		  AND EXISTS (
		      SELECT 1 FROM event_participants ep
		      JOIN events e ON e.id = ep.event_id
		      WHERE ep.team_id = t.id AND e.status = 'live'
		  )
		ORDER BY random()
		LIMIT $2`, sportSlug, limit)
	if err != nil {
		return nil, fmt.Errorf("list teams missing logo: %w", err)
	}
	defer rows.Close()

	var out []TeamLogoGap
	for rows.Next() {
		var item TeamLogoGap
		if err := rows.Scan(&item.ID, &item.Name); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

// SetTeamLogoIfMissing attaches a badge only when the team has none, so a
// better existing asset is never replaced by a bookmaker thumbnail.
func (im *Importer) SetTeamLogoIfMissing(ctx context.Context, teamID int64, provider, logoURL string) error {
	if logoURL == "" {
		return nil
	}
	_, err := im.pool.Exec(ctx, `
		INSERT INTO media_assets
			(entity_type, entity_id, asset_type, source_provider, source_url, license_metadata)
		VALUES ('team', $1, 'team_logo', $2, $3,
		        jsonb_build_object('provider', $2::text, 'terms', 'unverified'))
		ON CONFLICT (entity_type, entity_id, asset_type) DO NOTHING`,
		teamID, provider, logoURL)
	if err != nil {
		return fmt.Errorf("set team logo if missing: %w", err)
	}
	return nil
}

// SetCompetitionLogoIfMissing attaches a competition badge when none exists.
func (im *Importer) SetCompetitionLogoIfMissing(ctx context.Context, competitionSlug, provider, logoURL string) error {
	if logoURL == "" {
		return nil
	}
	_, err := im.pool.Exec(ctx, `
		INSERT INTO media_assets
			(entity_type, entity_id, asset_type, source_provider, source_url, license_metadata)
		SELECT 'competition', c.id, 'competition_logo', $2, $3,
		       jsonb_build_object('provider', $2::text, 'terms', 'unverified')
		FROM competitions c WHERE c.slug = $1
		ON CONFLICT (entity_type, entity_id, asset_type) DO NOTHING`,
		competitionSlug, provider, logoURL)
	if err != nil {
		return fmt.Errorf("set competition logo if missing: %w", err)
	}
	return nil
}
