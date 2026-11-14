"use client";

import Link from "next/link";
import { useState } from "react";
import {
  faArrowsRotate,
  faFilePen,
  faTrashCan,
} from "@fortawesome/free-solid-svg-icons";
import { FontAwesomeIcon } from "@fortawesome/react-fontawesome";

import { LoadingNote } from "@/components/loading-note";
import { SiteHeader } from "@/components/site-header";
import {
  deletePaperPick,
  formatAge,
  getPaperPicks,
  type PaperPick,
  type PaperPicks,
} from "@/lib/api";
import { useApiData } from "@/lib/use-api";

const STATUS_STYLES: Record<PaperPick["status"], string> = {
  pending: "border-line text-mist-300",
  won: "border-turf-400/40 bg-turf-500/10 text-turf-300",
  lost: "border-loss-400/40 bg-loss-400/10 text-loss-300",
  push: "border-line text-mist-300",
  void: "border-line text-mist-400",
};

export default function PaperPicksPage() {
  const { data, loading, error, reload } = useApiData<PaperPicks | null>(
    "paper-picks",
    () => getPaperPicks(200),
    { refreshMs: 60000 },
  );
  const [deleting, setDeleting] = useState<number | null>(null);

  const picks = data?.picks ?? [];
  const summary = data?.summary;

  async function remove(id: number) {
    setDeleting(id);
    await deletePaperPick(id);
    setDeleting(null);
    reload();
  }

  return (
    <>
      <SiteHeader />
      <main className="mx-auto max-w-5xl px-6 py-12">
        <div className="flex flex-wrap items-end justify-between gap-4">
          <div>
            <p className="font-mono text-[11px] tracking-widest text-turf-300 uppercase">
              Study · No stake
            </p>
            <h1 className="mt-2 flex items-center gap-2 text-3xl font-bold tracking-tight">
              <FontAwesomeIcon icon={faFilePen} className="size-6 text-turf-300" />
              Paper picks
            </h1>
            <p className="mt-2 max-w-2xl text-sm leading-relaxed text-mist-300">
              Corner selections recorded by hand with no money on them. They are
              separate from agent picks and from your bet slips, so they can be
              used as training data without pretending a stake was placed. The
              minute, corners, pace and projection at the moment of recording are
              frozen with each pick.
            </p>
          </div>
          <button
            type="button"
            onClick={reload}
            className="inline-flex items-center gap-2 rounded-xl border border-line px-3 py-2 font-mono text-[10px] tracking-widest text-mist-400 uppercase transition hover:text-mist-100"
          >
            <FontAwesomeIcon icon={faArrowsRotate} className="size-3" />
            refresh
          </button>
        </div>

        {summary ? (
          <div className="mt-5 grid grid-cols-2 gap-2 sm:grid-cols-5">
            <Summary label="Pending" value={summary.pending} />
            <Summary label="Won" value={summary.won} tone="text-turf-300" />
            <Summary label="Lost" value={summary.lost} tone="text-loss-300" />
            <Summary label="Push" value={summary.push} />
            <Summary label="Void" value={summary.void} />
          </div>
        ) : null}

        {error || (!loading && data === null) ? (
          <div className="mt-6 rounded-2xl border border-flag-400/30 bg-flag-400/[0.04] p-6">
            <p className="text-sm font-medium text-flag-400">
              Unable to load paper picks.
            </p>
            <p className="mt-1 text-xs text-mist-400">{error}</p>
            <button
              type="button"
              onClick={reload}
              className="mt-3 rounded-lg border border-flag-400/40 px-3 py-1.5 text-xs font-semibold text-flag-400"
            >
              Try again
            </button>
          </div>
        ) : loading && !data ? (
          <LoadingNote label="Reading paper picks…" />
        ) : picks.length === 0 ? (
          <div className="mt-6 rounded-2xl border border-line bg-ink-900/60 p-6">
            <p className="text-sm font-medium text-mist-100">
              No paper picks recorded yet.
            </p>
            <p className="mt-1 max-w-2xl text-xs leading-relaxed text-mist-400">
              Open a match on the{" "}
              <Link href="/corners" className="text-turf-300 hover:text-turf-200">
                corner board
              </Link>{" "}
              and use the record button on its card. The pick settles by itself
              once the final corner count is published.
            </p>
          </div>
        ) : (
          <div className="mt-6 overflow-x-auto rounded-2xl border border-line">
            <table className="w-full min-w-[820px] text-left text-xs">
              <thead className="border-b border-line bg-ink-900/60 font-mono text-[9px] tracking-widest text-mist-500 uppercase">
                <tr>
                  <th className="px-3 py-2 font-normal">Match</th>
                  <th className="px-3 py-2 font-normal">Pick</th>
                  <th className="px-3 py-2 font-normal">At save</th>
                  <th className="px-3 py-2 font-normal">Final</th>
                  <th className="px-3 py-2 font-normal">Status</th>
                  <th className="px-3 py-2" />
                </tr>
              </thead>
              <tbody className="divide-y divide-white/[0.05]">
                {picks.map((pick) => (
                  <tr key={pick.id} className="align-top">
                    <td className="px-3 py-2">
                      <Link
                        href={`/corners/detail?id=${pick.event_id}&line=${pick.line}`}
                        className="font-semibold text-mist-100 hover:text-turf-300"
                      >
                        {pick.home} v {pick.away}
                      </Link>
                      <p className="mt-0.5 font-mono text-[10px] text-mist-500">
                        {pick.competition} · {formatAge(pick.created_at)}
                      </p>
                    </td>
                    <td className="px-3 py-2 font-mono text-mist-200">
                      {pick.side} {pick.line}{" "}
                      <span className="text-mist-500">@ {pick.odds.toFixed(2)}</span>
                      <p className="mt-0.5 font-mono text-[10px] text-mist-500">
                        {pick.provider}
                      </p>
                    </td>
                    <td className="px-3 py-2 font-mono text-[10px] text-mist-400">
                      {pick.minute !== null ? `${pick.minute}'` : "-"} ·{" "}
                      {pick.total_corners !== null
                        ? `${pick.total_corners} corners`
                        : "no count"}
                      <br />
                      pace {pick.pace_per_minute !== null ? pick.pace_per_minute.toFixed(2) : "-"}
                      {" · "}proj{" "}
                      {pick.projected_total !== null
                        ? pick.projected_total.toFixed(1)
                        : pick.early_pace
                          ? "early"
                          : "-"}
                      {pick.model_risk ? (
                        <span className="text-flag-400"> · risk</span>
                      ) : null}
                    </td>
                    <td className="px-3 py-2 font-mono text-mist-200">
                      {pick.final_total !== null ? pick.final_total : "-"}
                    </td>
                    <td className="px-3 py-2">
                      <span
                        className={`inline-block rounded border px-1.5 py-0.5 font-mono text-[9px] tracking-widest uppercase ${STATUS_STYLES[pick.status]}`}
                      >
                        {pick.status}
                      </span>
                    </td>
                    <td className="px-3 py-2 text-right">
                      <button
                        type="button"
                        onClick={() => void remove(pick.id)}
                        disabled={deleting === pick.id}
                        title="Delete this recording"
                        className="grid size-7 place-items-center rounded-md border border-line text-mist-500 transition hover:border-loss-400/40 hover:text-loss-300 disabled:opacity-50"
                      >
                        <FontAwesomeIcon icon={faTrashCan} className="size-3" />
                      </button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}

        <section className="mt-6 rounded-2xl border border-line bg-ink-900/50 p-5">
          <h2 className="text-sm font-semibold text-mist-100">How settlement works</h2>
          <ul className="mt-2 space-y-1.5 text-xs leading-relaxed text-mist-300">
            <li>
              A pick settles only when the match is finished and both teams'
              final corner counts are published. Until then it stays pending.
            </li>
            <li>
              Over wins when the final total is above the line, under wins when
              it is below, and an exact hit is a push.
            </li>
            <li>
              These picks never touch your bet journal or your money totals. They
              exist to build a record of decisions you would have made.
            </li>
          </ul>
        </section>
      </main>
    </>
  );
}

function Summary({
  label,
  value,
  tone = "text-mist-100",
}: {
  label: string;
  value: number;
  tone?: string;
}) {
  return (
    <div className="rounded-xl border border-line bg-ink-950/40 px-3 py-2.5">
      <p className="font-mono text-[9px] tracking-widest text-mist-500 uppercase">
        {label}
      </p>
      <p className={`mt-0.5 font-mono text-lg tabular-nums ${tone}`}>{value}</p>
    </div>
  );
}
