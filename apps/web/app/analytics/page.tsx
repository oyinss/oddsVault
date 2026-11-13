"use client";

import Link from "next/link";
import { useSearchParams } from "next/navigation";
import { Suspense } from "react";

import { LoadingNote } from "@/components/loading-note";
import { SiteHeader } from "@/components/site-header";
import { getOddsRanges, type OddsRanges } from "@/lib/api";
import { useApiData } from "@/lib/use-api";

const MARKETS = [
  { key: "match_result", label: "Match result", line: undefined },
  { key: "total_goals", label: "Total goals 2.5", line: 2.5 },
  { key: "both_teams_to_score", label: "Both teams to score", line: undefined },
  { key: "double_chance", label: "Double chance", line: undefined },
];

export default function AnalyticsPage() {
  return (
    <Suspense
      fallback={
        <>
          <SiteHeader />
          <main className="mx-auto max-w-5xl px-6 py-12">
            <LoadingNote label="Loading analytics…" />
          </main>
        </>
      }
    >
      <AnalyticsView />
    </Suspense>
  );
}

function AnalyticsView() {
  const params = useSearchParams();
  const selected =
    MARKETS.find((market) => market.key === params.get("market")) ?? MARKETS[0];
  const bucket = params.get("bucket") === "0.05" ? 0.05 : 0.1;

  const { data: ranges, loading } = useApiData<OddsRanges | null>(
    `odds-ranges:${selected.key}:${bucket}`,
    () => getOddsRanges(selected.key, selected.line, bucket),
  );

  return (
    <>
      <SiteHeader />
      <main className="mx-auto max-w-5xl px-6 py-12">
        <div>
          <p className="font-mono text-[11px] tracking-widest text-turf-300 uppercase">
            Phase 6 · Historical analytics
          </p>
          <h1 className="mt-2 text-3xl font-bold tracking-tight">
            Odds ranges
          </h1>
          <p className="mt-2 max-w-2xl text-sm leading-relaxed text-mist-400">
            Settled selections grouped by the latest captured price. Only
            hits and misses enter the rate; pushes, voids and missing data are
            counted separately and every percentage carries its sample size.
          </p>
        </div>

        <div className="mt-6 flex flex-wrap gap-2">
          {MARKETS.map((market) => {
            const active = market.key === selected.key;
            return (
              <Link
                key={market.key}
                href={`/analytics?market=${market.key}&bucket=${bucket}`}
                className={`rounded-lg border px-3 py-1.5 text-sm transition ${
                  active
                    ? "border-turf-400/30 bg-turf-500/10 text-turf-300"
                    : "border-line-strong text-mist-300 hover:border-line-strong hover:bg-raised"
                }`}
              >
                {market.label}
              </Link>
            );
          })}
          <span className="ml-auto flex items-center gap-2 font-mono text-[11px] text-mist-500">
            bucket
            {[0.05, 0.1].map((size) => (
              <Link
                key={size}
                href={`/analytics?market=${selected.key}&bucket=${size}`}
                className={`rounded px-2 py-0.5 ${
                  size === bucket ? "bg-raised text-mist-200" : "text-mist-500"
                }`}
              >
                {size}
              </Link>
            ))}
          </span>
        </div>

        {loading ? (
          <LoadingNote label="Loading analytics…" />
        ) : ranges ? (
          <RangesTable ranges={ranges} />
        ) : (
          <div className="panel mt-8 border-flag-400/30 p-5">
            <p className="text-sm font-medium text-flag-400">
              Analytics unavailable
            </p>
            <p className="mt-1 text-xs text-mist-400">
              The API could not be reached.
            </p>
          </div>
        )}

        <p className="mt-6 max-w-3xl text-[11px] leading-relaxed text-mist-500">
          Historical percentages describe what happened at these prices; they are
          not a prediction. Closing prices are included, so early live prices for
          unplayed fixtures appear only once their selections settle.
        </p>
      </main>
    </>
  );
}

function RangesTable({ ranges }: { ranges: OddsRanges }) {
  const filtered = ranges.buckets.filter(
    (bucket) => bucket.hits + bucket.misses > 0,
  );

  return (
    <>
      <dl className="mt-8 grid grid-cols-2 gap-3 sm:grid-cols-4">
        <Stat label="Settled" value={String(ranges.settled)} />
        <Stat
          label="Hit rate"
          value={
            ranges.settled > 0 ? `${(ranges.total.hit_rate * 100).toFixed(1)}%` : "-"
          }
          hint={`${ranges.total.hits} hits · ${ranges.total.misses} misses`}
        />
        <Stat label="Excluded" value={String(ranges.excluded)} hint="push/void/missing" />
        <Stat
          label="Average odds"
          value={ranges.total.avg_odds ? ranges.total.avg_odds.toFixed(2) : "-"}
        />
      </dl>

      {filtered.length === 0 ? (
        <p className="mt-8 rounded-xl border border-line bg-ink-900/50 p-5 text-sm text-mist-400">
          No settled selections at these prices yet.
        </p>
      ) : (
        <div className="mt-8 overflow-hidden rounded-xl border border-line bg-ink-900/50">
          <table className="w-full text-left text-sm">
            <thead className="border-b border-line font-mono text-[10px] tracking-widest text-mist-500 uppercase">
              <tr>
                <th className="px-4 py-3">Odds range</th>
                <th className="px-4 py-3 text-right">Settled</th>
                <th className="px-4 py-3 text-right">Hits</th>
                <th className="px-4 py-3 text-right">Rate</th>
                <th className="px-4 py-3">Distribution</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-line">
              {filtered.map((bucket) => {
                const settled = bucket.hits + bucket.misses;
                return (
                  <tr key={bucket.low}>
                    <td className="px-4 py-2.5 font-mono text-xs text-mist-200">
                      {bucket.low.toFixed(2)}-{bucket.high.toFixed(2)}
                    </td>
                    <td className="px-4 py-2.5 text-right font-mono text-xs">
                      {settled}
                    </td>
                    <td className="px-4 py-2.5 text-right font-mono text-xs text-mist-300">
                      {bucket.hits}
                    </td>
                    <td className="px-4 py-2.5 text-right font-mono text-xs text-mist-100">
                      {(bucket.hit_rate * 100).toFixed(1)}%
                    </td>
                    <td className="px-4 py-2.5">
                      <div className="h-1.5 w-full overflow-hidden rounded-full bg-raised">
                        <div
                          className="h-full rounded-full bg-turf-400/70"
                          style={{ width: `${bucket.hit_rate * 100}%` }}
                        />
                      </div>
                    </td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        </div>
      )}
    </>
  );
}

function Stat({
  label,
  value,
  hint,
}: {
  label: string;
  value: string;
  hint?: string;
}) {
  return (
    <div className="rounded-xl border border-line bg-ink-900/60 p-4">
      <dt className="font-mono text-[10px] tracking-widest text-mist-500 uppercase">
        {label}
      </dt>
      <dd className="mt-1 text-lg font-semibold tracking-tight">{value}</dd>
      {hint && <p className="text-[11px] text-mist-500">{hint}</p>}
    </div>
  );
}
