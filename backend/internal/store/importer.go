// Package store owns transactional writes of normalized provider data.
package store

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/oyins/oddsvault/backend/internal/competitions"
	"github.com/oyins/oddsvault/backend/internal/normalize"
	"github.com/oyins/oddsvault/backend/internal/providers"
)

// Importer writes provider fixtures into canonical events.
type Importer struct {
	pool *pgxpool.Pool
	log  *slog.Logger
}

// NewImporter builds an importer.
func NewImporter(pool *pgxpool.Pool, log *slog.Logger) *Importer {
	if log == nil {
		log = slog.Default()
	}
	return &Importer{pool: pool, log: log}
}

// ImportStats counts what one season import changed.
type ImportStats struct {
	Received int
	Inserted int
	Updated  int
	Skipped  int
}

// StartRun opens an import_runs row so failures are visible too.
func (im *Importer) StartRun(ctx context.Context, provider, jobType string) (int64, error) {
	var id int64
	err := im.pool.QueryRow(ctx, `
		INSERT INTO import_runs (provider, job_type, status)
		VALUES ($1, $2, 'running')
		RETURNING id`, provider, jobType).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("start import run: %w", err)
	}
	return id, nil
}

// FinishRun records the outcome of an import run.
func (im *Importer) FinishRun(ctx context.Context, id int64, stats ImportStats, err error) error {
	status := "success"
	message := ""
	if err != nil {
		status = "failed"
		message = err.Error()
	}
	_, dbErr := im.pool.Exec(ctx, `
		UPDATE import_runs
		SET finished_at = now(), status = $2, error_message = NULLIF($3, ''),
		    records_received = $4, records_inserted = $5, records_updated = $6,
		    records_skipped = $7
		WHERE id = $1`,
		id, status, message, stats.Received, stats.Inserted, stats.Updated, stats.Skipped)
	if dbErr != nil {
		return fmt.Errorf("finish import run: %w", dbErr)
	}
	return nil
}

// ImportFixtures writes one season of fixtures in a single transaction.
func (im *Importer) ImportFixtures(ctx context.Context, comp competitions.Competition, season competitions.Season, fixtures []providers.Fixture, current bool) (ImportStats, error) {
	stats := ImportStats{Received: len(fixtures)}
	if len(fixtures) == 0 {
		return stats, nil
	}

	tx, err := im.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return stats, fmt.Errorf("begin import: %w", err)
	}
	defer func() { _ = tx.Rollback(context.WithoutCancel(ctx)) }()

	var sportID, competitionID int64
	if err := tx.QueryRow(ctx, `SELECT id FROM sports WHERE slug = 'football'`).Scan(&sportID); err != nil {
		return stats, fmt.Errorf("lookup sport: %w", err)
	}
	if err := tx.QueryRow(ctx,
		`SELECT id FROM competitions WHERE sport_id = $1 AND slug = $2`,
		sportID, comp.Slug).Scan(&competitionID); err != nil {
		return stats, fmt.Errorf("lookup competition %q: %w", comp.Slug, err)
	}

	var countryID *int64
	if err := tx.QueryRow(ctx, `SELECT country_id FROM competitions WHERE id = $1`, competitionID).Scan(&countryID); err != nil {
		return stats, fmt.Errorf("lookup competition country: %w", err)
	}

	seasonID, err := ensureSeason(ctx, tx, competitionID, season.Name, current)
	if err != nil {
		return stats, err
	}

	imp := &txImporter{
		tx:            tx,
		log:           im.log,
		sportID:       sportID,
		countryID:     countryID,
		competitionID: competitionID,
		seasonID:      seasonID,
		teamCache:     make(map[string]int64),
		odds:          newOddsWriter(tx, "closing"),
	}

	for i, fixture := range fixtures {
		if fixture.Home.Name == "" || fixture.Away.Name == "" || fixture.StartDate.IsZero() {
			stats.Skipped++
			continue
		}
		outcome, err := imp.upsertFixture(ctx, fixture)
		if err != nil {
			return stats, fmt.Errorf("import fixture %d (%s v %s): %w",
				i, fixture.Home.Name, fixture.Away.Name, err)
		}
		switch outcome {
		case outcomeInserted:
			stats.Inserted++
		case outcomeUpdated:
			stats.Updated++
		case outcomeSkipped:
			stats.Skipped++
		}
	}

	if err := imp.odds.finish(ctx); err != nil {
		return stats, err
	}

	if err := tx.Commit(ctx); err != nil {
		return stats, fmt.Errorf("commit import: %w", err)
	}
	return stats, nil
}

func ensureSeason(ctx context.Context, tx pgx.Tx, competitionID int64, name string, current bool) (int64, error) {
	var id int64
	err := tx.QueryRow(ctx,
		`SELECT id FROM seasons WHERE competition_id = $1 AND name = $2`,
		competitionID, name).Scan(&id)
	if err != nil && err != pgx.ErrNoRows {
		return 0, fmt.Errorf("lookup season %q: %w", name, err)
	}
	if err == pgx.ErrNoRows {
		if err := tx.QueryRow(ctx,
			`INSERT INTO seasons (competition_id, name, is_current)
			 VALUES ($1, $2, false) RETURNING id`,
			competitionID, name).Scan(&id); err != nil {
			return 0, fmt.Errorf("create season %q: %w", name, err)
		}
	}
	if current {
		// Clear any other current season first: a partial unique index enforces one.
		if _, err := tx.Exec(ctx,
			`UPDATE seasons SET is_current = false
			 WHERE competition_id = $1 AND is_current AND id <> $2`,
			competitionID, id); err != nil {
			return 0, fmt.Errorf("clear current season: %w", err)
		}
		if _, err := tx.Exec(ctx,
			`UPDATE seasons SET is_current = true WHERE id = $1 AND NOT is_current`,
			id); err != nil {
			return 0, fmt.Errorf("mark current season: %w", err)
		}
	}
	return id, nil
}

type outcome int

const (
	outcomeSkipped outcome = iota
	outcomeInserted
	outcomeUpdated
)

type txImporter struct {
	tx            pgx.Tx
	log           *slog.Logger
	sportID       int64
	countryID     *int64
	competitionID int64
	seasonID      int64
	teamCache     map[string]int64
	odds          *oddsWriter
}

func (t *txImporter) upsertFixture(ctx context.Context, f providers.Fixture) (outcome, error) {
	homeID, err := t.resolveTeam(ctx, f.Provider, f.Home.Name)
	if err != nil {
		return outcomeSkipped, err
	}
	awayID, err := t.resolveTeam(ctx, f.Provider, f.Away.Name)
	if err != nil {
		return outcomeSkipped, err
	}

	eventID, matched, ambiguous, err := t.findEvent(ctx, f, homeID, awayID)
	if err != nil {
		return outcomeSkipped, err
	}

	result := outcomeUpdated
	if eventID == 0 {
		result = outcomeInserted
		eventID, err = t.createEvent(ctx, f, ambiguous)
		if err != nil {
			return outcomeSkipped, err
		}
	}

	if err := t.upsertParticipants(ctx, eventID, homeID, awayID, f); err != nil {
		return outcomeSkipped, err
	}
	if err := t.linkProviderID(ctx, eventID, f, matched); err != nil {
		return outcomeSkipped, err
	}
	if err := t.updateEvent(ctx, eventID, f); err != nil {
		return outcomeSkipped, err
	}
	if f.Finished() {
		if err := t.upsertResult(ctx, eventID, f); err != nil {
			return outcomeSkipped, err
		}
	}
	if len(f.Odds) > 0 {
		if err := t.odds.write(ctx, f.Provider, f.ProviderEventID, eventID, f.Odds); err != nil {
			return outcomeSkipped, err
		}
	}
	return result, nil
}

func (t *txImporter) resolveTeam(ctx context.Context, provider, rawName string) (int64, error) {
	display, key := normalize.Resolve(rawName)
	if key == "" {
		return 0, fmt.Errorf("team name %q has no usable key", rawName)
	}
	// Names repeat across sports (Kings, Panthers, ...), so the cache is
	// scoped by sport as well as by name key.
	cacheKey := fmt.Sprintf("%d:%s", t.sportID, key)
	if id, ok := t.teamCache[cacheKey]; ok {
		return id, nil
	}

	var id int64
	err := t.tx.QueryRow(ctx, `SELECT id FROM teams WHERE sport_id = $1 AND name_key = $2`, t.sportID, key).Scan(&id)
	if err == pgx.ErrNoRows {
		err = t.tx.QueryRow(ctx, `
			SELECT ta.team_id
			FROM team_aliases ta
			JOIN teams t ON t.id = ta.team_id
			WHERE ta.alias_key = $1 AND t.sport_id = $2
			LIMIT 1`, key, t.sportID).Scan(&id)
	}
	if err == pgx.ErrNoRows {
		err = t.tx.QueryRow(ctx, `
			INSERT INTO teams (sport_id, country_id, name, name_key, slug)
			VALUES ($1, $2, $3, $4, $5)
			RETURNING id`,
			t.sportID, t.countryID, display, key, normalize.Slug(key)).Scan(&id)
		if err != nil {
			return 0, fmt.Errorf("create team %q: %w", display, err)
		}
	}
	if err != nil {
		return 0, fmt.Errorf("resolve team %q: %w", rawName, err)
	}

	t.teamCache[cacheKey] = id

	// Keep the provider spelling as an alias when it differs from the key.
	aliasKey := normalize.Key(rawName)
	if aliasKey != key {
		_, _ = t.tx.Exec(ctx, `
			INSERT INTO team_aliases (team_id, provider, alias, alias_key)
			VALUES ($1, $2, $3, $4)
			ON CONFLICT (alias_key, provider) DO NOTHING`,
			id, provider, rawName, aliasKey)
	}
	return id, nil
}

// findEvent returns an event id, or 0 when a new event is needed. matched
// reports that identity was established from an existing canonical record and
// ambiguous reports that several events could have matched.
func (t *txImporter) findEvent(ctx context.Context, f providers.Fixture, homeID, awayID int64) (id int64, matched, ambiguous bool, err error) {
	err = t.tx.QueryRow(ctx, `
		SELECT event_id FROM event_provider_ids
		WHERE provider = $1 AND provider_event_id = $2`,
		f.Provider, f.ProviderEventID).Scan(&id)
	if err == nil {
		return id, true, false, nil
	}
	if err != pgx.ErrNoRows {
		return 0, false, false, fmt.Errorf("lookup provider event id: %w", err)
	}

	rows, err := t.tx.Query(ctx, `
		SELECT e.id
		FROM events e
		JOIN event_participants hp ON hp.event_id = e.id AND hp.role = 'home'
		JOIN event_participants ap ON ap.event_id = e.id AND ap.role = 'away'
		WHERE e.competition_id = $1
		  AND e.season_id = $2
		  AND hp.team_id = $3
		  AND ap.team_id = $4
		  AND (
		      -- A verified kickoff must line up to the hour; two fixtures
		      -- between the same teams cannot be confused. Date-only events
		      -- keep the wider window so a provider day shift still matches.
		      (e.start_time IS NOT NULL
		       AND abs(extract(epoch FROM (e.start_time - $5))) < 12 * 3600)
		      OR
		      (e.start_time IS NULL
		       AND e.start_date BETWEEN $5::date - 3 AND $5::date + 3)
		  )
		ORDER BY abs(e.start_date - $5::date)
		LIMIT 2`,
		t.competitionID, t.seasonID, homeID, awayID, f.StartDate)
	if err != nil {
		return 0, false, false, fmt.Errorf("match existing event: %w", err)
	}
	defer rows.Close()

	var matches []int64
	for rows.Next() {
		var candidate int64
		if err := rows.Scan(&candidate); err != nil {
			return 0, false, false, fmt.Errorf("scan matched event: %w", err)
		}
		matches = append(matches, candidate)
	}
	if err := rows.Err(); err != nil {
		return 0, false, false, fmt.Errorf("iterate matched events: %w", err)
	}

	switch len(matches) {
	case 1:
		return matches[0], true, false, nil
	case 0:
		return 0, false, false, nil
	default:
		// Two events for the same pairing inside three days cannot be told apart.
		return 0, false, true, nil
	}
}

func (t *txImporter) createEvent(ctx context.Context, f providers.Fixture, ambiguous bool) (int64, error) {
	identity := "provisional"
	if ambiguous {
		identity = "unresolved"
	}
	status := string(f.Status)
	if status == "" {
		status = "unknown"
	}
	var id int64
	err := t.tx.QueryRow(ctx, `
		INSERT INTO events
			(sport_id, competition_id, season_id, start_date, start_time_local,
			 round_label, status, identity_status)
		VALUES ($1, $2, $3, $4, NULLIF($5, ''), NULLIF($6, ''), $7, $8)
		RETURNING id`,
		t.sportID, t.competitionID, t.seasonID, f.StartDate, f.StartTimeLocal,
		f.Round, status, identity).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("create event: %w", err)
	}
	return id, nil
}

func (t *txImporter) upsertParticipants(ctx context.Context, eventID, homeID, awayID int64, f providers.Fixture) error {
	_, err := t.tx.Exec(ctx, `
		INSERT INTO event_participants (event_id, team_id, role, display_order, provider_name)
		VALUES ($1, $2, 'home', 0, $3), ($1, $4, 'away', 1, $5)
		ON CONFLICT (event_id, role) DO UPDATE SET
			team_id = EXCLUDED.team_id,
			provider_name = EXCLUDED.provider_name`,
		eventID, homeID, f.Home.Name, awayID, f.Away.Name)
	if err != nil {
		return fmt.Errorf("upsert participants: %w", err)
	}
	return nil
}

func (t *txImporter) linkProviderID(ctx context.Context, eventID int64, f providers.Fixture, matched bool) error {
	method := "derived_fingerprint"
	if matched {
		method = "team_date_match"
	}
	_, err := t.tx.Exec(ctx, `
		INSERT INTO event_provider_ids
			(event_id, provider, provider_event_id, observed_start_time, observed_status, match_method)
		VALUES ($1, $2, $3, NULL, $4, $5)
		ON CONFLICT (provider, provider_event_id) DO NOTHING`,
		eventID, f.Provider, f.ProviderEventID, string(f.Status), method)
	if err != nil {
		return fmt.Errorf("link provider id: %w", err)
	}
	return nil
}

func (t *txImporter) updateEvent(ctx context.Context, eventID int64, f providers.Fixture) error {
	status := string(f.Status)
	if status == "" {
		status = "unknown"
	}
	_, err := t.tx.Exec(ctx, `
		UPDATE events SET
			-- Date-only providers must not move an event whose UTC kickoff has
			-- been verified by a provider that reports real timestamps.
			start_date = CASE WHEN start_time IS NULL THEN $2 ELSE start_date END,
			start_time_local = CASE WHEN start_time IS NULL
				THEN COALESCE(NULLIF($3, ''), start_time_local)
				ELSE start_time_local END,
			round_label = COALESCE(NULLIF($4, ''), round_label),
			status = CASE WHEN status = 'finished' THEN 'finished' ELSE $5 END,
			identity_status = CASE WHEN identity_status = 'unresolved' THEN 'provisional' ELSE identity_status END
		WHERE id = $1`,
		eventID, f.StartDate, f.StartTimeLocal, f.Round, status)
	if err != nil {
		return fmt.Errorf("update event: %w", err)
	}
	return nil
}

func (t *txImporter) upsertResult(ctx context.Context, eventID int64, f providers.Fixture) error {
	var prevHome, prevAway *int
	err := t.tx.QueryRow(ctx,
		`SELECT home_score, away_score FROM event_results WHERE event_id = $1`,
		eventID).Scan(&prevHome, &prevAway)
	if err != nil && err != pgx.ErrNoRows {
		return fmt.Errorf("read existing result: %w", err)
	}
	if err == nil && !sameScore(prevHome, f.HomeScore, prevAway, f.AwayScore) {
		t.log.WarnContext(ctx, "provider result corrected",
			slog.Int64("event_id", eventID),
			slog.String("provider", f.Provider),
			slog.Any("previous", [2]*int{prevHome, prevAway}),
			slog.Any("current", [2]*int{f.HomeScore, f.AwayScore}))
	}

	_, err = t.tx.Exec(ctx, `
		INSERT INTO event_results
			(event_id, home_score, away_score, home_score_ht, away_score_ht, provider)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (event_id) DO UPDATE SET
			home_score = EXCLUDED.home_score,
			away_score = EXCLUDED.away_score,
			home_score_ht = COALESCE(EXCLUDED.home_score_ht, event_results.home_score_ht),
			away_score_ht = COALESCE(EXCLUDED.away_score_ht, event_results.away_score_ht),
			provider = EXCLUDED.provider,
			captured_at = now()`,
		eventID, f.HomeScore, f.AwayScore, f.HomeScoreHT, f.AwayScoreHT, f.Provider)
	if err != nil {
		return fmt.Errorf("upsert result: %w", err)
	}
	return nil
}

func sameScore(prevHome, home, prevAway, away *int) bool {
	if prevHome == nil || home == nil || prevAway == nil || away == nil {
		return true // only report a correction when both sides are known
	}
	return *prevHome == *home && *prevAway == *away
}

// EnsureCompetition resolves an ESPN league code to a canonical competition
// slug, creating a reference row the first time the league is seen.
//
// Competition rows are product configuration like any other, so a league that
// only appears in live discovery still gets a stable slug that the events can
// point at. The name comes from the provider, not from a guess.
func (im *Importer) EnsureCompetition(ctx context.Context, sportSlug, code, name string) (string, error) {
	var slug string
	err := im.pool.QueryRow(ctx,
		`SELECT slug FROM competitions WHERE espn_code = $1`, code).Scan(&slug)
	if err == nil {
		return slug, nil
	}
	if err != pgx.ErrNoRows {
		return "", fmt.Errorf("lookup competition for %q: %w", code, err)
	}

	display := name
	if display == "" {
		display = code
	}
	slug = normalize.Slug(code)

	if _, err := im.pool.Exec(ctx, `
		INSERT INTO competitions (sport_id, name, slug, type, espn_code)
		SELECT id, $2, $3, 'league', $4 FROM sports WHERE slug = $1
		ON CONFLICT (sport_id, slug) DO UPDATE SET
			espn_code = COALESCE(competitions.espn_code, EXCLUDED.espn_code)`,
		sportSlug, display, slug, code); err != nil {
		return "", fmt.Errorf("create competition %q: %w", code, err)
	}

	if err := im.pool.QueryRow(ctx,
		`SELECT slug FROM competitions WHERE espn_code = $1`, code).Scan(&slug); err != nil {
		return "", fmt.Errorf("resolve competition %q: %w", code, err)
	}
	return slug, nil
}

// EnsureCompetitionByFotmob resolves a FotMob league id to a canonical
// competition slug, creating a reference row the first time the league is seen.
func (im *Importer) EnsureCompetitionByFotmob(ctx context.Context, sportSlug string, leagueID int64, name string) (string, error) {
	var slug string
	err := im.pool.QueryRow(ctx,
		`SELECT slug FROM competitions WHERE fotmob_league_id = $1`, leagueID).Scan(&slug)
	if err == nil {
		return slug, nil
	}
	if err != pgx.ErrNoRows {
		return "", fmt.Errorf("lookup competition for fotmob %d: %w", leagueID, err)
	}

	display := name
	if display == "" {
		display = fmt.Sprintf("FotMob league %d", leagueID)
	}
	_, key := normalize.Resolve(display)
	slug = normalize.Slug(key)
	if slug == "" {
		slug = fmt.Sprintf("fotmob-%d", leagueID)
	}

	if _, err := im.pool.Exec(ctx, `
		INSERT INTO competitions (sport_id, name, slug, type, fotmob_league_id)
		SELECT id, $2, $3, 'league', $4 FROM sports WHERE slug = $1
		ON CONFLICT (sport_id, slug) DO UPDATE SET
			fotmob_league_id = COALESCE(competitions.fotmob_league_id, EXCLUDED.fotmob_league_id)`,
		sportSlug, display, slug, leagueID); err != nil {
		return "", fmt.Errorf("create competition from fotmob %d: %w", leagueID, err)
	}

	if err := im.pool.QueryRow(ctx,
		`SELECT slug FROM competitions WHERE fotmob_league_id = $1`, leagueID).Scan(&slug); err != nil {
		return "", fmt.Errorf("resolve competition from fotmob %d: %w", leagueID, err)
	}
	return slug, nil
}

// EnsureCountry resolves a provider country name to a reference row, creating
// one when it is genuinely unknown. The ISO code stays empty in that case: a
// guessed flag is worse than no flag.
func (im *Importer) EnsureCountry(ctx context.Context, name string) (int64, error) {
	display, key := normalize.Resolve(name)
	if key == "" {
		return 0, nil
	}
	slug := normalize.Slug(key)
	// Providers spell countries differently; fold the ones that would
	// otherwise create a second row and lose the flag.
	lower := strings.ToLower(strings.TrimSpace(name))
	if alias, ok := countrySlugAliases[lower]; ok {
		slug = alias
	}

	var id int64
	err := im.pool.QueryRow(ctx, `SELECT id FROM countries WHERE slug = $1`, slug).Scan(&id)
	if err == nil {
		return id, nil
	}
	if err != pgx.ErrNoRows {
		return 0, fmt.Errorf("lookup country %q: %w", name, err)
	}

	err = im.pool.QueryRow(ctx, `
		INSERT INTO countries (name, slug) VALUES ($1, $2)
		ON CONFLICT (slug) DO UPDATE SET name = countries.name
		RETURNING id`, display, slug).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("create country %q: %w", name, err)
	}
	return id, nil
}

// countrySlugAliases maps provider spellings to the canonical country slug.
var countrySlugAliases = map[string]string{
	"uae":                          "united-arab-emirates",
	"u.a.e":                        "united-arab-emirates",
	"usa":                          "united-states",
	"u.s.a":                        "united-states",
	"czech republic":               "czechia",
	"bosnia-herzegovina":           "bosnia-and-herzegovina",
	"bosnia herzegovina":           "bosnia-and-herzegovina",
	"macedonia":                    "north-macedonia",
	"fyrom":                        "north-macedonia",
	"cote divoire":                 "ivory-coast",
	"côte d'ivoire":                "ivory-coast",
	"congo dr":                     "dr-congo",
	"democratic republic of congo": "dr-congo",
	"korea republic":               "south-korea",
	"republic of korea":            "south-korea",
	"ir iran":                      "iran",
	"iran islamic republic":        "iran",
}

// BackfillCompetitionCountries derives a country from a league name prefix
// ("UAE. Pro League") wherever the country is still missing.
func (im *Importer) BackfillCompetitionCountries(ctx context.Context) (int64, error) {
	tag, err := im.pool.Exec(ctx, `
		UPDATE competitions c
		SET country_id = co.id
		FROM countries co
		WHERE c.country_id IS NULL
		  AND co.slug = trim(both '-' from lower(regexp_replace(
		      CASE lower(split_part(c.name, '.', 1))
		          WHEN 'uae' THEN 'united arab emirates'
		          WHEN 'usa' THEN 'united states'
		          WHEN 'czech republic' THEN 'czechia'
		          WHEN 'bosnia-herzegovina' THEN 'bosnia-and-herzegovina'
		          ELSE split_part(c.name, '.', 1)
		      END, '[^a-zA-Z0-9]+', '-', 'g')))`)
	if err != nil {
		return 0, fmt.Errorf("backfill competition countries: %w", err)
	}
	return tag.RowsAffected(), nil
}

// EnsureCompetitionByOnexbet resolves a 1xBet league id to a canonical
// competition slug, creating a reference row the first time the league is seen.
func (im *Importer) EnsureCompetitionByOnexbet(ctx context.Context, sportSlug string, leagueID int64, name, countryName string) (string, error) {
	var slug string
	err := im.pool.QueryRow(ctx,
		`SELECT slug FROM competitions WHERE onexbet_league_id = $1`, leagueID).Scan(&slug)
	if err == nil {
		// Backfill the country on rows created before flags were tracked.
		if countryName != "" {
			if countryID, countryErr := im.EnsureCountry(ctx, countryName); countryErr == nil && countryID != 0 {
				_, _ = im.pool.Exec(ctx, `
					UPDATE competitions SET country_id = $2
					WHERE onexbet_league_id = $1 AND country_id IS NULL`, leagueID, countryID)
			}
		}
		return slug, nil
	}
	if err != pgx.ErrNoRows {
		return "", fmt.Errorf("lookup competition for 1xbet %d: %w", leagueID, err)
	}

	display := name
	if display == "" {
		display = fmt.Sprintf("1xBet league %d", leagueID)
	}
	_, key := normalize.Resolve(display)
	slug = normalize.Slug(key)
	if slug == "" {
		slug = fmt.Sprintf("onexbet-%d", leagueID)
	}

	var countryID *int64
	if countryName != "" {
		if id, countryErr := im.EnsureCountry(ctx, countryName); countryErr == nil && id != 0 {
			countryID = &id
		}
	}

	if _, err := im.pool.Exec(ctx, `
		INSERT INTO competitions (sport_id, name, slug, type, onexbet_league_id, country_id)
		SELECT id, $2, $3, 'league', $4, $5 FROM sports WHERE slug = $1
		ON CONFLICT (sport_id, slug) DO UPDATE SET
			onexbet_league_id = COALESCE(competitions.onexbet_league_id, EXCLUDED.onexbet_league_id),
			country_id = COALESCE(competitions.country_id, EXCLUDED.country_id)`,
		sportSlug, display, slug, leagueID, countryID); err != nil {
		return "", fmt.Errorf("create competition from 1xbet %d: %w", leagueID, err)
	}

	if err := im.pool.QueryRow(ctx,
		`SELECT slug FROM competitions WHERE onexbet_league_id = $1`, leagueID).Scan(&slug); err != nil {
		return "", fmt.Errorf("resolve competition from 1xbet %d: %w", leagueID, err)
	}
	return slug, nil
}
