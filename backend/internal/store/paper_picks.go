package store

import (
	"context"
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
	Side            string
	Line            float64
	Odds            float64
	Minute          *int
	TotalCorners    *int
	HomeCorners     *int
	AwayCorners     *int
	PacePerMinute   *float64
	ProjectedTotal  *float64
	ProjectionLow   *float64
	ProjectionHigh  *float64
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
}

// PaperPickInput carries one recording. Values come from the same metrics the
// board shows, so the pick and the card cannot disagree.
type PaperPickInput struct {
	EventID         int64
	CompetitionID   int64
	HomeTeamID      int64
	AwayTeamID      int64
	Side            string
	Line            float64
	Odds            float64
	Minute          *int
	HomeCorners     *int
	AwayCorners     *int
	TotalCorners    *int
	PacePerMinute   *float64
	ProjectedTotal  *float64
	ProjectionLow   *float64
	ProjectionHigh  *float64
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
			side, line, odds,
			minute, home_corners, away_corners, total_corners,
			pace_per_minute, projected_total, projection_low, projection_high,
			needed, needed_per_minute, model_risk, early_pace,
			event_status_at_save, data_cutoff_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11,
		        $12, $13, $14, $15, $16, $17, $18, $19, $20, $21)
		RETURNING id, status, created_at`,
		in.EventID, in.CompetitionID, in.HomeTeamID, in.AwayTeamID,
		in.Side, in.Line, in.Odds,
		in.Minute, in.HomeCorners, in.AwayCorners, in.TotalCorners,
		in.PacePerMinute, in.ProjectedTotal, in.ProjectionLow, in.ProjectionHigh,
		in.Needed, in.NeededPerMinute, in.ModelRisk, in.EarlyPace,
		in.EventStatus, in.DataCutoffAt).Scan(&out.ID, &out.Status, &out.CreatedAt)
	if err != nil {
		return out, fmt.Errorf("create paper pick: %w", err)
	}
	out.EventID = in.EventID
	out.Side = in.Side
	out.Line = in.Line
	out.Odds = in.Odds
	out.Minute = in.Minute
	out.HomeCorners = in.HomeCorners
	out.AwayCorners = in.AwayCorners
	out.TotalCorners = in.TotalCorners
	out.PacePerMinute = in.PacePerMinute
	out.ProjectedTotal = in.ProjectedTotal
	out.ProjectionLow = in.ProjectionLow
	out.ProjectionHigh = in.ProjectionHigh
	out.Needed = in.Needed
	out.NeededPerMinute = in.NeededPerMinute
	out.ModelRisk = in.ModelRisk
	out.EarlyPace = in.EarlyPace
	out.EventStatus = in.EventStatus
	out.DataCutoffAt = in.DataCutoffAt
	return out, nil
}

const paperPickSelect = `
	SELECT p.id, p.event_id, c.name, h.name, a.name,
	       p.side, p.line, p.odds,
	       p.minute, p.total_corners, p.home_corners, p.away_corners,
	       p.pace_per_minute, p.projected_total, p.projection_low, p.projection_high,
	       p.needed, p.needed_per_minute, p.model_risk, p.early_pace,
	       p.event_status_at_save, p.status, p.final_total, p.settlement_rule,
	       p.created_at, p.settled_at, p.data_cutoff_at
	FROM paper_picks p
	JOIN competitions c ON c.id = p.competition_id
	JOIN teams h ON h.id = p.home_team_id
	JOIN teams a ON a.id = p.away_team_id
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
			&item.Side, &item.Line, &item.Odds,
			&item.Minute, &item.TotalCorners, &item.HomeCorners, &item.AwayCorners,
			&item.PacePerMinute, &item.ProjectedTotal, &item.ProjectionLow, &item.ProjectionHigh,
			&item.Needed, &item.NeededPerMinute, &item.ModelRisk, &item.EarlyPace,
			&item.EventStatus, &item.Status, &item.FinalTotal, &item.SettlementRule,
			&item.CreatedAt, &item.SettledAt, &item.DataCutoffAt); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
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
	return tag.RowsAffected(), nil
}
