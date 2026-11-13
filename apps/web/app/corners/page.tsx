"use client";

import Link from "next/link";
import { useState } from "react";
import {
  faArrowsRotate,
  faCircleInfo,
  faFlag,
  faTriangleExclamation,
} from "@fortawesome/free-solid-svg-icons";
import { FontAwesomeIcon } from "@fortawesome/react-fontawesome";

import { LoadingNote } from "@/components/loading-note";
import { SiteHeader } from "@/components/site-header";
import { TeamCrest } from "@/components/team-crest";
import {
  absoluteAsset,
  flagUrl,
  formatAge,
  getLiveCorners,
  type CornerSort,
  type LiveCornerMatch,
  type LiveCorners,
} from "@/lib/api";
import { useApiData } from "@/lib/use-api";

const LINE_OPTIONS = [8.5, 9.5, 10, 10.5, 11.5, 12.5];

export default function CornersPage() {
  const [line, setLine] = useState(10);
  const [sort, setSort] = useState<CornerSort>("remaining");
  const [then, setThen] = useState<CornerSort | "">("");
  const sortParam = then ? `${sort},${then}` : sort;
  const { data, loading, error, reload } = useApiData<LiveCorners | null>(
    `live-corners:${line}:${sortParam}`,
    () => getLiveCorners(line, sortParam),
    { refreshMs: 30000 },
  );

  const matches = data?.matches ?? [];

  return (
    <>
      <SiteHeader />
      <main className="mx-auto max-w-5xl px-6 py-12">
        <div className="flex flex-wrap items-end justify-between gap-4">
          <div>
            <p className="font-mono text-[11px] tracking-widest text-turf-300 uppercase">
              Live · Corners
            </p>
            <h1 className="mt-2 text-3xl font-bold tracking-tight">
              Corner pace board
            </h1>
            <p className="mt-2 max-w-2xl text-sm leading-relaxed text-mist-300">
              Corner counts refresh from the live provider every 90 seconds.
              Pace and projection are straight arithmetic from the current count
              and the match clock, so you can compare a match with the line you
              are looking at on the bookmaker.
            </p>
          </div>
          <div className="flex items-center gap-2">
            <label className="flex items-center gap-2 rounded-xl border border-line bg-ink-900 px-3 py-2">
              <span className="font-mono text-[10px] tracking-widest text-mist-500 uppercase">
                Line
              </span>
              <select
                value={line}
                onChange={(event) => setLine(Number(event.target.value))}
                className="bg-transparent font-mono text-sm text-mist-100 outline-none"
              >
                {LINE_OPTIONS.map((option) => (
                  <option key={option} value={option} className="bg-ink-900">
                    {option}
                  </option>
                ))}
              </select>
            </label>
            <label className="flex items-center gap-2 rounded-xl border border-line bg-ink-900 px-3 py-2">
              <span className="font-mono text-[10px] tracking-widest text-mist-500 uppercase">
                Sort
              </span>
              <select
                value={sort}
                onChange={(event) => setSort(event.target.value as CornerSort)}
                className="bg-transparent font-mono text-sm text-mist-100 outline-none"
              >
                <option value="remaining" className="bg-ink-900">Most time left</option>
                <option value="remaining_asc" className="bg-ink-900">Least time left</option>
                <option value="projection" className="bg-ink-900">Highest projection</option>
                <option value="pace" className="bg-ink-900">Highest pace</option>
                <option value="corners" className="bg-ink-900">Most corners</option>
                <option value="competition" className="bg-ink-900">Competition A-Z</option>
              </select>
            </label>
            <label className="flex items-center gap-2 rounded-xl border border-line bg-ink-900 px-3 py-2">
              <span className="font-mono text-[10px] tracking-widest text-mist-500 uppercase">
                Then
              </span>
              <select
                value={then}
                onChange={(event) =>
                  setThen(event.target.value as CornerSort | "")
                }
                className="bg-transparent font-mono text-sm text-mist-100 outline-none"
              >
                <option value="" className="bg-ink-900">None</option>
                <option value="remaining" className="bg-ink-900">Most time left</option>
                <option value="remaining_asc" className="bg-ink-900">Least time left</option>
                <option value="projection" className="bg-ink-900">Highest projection</option>
                <option value="pace" className="bg-ink-900">Highest pace</option>
                <option value="corners" className="bg-ink-900">Most corners</option>
                <option value="competition" className="bg-ink-900">Competition A-Z</option>
              </select>
            </label>
            <button
              type="button"
              onClick={reload}
              title="Refresh corners"
              className="grid size-10 place-items-center rounded-xl border border-line text-mist-400 transition hover:border-white/20 hover:text-mist-100"
            >
              <FontAwesomeIcon icon={faArrowsRotate} className="size-3.5" />
            </button>
          </div>
        </div>

        <p className="mt-3 font-mono text-[10px] tracking-widest text-mist-500 uppercase">
          {data?.generated_at ? formatAge(data.generated_at) : "not loaded"} ·{" "}
          {matches.length} live {matches.length === 1 ? "match" : "matches"}
        </p>

        {error ? (
          <div className="mt-6 rounded-2xl border border-flag-400/30 bg-flag-400/[0.04] p-6">
            <p className="text-sm font-medium text-flag-400">
              Unable to load live corners.
            </p>
            <p className="mt-1 text-xs text-mist-400">{error}</p>
          </div>
        ) : loading && !data ? (
          <LoadingNote label="Reading live matches…" />
        ) : matches.length === 0 ? (
          <div className="mt-6 rounded-2xl border border-line bg-ink-900/60 p-6">
            <p className="text-sm font-medium text-mist-100">
              No football matches are live right now.
            </p>
            <p className="mt-1 max-w-2xl text-xs leading-relaxed text-mist-400">
              This board only lists matches the live provider is covering. It
              fills in as games kick off; nothing is simulated in the meantime.
            </p>
          </div>
        ) : (
          <div className="mt-6 space-y-3">
            {matches.map((match) => (
              <MatchCard key={match.event_id} match={match} line={line} />
            ))}
          </div>
        )}

        <section className="mt-8 rounded-2xl border border-line bg-ink-900/50 p-5">
          <h2 className="flex items-center gap-2 text-sm font-semibold text-mist-100">
            <FontAwesomeIcon icon={faCircleInfo} className="size-3 text-mist-500" />
            How to read the pace
          </h2>
          <ul className="mt-3 grid gap-2 text-xs leading-relaxed text-mist-300 sm:grid-cols-2">
            <li>
              <strong className="text-mist-100">Projected</strong> is current
              corners divided by minutes played, times 90. It is an
              extrapolation, not a prediction.
            </li>
            <li>
              <strong className="text-mist-100">Needed rate</strong> is the
              corners still required to clear the line, divided by the minutes
              left.
            </li>
            <li>
              <strong className="text-mist-100">Above pace</strong> means the
              match is adding corners faster than the line requires. Below pace
              is the reverse.
            </li>
            <li>
              No projection is shown before the 15th minute or when the provider
              has not published corner counts. An absent count is not a zero.
            </li>
          </ul>
        </section>
      </main>
    </>
  );
}

function kickoffClock(iso: string): string {
  const at = new Date(iso);
  if (Number.isNaN(at.valueOf())) return "-";
  return at.toLocaleTimeString("en-GB", {
    hour: "2-digit",
    minute: "2-digit",
  });
}

function MatchCard({ match, line }: { match: LiveCornerMatch; line: number }) {
  const tone =
    match.pace === "above"
      ? "border-turf-400/40 bg-turf-500/[0.06]"
      : match.pace === "below"
        ? "border-flag-400/30 bg-flag-400/[0.04]"
        : "border-line";

  return (
    <article className={`overflow-hidden rounded-2xl border ${tone}`}>
      <header className="flex flex-wrap items-center justify-between gap-2 border-b border-line px-4 py-2.5">
        <div className="flex items-center gap-2">
          <span className="flex items-center gap-1.5 rounded bg-white/[0.06] px-1.5 py-0.5 font-mono text-[9px] tracking-wider text-mist-400 uppercase">
            <CompetitionMark
              logo={match.competition_logo}
              iso={match.competition_country}
              name={match.competition}
            />
            {match.competition}
          </span>
          <span className="flex items-center gap-1.5 font-mono text-[11px] text-red-300">
            <span className="size-1.5 animate-pulse rounded-full bg-red-400" />
            {match.clock || "live"}
          </span>
          <span className="font-mono text-[10px] text-mist-500">
            kickoff {kickoffClock(match.kickoff)}
          </span>
        </div>
        <span className="font-mono text-[10px] text-mist-500">
          corners {formatAge(match.captured_at)}
        </span>
      </header>

      <div className="grid gap-4 px-4 py-3 sm:grid-cols-[minmax(0,1fr)_auto] sm:items-center">
        <div>
          <Link
            href={`/events/detail?id=${match.event_id}`}
            className="text-sm font-semibold text-mist-100 hover:text-turf-300"
          >
            <span className="flex flex-wrap items-center gap-2">
              <TeamCrest
                src={absoluteAsset(match.home.crest_url)}
                name={match.home.name}
                size={16}
              />
              <span>{match.home.name}</span>
              {match.home_score !== null && match.away_score !== null ? (
                <span className="font-mono text-xs text-mist-300">
                  {match.home_score}-{match.away_score}
                </span>
              ) : null}
              <TeamCrest
                src={absoluteAsset(match.away.crest_url)}
                name={match.away.name}
                size={16}
              />
              <span>{match.away.name}</span>
            </span>
          </Link>

          <div className="mt-3 flex flex-wrap items-center gap-2">
            <CornersCell
              label="Home corners"
              title={match.home.name}
              value={match.home_corners}
            />
            <CornersCell
              label="Away corners"
              title={match.away.name}
              value={match.away_corners}
            />
            <CornersCell
              label="Total corners"
              title="Both teams"
              value={match.total_corners}
              strong
            />
          </div>
        </div>

        <div className="grid grid-cols-3 gap-2 sm:w-[300px]">
          <Metric
            label="Pace / min"
            value={
              match.corners_per_minute !== null
                ? match.corners_per_minute.toFixed(2)
                : "-"
            }
          />
          <Metric
            label="Projected"
            value={
              match.projected_total !== null
                ? match.projected_total.toFixed(1)
                : "-"
            }
          />
          <Metric
            label={`Needed for ${line}`}
            value={match.needed !== null ? String(match.needed) : "-"}
            note={
              match.needed_per_minute !== null
                ? `${match.needed_per_minute.toFixed(2)}/min`
                : undefined
            }
          />
        </div>
      </div>

      <footer className="flex items-center justify-between gap-2 border-t border-line px-4 py-2">
        <PaceBadge pace={match.pace} line={line} />
        <span className="font-mono text-[10px] text-mist-500">
          {match.total_corners === null
            ? "corners not published by provider"
            : match.remaining_minutes !== null
              ? `${match.remaining_minutes} min left`
              : "clock unavailable"}
        </span>
      </footer>
    </article>
  );
}

function CompetitionMark({
  logo,
  iso,
  name,
}: {
  logo: string | null;
  iso: string | null;
  name: string;
}) {
  const logoSrc = logo ? absoluteAsset(logo) : null;
  if (logoSrc) {
    return (
      // eslint-disable-next-line @next/next/no-img-element
      <img
        src={logoSrc}
        alt=""
        width={16}
        height={16}
        title={name}
        className="size-4 shrink-0 object-contain"
      />
    );
  }
  const src = flagUrl(iso);
  if (!src) return null;
  return (
    // eslint-disable-next-line @next/next/no-img-element
    <img
      src={src}
      alt=""
      width={16}
      height={11}
      title={name}
      className="h-[11px] w-[16px] shrink-0 rounded-[2px] object-cover"
    />
  );
}

function CornersCell({
  label,
  title,
  value,
  strong,
}: {
  label: string;
  title: string;
  value: number | null;
  strong?: boolean;
}) {
  return (
    <span
      className={`rounded-lg border px-2.5 py-1 ${
        strong
          ? "border-turf-400/30 bg-turf-500/10"
          : "border-line bg-white/[0.02]"
      }`}
      title={title}
    >
      <span className="font-mono text-[9px] tracking-wide text-mist-500 uppercase">
        {label}
      </span>
      <span
        className={`ml-2 font-mono tabular-nums ${
          strong ? "text-sm text-turf-200" : "text-sm text-mist-100"
        }`}
      >
        {value !== null ? value : "-"}
      </span>
    </span>
  );
}

function Metric({
  label,
  value,
  note,
}: {
  label: string;
  value: string;
  note?: string;
}) {
  return (
    <div className="rounded-lg border border-line bg-ink-950/40 px-2.5 py-1.5">
      <p className="font-mono text-[9px] tracking-wide text-mist-500 uppercase">
        {label}
      </p>
      <p className="font-mono text-sm tabular-nums text-mist-100">{value}</p>
      {note ? (
        <p className="font-mono text-[9px] text-mist-500">{note}</p>
      ) : null}
    </div>
  );
}

function PaceBadge({
  pace,
  line,
}: {
  pace: LiveCornerMatch["pace"];
  line: number;
}) {
  if (pace === "above") {
    return (
      <span className="flex items-center gap-1.5 font-mono text-[10px] tracking-widest text-turf-300 uppercase">
        <FontAwesomeIcon icon={faFlag} className="size-2.5" />
        above the rate needed for {line}
      </span>
    );
  }
  if (pace === "below") {
    return (
      <span className="flex items-center gap-1.5 font-mono text-[10px] tracking-widest text-flag-400 uppercase">
        <FontAwesomeIcon icon={faTriangleExclamation} className="size-2.5" />
        below the rate needed for {line}
      </span>
    );
  }
  if (pace === "on_pace") {
    return (
      <span className="font-mono text-[10px] tracking-widest text-mist-400 uppercase">
        on the rate needed for {line}
      </span>
    );
  }
  return (
    <span className="font-mono text-[10px] tracking-widest text-mist-500 uppercase">
      pace unavailable
    </span>
  );
}
