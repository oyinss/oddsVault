// Command model-picks generates one frozen set of recommendations from settled
// history. A pick is written once with the odds and cutoff used; later results
// never change it.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"os"
	"sort"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/oyins/oddsvault/backend/internal/config"
	"github.com/oyins/oddsvault/backend/internal/database"
	"github.com/oyins/oddsvault/backend/internal/logging"
)

type rules struct {
	Market    string  `json:"market"`
	MinSample int     `json:"min_sample"`
	MinEdge   float64 `json:"min_edge"`
	MaxPicks  int     `json:"max_picks"`
	Bucket    float64 `json:"bucket"`
}

type bucketStats struct {
	hits, misses int
}

type candidate struct {
	selectionID int64
	eventID     int64
	home        string
	away        string
	kickoff     time.Time
	selection   string
	odds        float64
	bucket      float64
	samples     int
	hitRate     float64
	edge        float64
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "model-picks: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load("oddsvault-model-picks")
	if err != nil {
		return err
	}
	logger := logging.New(cfg.Env, cfg.LogLevel)
	slog.SetDefault(logger)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	pool, err := database.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()
	if err := database.Ping(ctx, pool, cfg.ConnectTimeout); err != nil {
		return fmt.Errorf("database unreachable: %w", err)
	}

	var versionID int64
	var version int
	var rawRules []byte
	err = pool.QueryRow(ctx, `
		SELECT sv.id, sv.version, sv.rules
		FROM strategy_versions sv
		JOIN strategy_definitions sd ON sd.id = sv.strategy_id
		WHERE sd.slug = 'match-result-edge'
		ORDER BY sv.version DESC LIMIT 1`).Scan(&versionID, &version, &rawRules)
	if err != nil {
		return fmt.Errorf("load strategy version: %w", err)
	}
	var r rules
	if err := json.Unmarshal(rawRules, &r); err != nil {
		return fmt.Errorf("decode strategy rules: %w", err)
	}
	if r.Bucket <= 0 {
		r.Bucket = 0.1
	}

	stats, err := historicalBuckets(ctx, pool, r)
	if err != nil {
		return err
	}
	candidates, err := upcomingCandidates(ctx, pool, r)
	if err != nil {
		return err
	}

	bestPerEvent := make(map[int64]candidate)
	for _, c := range candidates {
		stat, ok := stats[c.bucket]
		if !ok {
			continue
		}
		settled := stat.hits + stat.misses
		if settled < r.MinSample {
			continue
		}
		hitRate := float64(stat.hits) / float64(settled)
		implied := 1 / c.odds
		edge := hitRate - implied
		if edge < r.MinEdge {
			continue
		}
		c.samples = settled
		c.hitRate = hitRate
		c.edge = edge
		if existing, ok := bestPerEvent[c.eventID]; !ok || edge > existing.edge {
			bestPerEvent[c.eventID] = c
		}
	}

	selected := make([]candidate, 0, len(bestPerEvent))
	for _, c := range bestPerEvent {
		selected = append(selected, c)
	}
	sort.Slice(selected, func(i, j int) bool { return selected[i].edge > selected[j].edge })
	if len(selected) > r.MaxPicks {
		selected = selected[:r.MaxPicks]
	}

	notes, _ := json.Marshal(map[string]any{
		"strategy":        "match-result-edge",
		"rules":           r,
		"candidates":      len(candidates),
		"qualifying":      len(bestPerEvent),
		"historical_size": len(stats),
	})

	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(context.WithoutCancel(ctx)) }()

	var runID int64
	if err := tx.QueryRow(ctx, `
		INSERT INTO model_runs (strategy_version_id, data_cutoff_at, status, notes)
		VALUES ($1, now(), 'completed', $2)
		RETURNING id`, versionID, notes).Scan(&runID); err != nil {
		return fmt.Errorf("create model run: %w", err)
	}

	for _, pick := range selected {
		var betID int64
		if err := tx.QueryRow(ctx, `
			INSERT INTO model_bets (run_id, event_id, combined_odds, status)
			VALUES ($1, $2, $3, 'pending')
			RETURNING id`, runID, pick.eventID, pick.odds).Scan(&betID); err != nil {
			return fmt.Errorf("create model bet: %w", err)
		}
		evidence, _ := json.Marshal(map[string]any{
			"bucket":   pick.bucket,
			"samples":  pick.samples,
			"hit_rate": math.Round(pick.hitRate*10000) / 10000,
			"implied":  math.Round((1/pick.odds)*10000) / 10000,
			"edge":     math.Round(pick.edge*10000) / 10000,
		})
		if _, err := tx.Exec(ctx, `
			INSERT INTO model_bet_legs (model_bet_id, event_id, selection_id, market_type, selection, odds, evidence)
			VALUES ($1, $2, $3, $4, $5, $6, $7)`,
			betID, pick.eventID, pick.selectionID, r.Market, pick.selection, pick.odds, evidence); err != nil {
			return fmt.Errorf("create model leg: %w", err)
		}
		fmt.Printf("%s v %s - %s @ %.3f (edge %+.1f%%, n=%d)\n",
			pick.home, pick.away, pick.selection, pick.odds, pick.edge*100, pick.samples)
	}

	if err := tx.Commit(ctx); err != nil {
		return err
	}
	logger.Info("model run complete",
		slog.Int64("run_id", runID),
		slog.Int("picks", len(selected)),
		slog.Int("strategy_version", version),
		slog.Int("candidates", len(candidates)))
	if len(selected) == 0 {
		fmt.Println("no qualifying picks at this cutoff")
	}
	return nil
}

func historicalBuckets(ctx context.Context, pool *pgxpool.Pool, r rules) (map[float64]bucketStats, error) {
	rows, err := pool.Query(ctx, `
		WITH priced AS (
			SELECT latest.odds::float8 AS odds, st.status
			FROM market_selections se
			JOIN markets m ON m.id = se.market_id
			JOIN market_types mt ON mt.id = m.market_type_id
			JOIN selection_settlements st ON st.selection_id = se.id
			JOIN LATERAL (
				SELECT o.decimal_odds::float8 AS odds
				FROM odds_snapshots o
				WHERE o.selection_id = se.id
				ORDER BY o.captured_at DESC
				LIMIT 1
			) latest ON TRUE
			WHERE mt.slug = $1
		)
		SELECT floor(odds / $2) * $2 AS low,
		       count(*) FILTER (WHERE status = 'hit')::int,
		       count(*) FILTER (WHERE status = 'miss')::int
		FROM priced
		GROUP BY low`, r.Market, r.Bucket)
	if err != nil {
		return nil, fmt.Errorf("historical buckets: %w", err)
	}
	defer rows.Close()

	out := make(map[float64]bucketStats)
	for rows.Next() {
		var low float64
		var stat bucketStats
		if err := rows.Scan(&low, &stat.hits, &stat.misses); err != nil {
			return nil, err
		}
		out[math.Round(low*1000)/1000] = stat
	}
	return out, rows.Err()
}

func upcomingCandidates(ctx context.Context, pool *pgxpool.Pool, r rules) ([]candidate, error) {
	rows, err := pool.Query(ctx, `
		SELECT se.id, e.id, ht.name, at.name,
		       COALESCE(e.start_time, e.start_date::timestamptz) AS kickoff,
		       se.selection, latest.odds::float8
		FROM events e
		JOIN event_participants hp ON hp.event_id = e.id AND hp.role = 'home'
		JOIN teams ht ON ht.id = hp.team_id
		JOIN event_participants ap ON ap.event_id = e.id AND ap.role = 'away'
		JOIN teams at ON at.id = ap.team_id
		JOIN markets m ON m.event_id = e.id
		JOIN market_types mt ON mt.id = m.market_type_id AND mt.slug = $1
		JOIN market_selections se ON se.market_id = m.id
		JOIN LATERAL (
			SELECT o.decimal_odds::float8 AS odds
			FROM odds_snapshots o
			WHERE o.selection_id = se.id
			ORDER BY o.captured_at DESC
			LIMIT 1
		) latest ON TRUE
		WHERE e.status = 'scheduled'
		  AND COALESCE(e.start_time, e.start_date::timestamptz) > now()
		  -- One open pick per fixture per strategy: a re-run does not stack a
		  -- second recommendation on a match already recommended.
		  AND NOT EXISTS (
		      SELECT 1
		      FROM model_bet_legs ml
		      JOIN model_bets mb ON mb.id = ml.model_bet_id
		      JOIN model_runs mr ON mr.id = mb.run_id
		      JOIN strategy_versions sv ON sv.id = mr.strategy_version_id
		      JOIN strategy_definitions sd ON sd.id = sv.strategy_id
		      WHERE ml.event_id = e.id
		        AND mb.status = 'pending'
		        AND sd.slug = $2
		  )
		ORDER BY kickoff ASC
		LIMIT 500`, r.Market, "match-result-edge")
	if err != nil {
		return nil, fmt.Errorf("upcoming candidates: %w", err)
	}
	defer rows.Close()

	var out []candidate
	for rows.Next() {
		var c candidate
		if err := rows.Scan(&c.selectionID, &c.eventID, &c.home, &c.away, &c.kickoff, &c.selection, &c.odds); err != nil {
			return nil, err
		}
		c.bucket = math.Round(math.Floor(c.odds/r.Bucket)*r.Bucket*1000) / 1000
		out = append(out, c)
	}
	return out, rows.Err()
}
