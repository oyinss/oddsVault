package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/oyins/oddsvault/backend/internal/competitions"
	"github.com/oyins/oddsvault/backend/internal/normalize"
	"github.com/oyins/oddsvault/backend/internal/providers"
)

// BookmakerOddsEvent is one captured bookmaker event.
type BookmakerOddsEvent struct {
	ProviderEventID string
	Name            string
	StartTimeLocal  string
	StartDate       time.Time
	State           string
	Home            string
	Away            string
	Quotes          []providers.OddsQuote
}

// BookmakerOddsStats counts one capture.
type BookmakerOddsStats struct {
	Received   int
	Matched    int
	Unresolved int
	Offers     int
}

// ImportBookmakerOdds links captured bookmaker events to canonical events and
// appends their prices. It never creates canonical events: an unmatched
// bookmaker event stays unresolved for review.
func (im *Importer) ImportBookmakerOdds(ctx context.Context, bookmakerSlug, competitionSlug, collectorVersion string, events []BookmakerOddsEvent) (BookmakerOddsStats, error) {
	stats := BookmakerOddsStats{Received: len(events)}
	if len(events) == 0 {
		return stats, nil
	}

	comp, err := competitions.BySlug(competitionSlug)
	if err != nil {
		return stats, err
	}
	season := competitions.CurrentSeason(time.Now().UTC())

	tx, err := im.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return stats, fmt.Errorf("begin bookmaker import: %w", err)
	}
	defer func() { _ = tx.Rollback(context.WithoutCancel(ctx)) }()

	var sportID, competitionID, bookmakerID int64
	if err := tx.QueryRow(ctx, `SELECT id FROM sports WHERE slug = $1`, comp.SportSlug()).Scan(&sportID); err != nil {
		return stats, fmt.Errorf("lookup sport %q: %w", comp.SportSlug(), err)
	}
	if err := tx.QueryRow(ctx,
		`SELECT id FROM competitions WHERE sport_id = $1 AND slug = $2`,
		sportID, comp.Slug).Scan(&competitionID); err != nil {
		return stats, fmt.Errorf("lookup competition: %w", err)
	}
	if err := tx.QueryRow(ctx, `SELECT id FROM bookmakers WHERE slug = $1`, bookmakerSlug).Scan(&bookmakerID); err != nil {
		return stats, fmt.Errorf("lookup bookmaker %q: %w", bookmakerSlug, err)
	}
	seasonID, err := ensureSeason(ctx, tx, competitionID, season.Name, true)
	if err != nil {
		return stats, err
	}

	writer := newOddsWriter(tx, "live")
	var errs []error

	for _, event := range events {
		if event.StartDate.IsZero() || event.Home == "" || event.Away == "" {
			stats.Unresolved++
			continue
		}
		matched, eventID, err := linkBookmakerEvent(ctx, tx, bookmakerID, sportID, competitionID, seasonID, event)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", event.Name, err))
			stats.Unresolved++
			continue
		}
		if !matched {
			stats.Unresolved++
			continue
		}
		stats.Matched++
		if err := writer.write(ctx, bookmakerSlug, event.ProviderEventID, eventID, event.Quotes); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", event.Name, err))
		}
	}

	stats.Offers = writer.offers
	if err := writer.finish(ctx); err != nil {
		return stats, err
	}
	if err := tx.Commit(ctx); err != nil {
		return stats, fmt.Errorf("commit bookmaker import: %w", err)
	}
	return stats, errors.Join(errs...)
}

// linkBookmakerEvent resolves teams and a canonical event, then records the
// bookmaker event. It returns matched=false when identity is not certain.
func linkBookmakerEvent(ctx context.Context, tx pgx.Tx, bookmakerID, sportID, competitionID, seasonID int64, event BookmakerOddsEvent) (bool, int64, error) {
	homeID, err := teamIDByKey(ctx, tx, sportID, event.Home)
	if err != nil {
		return false, 0, err
	}
	awayID, err := teamIDByKey(ctx, tx, sportID, event.Away)
	if err != nil {
		return false, 0, err
	}

	var eventID int64
	matched := false

	if homeID != 0 && awayID != 0 {
		err := tx.QueryRow(ctx, `
			SELECT e.id
			FROM events e
			JOIN event_participants hp ON hp.event_id = e.id AND hp.role = 'home'
			JOIN event_participants ap ON ap.event_id = e.id AND ap.role = 'away'
			WHERE e.competition_id = $1 AND e.season_id = $2
			  AND hp.team_id = $3 AND ap.team_id = $4
			  AND e.start_date BETWEEN $5::date - 2 AND $5::date + 2
			ORDER BY abs(e.start_date - $5::date)
			LIMIT 1`,
			competitionID, seasonID, homeID, awayID, event.StartDate).Scan(&eventID)
		if err == nil {
			matched = true
		} else if err != pgx.ErrNoRows {
			return false, 0, fmt.Errorf("match canonical event: %w", err)
		}
	}

	identity := "unresolved"
	if matched {
		identity = "confirmed"
	}
	var bookmakerEventID int64
	if err := tx.QueryRow(ctx, `
		INSERT INTO bookmaker_events
			(bookmaker_id, provider_event_id, raw_name, matched_event_id,
			 identity_status, raw_start_time, last_seen_at)
		VALUES ($1, $2, $3, $4, $5, $6, now())
		ON CONFLICT (bookmaker_id, provider_event_id) DO UPDATE SET
			raw_name = EXCLUDED.raw_name,
			matched_event_id = COALESCE(EXCLUDED.matched_event_id, bookmaker_events.matched_event_id),
			identity_status = CASE
				WHEN bookmaker_events.identity_status = 'confirmed' THEN 'confirmed'
				ELSE EXCLUDED.identity_status END,
			raw_start_time = EXCLUDED.raw_start_time,
			last_seen_at = now()
		RETURNING id`,
		bookmakerID, event.ProviderEventID, event.Name,
		nullableInt64(eventID), identity, event.StartTimeLocal).Scan(&bookmakerEventID); err != nil {
		return false, 0, fmt.Errorf("record bookmaker event: %w", err)
	}
	return matched, eventID, nil
}

func teamIDByKey(ctx context.Context, tx pgx.Tx, sportID int64, name string) (int64, error) {
	_, key := normalize.Resolve(name)
	if key == "" {
		return 0, nil
	}
	var id int64
	err := tx.QueryRow(ctx, `SELECT id FROM teams WHERE sport_id = $1 AND name_key = $2`, sportID, key).Scan(&id)
	if err == pgx.ErrNoRows {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("lookup team %q: %w", name, err)
	}
	return id, nil
}

func nullableInt64(v int64) *int64 {
	if v == 0 {
		return nil
	}
	return &v
}
