package store

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/oyins/oddsvault/backend/internal/normalize"
)

// ParseLegEvent splits a provider leg description into its parts.
//
//	"Corners Anaitasuna - Corners Deportivo Alaves"
//	  -> market "corners", home "Anaitasuna", away "Deportivo Alaves"
//
// It never invents a selection: the provider wording says the market family but
// not the line or side.
func ParseLegEvent(raw string) (home, away, market string) {
	cleaned := strings.Join(strings.Fields(raw), " ")
	for _, separator := range []string{" - ", " v ", " vs "} {
		parts := strings.SplitN(cleaned, separator, 2)
		if len(parts) != 2 {
			continue
		}
		home, homeMarket := stripMarketPrefix(parts[0])
		away, awayMarket := stripMarketPrefix(parts[1])
		market = homeMarket
		if market == "" {
			market = awayMarket
		}
		return home, away, market
	}
	return "", "", market
}

// stripMarketPrefix removes a market word the provider puts in front of each
// side, e.g. "Corners Anaitasuna".
func stripMarketPrefix(side string) (name, market string) {
	side = strings.TrimSpace(side)
	lower := strings.ToLower(side)
	if strings.HasPrefix(lower, "corners ") {
		return strings.TrimSpace(side[len("corners "):]), "corners"
	}
	return side, ""
}

// ResolveBetLegs tries to attach journal legs to canonical events.
func (im *Importer) ResolveBetLegs(ctx context.Context) (resolved, unresolved int, err error) {
	rows, err := im.pool.Query(ctx, `
		SELECT l.id, l.event_raw, pb.placed_at
		FROM personal_bet_legs l
		JOIN personal_bets pb ON pb.id = l.personal_bet_id
		WHERE l.event_id IS NULL
		ORDER BY l.id`)
	if err != nil {
		return 0, 0, fmt.Errorf("query unresolved legs: %w", err)
	}

	type pending struct {
		id       int64
		raw      string
		placedAt *time.Time
	}
	var legs []pending
	for rows.Next() {
		var item pending
		if err := rows.Scan(&item.id, &item.raw, &item.placedAt); err != nil {
			rows.Close()
			return 0, 0, fmt.Errorf("scan leg: %w", err)
		}
		legs = append(legs, item)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, 0, err
	}

	for _, leg := range legs {
		home, away, market := ParseLegEvent(leg.raw)
		homeKey, awayKey := normalize.Key(home), normalize.Key(away)
		if homeKey == "" || awayKey == "" {
			if _, err := im.pool.Exec(ctx, `
				UPDATE personal_bet_legs SET
					home_raw = NULLIF($2, ''), away_raw = NULLIF($3, ''),
					market_hint = NULLIF($4, ''),
					resolution_note = 'unparsed_event_name'
				WHERE id = $1`, leg.id, home, away, market); err != nil {
				return resolved, unresolved, err
			}
			unresolved++
			continue
		}

		anchor := time.Now().UTC()
		if leg.placedAt != nil {
			anchor = *leg.placedAt
		}

		var eventID int64
		err := im.pool.QueryRow(ctx, `
			SELECT e.id
			FROM events e
			JOIN event_participants hp ON hp.event_id = e.id AND hp.role = 'home'
			JOIN teams ht ON ht.id = hp.team_id
			JOIN event_participants ap ON ap.event_id = e.id AND ap.role = 'away'
			JOIN teams at ON at.id = ap.team_id
			WHERE ht.name_key = $1 AND at.name_key = $2
			  AND e.start_date BETWEEN ($3::timestamptz AT TIME ZONE 'UTC')::date - 7
			                        AND ($3::timestamptz AT TIME ZONE 'UTC')::date + 7
			ORDER BY abs(e.start_date - ($3::timestamptz AT TIME ZONE 'UTC')::date)
			LIMIT 1`, homeKey, awayKey, anchor).Scan(&eventID)

		switch {
		case err == nil:
			if _, err := im.pool.Exec(ctx, `
				UPDATE personal_bet_legs SET
					home_raw = $2, away_raw = $3, market_hint = NULLIF($4, ''),
					event_id = $5, identity_status = 'provisional',
					resolution_note = 'team_date_match'
				WHERE id = $1`, leg.id, home, away, market, eventID); err != nil {
				return resolved, unresolved, err
			}
			resolved++
		case err == pgx.ErrNoRows:
			if _, err := im.pool.Exec(ctx, `
				UPDATE personal_bet_legs SET
					home_raw = $2, away_raw = $3, market_hint = NULLIF($4, ''),
					resolution_note = 'no_matching_fixture'
				WHERE id = $1`, leg.id, home, away, market); err != nil {
				return resolved, unresolved, err
			}
			unresolved++
		default:
			return resolved, unresolved, fmt.Errorf("match leg %d: %w", leg.id, err)
		}
	}

	im.log.Info("bet legs resolved", "resolved", resolved, "unresolved", unresolved)
	return resolved, unresolved, nil
}
