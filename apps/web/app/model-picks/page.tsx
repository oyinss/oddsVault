"use client";

import Link from "next/link";
import {
  faArrowUpRightFromSquare,
  faCircleInfo,
} from "@fortawesome/free-solid-svg-icons";
import { FontAwesomeIcon } from "@fortawesome/react-fontawesome";

import { LoadingNote } from "@/components/loading-note";
import { SiteHeader } from "@/components/site-header";
import { formatAge, getModelPicks, type ModelPicks } from "@/lib/api";
import { useApiData } from "@/lib/use-api";

const SELECTION_LABEL: Record<string, string> = {
  home: "Home win",
  draw: "Draw",
  away: "Away win",
};

function kickoffLabels(iso: string) {
  const at = new Date(iso);
  if (Number.isNaN(at.valueOf())) {
    return { dayLabel: "unknown", timeLabel: "" };
  }
  return {
    dayLabel: at.toLocaleDateString("en-GB", {
      weekday: "short",
      day: "numeric",
      month: "short",
    }),
    timeLabel: at.toLocaleTimeString("en-GB", {
      hour: "2-digit",
      minute: "2-digit",
    }),
  };
}

export default function ModelPicksPage() {
  const { data, error, loading } = useApiData<ModelPicks | null>(
    "model-picks",
    getModelPicks,
  );

  return (
    <>
      <SiteHeader />
      <main className="mx-auto max-w-5xl px-6 py-12">
        <div className="flex flex-wrap items-end justify-between gap-4">
          <div>
            <p className="font-mono text-[11px] tracking-widest text-turf-300 uppercase">
              Strategies · Frozen picks
            </p>
            <h1 className="mt-2 text-3xl font-bold tracking-tight">
              Model picks
            </h1>
            <p className="mt-2 max-w-2xl text-sm leading-relaxed text-mist-300">
              Every pick below was written once, before kickoff, from the
              recorded price at that moment. Results never rewrite a pick; a new
              run is a new set.
            </p>
          </div>
          <div className="rounded-2xl border border-line bg-ink-900/60 px-4 py-3 text-right">
            <p className="font-mono text-[10px] tracking-widest text-mist-500 uppercase">
              Last run
            </p>
            <p className="mt-1 font-mono text-sm text-mist-100">
              {data?.data_cutoff_at ? formatAge(data.data_cutoff_at) : "-"}
            </p>
            <p className="mt-0.5 font-mono text-[10px] text-mist-500">
              {data?.run_id ? `run ${data.run_id}` : "no run recorded"}
            </p>
          </div>
        </div>

        {loading && !data ? (
          <LoadingNote label="Loading frozen picks…" />
        ) : error ? (
          <div className="mt-6 rounded-2xl border border-flag-400/30 bg-flag-400/[0.04] p-6">
            <p className="text-sm font-medium text-flag-400">
              Unable to load picks.
            </p>
            <p className="mt-1 text-xs text-mist-400">{error}</p>
          </div>
        ) : !data || data.picks.length === 0 ? (
          <div className="mt-6 rounded-2xl border border-line bg-ink-900/60 p-6">
            <p className="text-sm font-medium text-mist-100">
              {data?.run_id
                ? "No price qualified in the latest run."
                : "No strategy run yet."}
            </p>
            <p className="mt-1 max-w-2xl text-xs leading-relaxed text-mist-400">
              {data?.run_id
                ? "Every upcoming match-result price was screened, and none had enough settled history in its odds bucket above the required edge. Nothing is added to fill the space."
                : "Run the strategy to freeze a set of recommendations."}
            </p>
            <code className="mt-3 inline-block rounded-lg border border-line bg-ink-950/60 px-3 py-1.5 font-mono text-[11px] text-mist-300">
              make model-picks
            </code>
          </div>
        ) : (
          <>
            <div className="mt-6 overflow-x-auto rounded-2xl border border-line">
              <table className="w-full min-w-[720px] border-collapse text-sm">
                <thead className="bg-ink-900/70">
                  <tr className="font-mono text-[10px] tracking-widest text-mist-500 uppercase">
                    <th className="px-4 py-3 text-left font-normal">Kickoff</th>
                    <th className="px-4 py-3 text-left font-normal">Fixture</th>
                    <th className="px-4 py-3 text-left font-normal">Pick</th>
                    <th className="px-4 py-3 text-right font-normal">Odds</th>
                    <th className="px-4 py-3 text-right font-normal">
                      Historical
                    </th>
                    <th className="px-4 py-3 text-right font-normal">Edge</th>
                    <th className="px-4 py-3" />
                  </tr>
                </thead>
                <tbody>
                  {data.picks.map((pick) => {
                    const kickoff = kickoffLabels(pick.kickoff);
                    return (
                      <tr
                        key={pick.id}
                        className="border-t border-line bg-ink-900/30 align-top"
                      >
                        <td className="px-4 py-3 whitespace-nowrap font-mono text-xs text-mist-300">
                          {kickoff.dayLabel}
                          <span className="block text-[10px] text-mist-500">
                            {kickoff.timeLabel}
                          </span>
                        </td>
                        <td className="px-4 py-3">
                          <span className="block text-mist-100">
                            {pick.home_name} v {pick.away_name}
                          </span>
                          <span className="block text-[11px] text-mist-500">
                            {pick.competition || pick.sport}
                          </span>
                        </td>
                        <td className="px-4 py-3">
                          <span className="inline-flex rounded-md bg-turf-500/10 px-2 py-0.5 text-xs text-turf-300">
                            {SELECTION_LABEL[pick.selection] ?? pick.selection}
                          </span>
                        </td>
                        <td className="px-4 py-3 text-right font-mono text-xs tabular-nums text-mist-100">
                          {pick.odds.toFixed(2)}
                        </td>
                        <td className="px-4 py-3 text-right font-mono text-xs tabular-nums text-mist-300">
                          {(pick.hit_rate * 100).toFixed(0)}%
                          <span className="block text-[10px] text-mist-500">
                            n={pick.samples} · implied{" "}
                            {(pick.implied * 100).toFixed(0)}%
                          </span>
                        </td>
                        <td className="px-4 py-3 text-right font-mono text-xs tabular-nums text-turf-300">
                          +{(pick.edge * 100).toFixed(1)}%
                          <span
                            className={`block text-[10px] ${
                              pick.status === "won"
                                ? "text-turf-300"
                                : pick.status === "lost"
                                  ? "text-loss-400"
                                  : "text-mist-500"
                            }`}
                          >
                            {pick.status}
                          </span>
                        </td>
                        <td className="px-4 py-3 text-right">
                          <Link
                            href={`/events/detail?id=${pick.event_id}`}
                            className="inline-flex items-center gap-1 font-mono text-[10px] tracking-widest text-mist-500 uppercase hover:text-mist-200"
                          >
                            Event
                            <FontAwesomeIcon
                              icon={faArrowUpRightFromSquare}
                              className="size-2.5"
                            />
                          </Link>
                        </td>
                      </tr>
                    );
                  })}
                </tbody>
              </table>
            </div>

            <p className="mt-3 font-mono text-[10px] text-mist-500">
              {data.count} {data.count === 1 ? "pick" : "picks"} ·{" "}
              {data.strategy} · cutoff{" "}
              {data.data_cutoff_at
                ? new Date(data.data_cutoff_at).toLocaleString()
                : "-"}
            </p>
          </>
        )}

        <section className="mt-8 rounded-2xl border border-line bg-ink-900/50 p-5">
          <h2 className="flex items-center gap-2 text-sm font-semibold text-mist-100">
            <FontAwesomeIcon icon={faCircleInfo} className="size-3 text-mist-500" />
            How a pick is made
          </h2>
          <ol className="mt-3 space-y-2 text-xs leading-relaxed text-mist-300">
            <li>
              1. Settled match-result selections are grouped by the price that
              was recorded, in 0.50 buckets.
            </li>
            <li>
              2. A bucket needs at least 20 settled picks before its hit rate is
              used. Below that, the rate is noise and nothing is selected.
            </li>
            <li>
              3. A price qualifies when the bucket hit rate is at least 3
              percentage points above its implied probability (1 / odds).
            </li>
            <li>
              4. The strongest pick per fixture is kept, then the top 5 by edge.
              The run is written with its data cutoff and never edited.
            </li>
          </ol>
          <p className="mt-3 border-t border-line pt-3 text-xs leading-relaxed text-mist-500">
            Honest limit: the settled history behind these rates is small, so a
            bucket rate is an estimate, not a guarantee. The sample size is shown
            on every row so you can judge it yourself.
          </p>
        </section>
      </main>
    </>
  );
}
