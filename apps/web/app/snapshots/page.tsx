"use client";

import Link from "next/link";
import { useSearchParams } from "next/navigation";
import { Suspense } from "react";

import { LoadingNote } from "@/components/loading-note";
import { SiteHeader } from "@/components/site-header";
import {
  formatAge,
  formatDate,
  getSnapshots,
  type SnapshotRow,
} from "@/lib/api";
import { useApiData } from "@/lib/use-api";

const KINDS = [
  { key: "", label: "All" },
  { key: "live", label: "Live" },
  { key: "closing", label: "Closing" },
];

const MARKETS = [
  { key: "", label: "All markets" },
  { key: "match_result", label: "1X2" },
  { key: "total_goals", label: "Total goals" },
  { key: "both_teams_to_score", label: "BTTS" },
  { key: "double_chance", label: "Double chance" },
];

const BOOKS = [
  { key: "", label: "All books" },
  { key: "livescorebet", label: "LiveScoreBet" },
  { key: "1xbet", label: "1xBet" },
  { key: "bet365", label: "Bet365" },
];

const MARKET_LABELS: Record<string, string> = {
  match_result: "1X2",
  total_goals: "Total goals",
  moneyline: "Moneyline",
  total_corners: "Total corners",
  corners_handicap: "Corners handicap",
  both_teams_to_score: "BTTS",
  double_chance: "Double chance",
  draw_no_bet: "Draw no bet",
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

export default function SnapshotsPage() {
  return (
    <Suspense
      fallback={
        <>
          <SiteHeader />
          <main className="mx-auto max-w-6xl px-6 py-12">
            <LoadingNote label="Loading snapshots…" />
          </main>
        </>
      }
    >
      <SnapshotsView />
    </Suspense>
  );
}

function SnapshotsView() {
  const params = useSearchParams();
  const requestedKind = params.get("kind") ?? "";
  const requestedMarket = params.get("market") ?? "";
  const requestedBook = params.get("bookmaker") ?? "";
  const kind = KINDS.some((k) => k.key === requestedKind) ? requestedKind : "";
  const market = MARKETS.some((m) => m.key === requestedMarket)
    ? requestedMarket
    : "";
  const bookmaker = BOOKS.some((b) => b.key === requestedBook)
    ? requestedBook
    : "";

  const { data, loading } = useApiData(
    `snapshots:${kind}:${market}:${bookmaker}`,
    () => getSnapshots({ kind, market, bookmaker, limit: 150 }),
  );
  const rows: SnapshotRow[] = data ?? [];

  const link = (next: { kind?: string; market?: string; bookmaker?: string }) => {
    const query = new URLSearchParams();
    const merged = { kind, market, bookmaker, ...next };
    if (merged.kind) query.set("kind", merged.kind);
    if (merged.market) query.set("market", merged.market);
    if (merged.bookmaker) query.set("bookmaker", merged.bookmaker);
    const suffix = query.toString();
    return suffix ? `/snapshots?${suffix}` : "/snapshots";
  };

  return (
    <>
      <SiteHeader />
      <main className="mx-auto max-w-6xl px-6 py-12">
        <div>
          <p className="font-mono text-[11px] tracking-widest text-turf-300 uppercase">
            Phase 4 · Snapshot history
          </p>
          <h1 className="mt-2 text-3xl font-bold tracking-tight">
            Every observation
          </h1>
          <p className="mt-2 max-w-2xl text-sm leading-relaxed text-mist-400">
            Raw captured prices, newest first. Nothing here is aggregated or
            overwritten - opening/current views are derived from these rows.
          </p>
        </div>

        {loading && <LoadingNote label="Loading snapshots…" />}

        <div className="mt-6 flex flex-wrap items-center gap-2">
          {KINDS.map((item) => (
            <Filter
              key={item.key || "all"}
              href={link({ kind: item.key })}
              active={kind === item.key}
              label={item.label}
            />
          ))}
          <span className="mx-1 h-4 w-px bg-raised" />
          {MARKETS.map((item) => (
            <Filter
              key={item.key || "all-markets"}
              href={link({ market: item.key })}
              active={market === item.key}
              label={item.label}
            />
          ))}
        </div>

        <div className="mt-3 flex flex-wrap items-center gap-2">
          {BOOKS.map((item) => (
            <Filter
              key={item.key || "all-books"}
              href={link({ bookmaker: item.key })}
              active={bookmaker === item.key}
              label={item.label}
            />
          ))}
          <span className="ml-auto font-mono text-[11px] text-mist-500">
            {rows.length} rows
          </span>
        </div>

        {loading ? null : rows.length === 0 ? (
          <p className="mt-8 rounded-xl border border-line bg-ink-900/50 p-5 text-sm text-mist-400">
            No snapshots match these filters.
          </p>
        ) : (
          <div className="mt-6 overflow-hidden rounded-2xl border border-line bg-ink-900/40">
            <table className="w-full text-left text-sm">
              <thead className="border-b border-line font-mono text-[10px] tracking-widest text-mist-500 uppercase">
                <tr>
                  <th className="px-4 py-3">Captured</th>
                  <th className="px-4 py-3">Fixture</th>
                  <th className="px-4 py-3">Market</th>
                  <th className="px-4 py-3 text-right">Odds</th>
                  <th className="px-4 py-3">Book</th>
                  <th className="px-4 py-3">Settled</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-line">
                {rows.map((row, index) => (
                  <tr key={`${row.event_id}-${row.market}-${row.selection}-${row.bookmaker}-${index}`} className="hover:bg-hover">
                    <td className="px-4 py-2.5 font-mono text-[11px] text-mist-400">
                      {formatAge(row.captured_at)}
                    </td>
                    <td className="px-4 py-2.5">
                      <Link
                        href={`/events/detail?id=${row.event_id}`}
                        className="text-mist-100 underline decoration-line-strong underline-offset-4 hover:text-turf-300"
                      >
                        {row.home} v {row.away}
                      </Link>
                      <p className="font-mono text-[10px] text-mist-500">
                        {row.competition_name}
                        {row.start_date ? ` · ${formatDate(row.start_date)}` : ""}
                      </p>
                    </td>
                    <td className="px-4 py-2.5">
                      <p className="text-xs text-mist-200">
                        {MARKET_LABELS[row.market] ?? row.market}
                        {row.line !== null ? ` ${row.line}` : ""}
                        {row.market === "total_goals" ? " (match goals)" : ""}
                      </p>
                      <p className="font-mono text-[10px] text-mist-500">
                        {SELECTION_LABELS[row.selection] ?? row.selection}
                      </p>
                    </td>
                    <td className="px-4 py-2.5 text-right font-mono text-sm text-mist-100">
                      {row.decimal_odds.toFixed(3)}
                    </td>
                    <td className="px-4 py-2.5">
                      <p className="text-xs text-mist-300">{row.bookmaker_name}</p>
                      <p className="font-mono text-[10px] text-mist-500">
                        {row.price_kind}
                      </p>
                    </td>
                    <td className="px-4 py-2.5">
                      <SettlementChip status={row.settlement_status} />
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}

        <p className="mt-6 max-w-3xl text-[11px] leading-relaxed text-mist-500">
          “Total goals” always means goals scored in the match (for example
          Over/Under 3.5 goals). Corner totals, cards and player props are not
          ingested yet, so they never appear in this table.
        </p>
      </main>
    </>
  );
}

function Filter({
  href,
  active,
  label,
}: {
  href: string;
  active: boolean;
  label: string;
}) {
  return (
    <Link
      href={href}
      className={`rounded-lg border px-3 py-1.5 text-xs transition ${
        active
          ? "border-turf-400/30 bg-turf-500/10 text-turf-300"
          : "border-line-strong text-mist-300 hover:border-line-strong hover:bg-raised"
      }`}
    >
      {label}
    </Link>
  );
}

function SettlementChip({ status }: { status: string | null }) {
  if (!status) {
    return <span className="font-mono text-[10px] text-mist-600">-</span>;
  }
  const tone =
    status === "hit"
      ? "border-turf-400/30 text-turf-300"
      : status === "push"
        ? "border-flag-400/25 text-flag-400"
        : "border-line-strong text-mist-400";
  return (
    <span className={`rounded-full border px-2 py-0.5 font-mono text-[9px] uppercase ${tone}`}>
      {status}
    </span>
  );
}
