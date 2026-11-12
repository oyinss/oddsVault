"use client";

import Link from "next/link";
import { useSearchParams } from "next/navigation";
import { Suspense } from "react";

import { LoadingNote } from "@/components/loading-note";
import { SiteHeader } from "@/components/site-header";
import { TeamCrest } from "@/components/team-crest";
import {
  absoluteAsset,
  formatAge,
  getEvent,
  getEventMovement,
  getEventPrices,
  kickoff,
  type EventDetail,
  type EventPrices,
  type MarketView,
  type PricedMarket,
  type MovementMarket,
  type MovementPoint,
  type MovementSelection,
  type SelectionView,
} from "@/lib/api";
import { useApiData } from "@/lib/use-api";

const MARKET_LABELS: Record<string, string> = {
  match_result: "Match result",
  double_chance: "Double chance",
  draw_no_bet: "Draw no bet",
  total_goals: "Total goals",
  both_teams_to_score: "Both teams to score",
};

const SELECTION_LABELS: Record<string, string> = {
  home: "Home",
  draw: "Draw",
  away: "Away",
  over: "Over",
  under: "Under",
  yes: "Yes",
  no: "No",
  home_or_draw: "Home or draw",
  home_or_away: "Home or away",
  draw_or_away: "Draw or away",
};

export default function EventPage() {
  return (
    <Suspense
      fallback={
        <>
          <SiteHeader />
          <main className="mx-auto max-w-5xl px-6 py-10">
            <LoadingNote label="Loading fixture…" />
          </main>
        </>
      }
    >
      <EventView />
    </Suspense>
  );
}

function EventView() {
  const searchParams = useSearchParams();
  const id = searchParams.get("id") ?? "";

  const { data, loading } = useApiData(`event:${id}`, () => {
    if (!id) {
      return Promise.resolve({
        detail: null as EventDetail | null,
        movement: [] as MovementMarket[],
        prices: null as EventPrices | null,
      });
    }
    return Promise.all([
      getEvent(id),
      getEventMovement(id),
      getEventPrices(id),
    ]).then(([detail, movement, prices]) => ({ detail, movement, prices }));
  });

  if (!id) {
    return <FixtureMissing message="No fixture was selected." />;
  }

  if (loading && !data) {
    return (
      <>
        <SiteHeader />
        <main className="mx-auto max-w-5xl px-6 py-10">
          <LoadingNote label="Loading fixture…" />
        </main>
      </>
    );
  }

  const detail = data?.detail ?? null;
  const movement = data?.movement ?? [];
  const prices = data?.prices ?? null;

  if (!detail) {
    return (
      <FixtureMissing message="This fixture is not stored in the database." />
    );
  }

  return (
    <>
      <SiteHeader />
      <main className="mx-auto max-w-5xl px-6 py-10">
        <Link
          href="/events"
          className="font-mono text-[11px] text-mist-500 transition hover:text-mist-200"
        >
          ← fixtures
        </Link>

        <EventHeader detail={detail} />

        {prices && <PricesSection prices={prices} />}

        <section className="mt-10">
          <h2 className="text-sm font-semibold tracking-tight">Markets</h2>
          <p className="mt-1 text-xs text-mist-500">
            Closing prices published by football-data.co.uk, imported once per
            match. Settlement uses verified final scores only.
          </p>

          {detail.markets.length === 0 ? (
            <p className="mt-6 rounded-xl border border-line bg-ink-900/50 p-5 text-sm text-mist-400">
              No markets stored for this event yet.
            </p>
          ) : (
            <div className="mt-5 space-y-3">
              {detail.markets.map((market) => (
                <MarketCard key={market.id} market={market} />
              ))}
            </div>
          )}
        </section>

        <MovementSection markets={movement} />

        <section className="mt-10">
          <h2 className="text-sm font-semibold tracking-tight">
            Identity &amp; sources
          </h2>
          <ul className="mt-4 divide-y divide-line overflow-hidden rounded-xl border border-line bg-ink-900/50">
            {detail.provider_links.map((link) => (
              <li
                key={`${link.provider}-${link.provider_event_id}`}
                className="flex flex-col gap-1 px-4 py-3 sm:flex-row sm:items-center sm:justify-between"
              >
                <span className="text-sm text-mist-200">{link.provider}</span>
                <span className="truncate font-mono text-[11px] text-mist-500">
                  {link.provider_event_id} · {link.match_method ?? "unknown"}
                </span>
              </li>
            ))}
          </ul>
        </section>
      </main>
    </>
  );
}

function FixtureMissing({ message }: { message: string }) {
  return (
    <>
      <SiteHeader />
      <main className="mx-auto max-w-5xl px-6 py-10">
        <Link
          href="/events"
          className="font-mono text-[11px] text-mist-500 transition hover:text-mist-200"
        >
          ← fixtures
        </Link>
        <div className="panel mt-6 border-flag-400/30 p-6">
          <p className="text-sm font-medium text-flag-400">Fixture unavailable</p>
          <p className="mt-1 text-xs text-mist-400">{message}</p>
        </div>
      </main>
    </>
  );
}

function EventHeader({ detail }: { detail: EventDetail }) {
  const { event } = detail;
  const when = kickoff(event);
  const score = event.result
    ? `${event.result.home_score ?? "—"} – ${event.result.away_score ?? "—"}`
    : null;

  return (
    <div className="mt-6 rounded-2xl border border-line bg-ink-900/60 p-6">
      <p className="font-mono text-[10px] tracking-widest text-mist-500 uppercase">
        {event.competition_name}
        {event.round ? ` · ${event.round}` : ""}
        {event.season ? ` · ${event.season}` : ""}
      </p>

      <div className="mt-5 grid items-center gap-6 sm:grid-cols-[1fr_auto_1fr]">
        <div className="flex items-center gap-3 sm:justify-end">
          <TeamCrest src={absoluteAsset(event.home.crest_url)} name={event.home.name} size={40} />
          <p className="text-lg font-semibold">{event.home.name}</p>
        </div>
        <div className="text-center">
          <p className="font-mono text-3xl font-semibold tracking-tight">
            {score ?? "v"}
          </p>
          <StatusBadge event={event} />
        </div>
        <div className="flex items-center gap-3">
          <TeamCrest src={absoluteAsset(event.away.crest_url)} name={event.away.name} size={40} />
          <p className="text-lg font-semibold">{event.away.name}</p>
        </div>
      </div>

      <dl className="mt-6 grid gap-3 border-t border-line pt-5 text-xs sm:grid-cols-3">
        <div>
          <dt className="font-mono text-[10px] tracking-widest text-mist-500 uppercase">
            Date
          </dt>
          <dd className="mt-1 text-mist-200">{when.date}</dd>
        </div>
        <div>
          <dt className="font-mono text-[10px] tracking-widest text-mist-500 uppercase">
            Kickoff
          </dt>
          <dd className="mt-1 text-mist-200" title={when.title}>
            {when.time}
            {when.zone ? (
              <span className="ml-1 font-mono text-[10px] text-mist-500">
                {when.zone}
              </span>
            ) : (
              <span className="ml-1 font-mono text-[10px] text-mist-500">
                local
              </span>
            )}
            {when.venueTime && when.zone ? (
              <span className="mt-0.5 block font-mono text-[10px] text-mist-500">
                venue {when.venueTime}
              </span>
            ) : null}
          </dd>
        </div>
        <div>
          <dt className="font-mono text-[10px] tracking-widest text-mist-500 uppercase">
            Half-time
          </dt>
          <dd className="mt-1 text-mist-200">
            {event.result?.home_score_ht !== null &&
            event.result?.home_score_ht !== undefined
              ? `${event.result.home_score_ht} – ${event.result.away_score_ht}`
              : "—"}
          </dd>
        </div>
      </dl>

      {event.live && (
        <p
          className={`mt-4 rounded-lg border px-3 py-2 text-xs ${
            event.live.status === "live"
              ? "border-flag-400/25 bg-flag-400/[0.06] text-flag-400"
              : "border-line bg-raised text-mist-400"
          }`}
        >
          {event.live.status === "live" ? (
            <>
              Live from {event.live.provider}:{" "}
              {event.live.home_score ?? "—"}–{event.live.away_score ?? "—"}
              {event.live.detail ? ` · ${event.live.detail}` : ""} ·{" "}
              {formatAge(event.live.captured_at)}
            </>
          ) : (
            <>
              Last provider check: {event.live.provider} ·{" "}
              {formatAge(event.live.captured_at)}
            </>
          )}
        </p>
      )}
    </div>
  );
}

function StatusBadge({ event }: { event: EventDetail["event"] }) {
  return (
    <p className="mt-1 font-mono text-[10px] tracking-widest text-mist-500 uppercase">
      {event.status}
      {event.identity_status !== "confirmed" ? " · provisional" : ""}
    </p>
  );
}

function MarketCard({ market }: { market: MarketView }) {
  return (
    <div className="rounded-xl border border-line bg-ink-900/50 p-4">
      <div className="flex items-baseline justify-between gap-4">
        <h3 className="text-sm font-medium">
          {MARKET_LABELS[market.type] ?? market.type}
        </h3>
        {market.line !== null && (
          <span className="font-mono text-[11px] text-mist-500">
            line {market.line}
          </span>
        )}
      </div>

      <div className="mt-3 divide-y divide-line">
        {market.selections.map((selection) => (
          <SelectionRow key={selection.id} selection={selection} />
        ))}
      </div>
    </div>
  );
}

function SelectionRow({ selection }: { selection: SelectionView }) {
  return (
    <div className="flex items-center justify-between gap-4 py-2.5">
      <div className="min-w-0">
        <p className="text-sm text-mist-200">
          {SELECTION_LABELS[selection.selection] ?? selection.selection}
        </p>
        {selection.odds_captured_at && (
          <p className="font-mono text-[10px] text-mist-500">
            {selection.bookmaker ?? "unknown"} · {selection.price_kind ?? "price"} ·{" "}
            {formatAge(selection.odds_captured_at)}
          </p>
        )}
      </div>

      <div className="flex items-center gap-4">
        <span className="font-mono text-sm text-mist-100">
          {selection.decimal_odds ?? "—"}
        </span>
        <SettlementChip status={selection.settlement_status} />
      </div>
    </div>
  );
}

function SettlementChip({ status }: { status: string | null }) {
  if (!status) {
    return (
      <span className="w-14 text-right font-mono text-[10px] text-mist-500">
        pending
      </span>
    );
  }
  const styles: Record<string, string> = {
    hit: "border-turf-400/30 bg-turf-500/10 text-turf-300",
    miss: "border-line-strong bg-hover text-mist-400",
    push: "border-flag-400/25 bg-flag-400/[0.06] text-flag-400",
  };
  return (
    <span
      className={`w-14 rounded-full border px-2 py-0.5 text-center font-mono text-[10px] uppercase ${
        styles[status] ?? "border-line-strong text-mist-400"
      }`}
    >
      {status}
    </span>
  );
}


function MovementSection({ markets }: { markets: MovementMarket[] }) {
  return (
    <section className="mt-10">
      <h2 className="text-sm font-semibold tracking-tight">Price movement</h2>
      <p className="mt-1 text-xs text-mist-500">
        Every captured observation, in order. Opening is the first stored price,
        current the latest — never overwritten.
      </p>

      {markets.length === 0 ? (
        <p className="mt-5 rounded-xl border border-line bg-ink-900/50 p-5 text-sm text-mist-400">
          No price history for this event yet.
        </p>
      ) : (
        <div className="mt-5 space-y-3">
          {markets.map((market) => (
            <div
              key={`${market.type}-${market.line ?? "none"}`}
              className="rounded-xl border border-line bg-ink-900/50 p-4"
            >
              <div className="flex items-baseline justify-between gap-4">
                <h3 className="text-sm font-medium">
                  {MARKET_LABELS[market.type] ?? market.type}
                </h3>
                {market.line !== null && (
                  <span className="font-mono text-[11px] text-mist-500">
                    line {market.line}
                  </span>
                )}
              </div>
              <div className="mt-2 divide-y divide-line">
                {market.selections.map((selection) => (
                  <MovementRow key={selection.selection} selection={selection} />
                ))}
              </div>
            </div>
          ))}
        </div>
      )}
    </section>
  );
}

function MovementRow({ selection }: { selection: MovementSelection }) {
  const delta = selection.opening
    ? ((selection.current - selection.opening) / selection.opening) * 100
    : 0;
  const shortened = selection.current < selection.opening;

  return (
    <div className="flex items-center justify-between gap-4 py-2.5">
      <div className="min-w-0 flex-1">
        <p className="text-sm text-mist-200">
          {SELECTION_LABELS[selection.selection] ?? selection.selection}
        </p>
        <p className="font-mono text-[10px] text-mist-500">
          {selection.samples} observations · high {selection.high.toFixed(2)} · low{" "}
          {selection.low.toFixed(2)}
        </p>
      </div>

      <Sparkline series={selection.series} />

      <div className="w-32 text-right">
        <p className="font-mono text-sm text-mist-100">
          {selection.opening.toFixed(2)} → {selection.current.toFixed(2)}
        </p>
        <p
          className={`font-mono text-[10px] ${
            Math.abs(delta) < 0.01
              ? "text-mist-500"
              : shortened
                ? "text-turf-300"
                : "text-flag-400"
          }`}
        >
          {delta > 0 ? "+" : ""}
          {delta.toFixed(1)}%
        </p>
      </div>
    </div>
  );
}

function Sparkline({ series }: { series: MovementPoint[] }) {
  if (series.length < 2) {
    return <span className="w-24 font-mono text-[10px] text-mist-500">—</span>;
  }
  const values = series.map((point) => point.odds);
  const min = Math.min(...values);
  const max = Math.max(...values);
  const span = max - min || 1;
  const points = values
    .map((value, index) => {
      const x = (index / (values.length - 1)) * 96 + 2;
      const y = 26 - ((value - min) / span) * 22 + 2;
      return `${x.toFixed(1)},${y.toFixed(1)}`;
    })
    .join(" ");

  return (
    <svg viewBox="0 0 100 32" className="h-8 w-24 shrink-0" aria-hidden>
      <polyline
        points={points}
        fill="none"
        stroke="currentColor"
        strokeWidth="1.5"
        className="text-turf-400/80"
      />
    </svg>
  );
}


function PricesSection({ prices }: { prices: EventPrices }) {
  if (prices.markets.length === 0) return null;

  return (
    <section className="mt-10">
      <h2 className="text-sm font-semibold tracking-tight">
        Prices by bookmaker
      </h2>
      <p className="mt-1 text-xs text-mist-500">
        Latest captured price for each selection at every bookmaker. The best
        captured price is highlighted; freshness is shown per price.
      </p>

      <div className="mt-5 space-y-3">
        {prices.markets.map((market) => (
          <MarketPrices key={`${market.type}-${market.line ?? "none"}`} market={market} books={prices.bookmakers} />
        ))}
      </div>
    </section>
  );
}

function MarketPrices({
  market,
  books,
}: {
  market: PricedMarket;
  books: { slug: string; name: string }[];
}) {
  return (
    <div className="overflow-hidden rounded-xl border border-line bg-ink-900/50">
      <div className="flex items-baseline justify-between gap-4 border-b border-line px-4 py-3">
        <h3 className="text-sm font-medium">
          {MARKET_LABELS[market.type] ?? market.type}
        </h3>
        {market.line !== null && (
          <span className="font-mono text-[11px] text-mist-500">
            line {market.line}
          </span>
        )}
      </div>

      <div className="divide-y divide-line">
        {market.selections.map((selection) => {
          const best = Math.max(...selection.prices.map((p) => p.odds), 0);
          return (
            <div key={selection.selection} className="flex items-center gap-4 px-4 py-2.5">
              <p className="w-28 shrink-0 text-sm text-mist-200">
                {SELECTION_LABELS[selection.selection] ?? selection.selection}
              </p>
              <div className="flex flex-1 flex-wrap items-center gap-4">
                {books.map((book) => {
                  const price = selection.prices.find((p) => p.bookmaker === book.slug);
                  if (!price) {
                    return (
                      <span key={book.slug} className="font-mono text-xs text-mist-600">
                        —
                      </span>
                    );
                  }
                  const isBest = price.odds === best && selection.prices.length > 1;
                  return (
                    <span key={book.slug} className="flex items-baseline gap-1.5">
                      <span className="font-mono text-[10px] tracking-wide text-mist-500 uppercase">
                        {book.name}
                      </span>
                      <span
                        className={`font-mono text-sm ${
                          isBest ? "font-semibold text-turf-300" : "text-mist-100"
                        }`}
                      >
                        {price.odds.toFixed(3)}
                      </span>
                    </span>
                  );
                })}
              </div>
            </div>
          );
        })}
      </div>
    </div>
  );
}
