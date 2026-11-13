"use client";

import Link from "next/link";
import { useEffect, useMemo, useState } from "react";

import { LoadingNote } from "@/components/loading-note";
import { SiteHeader } from "@/components/site-header";
import { formatMoney, getMyBets } from "@/lib/api";
import { useApiData } from "@/lib/use-api";

import { BetCard } from "@/features/my-bets/bet-card";
import {
  PerformanceChart,
  Sparkline,
  WinRateGauge,
} from "@/features/my-bets/charts";
import {
  DEFAULT_FILTERS,
  bookmakers as collectBookmakers,
  competitions as collectCompetitions,
  dailySeries,
  dayPerformance,
  filterBets,
  formatDayShort,
  marketRows,
  recentDailyProfit,
  todayLagos,
  totals,
  type DayPerformance,
  type Filters,
  type StatusKey,
  type Totals,
} from "@/features/my-bets/derive";
import { FilterBar } from "@/features/my-bets/filter-bar";
import { Icon, type IconName } from "@/features/my-bets/icons";
import { MarketTable } from "@/features/my-bets/market-table";

const PAGE_SIZE = 8;

const STATUS_TABS: { key: StatusKey; label: string; dot: string }[] = [
  { key: "all", label: "All bets", dot: "bg-mist-400" },
  { key: "won", label: "Won", dot: "bg-turf-400" },
  { key: "lost", label: "Lost", dot: "bg-loss-400" },
  { key: "pending", label: "Pending", dot: "bg-flag-400" },
];

export default function MyBetsPage() {
  const state = useApiData("my-bets", getMyBets);
  const [filters, setFilters] = useState<Filters>(DEFAULT_FILTERS);
  const [visible, setVisible] = useState(PAGE_SIZE);
  const [chartMode, setChartMode] = useState<"pl" | "rate">("pl");

  const today = todayLagos();
  const data = state.data;
  const allBets = data?.ok ? data.bets : [];

  const filtered = useMemo(
    () => filterBets(allBets, filters, today),
    [allBets, filters, today],
  );
  const summary = useMemo(() => totals(filtered), [filtered]);

  // Tab counts ignore the status filter so the numbers do not collapse while a
  // tab is active.
  const counts = useMemo(
    () =>
      totals(
        filterBets(allBets, { ...filters, status: "all" }, today),
      ),
    [allBets, filters, today],
  );

  const day = useMemo(() => dayPerformance(allBets, today), [allBets, today]);
  const series = useMemo(() => dailySeries(filtered), [filtered]);
  const markets = useMemo(() => marketRows(filtered), [filtered]);
  const spark = useMemo(() => recentDailyProfit(allBets, 7), [allBets]);
  const bookmakerOptions = useMemo(() => collectBookmakers(allBets), [allBets]);
  const competitionOptions = useMemo(
    () => collectCompetitions(allBets),
    [allBets],
  );

  const filterKey = JSON.stringify(filters);
  useEffect(() => {
    setVisible(PAGE_SIZE);
  }, [filterKey]);

  const reset = () => setFilters(DEFAULT_FILTERS);
  const patch = (next: Partial<Filters>) =>
    setFilters((current) => ({ ...current, ...next }));
  const dirty = filterKey !== JSON.stringify(DEFAULT_FILTERS);

  if (state.loading && !data) {
    return (
      <>
        <SiteHeader />
        <main className="mx-auto max-w-6xl px-4 py-12 sm:px-6">
          <LoadingNote label="Loading bet journal…" />
        </main>
      </>
    );
  }

  if (!data || !data.ok) {
    return (
      <>
        <SiteHeader />
        <main className="mx-auto max-w-6xl px-4 py-12 sm:px-6">
          <div className="panel border-flag-400/30 p-5">
            <p className="text-sm font-medium text-flag-400">Bets unavailable</p>
            <p className="mt-1 text-xs text-mist-400">
              {data ? data.error : (state.error ?? "unreachable")}
            </p>
          </div>
        </main>
      </>
    );
  }

  return (
    <>
      <SiteHeader />
      <main className="mx-auto max-w-6xl px-4 pt-6 pb-16 sm:px-6">
        <section className="relative overflow-hidden rounded-2xl border border-line">
          <div aria-hidden className="pointer-events-none absolute inset-0">
            <div className="hero-glow absolute inset-0" />
            <div className="pitch-lines absolute inset-0 opacity-60" />
          </div>

          <div className="relative grid gap-6 px-5 py-6 sm:px-7 sm:py-8 lg:grid-cols-[minmax(0,1fr)_340px] lg:items-center">
            <div>
              <p className="font-mono text-[10px] tracking-[0.2em] text-turf-300 uppercase">
                Phase 7 · Personal journal
              </p>
              <h1 className="mt-2 text-3xl font-semibold tracking-tight sm:text-4xl">
                My bets
              </h1>
              <p className="mt-2 max-w-xl text-sm leading-relaxed text-mist-300">
                Imported from the bookmaker&apos;s own bet history. Stakes, odds
                and results are the provider&apos;s values; a later price never
                replaces the price actually taken.
              </p>
              <div className="mt-4 flex flex-wrap gap-2">
                <Link
                  href="/my-bets/insights"
                  className="inline-flex items-center gap-2 rounded-lg border border-white/10 px-3 py-1.5 text-xs text-mist-200 transition hover:border-white/25 hover:bg-white/5"
                >
                  Insights
                </Link>
                <Link
                  href="/admin/journal"
                  className="inline-flex items-center gap-2 rounded-lg border border-white/10 px-3 py-1.5 text-xs text-mist-200 transition hover:border-white/25 hover:bg-white/5"
                >
                  Import screenshots
                </Link>
              </div>
            </div>

            {day && <DayCard day={day} spark={spark} />}
          </div>
        </section>

        {allBets.length === 0 ? (
          <p className="mt-6 rounded-xl border border-line bg-ink-900/50 p-6 text-sm text-mist-400">
            No bets stored yet. Import a history export with{" "}
            <code className="font-mono text-mist-200">make import-bets</code>.
          </p>
        ) : (
          <>
            <StatCards summary={summary} />

            <FilterBar
              filters={filters}
              onChange={patch}
              onReset={reset}
              dirty={dirty}
              bookmakers={bookmakerOptions}
              competitions={competitionOptions}
            />

            <div className="mt-4 flex flex-wrap items-center gap-2">
              {STATUS_TABS.map((tab) => {
                const active = filters.status === tab.key;
                const count =
                  tab.key === "all"
                    ? counts.bets
                    : tab.key === "won"
                      ? counts.won
                      : tab.key === "lost"
                        ? counts.lost
                        : counts.pending;
                return (
                  <button
                    key={tab.key}
                    type="button"
                    data-status={tab.key}
                    aria-pressed={active}
                    onClick={() => patch({ status: tab.key })}
                    className={`inline-flex items-center gap-2 rounded-full border px-3 py-1 text-[12px] transition ${
                      active
                        ? "border-turf-400/30 bg-turf-500/10 text-turf-300"
                        : "border-line-strong text-mist-400 hover:border-line-strong hover:text-mist-200"
                    }`}
                  >
                    <span aria-hidden className={`size-1.5 rounded-full ${tab.dot}`} />
                    {tab.label}
                    <span className="font-mono text-[10px] text-mist-500">
                      ({count})
                    </span>
                  </button>
                );
              })}

              <span className="ml-auto font-mono text-[10px] tracking-widest text-mist-500 uppercase">
                {summary.bets} slips · {formatMoney(summary.profit, summary.currency)}
              </span>
            </div>

            <div className="mt-4 grid gap-4 lg:grid-cols-[minmax(0,1.4fr)_minmax(0,1fr)]">
              <MarketTable rows={markets} />
              <PerformancePanel
                series={series}
                summary={summary}
                mode={chartMode}
                onMode={setChartMode}
              />
            </div>

            <section className="mt-6">
              <div className="flex flex-wrap items-baseline justify-between gap-2">
                <h2 className="text-sm font-semibold tracking-tight">
                  Bet history
                </h2>
                <p className="font-mono text-[10px] tracking-widest text-mist-500 uppercase">
                  {filtered.length} matching {filtered.length === 1 ? "slip" : "slips"}
                </p>
              </div>

              {filtered.length === 0 ? (
                <div className="mt-3 rounded-xl border border-line bg-ink-900/50 p-5 text-sm text-mist-400">
                  No bets match these filters.{" "}
                  <button
                    type="button"
                    onClick={reset}
                    className="text-turf-300 underline decoration-turf-400/30 underline-offset-4"
                  >
                    Clear filters
                  </button>
                </div>
              ) : (
                <>
                  <ul className="mt-3 space-y-3">
                    {filtered.slice(0, visible).map((bet) => (
                      <BetCard key={bet.id} bet={bet} />
                    ))}
                  </ul>

                  {visible < filtered.length && (
                    <button
                      type="button"
                      onClick={() => setVisible((value) => value + PAGE_SIZE)}
                      className="mt-4 w-full rounded-xl border border-line-strong py-2.5 text-sm text-mist-300 transition hover:border-line-strong hover:bg-hover"
                    >
                      Show {Math.min(PAGE_SIZE, filtered.length - visible)} more
                      <span className="ml-2 font-mono text-[10px] text-mist-500">
                        {filtered.length - visible} left
                      </span>
                    </button>
                  )}
                </>
              )}
            </section>
          </>
        )}

        <p className="mt-6 max-w-3xl text-[11px] leading-relaxed text-mist-500">
          Leg descriptions are stored exactly as the bookmaker published them
          (for example “Corners A - Corners B”). Market and selection stay
          unresolved until a verified mapping exists - the journal never guesses
          what was bet. P/L is the provider&apos;s settled return; an
          accumulator&apos;s result is not allocated to individual legs.
        </p>

        <a
          href="/__journal/logout"
          className="mt-3 inline-block font-mono text-[10px] tracking-widest text-mist-500 uppercase transition hover:text-mist-200"
        >
          Lock journal
        </a>
      </main>
    </>
  );
}

/* ------------------------------------------------------------------ cards */

function DayCard({
  day,
  spark,
}: {
  day: DayPerformance;
  spark: number[];
}) {
  return (
    <div className="rounded-xl border border-turf-400/20 bg-ink-950/60 p-4 backdrop-blur">
      <div className="flex items-start justify-between gap-3">
        <p className="flex items-center gap-1.5 font-mono text-[10px] tracking-widest text-mist-400 uppercase">
          <Icon name="calendar" className="size-3.5 text-turf-300" />
          {day.isToday
            ? "Today's performance"
            : `Last active · ${formatDayShort(day.day)}`}
        </p>
        <Sparkline values={spark} className="h-10 w-24" />
      </div>

      <p
        className={`mt-2 text-2xl font-semibold tracking-tight ${
          day.profit >= 0 ? "text-turf-300" : "text-loss-300"
        }`}
      >
        {formatMoney(day.profit, day.currency)}
      </p>

      <p className="mt-0.5 text-[11px] text-mist-400">
        {day.bets} {day.bets === 1 ? "bet" : "bets"} · {day.won} won ·{" "}
        {day.winRate === null
          ? "- win rate"
          : `${(day.winRate * 100).toFixed(1)}% win rate`}
        <span className="text-mist-500"> ({day.settled} settled)</span>
      </p>

      {!day.isToday && (
        <p className="mt-1 font-mono text-[10px] text-mist-500">
          No bets placed today.
        </p>
      )}
    </div>
  );
}

function StatCards({ summary }: { summary: Totals }) {
  const roi = summary.roi;
  return (
    <dl className="mt-4 grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-4">
      <StatCard
        icon="wallet"
        label="Staked"
        value={formatMoney(summary.staked, summary.currency)}
        hint={`${summary.bets} slips`}
      />
      <StatCard
        icon="refresh"
        label="Returned"
        value={formatMoney(summary.returned, summary.currency)}
        hint="settled returns"
      />
      <StatCard
        icon="trend"
        label="Profit / loss"
        value={formatMoney(summary.profit, summary.currency)}
        valueTone={summary.profit >= 0 ? "good" : "bad"}
        hint={
          roi === null ? (
            "no stake on this selection"
          ) : (
            <span
              className={`inline-flex items-center gap-1 rounded-full border px-1.5 py-0.5 font-mono text-[9px] ${
                roi >= 0
                  ? "border-turf-400/25 bg-turf-500/10 text-turf-300"
                  : "border-loss-400/25 bg-loss-500/10 text-loss-300"
              }`}
            >
              <Icon
                name={roi >= 0 ? "arrow-up" : "arrow-down"}
                className="size-2.5"
              />
              {roi >= 0 ? "+" : "-"}
              {Math.abs(roi).toFixed(1)}% ROI
            </span>
          )
        }
      />
      <StatCard
        icon="trophy"
        label="Record"
        value={`${summary.won}W · ${summary.lost}L`}
        hint={`${summary.bets} slips · ${summary.pending} pending`}
        gauge={summary.winRate}
      />
    </dl>
  );
}

function StatCard({
  icon,
  label,
  value,
  hint,
  valueTone,
  gauge,
}: {
  icon: IconName;
  label: string;
  value: string;
  hint: React.ReactNode;
  valueTone?: "good" | "bad";
  gauge?: number | null;
}) {
  return (
    <div className="rounded-xl border border-line bg-ink-900/60 p-4">
      <div className="flex items-start gap-3">
        <span className="grid size-9 shrink-0 place-items-center rounded-lg border border-turf-400/20 bg-turf-500/10 text-turf-300">
          <Icon name={icon} className="size-4" />
        </span>

        <div className="min-w-0 flex-1">
          <dt className="font-mono text-[9px] tracking-widest text-mist-500 uppercase">
            {label}
          </dt>
          <dd
            className={`mt-0.5 truncate text-[17px] font-semibold tracking-tight ${
              valueTone === "good"
                ? "text-turf-300"
                : valueTone === "bad"
                  ? "text-loss-300"
                  : "text-mist-100"
            }`}
          >
            {value}
          </dd>
          <div className="mt-1 text-[10px] text-mist-500">{hint}</div>
        </div>

        {gauge !== undefined && (
          <div className="flex shrink-0 flex-col items-center gap-1">
            <WinRateGauge value={gauge} size={52} />
            <span className="font-mono text-[8px] tracking-widest text-mist-500 uppercase">
              win rate
            </span>
          </div>
        )}
      </div>
    </div>
  );
}

function PerformancePanel({
  series,
  summary,
  mode,
  onMode,
}: {
  series: ReturnType<typeof dailySeries>;
  summary: Totals;
  mode: "pl" | "rate";
  onMode: (mode: "pl" | "rate") => void;
}) {
  return (
    <section className="flex h-full flex-col rounded-2xl border border-line bg-ink-900/40 p-4">
      <header className="flex flex-wrap items-center justify-between gap-2">
        <h2 className="flex items-center gap-2 text-sm font-semibold tracking-tight">
          <Icon name="trend" className="size-4 text-turf-300" />
          Performance trend
        </h2>
        <div className="flex rounded-full border border-line-strong bg-ink-950/40 p-0.5">
          {(["pl", "rate"] as const).map((key) => (
            <button
              key={key}
              type="button"
              onClick={() => onMode(key)}
              className={`rounded-full px-2.5 py-0.5 text-[10px] transition ${
                mode === key
                  ? "bg-turf-500/15 text-turf-300"
                  : "text-mist-500 hover:text-mist-200"
              }`}
            >
              {key === "pl" ? "Profit / Loss" : "Win rate"}
            </button>
          ))}
        </div>
      </header>

      <div className="mt-3 flex flex-1 items-center">
        <PerformanceChart points={series} mode={mode} />
      </div>

      <dl className="mt-3 grid grid-cols-3 gap-2 border-t border-line pt-3">
        <Mini
          label="Total P/L"
          value={formatMoney(summary.profit, summary.currency)}
          tone={summary.profit >= 0 ? "good" : "bad"}
        />
        <Mini
          label="ROI"
          value={
            summary.roi === null
              ? "-"
              : `${summary.roi >= 0 ? "+" : ""}${summary.roi.toFixed(1)}%`
          }
        />
        <Mini
          label="Win rate"
          value={
            summary.winRate === null
              ? "-"
              : `${(summary.winRate * 100).toFixed(1)}%`
          }
          hint={`${summary.settled} settled`}
        />
      </dl>
    </section>
  );
}

function Mini({
  label,
  value,
  tone = "plain",
  hint,
}: {
  label: string;
  value: string;
  tone?: "plain" | "good" | "bad";
  hint?: string;
}) {
  return (
    <div>
      <dt className="font-mono text-[8px] tracking-widest text-mist-500 uppercase">
        {label}
      </dt>
      <dd
        className={`mt-0.5 font-mono text-[12px] ${
          tone === "good"
            ? "text-turf-300"
            : tone === "bad"
              ? "text-loss-300"
              : "text-mist-100"
        }`}
      >
        {value}
      </dd>
      {hint && <p className="font-mono text-[9px] text-mist-500">{hint}</p>}
    </div>
  );
}
