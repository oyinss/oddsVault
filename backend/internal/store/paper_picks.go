package store

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// PaperPick is a corner selection recorded by hand with no stake. The board
// context captured with it is deliberately frozen: it is training data, and a
// value filled in after the fact would not be what the picker actually saw.
type PaperPick struct {
	ID              int64
	EventID         int64
	Competition     string
	Home            string
	Away            string
	HomeTeamID      int64
	AwayTeamID      int64
	CurrentStatus   string
	Kickoff         *time.Time
	Provider        string
	ProviderSlug    string
	Side            string
	Line            float64
	Odds            float64
	Minute          *int
	HomeScore       *int
	AwayScore       *int
	TotalCorners    *int
	HomeCorners     *int
	AwayCorners     *int
	PacePerMinute   *float64
	ProjectedTotal  *float64
	ProjectionLow   *float64
	ProjectionHigh  *float64
	RecOverLine     *float64
	RecUnderLine    *float64
	Needed          *int
	NeededPerMinute *float64
	ModelRisk       bool
	EarlyPace       bool
	EventStatus     string
	Status          string
	FinalTotal      *int
	SettlementRule  *string
	CreatedAt       time.Time
	SettledAt       *time.Time
	DataCutoffAt    time.Time
	RecoveredAt     *time.Time
}

// PaperPickInput carries one recording. Values come from the same metrics the
// board shows, so the pick and the card cannot disagree.
type PaperPickInput struct {
	EventID         int64
	CompetitionID   int64
	HomeTeamID      int64
	AwayTeamID      int64
	ProviderSlug    string
	Side            string
	Line            float64
	Odds            float64
	Minute          *int
	HomeScore       *int
	AwayScore       *int
	HomeCorners     *int
	AwayCorners     *int
	TotalCorners    *int
	PacePerMinute   *float64
	ProjectedTotal  *float64
	ProjectionLow   *float64
	ProjectionHigh  *float64
	RecOverLine     *float64
	RecUnderLine    *float64
	Needed          *int
	NeededPerMinute *float64
	ModelRisk       bool
	EarlyPace       bool
	EventStatus     string
	DataCutoffAt    time.Time
}

// CreatePaperPick stores one recording and returns it.
func (im *Importer) CreatePaperPick(ctx context.Context, in PaperPickInput) (PaperPick, error) {
	var out PaperPick
	err := im.pool.QueryRow(ctx, `
		INSERT INTO paper_picks (
			event_id, competition_id, home_team_id, away_team_id,
			provider_slug, side, line, odds,
			minute, home_score_at_save, away_score_at_save,
			home_corners, away_corners, total_corners,
			pace_per_minute, projected_total, projection_low, projection_high,
			rec_over_line, rec_under_line,
			needed, needed_per_minute, model_risk, early_pace,
			event_status_at_save, data_cutoff_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12,
		        $13, $14, $15, $16, $17, $18, $19, $20, $21, $22, $23, $24, $25, $26)
		RETURNING id, status, created_at`,
		in.EventID, in.CompetitionID, in.HomeTeamID, in.AwayTeamID,
		in.ProviderSlug, in.Side, in.Line, in.Odds,
		in.Minute, in.HomeScore, in.AwayScore,
		in.HomeCorners, in.AwayCorners, in.TotalCorners,
		in.PacePerMinute, in.ProjectedTotal, in.ProjectionLow, in.ProjectionHigh,
		in.RecOverLine, in.RecUnderLine,
		in.Needed, in.NeededPerMinute, in.ModelRisk, in.EarlyPace,
		in.EventStatus, in.DataCutoffAt).Scan(&out.ID, &out.Status, &out.CreatedAt)
	if err != nil {
		return out, fmt.Errorf("create paper pick: %w", err)
	}
	out.EventID = in.EventID
	out.ProviderSlug = in.ProviderSlug
	out.Side = in.Side
	out.Line = in.Line
	out.Odds = in.Odds
	out.Minute = in.Minute
	out.HomeScore = in.HomeScore
	out.AwayScore = in.AwayScore
	out.HomeCorners = in.HomeCorners
	out.AwayCorners = in.AwayCorners
	out.TotalCorners = in.TotalCorners
	out.PacePerMinute = in.PacePerMinute
	out.ProjectedTotal = in.ProjectedTotal
	out.ProjectionLow = in.ProjectionLow
	out.ProjectionHigh = in.ProjectionHigh
	out.RecOverLine = in.RecOverLine
	out.RecUnderLine = in.RecUnderLine
	out.Needed = in.Needed
	out.NeededPerMinute = in.NeededPerMinute
	out.ModelRisk = in.ModelRisk
	out.EarlyPace = in.EarlyPace
	out.EventStatus = in.EventStatus
	out.DataCutoffAt = in.DataCutoffAt
	return out, nil
}

const paperPickSelect = `
	SELECT p.id, p.event_id, c.name, h.name, a.name, h.id, a.id,
	       e.status, COALESCE(e.start_time, e.start_date::timestamptz),
	       b.name, p.provider_slug,
	       p.side, p.line, p.odds,
	       p.minute, p.home_score_at_save, p.away_score_at_save,
	       p.total_corners, p.home_corners, p.away_corners,
	       p.pace_per_minute, p.projected_total, p.projection_low, p.projection_high,
	       p.rec_over_line, p.rec_under_line,
	       p.needed, p.needed_per_minute, p.model_risk, p.early_pace,
	       p.event_status_at_save, p.status, p.final_total, p.settlement_rule,
	       p.created_at, p.settled_at, p.data_cutoff_at, p.recovered_at
	FROM paper_picks p
	JOIN competitions c ON c.id = p.competition_id
	JOIN teams h ON h.id = p.home_team_id
	JOIN teams a ON a.id = p.away_team_id
	JOIN events e ON e.id = p.event_id
	JOIN bookmakers b ON b.slug = p.provider_slug
	`

// ListPaperPicks returns recent recordings, newest first.
func (im *Importer) ListPaperPicks(ctx context.Context, limit int) ([]PaperPick, error) {
	if limit <= 0 || limit > 500 {
		limit = 200
	}
	rows, err := im.pool.Query(ctx, paperPickSelect+`
		ORDER BY p.created_at DESC
		LIMIT $1`, limit)
	if err != nil {
		return nil, fmt.Errorf("list paper picks: %w", err)
	}
	defer rows.Close()

	var out []PaperPick
	for rows.Next() {
		var item PaperPick
		if err := rows.Scan(&item.ID, &item.EventID, &item.Competition, &item.Home, &item.Away,
			&item.HomeTeamID, &item.AwayTeamID, &item.CurrentStatus, &item.Kickoff,
			&item.Provider, &item.ProviderSlug,
			&item.Side, &item.Line, &item.Odds,
			&item.Minute, &item.HomeScore, &item.AwayScore,
			&item.TotalCorners, &item.HomeCorners, &item.AwayCorners,
			&item.PacePerMinute, &item.ProjectedTotal, &item.ProjectionLow, &item.ProjectionHigh,
			&item.RecOverLine, &item.RecUnderLine,
			&item.Needed, &item.NeededPerMinute, &item.ModelRisk, &item.EarlyPace,
			&item.EventStatus, &item.Status, &item.FinalTotal, &item.SettlementRule,
			&item.CreatedAt, &item.SettledAt, &item.DataCutoffAt, &item.RecoveredAt); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

// ErrPaperPickSettled means the pick already settled, so its line and side are
// part of the record and may not be rewritten.
var ErrPaperPickSettled = errors.New("paper pick already settled")

// UpdatePaperPick corrects a recording that has not settled yet. The frozen
// context stays as captured; only the selection the user typed changes.
func (im *Importer) UpdatePaperPick(ctx context.Context, id int64, providerSlug, side string, line, odds float64) (PaperPick, error) {
	var status string
	if err := im.pool.QueryRow(ctx, `SELECT status FROM paper_picks WHERE id = $1`, id).Scan(&status); err != nil {
		return PaperPick{}, fmt.Errorf("load paper pick: %w", err)
	}
	if status != "pending" {
		return PaperPick{}, ErrPaperPickSettled
	}
	if _, err := im.pool.Exec(ctx, `
		UPDATE paper_picks
		SET provider_slug = $2, side = $3, line = $4, odds = $5
		WHERE id = $1`, id, providerSlug, side, line, odds); err != nil {
		return PaperPick{}, fmt.Errorf("update paper pick: %w", err)
	}
	var out PaperPick
	err := im.pool.QueryRow(ctx, `
		SELECT p.id, p.event_id, p.provider_slug, p.side, p.line, p.odds,
		       p.status, p.model_risk, p.early_pace, p.event_status_at_save,
		       p.created_at, p.data_cutoff_at, p.final_total, p.settlement_rule,
		       p.settled_at, p.recovered_at, p.projection_low, p.projection_high,
		       p.rec_over_line, p.rec_under_line
		FROM paper_picks p WHERE p.id = $1`, id).Scan(
		&out.ID, &out.EventID, &out.ProviderSlug, &out.Side, &out.Line, &out.Odds,
		&out.Status, &out.ModelRisk, &out.EarlyPace, &out.EventStatus,
		&out.CreatedAt, &out.DataCutoffAt, &out.FinalTotal, &out.SettlementRule,
		&out.SettledAt, &out.RecoveredAt, &out.ProjectionLow, &out.ProjectionHigh,
		&out.RecOverLine, &out.RecUnderLine)
	if err != nil {
		return PaperPick{}, fmt.Errorf("reload paper pick: %w", err)
	}
	return out, nil
}

// DeletePaperPick removes one recording, for a mistyped line or odds.
func (im *Importer) DeletePaperPick(ctx context.Context, id int64) (bool, error) {
	tag, err := im.pool.Exec(ctx, `DELETE FROM paper_picks WHERE id = $1`, id)
	if err != nil {
		return false, fmt.Errorf("delete paper pick: %w", err)
	}
	return tag.RowsAffected() > 0, nil
}

// SettlePaperPicks settles pending picks once their match is finished and both
// final corner counts are known. The rule is the standard one: over needs more
// than the line, under needs fewer, an exact hit is a push.
func (im *Importer) SettlePaperPicks(ctx context.Context) (int64, error) {
	tag, err := im.pool.Exec(ctx, `
		WITH finals AS (
			SELECT e.id AS event_id, sum(s.corners)::int AS total
			FROM events e
			JOIN team_match_stats s ON s.event_id = e.id
			WHERE e.status = 'finished' AND s.corners IS NOT NULL
			GROUP BY e.id
			HAVING count(*) = 2
		)
		UPDATE paper_picks p
		SET final_total = f.total,
		    status = CASE
		        WHEN f.total::numeric = p.line THEN 'push'
		        WHEN p.side = 'over' AND f.total::numeric > p.line THEN 'won'
		        WHEN p.side = 'under' AND f.total::numeric < p.line THEN 'won'
		        ELSE 'lost'
		    END,
		    settled_at = now(),
		    settlement_rule = 'total_corners_final'
		FROM finals f
		WHERE p.event_id = f.event_id
		  AND p.status = 'pending'`)
	if err != nil {
		return 0, fmt.Errorf("settle paper picks: %w", err)
	}
	settled := tag.RowsAffected()

	// A provider can drop a match without ever publishing a final status, which
	// leaves the pick pending long after the game ended. A full match plus
	// stoppage fits inside two and a quarter hours, and a live feed that has
	// been silent for twenty minutes is not still playing, so the stored pair is
	// the only final count that will ever exist. Settle with an explicit rule
	// name so a training set can filter these out.
	timeoutTag, err := im.pool.Exec(ctx, `
		WITH finals AS (
			SELECT e.id AS event_id, sum(s.corners)::int AS total
			FROM events e
			JOIN team_match_stats s ON s.event_id = e.id
			LEFT JOIN event_live_states l ON l.event_id = e.id
			WHERE e.status IN ('live', 'unknown')
			  AND COALESCE(e.start_time, e.start_date::timestamptz) < now() - interval '2 hours 15 minutes'
			  AND (l.captured_at IS NULL OR l.captured_at < now() - interval '20 minutes')
			  AND s.corners IS NOT NULL
			GROUP BY e.id
			HAVING count(*) = 2
		)
		UPDATE paper_picks p
		SET final_total = f.total,
		    status = CASE
		        WHEN f.total::numeric = p.line THEN 'push'
		        WHEN p.side = 'over' AND f.total::numeric > p.line THEN 'won'
		        WHEN p.side = 'under' AND f.total::numeric < p.line THEN 'won'
		        ELSE 'lost'
		    END,
		    settled_at = now(),
		    settlement_rule = 'total_corners_after_timeout'
		FROM finals f
		WHERE p.event_id = f.event_id
		  AND p.status = 'pending'`)
	if err != nil {
		return settled, fmt.Errorf("settle paper picks after timeout: %w", err)
	}
	return settled + timeoutTag.RowsAffected(), nil
}
