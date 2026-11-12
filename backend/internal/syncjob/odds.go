package syncjob

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/oyins/oddsvault/backend/internal/bookmakers"
	"github.com/oyins/oddsvault/backend/internal/providers"
	"github.com/oyins/oddsvault/backend/internal/providers/livescorebet"
	"github.com/oyins/oddsvault/backend/internal/store"
)

// Odds captures live bookmaker prices for the configured feeds.
type Odds struct {
	http     *providers.Client
	importer *store.Importer
	log      *slog.Logger
}

// NewOdds builds the bookmaker capture job.
func NewOdds(client *providers.Client, importer *store.Importer, log *slog.Logger) *Odds {
	if log == nil {
		log = slog.Default()
	}
	return &Odds{http: client, importer: importer, log: log}
}

// Run captures every configured feed. One feed failing does not stop the rest.
func (o *Odds) Run(ctx context.Context) error {
	var errs []error
	for _, feed := range bookmakers.Feeds {
		if err := o.capture(ctx, feed); err != nil {
			errs = append(errs, err)
			o.log.Error("bookmaker capture failed",
				slog.String("bookmaker", feed.BookmakerSlug),
				slog.String("competition", feed.CompetitionSlug),
				slog.String("error", err.Error()))
		}
	}
	return errors.Join(errs...)
}

func (o *Odds) capture(ctx context.Context, feed bookmakers.Feed) error {
	runID, err := o.importer.StartRun(ctx, feed.BookmakerSlug, "capture_bookmaker_odds")
	if err != nil {
		return err
	}

	stats, captureErr := o.captureOnce(ctx, feed)

	if finishErr := o.importer.FinishRun(ctx, runID, store.ImportStats{
		Received: stats.Received,
		Inserted: stats.Matched,
		Skipped:  stats.Unresolved,
	}, captureErr); finishErr != nil {
		o.log.Error("failed to record capture run", slog.String("error", finishErr.Error()))
	}
	if captureErr != nil {
		return fmt.Errorf("%s %s: %w", feed.BookmakerSlug, feed.CompetitionSlug, captureErr)
	}

	o.log.Info("bookmaker odds captured",
		slog.String("bookmaker", feed.BookmakerSlug),
		slog.String("competition", feed.CompetitionSlug),
		slog.Int("events", stats.Received),
		slog.Int("matched", stats.Matched),
		slog.Int("unresolved", stats.Unresolved),
		slog.Int("offers", stats.Offers))
	return nil
}

func (o *Odds) captureOnce(ctx context.Context, feed bookmakers.Feed) (store.BookmakerOddsStats, error) {
	adapter := livescorebet.New(o.http)
	raw, err := adapter.FetchCompetitionEvents(ctx, feed.CategoryID)
	if err != nil {
		return store.BookmakerOddsStats{}, err
	}

	events := make([]store.BookmakerOddsEvent, 0, len(raw))
	for _, item := range raw {
		quotes := livescorebet.Quotes(item)
		if len(quotes) == 0 {
			continue
		}
		events = append(events, store.BookmakerOddsEvent{
			ProviderEventID: item.ProviderEventID,
			Name:            item.Name,
			StartTimeLocal:  item.StartTimeLocal,
			StartDate:       parseProviderTime(item.StartTimeLocal),
			State:           item.State,
			Home:            item.Home,
			Away:            item.Away,
			Quotes:          quotes,
		})
	}
	if len(events) == 0 {
		return store.BookmakerOddsStats{Received: len(raw)}, nil
	}

	return o.importer.ImportBookmakerOdds(ctx, feed.BookmakerSlug, feed.CompetitionSlug,
		"livescorebet/1", events)
}

// parseProviderTime reads the provider's timezone-less wall clock. The date is
// used for identity matching only; no UTC is inferred.
func parseProviderTime(raw string) time.Time {
	for _, layout := range []string{"2006-01-02 15:04:05", "2006-01-02 15:04"} {
		if parsed, err := time.Parse(layout, raw); err == nil {
			return parsed
		}
	}
	return time.Time{}
}
