"use client";

import Link from "next/link";
import { useSearchParams } from "next/navigation";
import { Suspense } from "react";

import { LoadingNote } from "@/components/loading-note";
import { SiteHeader } from "@/components/site-header";
import {
  formatMoney,
  getBetInsights,
  type BetInsights,
} from "@/lib/api";
import { useApiData } from "@/lib/use-api";

const FAMILY_LABELS: Record<string, string> = {
  corners_total: "Corners total",
  corners_handicap: "Corners handicap",
  total_goals: "Total goals",
  moneyline: "Moneyline",
  total_corners: "Total corners",
  both_teams_to_score: "BTTS",
  match_result: "Match result",
  handicap: "Handicap",
  correct_score: "Correct score",
};

export default function InsightsPage() {
  return (
    <Suspense
      fallback={
        <>
          <SiteHeader />
          <main className="mx-auto max-w-5xl px-6 py-12">
            <LoadingNote label="Reading your journal…" />
          </main>
        </>
      }
    >
      <InsightsView />
    </Suspense>
  );
}

function InsightsView() {
  const params = useSearchParams();
  const minSample = Number(params.get("min_sample") ?? "5") || 5;
  const { data, loading } = useApiData<BetInsights | null>(
    `bet-insights:${minSample}`,
    () => getBetInsights(minSample),
  );

  return (
    <>
      <SiteHeader />
      <main className="mx-auto max-w-5xl px-6 py-12">
        <div className="flex flex-wrap items-end justify-between gap-4">
          <div>
            <p className="font-mono text-[11px] tracking-widest text-turf-300 uppercase">
              Journal · Insights
            </p>
            <h1 className="mt-2 text-3xl font-bold tracking-tight">
              What you actually play
            </h1>
            <p className="mt-2 max-w-2xl text-sm leading-relaxed text-mist-400">
              Rates count only settled legs and always show the sample size.
              Rows below the minimum are hidden rather than presented as trends.
            </p>
          </div>

          <div className="flex items-center gap-1 rounded-lg border border-line p-1">
            {[5, 10, 20].map((value) => (
              <Link
                key={value}
                href={`/my-bets/insights?min_sample=${value}`}
                className={`rounded px-2.5 py-1 font-mono text-[11px] ${
                  value === minSample
                    ? "bg-white/10 text-mist-100"
                    : "text-mist-500"
                }`}
              >
                min {value}
              </Link>
            ))}
          </div>
        </div>

        {loading && <LoadingNote label="Reading your journal…" />}

        {!loading && !data && (
          <div className="panel mt-8 border-flag-400/30 p-5">
            <p className="text-sm font-medium text-flag-400">
              Insights unavailable
            </p>
            <p className="mt-1 text-xs text-mist-400">
              The API could not be reached, or the journal endpoint is locked.
            </p>
          </div>
        )}

        {data && (
          <>
            <dl className="mt-8 grid grid-cols-2 gap-3 sm:grid-cols-3 lg:grid-cols-6">
              <Stat label="Slips" value={String(data.overall.bets)} />
              <Stat label="Staked" value={formatMoney(data.overall.staked)} />
              <Stat
                label="P / L"
                value={formatMoney(data.overall.profit_loss)}
                tone={data.overall.profit_loss >= 0 ? "good" : "bad"}
              />
              <Stat
                label="ROI"
                value={`${(data.overall.roi * 100).toFixed(1)}%`}
                tone={data.overall.roi >= 0 ? "good" : "bad"}
              />
              <Stat label="Settled legs" value={String(data.overall.settled_legs)} />
              <Stat
                label="Verified legs"
                value={String(data.overall.verified_legs)}
                hint={`${data.overall.disagreements} disagree`}
                tone={data.overall.disagreements > 0 ? "warn" : "good"}
              />
            </dl>

            <Section
              title="By line"
              note="The market family, side and line you actually took."
            >
              <Table
                head={["Market", "Legs", "Won", "Lost", "Rate", "Verified"]}
                rows={data.lines.map((row) => [
                  `${FAMILY_LABELS[row.family] ?? row.family}${
                    row.side ? ` · ${capitalise(row.side)}` : ""
                  }${row.line !== null ? ` ${row.line}` : ""}`,
                  String(row.legs),
                  String(row.won),
                  String(row.lost),
                  `${(row.rate * 100).toFixed(1)}% (${row.won + row.lost})`,
                  row.verified > 0
                    ? `${row.verified}${row.disagreements ? ` · ${row.disagreements} disagree` : ""}`
                    : "-",
                ])}
              />
            </Section>

            <Section title="By competition" note="Where the legs came from.">
              <Table
                head={["Competition", "Legs", "Won", "Lost", "Rate"]}
                rows={data.competitions.map((row) => [
                  row.label,
                  String(row.legs),
                  String(row.won),
                  String(row.lost),
                  `${(row.rate * 100).toFixed(1)}% (${row.won + row.lost})`,
                ])}
              />
            </Section>

            <Section
              title="What changes the outcome"
              note="Slip-level patterns: bet type, time of day and slip size. Each row shows its own slip count."
            >
              <div className="divide-y divide-white/[0.05]">
                {(data.factors ?? []).map((group) => (
                  <div key={group.group} className="px-4 py-3">
                    <p className="font-mono text-[10px] tracking-widest text-mist-500 uppercase">
                      {group.group.replace("_", " ")}
                    </p>
                    <p className="mt-0.5 text-[11px] text-mist-500">{group.note}</p>
                    <table className="mt-2 w-full text-left">
                      <tbody>
                        {group.rows.map((row) => (
                          <tr key={row.label} className="border-t border-white/[0.04]">
                            <td className="py-1.5 pr-3 text-xs text-mist-200">
                              {row.label}
                            </td>
                            <td className="px-2 py-1.5 font-mono text-[11px] text-mist-400">
                              {row.bets} {row.bets === 1 ? "slip" : "slips"}
                            </td>
                            <td className="px-2 py-1.5 font-mono text-[11px] text-mist-400">
                              {row.won}W / {row.lost}L
                            </td>
                            <td
                              className={`px-2 py-1.5 text-right font-mono text-[11px] ${
                                row.profit_loss >= 0 ? "text-turf-300" : "text-flag-400"
                              }`}
                            >
                              {formatMoney(row.profit_loss)}
                            </td>
                            <td
                              className={`py-1.5 pl-2 text-right font-mono text-[11px] ${
                                row.roi >= 0 ? "text-turf-300" : "text-flag-400"
                              }`}
                            >
                              {(row.roi * 100).toFixed(1)}%
                            </td>
                          </tr>
                        ))}
                      </tbody>
                    </table>
                  </div>
                ))}
                {(data.factors ?? []).length === 0 && (
                  <p className="px-4 py-4 text-xs text-mist-500">
                    Not enough samples yet.
                  </p>
                )}
              </div>
            </Section>

            <Section
              title="By fixture"
              note="Only legs linked to a stored match. Corner bets are not attributable to one team, so the pairing is the unit."
            >
              <Table
                head={["Fixture", "Legs", "Won", "Lost", "Rate"]}
                rows={(data.fixtures ?? []).map((row) => [
                  row.label,
                  String(row.legs),
                  String(row.won),
                  String(row.lost),
                  `${(row.rate * 100).toFixed(1)}% (${row.won + row.lost})`,
                ])}
              />
            </Section>

            <Section title="By leg odds" note="Price you took for each leg.">
              <Table
                head={["Odds range", "Legs", "Won", "Lost", "Rate"]}
                rows={data.odds_buckets.map((row) => [
                  `${row.low.toFixed(2)}-${(row.high ?? row.low + 0.25).toFixed(2)}`,
                  String(row.legs),
                  String(row.won),
                  String(row.lost),
                  `${(row.rate * 100).toFixed(1)}% (${row.won + row.lost})`,
                ])}
              />
            </Section>

            <Section title="By stake" note="Slip-level stake bands, with real P/L.">
              <Table
                head={["Stake", "Slips", "Staked", "Returned", "P / L"]}
                rows={data.stakes.map((row) => [
                  row.label,
                  String(row.bets),
                  formatMoney(row.staked),
                  formatMoney(row.returned),
                  formatMoney(row.profit_loss),
                ])}
              />
            </Section>

            <Section
              title="Provider vs verified"
              note="Corner totals recomputed from provider corner counts."
            >
              {data.disagreements.length === 0 ? (
                <p className="px-4 py-4 text-xs text-mist-500">
                  No disagreements among verified legs.
                </p>
              ) : (
                <ul className="divide-y divide-white/[0.06]">
                  {data.disagreements.map((row) => (
                    <li key={row.leg_id} className="px-4 py-3">
                      <p className="text-sm text-mist-200">
                        {row.event_raw}
                      </p>
                      <p className="mt-1 font-mono text-[11px] text-mist-500">
                        {capitalise(row.side)} {row.line ?? "-"} · provider{" "}
                        {row.provider_status} · verified {row.verified_status} ·{" "}
                        {row.note}
                      </p>
                      {row.event_id && (
                        <Link
                          href={`/events/detail?id=${row.event_id}`}
                          className="mt-1 inline-block font-mono text-[11px] text-turf-300"
                        >
                          open fixture →
                        </Link>
                      )}
                    </li>
                  ))}
                </ul>
              )}
            </Section>
          </>
        )}
      </main>
    </>
  );
}

function capitalise(value: string) {
  return value ? value[0].toUpperCase() + value.slice(1) : value;
}

function Section({
  title,
  note,
  children,
}: {
  title: string;
  note: string;
  children: React.ReactNode;
}) {
  return (
    <section className="mt-10">
      <h2 className="text-sm font-semibold tracking-tight">{title}</h2>
      <p className="mt-1 text-xs text-mist-500">{note}</p>
      <div className="mt-4 overflow-hidden rounded-xl border border-line bg-ink-900/50">
        {children}
      </div>
    </section>
  );
}

function Table({ head, rows }: { head: string[]; rows: React.ReactNode[][] }) {
  if (rows.length === 0) {
    return <p className="px-4 py-4 text-xs text-mist-500">Not enough samples yet.</p>;
  }
  return (
    <table className="w-full text-left text-sm">
      <thead className="border-b border-line font-mono text-[10px] tracking-widest text-mist-500 uppercase">
        <tr>
          {head.map((label) => (
            <th key={label} className="px-4 py-2.5">
              {label}
            </th>
          ))}
        </tr>
      </thead>
      <tbody className="divide-y divide-white/[0.05]">
        {rows.map((row, index) => (
          <tr key={index}>
            {row.map((cell, cellIndex) => (
              <td key={cellIndex} className="px-4 py-2 text-xs text-mist-300">
                {cell}
              </td>
            ))}
          </tr>
        ))}
      </tbody>
    </table>
  );
}

function Stat({
  label,
  value,
  hint,
  tone,
}: {
  label: string;
  value: string;
  hint?: string;
  tone?: "good" | "bad" | "warn";
}) {
  const colour =
    tone === "good"
      ? "text-turf-300"
      : tone === "bad"
        ? "text-flag-400"
        : tone === "warn"
          ? "text-flag-400"
          : "";
  return (
    <div className="rounded-xl border border-line bg-ink-900/60 p-4">
      <dt className="font-mono text-[10px] tracking-widest text-mist-500 uppercase">
        {label}
      </dt>
      <dd className={`mt-1 text-lg font-semibold tracking-tight ${colour}`}>
        {value}
      </dd>
      {hint && <p className="text-[11px] text-mist-500">{hint}</p>}
    </div>
  );
}
