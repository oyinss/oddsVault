"use client";

import Link from "next/link";
import { useMemo, useState } from "react";
import {
  faArrowsRotate,
  faFilePen,
} from "@fortawesome/free-solid-svg-icons";
import { FontAwesomeIcon } from "@fortawesome/react-fontawesome";

import { LoadingNote } from "@/components/loading-note";
import { SiteHeader } from "@/components/site-header";
import { PaperPickCard } from "@/features/corners/paper-pick-card";
import { PaperPickForm } from "@/features/corners/paper-pick-form";
import {
  deletePaperPick,
  getPaperPicks,
  type PaperPick,
  type PaperPicks,
} from "@/lib/api";
import { useApiData } from "@/lib/use-api";

export default function PaperPicksPage() {
  const { data, loading, error, reload } = useApiData<PaperPicks | null>(
    "paper-picks",
    () => getPaperPicks(200),
    { refreshMs: 60000 },
  );
  const [status, setStatus] = useState<"all" | PaperPick["status"]>("all");
  const [book, setBook] = useState("");
  const [competition, setCompetition] = useState("");
  const [query, setQuery] = useState("");

  const picks = data?.picks ?? [];
  const books = useMemo(
    () => [...new Set(picks.map((pick) => pick.provider))].sort(),
    [picks],
  );
  const competitions = useMemo(
    () => [...new Set(picks.map((pick) => pick.competition))].sort(),
    [picks],
  );
  const needle = query.trim().toLowerCase();
  const visible = picks.filter((pick) => {
    if (status !== "all" && pick.status !== status) return false;
    if (book && pick.provider !== book) return false;
    if (competition && pick.competition !== competition) return false;
    if (
      needle &&
      ![pick.home, pick.away, pick.competition, pick.provider, pick.side]
        .join(" ")
        .toLowerCase()
        .includes(needle)
    ) {
      return false;
    }
    return true;
  }).sort((a, b) => {
    const pendingFirst = (a.status === "pending" ? 0 : 1) - (b.status === "pending" ? 0 : 1);
    if (pendingFirst !== 0) return pendingFirst;
    return Date.parse(b.created_at) - Date.parse(a.created_at);
  });

  const [editing, setEditing] = useState<PaperPick | null>(null);

  async function remove(id: number) {
    await deletePaperPick(id);
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
              Corner selections recorded by hand with no money on them. Each card
              keeps the board view you already read: corners, pace, projection and
              the line, with the pick you recorded and its result.
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

        {data ? (
          <div className="mt-5 grid grid-cols-2 gap-2 sm:grid-cols-5">
            <Summary label="Pending" value={data.summary.pending} />
            <Summary label="Won" value={data.summary.won} tone="text-turf-300" />
            <Summary label="Lost" value={data.summary.lost} tone="text-loss-300" />
            <Summary label="Push" value={data.summary.push} />
            <Summary label="Void" value={data.summary.void} />
          </div>
        ) : null}

        <div className="mt-4 flex flex-wrap items-center gap-2">
          <select
            value={status}
            onChange={(event) => setStatus(event.target.value as typeof status)}
            aria-label="Status"
            className="rounded-xl border border-line bg-ink-900 px-3 py-2 text-[12px] text-mist-100"
          >
            <option value="all">All statuses</option>
            <option value="pending">Pending</option>
            <option value="won">Won</option>
            <option value="lost">Lost</option>
            <option value="push">Push</option>
            <option value="void">Void</option>
          </select>
          <select
            value={book}
            onChange={(event) => setBook(event.target.value)}
            aria-label="Bookmaker"
            className="rounded-xl border border-line bg-ink-900 px-3 py-2 text-[12px] text-mist-100"
          >
            <option value="">All bookmakers</option>
            {books.map((name) => (
              <option key={name} value={name}>
                {name}
              </option>
            ))}
          </select>
          <select
            value={competition}
            onChange={(event) => setCompetition(event.target.value)}
            aria-label="Competition"
            className="max-w-[240px] rounded-xl border border-line bg-ink-900 px-3 py-2 text-[12px] text-mist-100"
          >
            <option value="">All competitions</option>
            {competitions.map((name) => (
              <option key={name} value={name}>
                {name}
              </option>
            ))}
          </select>
          <input
            type="search"
            value={query}
            onChange={(event) => setQuery(event.target.value)}
            placeholder="Search team, side, book…"
            className="min-w-[200px] flex-1 rounded-xl border border-line bg-ink-900 px-3 py-2 text-[12px] text-mist-100 placeholder:text-mist-500"
          />
          <span className="font-mono text-[10px] tracking-widest text-mist-500 uppercase">
            {visible.length} of {picks.length}
          </span>
        </div>

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
        ) : visible.length === 0 ? (
          <p className="mt-6 rounded-2xl border border-line bg-ink-900/60 p-5 text-sm text-mist-400">
            No paper pick matches these filters.
          </p>
        ) : (
          <div className="mt-4 space-y-3">
            {visible.map((pick) => (
              <div key={pick.id}>
                {editing?.id === pick.id ? (
                  <div className="mb-1 rounded-xl border border-turf-400/40 bg-ink-950/40 px-3 py-2">
                    <PaperPickForm
                      eventId={pick.event_id}
                      defaultLine={pick.line}
                      editing={{
                        id: pick.id,
                        provider_slug: pick.provider_slug,
                        side: pick.side,
                        line: pick.line,
                        odds: pick.odds,
                      }}
                      onCancel={() => setEditing(null)}
                      onSaved={() => {
                        setEditing(null);
                        reload();
                      }}
                    />
                  </div>
                ) : null}
                <PaperPickCard
                  pick={pick}
                  onEdit={(target) => setEditing(target)}
                  onDelete={(id) => void remove(id)}
                />
              </div>
            ))}
          </div>
        )}

        <section className="mt-6 rounded-2xl border border-line bg-ink-900/50 p-5">
          <h2 className="text-sm font-semibold text-mist-100">
            How settlement works
          </h2>
          <ul className="mt-2 space-y-1.5 text-xs leading-relaxed text-mist-300">
            <li>
              A pick settles only when the match is finished and both teams&apos;
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
