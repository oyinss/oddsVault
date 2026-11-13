package store

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/oyins/oddsvault/backend/internal/competitions"
	"github.com/oyins/oddsvault/backend/internal/normalize"
	"github.com/oyins/oddsvault/backend/internal/providers"
)

// LiveStats counts what one live sync changed.
type LiveStats struct {
	Received int
	Matched  int
	Created  int
	Updated  int
	Skipped  int
}

// ApplyLive writes provider live states, matching them to existing events and
// confirming identity because ESPN supplies real event ids.
func (im *Importer) ApplyLive(ctx context.Context, states []providers.LiveState) (LiveStats, error) {
	stats := LiveStats{Received: len(states)}
	season := competitions.CurrentSeason(time.Now().UTC())
	teamCache := make(map[string]int64)

	var errs []error
	for _, state := range states {
		changed, err := im.applyLiveState(ctx, state, season, teamCache)
		if err != nil {
			stats.Skipped++
			errs = append(errs, fmt.Errorf("%s %s v %s: %w", state.CompetitionSlug, state.HomeName, state.AwayName, err))
			continue
		}
		switch changed {
		case liveCreated:
			stats.Created++
		case liveMatched:
			stats.Matched++
		case liveUpdated:
			stats.Updated++
		default:
			stats.Skipped++
		}
	}
	return stats, errors.Join(errs...)
}

type liveOutcome int

const (
	liveSkipped liveOutcome = iota
	liveCreated
	liveMatched
	liveUpdated
)

func (im *Importer) applyLiveState(ctx context.Context, state providers.LiveState, season competitions.Season, teamCache map[string]int64) (liveOutcome, error) {
	tx, err := im.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return liveSkipped, fmt.Errorf("begin live update: %w", err)
	}
	defer func() { _ = tx.Rollback(context.WithoutCancel(ctx)) }()

	provider := state.Provider
	if provider == "" {
		provider = "espn"
	}

	sportSlug := state.Sport
	if sportSlug == "" {
		sportSlug = "football"
	}

	var sportID, competitionID int64
	if err := tx.QueryRow(ctx, `SELECT id FROM sports WHERE slug = $1`, sportSlug).Scan(&sportID); err != nil {
		return liveSkipped, fmt.Errorf("lookup sport %q: %w", sportSlug, err)
	}
	if err := tx.QueryRow(ctx,
		`SELECT id FROM competitions WHERE sport_id = $1 AND slug = $2`,
		sportID, state.CompetitionSlug).Scan(&competitionID); err != nil {
		return liveSkipped, fmt.Errorf("lookup competition %q: %w", state.CompetitionSlug, err)
	}

	var countryID *int64
	if err := tx.QueryRow(ctx, `SELECT country_id FROM competitions WHERE id = $1`, competitionID).Scan(&countryID); err != nil {
		return liveSkipped, fmt.Errorf("lookup competition country: %w", err)
	}

	seasonID, err := ensureSeason(ctx, tx, competitionID, season.Name, true)
	if err != nil {
		return liveSkipped, err
	}

	t := &txImporter{
		tx:            tx,
		log:           im.log,
		sportID:       sportID,
		countryID:     countryID,
		competitionID: competitionID,
		seasonID:      seasonID,
		teamCache:     teamCache,
	}

	homeID, err := t.resolveTeam(ctx, provider, state.HomeName)
	if err != nil {
		return liveSkipped, err
	}
	awayID, err := t.resolveTeam(ctx, provider, state.AwayName)
	if err != nil {
		return liveSkipped, err
	}

	fixture := providers.Fixture{
		Provider:        provider,
		ProviderEventID: state.ProviderEventID,
		CompetitionSlug: state.CompetitionSlug,
		SeasonName:      season.Name,
		StartDate:       state.StartUTC,
		Status:          state.Status,
		Home:            providers.TeamRef{Name: state.HomeName},
		Away:            providers.TeamRef{Name: state.AwayName},
		HomeScore:       state.HomeScore,
		AwayScore:       state.AwayScore,
	}

	outcome := liveUpdated
	eventID, _, _, err := t.findEvent(ctx, fixture, homeID, awayID)
	if err != nil {
		return liveSkipped, err
	}
	if eventID == 0 {
		eventID, err = t.createEvent(ctx, fixture, false)
		if err != nil {
			return liveSkipped, err
		}
		outcome = liveCreated
	} else {
		outcome = liveMatched
	}

	if err := t.upsertParticipants(ctx, eventID, homeID, awayID, fixture); err != nil {
		return liveSkipped, err
	}

	// ESPN supplies a verified UTC kickoff and a real event id, so this link
	// confirms identity rather than leaving it provisional. The calendar date
	// is re-derived in the competition's timezone: providers disagree when a
	// fixture is moved (OpenFootball had this one a day late).
	timezone := "UTC"
	if comp, err := competitions.BySlug(state.CompetitionSlug); err == nil && comp.Timezone != "" {
		timezone = comp.Timezone
	}
	if _, err := tx.Exec(ctx, `
		UPDATE events SET
			identity_status = CASE WHEN $5 = 'espn' THEN 'confirmed' ELSE 'provisional' END,
			start_time = $2,
			start_date = ($2 AT TIME ZONE $4)::date,
			start_time_local = to_char($2 AT TIME ZONE $4, 'HH24:MI'),
			status = CASE WHEN status = 'finished' THEN 'finished' ELSE $3 END
		WHERE id = $1`,
		eventID, state.StartUTC, string(state.Status), timezone, provider); err != nil {
		return liveSkipped, fmt.Errorf("update event from espn: %w", err)
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO event_provider_ids
			(event_id, provider, provider_event_id, observed_start_time, observed_status, match_method)
		VALUES ($1, $5, $2, $3, $4,
		        CASE WHEN $5 = 'espn' THEN 'espn_id' ELSE 'provider_id' END)
		ON CONFLICT (provider, provider_event_id) DO UPDATE SET
			observed_start_time = EXCLUDED.observed_start_time,
			observed_status = EXCLUDED.observed_status`,
		eventID, state.ProviderEventID, state.StartUTC, string(state.Status), provider); err != nil {
		return liveSkipped, fmt.Errorf("link provider id: %w", err)
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO event_live_states
			(event_id, provider, status, home_score, away_score, detail, captured_at)
		VALUES ($1, $6, $2, $3, $4, NULLIF($5, ''), now())
		ON CONFLICT (event_id) DO UPDATE SET
			provider = EXCLUDED.provider,
			status = EXCLUDED.status,
			home_score = EXCLUDED.home_score,
			away_score = EXCLUDED.away_score,
			detail = COALESCE(EXCLUDED.detail, event_live_states.detail),
			captured_at = now()`,
		eventID, string(state.Status), state.HomeScore, state.AwayScore, state.Detail, provider); err != nil {
		return liveSkipped, fmt.Errorf("upsert live state: %w", err)
	}

	if state.Status == providers.StatusFinished && state.HomeScore != nil && state.AwayScore != nil {
		fixture.Status = providers.StatusFinished
		if err := t.upsertResult(ctx, eventID, fixture); err != nil {
			return liveSkipped, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return liveSkipped, fmt.Errorf("commit live update: %w", err)
	}
	return outcome, nil
}

// LogLive is a small helper so workers log one structured line per run.
func (im *Importer) LogLive(stats LiveStats, err error) {
	attrs := []any{
		slog.Int("received", stats.Received),
		slog.Int("matched", stats.Matched),
		slog.Int("created", stats.Created),
		slog.Int("updated", stats.Updated),
		slog.Int("skipped", stats.Skipped),
	}
	if err != nil {
		attrs = append(attrs, slog.String("error", err.Error()))
		im.log.Warn("live sync finished with errors", attrs...)
		return
	}
	im.log.Info("live sync finished", attrs...)
}

// ResolveTeamKey exposes the canonical key for diagnostics.
func ResolveTeamKey(name string) string {
	_, key := normalize.Resolve(name)
	return key
}

// UnfinishedEventDates lists the distinct dates of events that still need a
// provider update, so the live job fetches only days it can act on.
func (im *Importer) UnfinishedEventDates(ctx context.Context, from, to time.Time) ([]time.Time, error) {
	rows, err := im.pool.Query(ctx, `
		SELECT DISTINCT start_date FROM events
		WHERE status <> 'finished'
		  AND start_date BETWEEN $1::date AND $2::date
		ORDER BY start_date`, from, to)
	if err != nil {
		return nil, fmt.Errorf("list unfinished event dates: %w", err)
	}
	defer rows.Close()

	var dates []time.Time
	for rows.Next() {
		var d time.Time
		if err := rows.Scan(&d); err != nil {
			return nil, fmt.Errorf("scan event date: %w", err)
		}
		dates = append(dates, d)
	}
	return dates, rows.Err()
}

// EventsInWindow counts events whose local start date falls in [from, to).
func (im *Importer) EventsInWindow(ctx context.Context, from, to time.Time) (int, error) {
	var count int
	err := im.pool.QueryRow(ctx, `
		SELECT count(*) FROM events
		WHERE start_date >= $1::date AND start_date < $2::date`,
		from, to).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count events in window: %w", err)
	}
	return count, nil
}

// EventLinkedToProvider reports whether an event already carries this
// provider's id, which is the only case where its live state may be refreshed
// from that provider.
func (im *Importer) EventLinkedToProvider(ctx context.Context, eventID int64, provider string) (bool, error) {
	var exists bool
	err := im.pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM event_provider_ids
			WHERE event_id = $1 AND provider = $2
		)`, eventID, provider).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("check provider link: %w", err)
	}
	return exists, nil
}

// RefreshLiveState updates the clock and score of a live event from the
// provider that owns it.
func (im *Importer) RefreshLiveState(ctx context.Context, eventID int64, provider, detail string, home, away *int) error {
	_, err := im.pool.Exec(ctx, `
		INSERT INTO event_live_states
			(event_id, provider, status, home_score, away_score, detail, captured_at)
		VALUES ($1, $2, 'live', $3, $4, $5, now())
		ON CONFLICT (event_id) DO UPDATE SET
			provider = EXCLUDED.provider,
			status = 'live',
			home_score = EXCLUDED.home_score,
			away_score = EXCLUDED.away_score,
			detail = EXCLUDED.detail,
			captured_at = now()`,
		eventID, provider, home, away, detail)
	if err != nil {
		return fmt.Errorf("refresh live state: %w", err)
	}
	return nil
}
