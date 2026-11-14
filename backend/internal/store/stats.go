package store

import (
	"context"
	"fmt"
	"time"

	"github.com/oyins/oddsvault/backend/internal/normalize"
)

// StatsCandidate is a finished event whose provider stats are still missing.
type StatsCandidate struct {
	EventID int64
	// ESPNRef is the provider's league code when known, otherwise the
	// canonical competition slug the adapter maps itself.
	ESPNRef         string
	ProviderEventID string
	HomeTeamID      int64
	AwayTeamID      int64
}

// EventsNeedingStats lists finished ESPN-linked events without complete corner
// counts.
func (im *Importer) EventsNeedingStats(ctx context.Context, limit int) ([]StatsCandidate, error) {
	rows, err := im.pool.Query(ctx, `
		SELECT e.id, COALESCE(c.espn_code, c.slug), epi.provider_event_id, hp.team_id, ap.team_id
		FROM events e
		JOIN competitions c ON c.id = e.competition_id
		JOIN event_provider_ids epi ON epi.event_id = e.id AND epi.provider = 'espn'
		JOIN event_participants hp ON hp.event_id = e.id AND hp.role = 'home'
		JOIN event_participants ap ON ap.event_id = e.id AND ap.role = 'away'
		LEFT JOIN team_match_stats hs ON hs.event_id = e.id AND hs.team_id = hp.team_id
		LEFT JOIN team_match_stats as_ ON as_.event_id = e.id AND as_.team_id = ap.team_id
		LEFT JOIN personal_bet_legs jl ON jl.event_id = e.id
		WHERE e.status = 'finished'
		  AND (hs.corners IS NULL OR as_.corners IS NULL)
		ORDER BY (jl.id IS NULL), e.start_date DESC
		LIMIT $1`, limit)
	if err != nil {
		return nil, fmt.Errorf("list events needing stats: %w", err)
	}
	defer rows.Close()

	var out []StatsCandidate
	for rows.Next() {
		var item StatsCandidate
		if err := rows.Scan(&item.EventID, &item.ESPNRef, &item.ProviderEventID,
			&item.HomeTeamID, &item.AwayTeamID); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

// SaveCorners records both teams' corner counts for an event. A nil count
// clears a previous value: a provider that stops publishing statistics must not
// leave a false zero behind.
func (im *Importer) SaveCorners(ctx context.Context, eventID int64, provider string, homeTeamID, awayTeamID int64, home, away *int) error {
	for _, row := range []struct {
		teamID int64
		value  *int
	}{{homeTeamID, home}, {awayTeamID, away}} {
		if _, err := im.pool.Exec(ctx, `
			INSERT INTO team_match_stats (event_id, team_id, corners, provider)
			VALUES ($1, $2, $3, $4)
			ON CONFLICT (event_id, team_id) DO UPDATE SET
				corners = EXCLUDED.corners,
				provider = EXCLUDED.provider,
				captured_at = now()
			-- Provider precedence: the bookmaker the user actually bets with
			-- wins over an aggregator, which wins over ESPN. An inferior source
			-- must not overwrite a published count with a placeholder zero.
			WHERE (CASE EXCLUDED.provider
			           WHEN 'onexbet' THEN 3
			           WHEN 'fotmob' THEN 2
			           ELSE 1
			       END)
			   >= (CASE team_match_stats.provider
			           WHEN 'onexbet' THEN 3
			           WHEN 'fotmob' THEN 2
			           ELSE 1
			       END)`,
			eventID, row.teamID, row.value, provider); err != nil {
			return fmt.Errorf("save corners: %w", err)
		}
	}
	return nil
}

// VerifyCornerLegs recomputes corner-total journal legs from provider stats.
// It returns how many were verified and how many disagree with the bookmaker.
func (im *Importer) VerifyCornerLegs(ctx context.Context) (verified, mismatches int, err error) {
	rows, err := im.pool.Query(ctx, `
		SELECT l.id, l.market_side, l.market_line::float8, l.leg_status,
		       hs.corners, as_.corners
		FROM personal_bet_legs l
		JOIN event_participants hp ON hp.event_id = l.event_id AND hp.role = 'home'
		JOIN team_match_stats hs ON hs.event_id = l.event_id AND hs.team_id = hp.team_id
		JOIN event_participants ap ON ap.event_id = l.event_id AND ap.role = 'away'
		JOIN team_match_stats as_ ON as_.event_id = l.event_id AND as_.team_id = ap.team_id
		WHERE l.market_family = 'corners_total'
		  AND l.market_line IS NOT NULL
		  AND l.market_side IN ('over', 'under')
		  AND hs.corners IS NOT NULL
		  AND as_.corners IS NOT NULL`)
	if err != nil {
		return 0, 0, fmt.Errorf("query corner legs: %w", err)
	}
	defer rows.Close()

	type item struct {
		id        int64
		side      string
		line      float64
		legStatus string
		home      int
		away      int
	}
	var legs []item
	for rows.Next() {
		var leg item
		if err := rows.Scan(&leg.id, &leg.side, &leg.line, &leg.legStatus, &leg.home, &leg.away); err != nil {
			return 0, 0, err
		}
		legs = append(legs, leg)
	}
	if err := rows.Err(); err != nil {
		return 0, 0, err
	}

	for _, leg := range legs {
		total := leg.home + leg.away
		if total == 0 {
			// Lower-round feeds sometimes omit corner stats entirely and report
			// 0-0. Treat that as missing, not as a zero-corner match.
			if _, err := im.pool.Exec(ctx, `
				UPDATE personal_bet_legs SET
					verified_status = 'needs_data',
					verified_value = NULL,
					verification_note = 'provider reported 0+0 corners; treat as incomplete',
					verified_at = now()
				WHERE id = $1`, leg.id); err != nil {
				return verified, mismatches, err
			}
			verified++
			continue
		}
		status := ""
		switch {
		case float64(total) == leg.line:
			status = "push"
		case leg.side == "over" && float64(total) > leg.line:
			status = "hit"
		case leg.side == "under" && float64(total) < leg.line:
			status = "hit"
		default:
			status = "miss"
		}

		provider := map[string]string{"won": "hit", "lost": "miss", "void": "push"}[leg.legStatus]
		note := fmt.Sprintf("%d corners (%d+%d)", total, leg.home, leg.away)
		if provider != "" && provider != status {
			note += fmt.Sprintf("; provider says %s", leg.legStatus)
			mismatches++
		}

		if _, err := im.pool.Exec(ctx, `
			UPDATE personal_bet_legs SET
				verified_status = $2,
				verified_value = $3,
				verification_note = $4,
				verified_at = now()
			WHERE id = $1`, leg.id, status, note, note); err != nil {
			return verified, mismatches, fmt.Errorf("update verified leg: %w", err)
		}
		verified++
	}
	return verified, mismatches, nil
}

// LiveCornerCandidate is a live football event whose corner counts can be
// refreshed from the provider.
type LiveCornerCandidate struct {
	EventID         int64
	ESPNRef         string
	ProviderEventID string
	HomeTeamID      int64
	AwayTeamID      int64
}

// LiveCornerCandidates lists events in progress that ESPN can report corners
// for. Only linked events qualify: an unlinked live match would need a guess.
func (im *Importer) LiveCornerCandidates(ctx context.Context, limit int) ([]LiveCornerCandidate, error) {
	rows, err := im.pool.Query(ctx, `
		SELECT e.id, COALESCE(c.espn_code, c.slug), epi.provider_event_id, hp.team_id, ap.team_id
		FROM events e
		JOIN competitions c ON c.id = e.competition_id
		JOIN sports sp ON sp.id = e.sport_id
		JOIN event_provider_ids epi ON epi.event_id = e.id AND epi.provider = 'espn'
		JOIN event_participants hp ON hp.event_id = e.id AND hp.role = 'home'
		JOIN event_participants ap ON ap.event_id = e.id AND ap.role = 'away'
		WHERE e.status = 'live' AND sp.slug = 'football'
		ORDER BY COALESCE(e.start_time, e.start_date::timestamptz) ASC
		LIMIT $1`, limit)
	if err != nil {
		return nil, fmt.Errorf("list live corner candidates: %w", err)
	}
	defer rows.Close()

	var out []LiveCornerCandidate
	for rows.Next() {
		var item LiveCornerCandidate
		if err := rows.Scan(&item.EventID, &item.ESPNRef, &item.ProviderEventID,
			&item.HomeTeamID, &item.AwayTeamID); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

// EventTeams identifies a matched event and its two team ids.
type EventTeams struct {
	EventID         int64
	HomeTeamID      int64
	AwayTeamID      int64
	CompetitionSlug string
}

// FindEventByTeamNames matches an event by both team names and a nearby date.
// It returns nil when the pairing is not stored, so a second provider can fill
// statistics without ever creating a fixture.
func (im *Importer) FindEventByTeamNames(ctx context.Context, sportSlug, home, away string, day time.Time) (*EventTeams, error) {
	_, homeKey := normalize.Resolve(home)
	_, awayKey := normalize.Resolve(away)
	if homeKey == "" || awayKey == "" {
		return nil, nil
	}

	var out EventTeams
	err := im.pool.QueryRow(ctx, `
		SELECT e.id, hp.team_id, ap.team_id, c.slug
		FROM events e
		JOIN competitions c ON c.id = e.competition_id
		JOIN sports sp ON sp.id = e.sport_id
		JOIN event_participants hp ON hp.event_id = e.id AND hp.role = 'home'
		JOIN teams ht ON ht.id = hp.team_id AND ht.sport_id = sp.id
		JOIN event_participants ap ON ap.event_id = e.id AND ap.role = 'away'
		JOIN teams at ON at.id = ap.team_id AND at.sport_id = sp.id
		WHERE sp.slug = $1
		  AND ht.name_key = $2
		  AND at.name_key = $3
		  AND e.start_date BETWEEN $4::date - 1 AND $4::date + 1
		ORDER BY abs(e.start_date - $4::date), e.id
		LIMIT 1`, sportSlug, homeKey, awayKey, day).Scan(&out.EventID, &out.HomeTeamID, &out.AwayTeamID, &out.CompetitionSlug)
	if err != nil {
		if err.Error() == "no rows in result set" {
			return nil, nil
		}
		return nil, fmt.Errorf("match event by teams: %w", err)
	}
	return &out, nil
}

// FindEventByPartialTeams matches an event when one team name is exact and the
// other starts the same way. Providers spell clubs differently ("Comoros" and
// "Comoro Islands"), and without this a discovery run would create a second
// event for a match already stored.
func (im *Importer) FindEventByPartialTeams(ctx context.Context, sportSlug, home, away string, start time.Time) (*EventTeams, error) {
	_, homeKey := normalize.Resolve(home)
	_, awayKey := normalize.Resolve(away)
	if homeKey == "" || awayKey == "" {
		return nil, nil
	}

	var out EventTeams
	err := im.pool.QueryRow(ctx, `
		SELECT e.id, hp.team_id, ap.team_id, c.slug
		FROM events e
		JOIN competitions c ON c.id = e.competition_id
		JOIN sports sp ON sp.id = e.sport_id
		JOIN event_participants hp ON hp.event_id = e.id AND hp.role = 'home'
		JOIN teams ht ON ht.id = hp.team_id AND ht.sport_id = sp.id
		JOIN event_participants ap ON ap.event_id = e.id AND ap.role = 'away'
		JOIN teams at ON at.id = ap.team_id AND at.sport_id = sp.id
		WHERE sp.slug = $1
		  AND e.start_date BETWEEN $4::date - 1 AND $4::date + 1
		  AND (
		      (ht.name_key = $2 AND (at.name_key = $3 OR left(at.name_key, 5) = left($3, 5)
		          -- "Ireland U21" inside "Republic of Ireland U21": a shorter
		          -- name with a qualifier is the same side.
		          OR (position(' ' in $3) > 0 AND at.name_key LIKE '% ' || $3)
		          -- and the reverse: the provider sends the longer spelling.
		          OR (position(' ' in at.name_key) > 0 AND $3 LIKE '% ' || at.name_key)))
		      OR
		      (at.name_key = $3 AND (ht.name_key = $2 OR left(ht.name_key, 5) = left($2, 5)
		          OR (position(' ' in $2) > 0 AND ht.name_key LIKE '% ' || $2)
		          OR (position(' ' in ht.name_key) > 0 AND $2 LIKE '% ' || ht.name_key)))
		  )
		ORDER BY abs(e.start_date - $4::date),
		         abs(extract(epoch FROM (COALESCE(e.start_time, e.start_date::timestamptz) - $4::timestamptz))),
		         e.id
		LIMIT 1`, sportSlug, homeKey, awayKey, start).Scan(&out.EventID, &out.HomeTeamID, &out.AwayTeamID, &out.CompetitionSlug)
	if err != nil {
		if err.Error() == "no rows in result set" {
			return nil, nil
		}
		return nil, fmt.Errorf("match event by partial teams: %w", err)
	}
	return &out, nil
}

// RecordCornerSnapshot stores what the board showed while a match was live.
// Identical total and minute pairs are skipped, so the history stays compact
// while still showing when each corner arrived.
func (im *Importer) RecordCornerSnapshot(ctx context.Context, eventID int64, minute *int, home, away int) error {
	total := home + away
	var projected *float64
	if minute != nil && *minute >= 15 {
		value := float64(total) / float64(*minute) * 90
		projected = &value
	}

	if _, err := im.pool.Exec(ctx, `
		INSERT INTO corner_snapshots
			(event_id, minute, home_corners, away_corners, total_corners, projected_total)
		SELECT $1, $2, $3, $4, $5, $6
		WHERE NOT EXISTS (
			SELECT 1 FROM corner_snapshots
			WHERE event_id = $1
			  AND total_corners = $5
			  AND minute IS NOT DISTINCT FROM $2
		)`,
		eventID, minute, home, away, total, projected); err != nil {
		return fmt.Errorf("record corner snapshot: %w", err)
	}
	return nil
}

// BoardTeamKeys maps the canonical and alias name keys of every team that is
// live or playing in the next day to its team id. Only these teams need recent
// history, so the backfill never crawls the whole world.
func (im *Importer) BoardTeamKeys(ctx context.Context) (map[string]int64, error) {
	rows, err := im.pool.Query(ctx, `
		WITH board AS (
			SELECT DISTINCT ep.team_id
			FROM events e
			JOIN sports sp ON sp.id = e.sport_id
			JOIN event_participants ep ON ep.event_id = e.id
			WHERE sp.slug = 'football'
			  AND (
			    (e.status = 'live' AND EXISTS (
			        SELECT 1 FROM event_live_states l
			        WHERE l.event_id = e.id
			          AND l.captured_at > now() - interval '15 minutes'))
			    OR (e.status = 'scheduled'
			        AND COALESCE(e.start_time, e.start_date::timestamptz)
			            BETWEEN now() AND now() + interval '24 hours')
			  )
		)
		SELECT t.id, t.name_key FROM teams t JOIN board b ON b.team_id = t.id
		UNION
		SELECT a.team_id, a.alias_key FROM team_aliases a JOIN board b ON b.team_id = a.team_id`)
	if err != nil {
		return nil, fmt.Errorf("list board team keys: %w", err)
	}
	defer rows.Close()

	out := make(map[string]int64, 512)
	for rows.Next() {
		var teamID int64
		var key string
		if err := rows.Scan(&teamID, &key); err != nil {
			return nil, err
		}
		if key == "" {
			continue
		}
		// The first owner wins when two teams share a key: an ambiguous key is
		// not evidence, and a wrong link is worse than a missed one.
		if _, exists := out[key]; !exists {
			out[key] = teamID
		}
	}
	return out, rows.Err()
}

// EventsWithCornerStats returns the subset of event ids that already hold a
// corner count for both teams.
func (im *Importer) EventsWithCornerStats(ctx context.Context, eventIDs []int64) (map[int64]bool, error) {
	out := make(map[int64]bool, len(eventIDs))
	if len(eventIDs) == 0 {
		return out, nil
	}
	rows, err := im.pool.Query(ctx, `
		SELECT event_id
		FROM team_match_stats
		WHERE event_id = ANY($1) AND corners IS NOT NULL
		GROUP BY event_id
		HAVING count(*) >= 2`, eventIDs)
	if err != nil {
		return nil, fmt.Errorf("list events with corner stats: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out[id] = true
	}
	return out, rows.Err()
}

// EventsByProviderIDs resolves provider match ids to the stored event and its
// two team ids. It is exact: the provider id is the identity, not a name.
func (im *Importer) EventsByProviderIDs(ctx context.Context, provider string, ids []string) (map[string]EventTeams, error) {
	out := make(map[string]EventTeams, len(ids))
	if len(ids) == 0 {
		return out, nil
	}
	rows, err := im.pool.Query(ctx, `
		SELECT p.provider_event_id, e.id, hp.team_id, ap.team_id, c.slug
		FROM event_provider_ids p
		JOIN events e ON e.id = p.event_id
		JOIN competitions c ON c.id = e.competition_id
		JOIN event_participants hp ON hp.event_id = e.id AND hp.role = 'home'
		JOIN event_participants ap ON ap.event_id = e.id AND ap.role = 'away'
		WHERE p.provider = $1 AND p.provider_event_id = ANY($2)`, provider, ids)
	if err != nil {
		return nil, fmt.Errorf("list events by provider ids: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var providerID string
		var item EventTeams
		if err := rows.Scan(&providerID, &item.EventID, &item.HomeTeamID, &item.AwayTeamID, &item.CompetitionSlug); err != nil {
			return nil, err
		}
		out[providerID] = item
	}
	return out, rows.Err()
}

// LiveProviderEvent is a match in progress that is linked to one provider, with
// the ids needed to fetch and store its statistics.
type LiveProviderEvent struct {
	EventID         int64
	ProviderEventID string
	HomeTeamID      int64
	AwayTeamID      int64
	Detail          string
}

// LiveFotmobCandidates lists live events linked to FotMob. It does not depend on
// the match appearing on today's FotMob board, which is how a match that
// started before midnight kept its first minute and lost its corners.
func (im *Importer) LiveFotmobCandidates(ctx context.Context, limit int) ([]LiveProviderEvent, error) {
	rows, err := im.pool.Query(ctx, `
		SELECT e.id, p.provider_event_id, hp.team_id, ap.team_id, COALESCE(l.detail, '')
		FROM events e
		JOIN event_provider_ids p ON p.event_id = e.id AND p.provider = 'fotmob'
		JOIN event_live_states l ON l.event_id = e.id
		JOIN event_participants hp ON hp.event_id = e.id AND hp.role = 'home'
		JOIN event_participants ap ON ap.event_id = e.id AND ap.role = 'away'
		WHERE e.status = 'live'
		  AND l.captured_at > now() - interval '15 minutes'
		ORDER BY l.captured_at ASC
		LIMIT $1`, limit)
	if err != nil {
		return nil, fmt.Errorf("list live fotmob events: %w", err)
	}
	defer rows.Close()

	var out []LiveProviderEvent
	for rows.Next() {
		var item LiveProviderEvent
		if err := rows.Scan(&item.EventID, &item.ProviderEventID,
			&item.HomeTeamID, &item.AwayTeamID, &item.Detail); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}
