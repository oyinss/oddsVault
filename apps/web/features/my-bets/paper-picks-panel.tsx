"use client";

import Link from "next/link";
import { useMemo, useState } from "react";

import { LoadingNote } from "@/components/loading-note";
import { PaperPickCard } from "@/features/corners/paper-pick-card";
import { formatAge, getPaperPicks, type PaperPick, type PaperPicks } from "@/lib/api";
import { useApiData } from "@/lib/use-api";

// PaperPicksPanel shows the hand-recorded picks inside My Bets, with their own
// filters. They are kept apart from the money journal: no stake, no P/L, and
// they never change the bet totals above.
export function PaperPicksPanel() {
  const { data, loading, error, reload } = useApiData<PaperPicks | null>(
    "my-bets-paper",
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

  if (error || (!loading && data === null)) {
    return (
      <div className="mt-6 rounded-xl border border-flag-400/30 bg-flag-400/[0.04] p-5">
        <p className="text-sm font-medium text-flag-400">Paper picks unavailable</p>
        <p className="mt-1 text-xs text-mist-400">{error}</p>
        <button
          type="button"
          onClick={reload}
          className="mt-3 rounded-lg border border-flag-400/40 px-3 py-1.5 text-xs font-semibold text-flag-400"
        >
          Try again
        </button>
      </div>
    );
  }
  if (loading && !data) {
    return <LoadingNote label="Reading paper picks…" />;
  }

  return (
    <section className="mt-5">
      <div className="flex flex-wrap items-center gap-2">
        <select
          value={status}
          onChange={(event) => setStatus(event.target.value as typeof status)}
          aria-label="Paper pick status"
          className="rounded-lg border border-line bg-ink-900 px-2.5 py-1.5 text-[12px] text-mist-100"
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
          className="rounded-lg border border-line bg-ink-900 px-2.5 py-1.5 text-[12px] text-mist-100"
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
          className="rounded-lg border border-line bg-ink-900 px-2.5 py-1.5 text-[12px] text-mist-100"
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
          className="min-w-[200px] flex-1 rounded-lg border border-line bg-ink-950/40 px-3 py-1.5 text-[12px] text-mist-100 placeholder:text-mist-500"
        />
        <Link
          href="/paper-picks"
          className="font-mono text-[10px] tracking-widest text-turf-300 uppercase hover:text-turf-200"
        >
          manage
        </Link>
      </div>

      <div className="mt-3 grid grid-cols-2 gap-2 sm:grid-cols-5">
        <Stat label="Pending" value={data?.summary.pending ?? 0} />
        <Stat label="Won" value={data?.summary.won ?? 0} tone="text-turf-300" />
        <Stat label="Lost" value={data?.summary.lost ?? 0} tone="text-loss-300" />
        <Stat label="Push" value={data?.summary.push ?? 0} />
        <Stat label="Void" value={data?.summary.void ?? 0} />
      </div>

      {visible.length === 0 ? (
        <p className="mt-4 rounded-xl border border-line bg-ink-900/50 p-5 text-sm text-mist-400">
          No paper picks match these filters. Record one from the{" "}
          <Link href="/corners" className="text-turf-300 hover:text-turf-200">
            corner board
          </Link>
          .
        </p>
      ) : (
        <div className="mt-4 space-y-3">
          {visible.map((pick) => (
            <PaperPickCard key={pick.id} pick={pick} />
          ))}
        </div>
      )}

      <p className="mt-3 text-[11px] leading-relaxed text-mist-500">
        Paper picks carry no stake and never change the journal totals. They
        settle on the final corner count: over needs more than the line, under
        fewer, and an exact hit is a push.
      </p>
    </section>
  );
}

function Stat({
  label,
  value,
  tone = "text-mist-100",
}: {
  label: string;
  value: number;
  tone?: string;
}) {
  return (
    <div className="rounded-lg border border-line bg-ink-950/40 px-3 py-2">
      <p className="font-mono text-[9px] tracking-widest text-mist-500 uppercase">
        {label}
      </p>
      <p className={`mt-0.5 font-mono text-base tabular-nums ${tone}`}>{value}</p>
    </div>
  );
}
