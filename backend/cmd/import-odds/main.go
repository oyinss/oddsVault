// Command import-odds stores a browser-captured bookmaker snapshot.
//
// Some bookmakers block server-side reads (or render prices client-side). For
// those, a capture from the logged-in page is the evidence. This command takes
// that JSON, links each event to a canonical fixture and appends the prices.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/oyins/oddsvault/backend/internal/config"
	"github.com/oyins/oddsvault/backend/internal/database"
	"github.com/oyins/oddsvault/backend/internal/logging"
	"github.com/oyins/oddsvault/backend/internal/providers"
	"github.com/oyins/oddsvault/backend/internal/store"
)

type captureFile struct {
	Bookmaker   string    `json:"bookmaker"`
	Competition string    `json:"competition"`
	Source      string    `json:"source"`
	CapturedAt  string    `json:"captured_at"`
	Events      []capture `json:"events"`
}

type capture struct {
	Home      string  `json:"home"`
	Away      string  `json:"away"`
	StartDate string  `json:"start_date"`
	Quotes    []quote `json:"quotes"`
}

type quote struct {
	Market    string   `json:"market"`
	Line      *float64 `json:"line"`
	Selection string   `json:"selection"`
	Odds      float64  `json:"odds"`
}

func main() {
	file := flag.String("file", "", "path to the capture JSON")
	flag.Parse()
	if *file == "" {
		fmt.Fprintln(os.Stderr, "import-odds: -file is required")
		os.Exit(2)
	}
	if err := run(*file); err != nil {
		fmt.Fprintf(os.Stderr, "import-odds: %v\n", err)
		os.Exit(1)
	}
}

func run(path string) error {
	cfg, err := config.Load("oddsvault-import-odds")
	if err != nil {
		return err
	}
	logger := logging.New(cfg.Env, cfg.LogLevel)
	slog.SetDefault(logger)

	raw, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}
	var doc captureFile
	if err := json.Unmarshal(raw, &doc); err != nil {
		return fmt.Errorf("decode %s: %w", path, err)
	}
	if doc.Bookmaker == "" || doc.Competition == "" {
		return fmt.Errorf("capture must name a bookmaker and competition")
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

	events := make([]store.BookmakerOddsEvent, 0, len(doc.Events))
	for _, item := range doc.Events {
		start, err := time.Parse("2006-01-02", item.StartDate)
		if err != nil {
			return fmt.Errorf("event %s v %s: invalid start_date %q", item.Home, item.Away, item.StartDate)
		}
		quotes := make([]providers.OddsQuote, 0, len(item.Quotes))
		for _, q := range item.Quotes {
			quotes = append(quotes, providers.OddsQuote{
				Bookmaker:  doc.Bookmaker,
				MarketType: q.Market,
				Line:       q.Line,
				Selection:  q.Selection,
				Decimal:    q.Odds,
			})
		}
		events = append(events, store.BookmakerOddsEvent{
			ProviderEventID: fmt.Sprintf("%s:%s:%s:%s",
				doc.Bookmaker, item.StartDate, item.Home, item.Away),
			Name:      item.Home + " - " + item.Away,
			StartDate: start,
			Home:      item.Home,
			Away:      item.Away,
			Quotes:    quotes,
		})
	}

	importer := store.NewImporter(pool, logger)
	stats, err := importer.ImportBookmakerOdds(ctx, doc.Bookmaker, doc.Competition, doc.Source, events)
	if err != nil {
		return err
	}
	logger.Info("capture imported",
		slog.String("bookmaker", doc.Bookmaker),
		slog.Int("events", stats.Received),
		slog.Int("matched", stats.Matched),
		slog.Int("unresolved", stats.Unresolved),
		slog.Int("offers", stats.Offers))
	fmt.Printf("imported %d events: %d matched, %d unresolved, %d prices\n",
		stats.Received, stats.Matched, stats.Unresolved, stats.Offers)
	return nil
}
