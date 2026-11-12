package store

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/oyins/oddsvault/backend/internal/providers"
)

// oddsWriter lazily creates one snapshot batch per import and writes prices.
type oddsWriter struct {
	tx     pgx.Tx
	batch  int64
	at     time.Time
	offers int
	kind   string

	bookmakers   map[string]int64
	marketTypes  map[string]int64
	marketCache  map[string]int64
	selectionKey map[string]int64
}

func newOddsWriter(tx pgx.Tx, kind string) *oddsWriter {
	return &oddsWriter{
		tx:           tx,
		kind:         kind,
		at:           time.Now().UTC(),
		bookmakers:   make(map[string]int64),
		marketTypes:  make(map[string]int64),
		marketCache:  make(map[string]int64),
		selectionKey: make(map[string]int64),
	}
}

func (w *oddsWriter) write(ctx context.Context, provider, providerEventID string, eventID int64, quotes []providers.OddsQuote) error {
	if len(quotes) == 0 {
		return nil
	}

	bookmakerID, err := w.bookmaker(ctx, quotes[0].Bookmaker)
	if err != nil {
		return err
	}
	if w.batch == 0 {
		if err := w.tx.QueryRow(ctx, `
			INSERT INTO snapshot_batches (bookmaker_id, status, source_kind, collector_version)
			VALUES ($1, 'running', $3, $2)
			RETURNING id`, bookmakerID, provider+"/"+w.kind, w.kind).Scan(&w.batch); err != nil {
			return fmt.Errorf("create snapshot batch: %w", err)
		}
	}

	// A closing price is one published fact per selection, not a stream:
	// re-importing the same season replaces the previous values for the event.
	// Live observations are append-only, so nothing is deleted.
	if w.kind == "closing" {
		if _, err := w.tx.Exec(ctx, `
			DELETE FROM odds_snapshots
			WHERE event_id = $1 AND bookmaker_id = $2 AND price_kind = 'closing'`,
			eventID, bookmakerID); err != nil {
			return fmt.Errorf("clear previous closing odds: %w", err)
		}
	}

	for _, quote := range quotes {
		marketTypeID, err := w.marketType(ctx, quote.MarketType)
		if err != nil {
			return err
		}
		marketID, err := w.market(ctx, eventID, marketTypeID, quote.Line)
		if err != nil {
			return err
		}
		selectionID, err := w.selection(ctx, marketID, quote.Selection)
		if err != nil {
			return err
		}

		hashPayload := fmt.Sprintf("%s|%s|%s|%s|%.3f", provider, providerEventID, quote.MarketType, quote.Selection, quote.Decimal)
		sum := sha256.Sum256([]byte(hashPayload))

		if _, err := w.tx.Exec(ctx, `
			INSERT INTO odds_snapshots
				(captured_at, snapshot_batch_id, bookmaker_id, event_id, market_id,
				 selection_id, decimal_odds, availability_status, price_kind, source_hash)
			VALUES ($1, $2, $3, $4, $5, $6, $7, 'available', $9, $8)`,
			w.at, w.batch, bookmakerID, eventID, marketID, selectionID, quote.Decimal,
			hex.EncodeToString(sum[:]), w.kind); err != nil {
			return fmt.Errorf("insert odds snapshot: %w", err)
		}
		w.offers++
	}
	return nil
}

func (w *oddsWriter) finish(ctx context.Context) error {
	if w.batch == 0 {
		return nil
	}
	var events int
	if err := w.tx.QueryRow(ctx, `
		SELECT count(DISTINCT event_id) FROM odds_snapshots WHERE snapshot_batch_id = $1`,
		w.batch).Scan(&events); err != nil {
		return fmt.Errorf("count batch events: %w", err)
	}
	_, err := w.tx.Exec(ctx, `
		UPDATE snapshot_batches
		SET completed_at = now(), status = 'success',
		    event_count = $2, market_count = $3, offer_count = $4
		WHERE id = $1`, w.batch, events, 0, w.offers)
	if err != nil {
		return fmt.Errorf("finish snapshot batch: %w", err)
	}
	return nil
}

func (w *oddsWriter) bookmaker(ctx context.Context, slug string) (int64, error) {
	if id, ok := w.bookmakers[slug]; ok {
		return id, nil
	}
	var id int64
	if err := w.tx.QueryRow(ctx, `SELECT id FROM bookmakers WHERE slug = $1`, slug).Scan(&id); err != nil {
		return 0, fmt.Errorf("lookup bookmaker %q: %w", slug, err)
	}
	w.bookmakers[slug] = id
	return id, nil
}

func (w *oddsWriter) marketType(ctx context.Context, slug string) (int64, error) {
	if id, ok := w.marketTypes[slug]; ok {
		return id, nil
	}
	var id int64
	if err := w.tx.QueryRow(ctx, `SELECT id FROM market_types WHERE slug = $1`, slug).Scan(&id); err != nil {
		return 0, fmt.Errorf("lookup market type %q: %w", slug, err)
	}
	w.marketTypes[slug] = id
	return id, nil
}

func (w *oddsWriter) market(ctx context.Context, eventID, marketTypeID int64, line *float64) (int64, error) {
	key := fmt.Sprintf("%d:%d:%s", eventID, marketTypeID, lineKey(line))
	if id, ok := w.marketCache[key]; ok {
		return id, nil
	}

	var id int64
	err := w.tx.QueryRow(ctx, `
		SELECT id FROM markets
		WHERE event_id = $1 AND market_type_id = $2 AND period = 'full_game'
		  AND line IS NOT DISTINCT FROM $3::numeric AND subject = 'match'`,
		eventID, marketTypeID, line).Scan(&id)
	if err == pgx.ErrNoRows {
		if err := w.tx.QueryRow(ctx, `
			INSERT INTO markets (event_id, market_type_id, period, line, subject)
			VALUES ($1, $2, 'full_game', $3, 'match')
			RETURNING id`, eventID, marketTypeID, line).Scan(&id); err != nil {
			return 0, fmt.Errorf("create market: %w", err)
		}
	} else if err != nil {
		return 0, fmt.Errorf("lookup market: %w", err)
	}
	w.marketCache[key] = id
	return id, nil
}

func lineKey(line *float64) string {
	if line == nil {
		return "null"
	}
	return strconv.FormatFloat(*line, 'f', 2, 64)
}

func (w *oddsWriter) selection(ctx context.Context, marketID int64, selection string) (int64, error) {
	key := fmt.Sprintf("%d:%s", marketID, selection)
	if id, ok := w.selectionKey[key]; ok {
		return id, nil
	}
	var id int64
	err := w.tx.QueryRow(ctx, `
		SELECT id FROM market_selections WHERE market_id = $1 AND selection = $2`,
		marketID, selection).Scan(&id)
	if err == pgx.ErrNoRows {
		if err := w.tx.QueryRow(ctx, `
			INSERT INTO market_selections (market_id, selection)
			VALUES ($1, $2) RETURNING id`, marketID, selection).Scan(&id); err != nil {
			return 0, fmt.Errorf("create selection: %w", err)
		}
	} else if err != nil {
		return 0, fmt.Errorf("lookup selection: %w", err)
	}
	w.selectionKey[key] = id
	return id, nil
}
