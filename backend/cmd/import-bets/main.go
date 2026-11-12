// Command import-bets stores a bookmaker bet-history export.
//
// The input is a JSON snapshot captured from the bookmaker's own history page
// (see .local/imports). Re-importing the same snapshot is idempotent; a changed
// bet creates a new version rather than rewriting history silently.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/oyins/oddsvault/backend/internal/config"
	"github.com/oyins/oddsvault/backend/internal/database"
	"github.com/oyins/oddsvault/backend/internal/logging"
	"github.com/oyins/oddsvault/backend/internal/store"
)

type exportFile struct {
	Bookmaker string `json:"bookmaker"`
	Currency  string `json:"currency"`
	Source    string `json:"source"`
	Bets      []bet  `json:"bets"`
}

type bet struct {
	ExternalID      string `json:"external_id"`
	PlacedAt        string `json:"placed_at"`
	BetType         string `json:"bet_type"`
	Stake           string `json:"stake"`
	Odds            string `json:"odds"`
	PotentialReturn string `json:"potential_return"`
	StatusText      string `json:"status_text"`
	Legs            []leg  `json:"legs"`
}

type leg struct {
	Competition string `json:"competition"`
	RawEvent    string `json:"raw_event"`
	MarketRaw   string `json:"market_raw"`
	OddsTaken   string `json:"odds_taken"`
	Status      string `json:"status"`
}

func main() {
	file := flag.String("file", "", "path to the bet-history JSON export")
	flag.Parse()
	if *file == "" {
		fmt.Fprintln(os.Stderr, "import-bets: -file is required")
		os.Exit(2)
	}
	if err := run(*file); err != nil {
		fmt.Fprintf(os.Stderr, "import-bets: %v\n", err)
		os.Exit(1)
	}
}

func run(path string) error {
	cfg, err := config.Load("oddsvault-import-bets")
	if err != nil {
		return err
	}
	logger := logging.New(cfg.Env, cfg.LogLevel)
	slog.SetDefault(logger)

	raw, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}
	var doc exportFile
	if err := json.Unmarshal(raw, &doc); err != nil {
		return fmt.Errorf("decode %s: %w", path, err)
	}
	if doc.Bookmaker == "" || doc.Source == "" {
		return fmt.Errorf("export must name a bookmaker and a source")
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	pool, err := database.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()
	if err := database.Ping(ctx, pool, cfg.ConnectTimeout); err != nil {
		return fmt.Errorf("database unreachable: %w", err)
	}

	var bookmakerID int64
	if err := pool.QueryRow(ctx, `SELECT id FROM bookmakers WHERE slug = $1`, doc.Bookmaker).Scan(&bookmakerID); err != nil {
		return fmt.Errorf("unknown bookmaker %q: %w", doc.Bookmaker, err)
	}

	inserted, updated, unchanged := 0, 0, 0
	for _, item := range doc.Bets {
		changed, existed, err := importBet(ctx, pool, bookmakerID, doc, item)
		if err != nil {
			return fmt.Errorf("bet %s: %w", item.ExternalID, err)
		}
		switch {
		case !existed:
			inserted++
		case changed:
			updated++
		default:
			unchanged++
		}
	}

	logger.Info("bet history imported",
		slog.String("bookmaker", doc.Bookmaker),
		slog.Int("inserted", inserted),
		slog.Int("updated", updated),
		slog.Int("unchanged", unchanged))
	importer := store.NewImporter(pool, logger)
	if resolved, unresolved, err := importer.ResolveBetLegs(ctx); err != nil {
		logger.Warn("bet leg resolution failed", slog.String("error", err.Error()))
	} else {
		logger.Info("bet legs resolved", slog.Int("resolved", resolved), slog.Int("unresolved", unresolved))
	}

	fmt.Printf("imported %d bets: %d new, %d updated, %d unchanged\n", len(doc.Bets), inserted, updated, unchanged)
	return nil
}

func importBet(ctx context.Context, pool *pgxpool.Pool, bookmakerID int64, doc exportFile, item bet) (changed, existed bool, err error) {
	tx, err := pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return false, false, err
	}
	defer func() { _ = tx.Rollback(context.WithoutCancel(ctx)) }()

	snapshot, err := json.Marshal(item)
	if err != nil {
		return false, false, err
	}

	stake := parseDecimal(item.Stake)
	odds := parseDecimal(item.Odds)
	potential := parseDecimal(item.PotentialReturn)
	status := normalizeStatus(item.StatusText)
	actual, profit := outcome(status, stake, potential)
	placedAt, placedRaw := parsePlacedAt(item.PlacedAt)

	var existingID int64
	var existingRaw []byte
	var version int
	err = tx.QueryRow(ctx, `
		SELECT id, raw, COALESCE((SELECT max(version) FROM personal_bet_versions v WHERE v.personal_bet_id = b.id), 0)
		FROM personal_bets b
		WHERE bookmaker_id = $1 AND external_id = $2`,
		bookmakerID, item.ExternalID).Scan(&existingID, &existingRaw, &version)

	switch {
	case err == pgx.ErrNoRows:
		if err := tx.QueryRow(ctx, `
			INSERT INTO personal_bets
				(bookmaker_id, external_id, currency, placed_at, placed_at_raw, bet_type,
				 stake, combined_odds, potential_return, status, actual_return, profit_loss, source, raw)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
			RETURNING id`,
			bookmakerID, item.ExternalID, doc.Currency, placedAt, placedRaw, item.BetType,
			stake, odds, potential, status, actual, profit, doc.Source, snapshot).Scan(&existingID); err != nil {
			return false, false, fmt.Errorf("insert bet: %w", err)
		}
		if err := insertVersion(ctx, tx, existingID, 1, snapshot); err != nil {
			return false, false, err
		}
	case err != nil:
		return false, false, fmt.Errorf("read existing bet: %w", err)
	default:
		existed = true
		if sameJSON(existingRaw, snapshot) {
			if err := replaceLegs(ctx, tx, existingID, item.Legs); err != nil {
				return false, true, err
			}
			if err := tx.Commit(ctx); err != nil {
				return false, true, err
			}
			return false, true, nil
		}
		version++
		if _, err := tx.Exec(ctx, `
			UPDATE personal_bets SET
				currency = $2, placed_at = $3, placed_at_raw = $4, bet_type = $5,
				stake = $6, combined_odds = $7, potential_return = $8, status = $9,
				actual_return = $10, profit_loss = $11, source = $12, raw = $13
			WHERE id = $1`,
			existingID, doc.Currency, placedAt, placedRaw, item.BetType,
			stake, odds, potential, status, actual, profit, doc.Source, snapshot); err != nil {
			return false, true, fmt.Errorf("update bet: %w", err)
		}
		if err := insertVersion(ctx, tx, existingID, version, snapshot); err != nil {
			return false, true, err
		}
		changed = true
	}

	if err := replaceLegs(ctx, tx, existingID, item.Legs); err != nil {
		return false, existed, err
	}
	if err := tx.Commit(ctx); err != nil {
		return false, existed, err
	}
	return changed, existed, nil
}

func insertVersion(ctx context.Context, tx pgx.Tx, betID int64, version int, snapshot []byte) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO personal_bet_versions (personal_bet_id, version, snapshot)
		VALUES ($1, $2, $3)
		ON CONFLICT (personal_bet_id, version) DO NOTHING`, betID, version, snapshot)
	if err != nil {
		return fmt.Errorf("insert bet version: %w", err)
	}
	return nil
}

func replaceLegs(ctx context.Context, tx pgx.Tx, betID int64, legs []leg) error {
	if _, err := tx.Exec(ctx, `DELETE FROM personal_bet_legs WHERE personal_bet_id = $1`, betID); err != nil {
		return fmt.Errorf("clear bet legs: %w", err)
	}
	for i, item := range legs {
		home, away, market := store.ParseLegEvent(item.RawEvent)
		parsed := store.ParseMarketText(item.MarketRaw)
		var legOdds *float64
		if value := parseDecimal(item.OddsTaken); value != nil {
			legOdds = value
		}
		_, err := tx.Exec(ctx, `
			INSERT INTO personal_bet_legs
				(personal_bet_id, leg_number, competition_raw, event_raw, leg_status,
				 home_raw, away_raw, market_hint, market_raw, odds_taken,
				 market_family, market_side, market_line)
			VALUES ($1, $2, $3, $4, $5, NULLIF($6, ''), NULLIF($7, ''),
			        NULLIF($8, ''), NULLIF($9, ''), $10, NULLIF($11, ''),
			        NULLIF($12, ''), $13)`,
			betID, i+1, item.Competition, item.RawEvent, normalizeLegStatus(item.Status),
			home, away, market, item.MarketRaw, legOdds,
			parsed.Family, parsed.Side, parsed.Line)
		if err != nil {
			return fmt.Errorf("insert leg %d: %w", i+1, err)
		}
	}
	return nil
}

func parseDecimal(raw string) *float64 {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	value, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return nil
	}
	return &value
}

// parsePlacedAt uses the account's local timezone. 1xBet Nigeria reports
// wall-clock time with no offset, so WAT is the documented assumption.
func parsePlacedAt(raw string) (*time.Time, *string) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	layout := "02/01/2006 / 15:04"
	location, err := time.LoadLocation("Africa/Lagos")
	if err != nil {
		location = time.FixedZone("WAT", 60*60)
	}
	parsed, err := time.ParseInLocation(layout, raw, location)
	if err != nil {
		return nil, &raw
	}
	return &parsed, &raw
}

func normalizeStatus(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "win", "won":
		return "won"
	case "loss", "lost", "lose":
		return "lost"
	case "void", "return", "returned":
		return "void"
	case "pending", "open":
		return "pending"
	default:
		return "unknown"
	}
}

func normalizeLegStatus(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "win", "won":
		return "won"
	case "loss", "lost":
		return "lost"
	case "void", "return":
		return "void"
	case "pending":
		return "pending"
	default:
		return "unknown"
	}
}

func outcome(status string, stake, potential *float64) (*float64, *float64) {
	if stake == nil {
		return nil, nil
	}
	switch status {
	case "won":
		if potential == nil {
			return nil, nil
		}
		profit := *potential - *stake
		return potential, &profit
	case "lost":
		zero := 0.0
		loss := -*stake
		return &zero, &loss
	case "void":
		profit := 0.0
		return stake, &profit
	default:
		return nil, nil
	}
}

func sameJSON(left, right []byte) bool {
	var a, b any
	if json.Unmarshal(left, &a) != nil || json.Unmarshal(right, &b) != nil {
		return string(left) == string(right)
	}
	leftCanonical, _ := json.Marshal(a)
	rightCanonical, _ := json.Marshal(b)
	return string(leftCanonical) == string(rightCanonical)
}
