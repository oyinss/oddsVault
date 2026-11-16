"use client";

import Link from "next/link";
import { useState } from "react";
import {
  faArrowDownWideShort,
  faArrowsRotate,
  faBell,
  faBellSlash,
  faBullseye,
  faChartLine,
  faCircleInfo,
  faClockRotateLeft,
  faFilePen,
  faFlag,
  faGaugeHigh,
  faLayerGroup,
  faMagnifyingGlass,
  faRulerHorizontal,
  faStar,
  faXmark,
  faTriangleExclamation,
  type IconDefinition,
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
import { useCornerAlerts } from "@/features/corners/use-corner-alerts";
import { useCornerFavorites } from "@/features/corners/use-corner-favorites";
import { PaperPickForm } from "@/features/corners/paper-pick-form";

const LINE_OPTIONS: { value: number; hint: string }[] = [
  { value: 8.5, hint: "overs" },
  { value: 9.5, hint: "overs" },
  { value: 10, hint: "median" },
  { value: 10.5, hint: "" },
  { value: 11.5, hint: "unders" },
  { value: 12.5, hint: "unders" },
];

export default function CornersPage() {
  const [line, setLine] = useState(9.5);
  const [search, setSearch] = useState("");
  const [sort, setSort] = useState<CornerSort>("remaining");
  const [then, setThen] = useState<CornerSort | "">("");
  const sortParam = then ? `${sort},${then}` : sort;
  const { data, loading, error, reload } = useApiData<LiveCorners | null>(
    `live-corners:${line}:${sortParam}`,
    () => getLiveCorners(line, sortParam),
    { refreshMs: 30000 },
  );

  const matches = data?.matches ?? [];
  const alerts = useCornerAlerts(matches, line);
  const favorites = useCornerFavorites(matches);
  const [onlyFollowed, setOnlyFollowed] = useState(false);
  const [onlyRecorded, setOnlyRecorded] = useState(false);
  const needle = search.trim().toLowerCase();
  const searched = needle
    ? matches.filter((match) =>
        [match.home.name, match.away.name, match.competition]
          .join(" ")
          .toLowerCase()
          .includes(needle),
      )
    : matches;
  const recordedCount = matches.filter(
    (match) => (match.paper_picks?.length ?? 0) > 0,
  ).length;
  const visible = searched
    .filter((match) => !onlyFollowed || favorites.isFollowed(match.event_id))
    .filter(
      (match) => !onlyRecorded || (match.paper_picks?.length ?? 0) > 0,
    );
  // A followed match leaves the live list when it ends. Keep it visible with
  // the last count we saw instead of pretending it was never followed.
  const offBoard = favorites.list.filter(
    (item) => !matches.some((match) => match.event_id === item.eventId),
  );
  const [editingId, setEditingId] = useState<number | null>(null);
  const [draft, setDraft] = useState("10");
  const [recordingId, setRecordingId] = useState<number | null>(null);

  function openEditor(match: LiveCornerMatch) {
    const current = match.total_corners ?? 0;
    const existing = alerts.watches.find(
      (watch) => watch.eventId === match.event_id,
    );
    setDraft(String(existing?.threshold ?? Math.max(line, current + 1)));
    setEditingId(match.event_id);
  }

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
          <div className="flex w-full flex-wrap items-center gap-2 sm:w-auto">
            <label className="flex items-center gap-2 rounded-xl border border-line bg-ink-900 px-3 py-2">
              <span className="flex items-center gap-1.5 font-mono text-[10px] tracking-widest text-mist-500 uppercase">
                <FontAwesomeIcon icon={faRulerHorizontal} className="size-3" />
                Line
              </span>
              <select
                value={line}
                onChange={(event) => setLine(Number(event.target.value))}
                className="bg-transparent font-mono text-sm text-mist-100 outline-none"
              >
                {LINE_OPTIONS.map((option) => (
                  <option key={option.value} value={option.value} className="bg-ink-900">
                    {option.value} {option.hint ? `· ${option.hint}` : ""}
                  </option>
                ))}
              </select>
            </label>
            <label className="flex items-center gap-2 rounded-xl border border-line bg-ink-900 px-3 py-2">
              <span className="flex items-center gap-1.5 font-mono text-[10px] tracking-widest text-mist-500 uppercase">
                <FontAwesomeIcon icon={faArrowDownWideShort} className="size-3" />
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
                <option value="played" className="bg-ink-900">Most journal legs</option>
                <option value="played_league" className="bg-ink-900">Leagues I have played first</option>
                <option value="played_teams" className="bg-ink-900">Teams I have played first</option>
              </select>
            </label>
            <label className="flex items-center gap-2 rounded-xl border border-line bg-ink-900 px-3 py-2">
              <span className="flex items-center gap-1.5 font-mono text-[10px] tracking-widest text-mist-500 uppercase">
                <FontAwesomeIcon icon={faLayerGroup} className="size-3" />
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
                <option value="played" className="bg-ink-900">Most journal legs</option>
                <option value="played_league" className="bg-ink-900">Leagues I have played first</option>
                <option value="played_teams" className="bg-ink-900">Teams I have played first</option>
              </select>
            </label>
            <div className="relative w-full sm:w-auto sm:min-w-[200px] sm:flex-1">
              <FontAwesomeIcon
                icon={faMagnifyingGlass}
                className="pointer-events-none absolute top-1/2 left-3 size-3 -translate-y-1/2 text-mist-500"
              />
              <input
                value={search}
                onChange={(event) => setSearch(event.target.value)}
                placeholder="Search team or competition"
                aria-label="Search live matches"
                className="w-full rounded-xl border border-line bg-ink-900 py-2 pr-9 pl-9 text-sm text-mist-100 placeholder:text-mist-500 focus:border-turf-400/50 focus:outline-none"
              />
              {search && (
                <button
                  type="button"
                  onClick={() => setSearch("")}
                  aria-label="Clear search"
                  className="absolute top-1/2 right-2 grid size-6 -translate-y-1/2 place-items-center rounded-md text-mist-500 hover:bg-raised hover:text-mist-200"
                >
                  <FontAwesomeIcon icon={faXmark} className="size-2.5" />
                </button>
              )}
            </div>
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

        <div className="mt-3 flex flex-wrap items-center gap-3">
          <p className="font-mono text-[10px] tracking-widest text-mist-500 uppercase">
            {data?.generated_at ? formatAge(data.generated_at) : "not loaded"} ·{" "}
            {needle
              ? `${visible.length} of ${matches.length} matches`
              : `${matches.length} live ${matches.length === 1 ? "match" : "matches"}`}
          </p>
          {alerts.permission !== "granted" && (
            <button
              type="button"
              onClick={() => void alerts.requestPermission()}
              className="inline-flex items-center gap-1.5 rounded-lg border border-turf-400/40 bg-turf-500/10 px-3 py-1.5 font-mono text-[10px] tracking-widest text-turf-200 uppercase transition hover:bg-turf-500/20"
              title="Alerts fire while this page is open"
            >
              <FontAwesomeIcon icon={faBell} className="size-3" />
              {alerts.permission === "unsupported"
                ? "alerts on this page only"
                : "enable corner alerts"}
            </button>
          )}
          <span className="font-mono text-[10px] text-mist-500">
            {alerts.watches.length} watched · use the bell on a match to add one
          </span>
          <Link
            href="/paper-picks"
            className="inline-flex items-center gap-1.5 rounded-lg border border-line px-3 py-1.5 font-mono text-[10px] tracking-widest text-mist-400 uppercase transition hover:border-turf-400/40 hover:text-turf-200"
            title="Picks recorded by hand, no stake, settled at full time"
          >
            <FontAwesomeIcon icon={faFilePen} className="size-3" />
            paper picks
          </Link>
          <button
            type="button"
            onClick={() => setOnlyRecorded((value) => !value)}
            aria-pressed={onlyRecorded}
            title={
              onlyRecorded
                ? "Show every live match again"
                : "Show only matches with a recorded paper pick"
            }
            className={`inline-flex items-center gap-1.5 rounded-lg border px-3 py-1.5 font-mono text-[10px] tracking-widest uppercase transition ${
              onlyRecorded
                ? "border-turf-400/50 bg-turf-500/10 text-turf-200"
                : "border-line text-mist-400 hover:border-turf-400/40 hover:text-turf-200"
            }`}
          >
            <FontAwesomeIcon icon={faFilePen} className="size-3" />
            recorded {recordedCount}
          </button>
          <button
            type="button"
            onClick={() => setOnlyFollowed((value) => !value)}
            aria-pressed={onlyFollowed}
            title={
              onlyFollowed
                ? "Show every live match again"
                : "Show only the matches you follow"
            }
            className={`inline-flex items-center gap-1.5 rounded-lg border px-3 py-1.5 font-mono text-[10px] tracking-widest uppercase transition ${
              onlyFollowed
                ? "border-turf-400/50 bg-turf-500/10 text-turf-200"
                : "border-line text-mist-400 hover:border-turf-400/40 hover:text-turf-200"
            }`}
          >
            <FontAwesomeIcon
              icon={faStar}
              className={`size-3 ${onlyFollowed ? "text-turf-300" : ""}`}
            />
            following {favorites.count}
          </button>
        </div>

        {error || (!loading && data === null) ? (
          <div className="mt-6 rounded-2xl border border-flag-400/30 bg-flag-400/[0.04] p-6">
            <p className="text-sm font-medium text-flag-400">
              Unable to load live corners.
            </p>
            <p className="mt-1 text-xs text-mist-400">
              {error ?? "The API did not respond. This is usually the tunnel or the network, not an empty board."}
            </p>
            <button
              type="button"
              onClick={reload}
              className="mt-3 inline-flex items-center gap-2 rounded-lg border border-flag-400/40 px-3 py-1.5 text-xs font-semibold text-flag-400 transition hover:bg-flag-400/10"
            >
              <FontAwesomeIcon icon={faArrowsRotate} className="size-3" />
              Try again
            </button>
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
        ) : onlyRecorded && visible.length === 0 ? (
          <div className="mt-6 rounded-2xl border border-line bg-ink-900/60 p-6">
            <p className="text-sm font-medium text-mist-100">
              No live match has a recorded paper pick right now.
            </p>
            <p className="mt-1 max-w-2xl text-xs leading-relaxed text-mist-400">
              Record one with the record button on a card, or open the paper
              picks page to see settled ones.
            </p>
            <button
              type="button"
              onClick={() => setOnlyRecorded(false)}
              className="mt-3 rounded-lg border border-line px-3 py-1.5 text-xs font-semibold text-mist-200 hover:border-white/20 hover:text-mist-100"
            >
              Show all live matches
            </button>
          </div>
        ) : onlyFollowed && visible.length === 0 ? (
          <div className="mt-6 rounded-2xl border border-line bg-ink-900/60 p-6">
            <p className="text-sm font-medium text-mist-100">
              None of your followed matches is live right now.
            </p>
            <p className="mt-1 max-w-2xl text-xs leading-relaxed text-mist-400">
              Followed matches are stored in this browser. When one leaves the
              live board it stays in the list below with the last corner count
              we saw, so you can still open its detail page.
            </p>
            <button
              type="button"
              onClick={() => setOnlyFollowed(false)}
              className="mt-3 rounded-lg border border-line px-3 py-1.5 text-xs font-semibold text-mist-200 hover:border-white/20 hover:text-mist-100"
            >
              Show all live matches
            </button>
          </div>
        ) : visible.length === 0 ? (
          <div className="mt-6 rounded-2xl border border-line bg-ink-900/60 p-6">
            <p className="text-sm font-medium text-mist-100">
              No live match matches “{search.trim()}”.
            </p>
            <p className="mt-1 text-xs text-mist-400">
              The search covers team and competition names on the live board
              only. Clear it to see all {matches.length} matches.
            </p>
            <button
              type="button"
              onClick={() => setSearch("")}
              className="mt-3 rounded-lg border border-line px-3 py-1.5 text-xs font-semibold text-mist-200 hover:border-white/20 hover:text-mist-100"
            >
              Clear search
            </button>
          </div>
        ) : (
          <div className="mt-6 space-y-3">
            {visible.map((match) => (
              <MatchCard
                key={match.event_id}
                match={match}
                line={line}
                watched={alerts.isWatched(match.event_id)}
                followed={favorites.isFollowed(match.event_id)}
                onToggleFollow={() => favorites.toggle(match)}
                threshold={
                  alerts.watches.find((watch) => watch.eventId === match.event_id)
                    ?.threshold ?? null
                }
                editing={editingId === match.event_id}
                draft={draft}
                onOpenEditor={() => openEditor(match)}
                onDraft={setDraft}
                onClose={() => setEditingId(null)}
                onSave={() => {
                  void alerts.setWatch(match, Number(draft));
                  setEditingId(null);
                }}
                onRemove={() => {
                  alerts.removeWatch(match.event_id);
                  setEditingId(null);
                }}
                recording={recordingId === match.event_id}
                onOpenRecord={() =>
                  setRecordingId((value) =>
                    value === match.event_id ? null : match.event_id,
                  )
                }
                onCloseRecord={() => setRecordingId(null)}
              />
            ))}
          </div>
        )}

        {onlyFollowed && offBoard.length > 0 && (
          <section className="mt-6 rounded-2xl border border-line bg-ink-900/50 p-5">
            <h2 className="flex items-center gap-2 text-sm font-semibold text-mist-100">
              <FontAwesomeIcon icon={faStar} className="size-3 text-turf-300" />
              Followed, no longer live
            </h2>
            <p className="mt-1 text-xs text-mist-500">
              These left the live board. The count and minute are the last ones
              we recorded before it happened.
            </p>
            <ul className="mt-3 divide-y divide-white/[0.05]">
              {offBoard.map((item) => (
                <li
                  key={item.eventId}
                  className="flex flex-wrap items-center justify-between gap-2 py-2"
                >
                  <div className="min-w-0">
                    <Link
                      href={`/corners/detail?id=${item.eventId}&line=${line}`}
                      className="text-sm font-semibold text-mist-100 hover:text-turf-300"
                    >
                      {item.home} v {item.away}
                    </Link>
                    <p className="font-mono text-[10px] text-mist-500">
                      {item.competition}
                    </p>
                  </div>
                  <div className="flex items-center gap-3 font-mono text-[10px] text-mist-400">
                    <span>
                      last seen{" "}
                      {item.lastTotal !== null ? `${item.lastTotal} corners` : "no count"}
                      {item.lastMinute !== null ? ` at ${item.lastMinute}'` : ""}
                    </span>
                    <span className="text-mist-500">{formatAge(item.seenAt)}</span>
                    <button
                      type="button"
                      onClick={() => favorites.unfollow(item.eventId)}
                      className="rounded-md border border-line px-2 py-1 tracking-widest text-mist-400 uppercase hover:border-loss-400/40 hover:text-loss-300"
                    >
                      unfollow
                    </button>
                  </div>
                </li>
              ))}
            </ul>
          </section>
        )}

        <section className="mt-8 rounded-2xl border border-line bg-ink-900/50 p-5">
          <h2 className="text-sm font-semibold text-mist-100">Line guide</h2>
          <p className="mt-1 text-xs text-mist-500">
            Base rates from 338 finished matches with final corner counts. This
            is the neutral reference, before any team or live read.
          </p>
          <div className="mt-3 grid grid-cols-3 gap-1.5 sm:grid-cols-6">
            {[
              { line: 8.5, over: 61 },
              { line: 9.5, over: 48 },
              { line: 10.5, over: 38 },
              { line: 11.5, over: 30 },
              { line: 12.5, over: 22 },
              { line: 13.5, over: 16 },
            ].map((row) => (
              <div
                key={row.line}
                className={`rounded-lg border px-2 py-1.5 ${
                  row.line === line
                    ? "border-turf-400/50 bg-turf-500/10"
                    : "border-line bg-ink-950/40"
                }`}
              >
                <p className="font-mono text-[9px] tracking-widest text-mist-500 uppercase">
                  {row.line}
                </p>
                <p className="font-mono text-sm tabular-nums text-mist-100">
                  {row.over}%
                </p>
                <p className="font-mono text-[9px] text-mist-500 uppercase">
                  over
                </p>
              </div>
            ))}
          </div>
          <ul className="mt-3 space-y-1 text-[11px] leading-relaxed text-mist-300">
            <li>
              <strong className="text-mist-100">Overs</strong> are priced sensibly
              at 8.5 or 9.5; over 10.5 lands only 38% of the time.
            </li>
            <li>
              <strong className="text-mist-100">Unders</strong> belong at 11.5 or
              12.5. Line 10 is the median, so it carries no edge either way.
            </li>
            <li>
              <strong className="text-mist-100">At 60 minutes:</strong> if the
              favourite leads, unders are the side (21% of those matches went over
              10.5); if it is level or behind, overs (62% went over 9.5).
            </li>
          </ul>
        </section>

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
              Pace is shown from the first minute but labelled early before the
              15th. No projection is shown until then, or when the provider has
              not published corner counts. An absent count is not a zero.
            </li>
          </ul>
        </section>
        {alerts.notice && (
          <div className="fixed inset-x-4 bottom-4 z-50 mx-auto flex max-w-md items-start gap-3 rounded-xl border border-turf-400/40 bg-ink-900 px-4 py-3 shadow-2xl">
            <FontAwesomeIcon icon={faBell} className="mt-0.5 size-3.5 text-turf-300" />
            <p className="flex-1 text-xs leading-relaxed text-mist-200">
              {alerts.notice}
            </p>
            <button
              type="button"
              onClick={alerts.dismissNotice}
              className="font-mono text-[10px] tracking-widest text-mist-500 uppercase hover:text-mist-200"
            >
              close
            </button>
          </div>
        )}
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

function MatchCard({
  match,
  line,
  watched,
  followed,
  onToggleFollow,
  threshold,
  editing,
  draft,
  onOpenEditor,
  onDraft,
  onClose,
  onSave,
  onRemove,
  recording,
  onOpenRecord,
  onCloseRecord,
}: {
  match: LiveCornerMatch;
  line: number;
  watched: boolean;
  followed: boolean;
  onToggleFollow: () => void;
  threshold: number | null;
  editing: boolean;
  draft: string;
  onOpenEditor: () => void;
  onDraft: (value: string) => void;
  onClose: () => void;
  onSave: () => void;
  onRemove: () => void;
  recording: boolean;
  onOpenRecord: () => void;
  onCloseRecord: () => void;
}) {
  const tone =
    match.pace === "above"
      ? "border-turf-400/40 bg-turf-500/[0.06]"
      : match.pace === "below"
        ? "border-flag-400/30 bg-flag-400/[0.04]"
        : "border-line";

  return (
    <article
      className={`rounded-2xl border ${tone} ${
        followed ? "ring-1 ring-turf-400/25" : ""
      }`}
    >
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
        <div className="flex items-center gap-2">
          <span className="font-mono text-[10px] text-mist-500">
            corners {formatAge(match.captured_at)}
          </span>
          <button
            type="button"
            onClick={onToggleFollow}
            aria-pressed={followed}
            title={
              followed
                ? "Stop following this match"
                : "Follow this match so it keeps its place on this board"
            }
            className={`grid size-7 place-items-center rounded-md border transition ${
              followed
                ? "border-turf-400/50 bg-turf-500/10 text-turf-300"
                : "border-line text-mist-500 hover:border-turf-400/40 hover:text-turf-200"
            }`}
          >
            <FontAwesomeIcon icon={faStar} className="size-3" />
          </button>
          <button
            type="button"
            onClick={onOpenRecord}
            aria-pressed={recording}
            title="Record a paper pick: side, line and odds, settled at full time"
            className={`inline-flex items-center gap-1.5 rounded-md border px-2 py-1 font-mono text-[9px] tracking-widest uppercase transition ${
              recording
                ? "border-turf-400/40 bg-turf-500/10 text-turf-300"
                : "border-line text-mist-400 hover:border-turf-400/40 hover:text-turf-200"
            }`}
          >
            <FontAwesomeIcon icon={faFilePen} className="size-2.5" />
            record
          </button>
          <button
            type="button"
            onClick={onOpenEditor}
            aria-pressed={watched}
            title={
              watched
                ? `Alert at ${threshold} corners. Click to stop watching.`
                : "Alert me when the total corners reaches the line"
            }
            className={`inline-flex items-center gap-1.5 rounded-md border px-2 py-1 font-mono text-[9px] tracking-widest uppercase transition ${
              watched
                ? "border-turf-400/40 bg-turf-500/10 text-turf-300"
                : "border-line text-mist-400 hover:border-turf-400/40 hover:text-turf-200"
            }`}
          >
            <FontAwesomeIcon
              icon={watched ? faBell : faBellSlash}
              className="size-2.5"
            />
            {watched && threshold !== null ? `alert ${threshold}` : "notify"}
          </button>
        </div>
      </header>

      {editing && (
        <div className="flex flex-wrap items-center gap-2 border-b border-line bg-ink-950/40 px-4 py-2">
          <span className="font-mono text-[10px] tracking-widest text-mist-500 uppercase">
            Alert at
          </span>
          <input
            type="number"
            min={1}
            max={40}
            value={draft}
            onChange={(event) => onDraft(event.target.value)}
            aria-label="Corner threshold"
            className="w-16 rounded-md border border-line bg-ink-900 px-2 py-1 font-mono text-sm text-mist-100 outline-none focus:border-turf-400/50"
          />
          <span className="font-mono text-[10px] text-mist-500 uppercase">corners</span>
          <button
            type="button"
            onClick={onSave}
            className="rounded-md border border-turf-400/40 bg-turf-500/10 px-2.5 py-1 font-mono text-[10px] tracking-widest text-turf-200 uppercase hover:bg-turf-500/20"
          >
            {watched ? "update" : "save"}
          </button>
          {watched && (
            <button
              type="button"
              onClick={onRemove}
              className="rounded-md border border-loss-400/30 px-2.5 py-1 font-mono text-[10px] tracking-widest text-loss-300 uppercase hover:bg-loss-400/10"
            >
              remove
            </button>
          )}
          <button
            type="button"
            onClick={onClose}
            className="rounded-md border border-line px-2.5 py-1 font-mono text-[10px] tracking-widest text-mist-400 uppercase hover:text-mist-200"
          >
            cancel
          </button>
          <span className="text-[10px] text-mist-500">
            Defaults to the board line, or the next corner if the match is already past it.
          </span>
        </div>
      )}

      {recording && (
        <div className="border-b border-line bg-ink-950/40 px-4 py-2">
          <PaperPickForm
            eventId={match.event_id}
            defaultLine={line}
            onCancel={onCloseRecord}
          />
        </div>
      )}

      <div className="grid min-w-0 gap-4 px-4 py-3 sm:grid-cols-[minmax(0,1fr)_auto] sm:items-center">
        <div>
          <Link
            href={`/corners/detail?id=${match.event_id}&line=${line}`}
            title="Open match detail"
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

        <div className="grid min-w-0 grid-cols-2 gap-1.5 sm:w-[460px] sm:gap-2">
          <Metric
            label="Pace"
            icon={faGaugeHigh}
            hint="Corners per minute so far: total corners divided by minutes played. Shown from the first minute; before the 15th it is early and noisy."
            value={
              match.corners_per_minute !== null
                ? match.corners_per_minute.toFixed(2)
                : "-"
            }
            note={
              match.early_pace
                ? "early"
                : match.corners_per_minute === null
                  ? "no count"
                  : undefined
            }
          />
          <Metric
            label="Proj."
            icon={faChartLine}
            hint="Current corners divided by minutes played, multiplied by 90. A straight extrapolation of the match so far, not a prediction."
            value={
              match.projected_total !== null
                ? match.projected_total.toFixed(1)
                : "-"
            }
            note={
              match.projection_low !== null && match.projection_high !== null
                ? `${match.projection_low.toFixed(0)}-${match.projection_high.toFixed(0)} likely`
                : undefined
            }
          />
          <Metric
            label="Over line"
            icon={faFlag}
            hint="Highest over line that still sits below the low end of the likely range. If it shows a dash, the range does not clear a standard line."
            value={
              match.rec_over_line !== null ? match.rec_over_line.toFixed(1) : "-"
            }
            note={
              match.rec_over_line !== null && match.total_corners !== null
                ? `${Math.max(0, Math.ceil(match.rec_over_line) - match.total_corners)} needed`
                : undefined
            }
          />
          <Metric
            label="Under line"
            icon={faFlag}
            hint="Lowest under line that still sits above the high end of the likely range. If it shows a dash, the range does not clear a standard line."
            value={
              match.rec_under_line !== null ? match.rec_under_line.toFixed(1) : "-"
            }
            note={
              match.rec_under_line !== null && match.total_corners !== null
                ? `${Math.max(0, Math.floor(match.rec_under_line) - match.total_corners)} room`
                : undefined
            }
          />
        </div>
      </div>

      <footer className="flex flex-wrap items-center justify-between gap-2 border-t border-line px-4 py-2">
        <span className="flex flex-wrap items-center gap-2">
          {match.paper_picks?.map((pick, index) => (
            <span
              key={`${pick.side}-${pick.line}-${index}`}
              title={`${pick.provider} ${pick.side} ${pick.line} at ${pick.odds}. Settles on the final corner count.`}
              className="inline-flex items-center gap-1.5 rounded border border-turf-400/40 bg-turf-500/10 px-1.5 py-0.5 font-mono text-[9px] font-bold tracking-widest text-turf-200 uppercase"
            >
              <FontAwesomeIcon icon={faFilePen} className="size-2.5" />
              {pick.side === "under" ? "U" : "O"} {pick.line}
            </span>
          ))}
          {match.bet_history ? (
            <Link
              href={`/corners/detail?id=${match.event_id}&line=${line}`}
              title={`${match.bet_history.legs} linked corner legs from your journal on this league or these teams. Over ${match.bet_history.over_won}W-${match.bet_history.over_lost}L, Under ${match.bet_history.under_won}W-${match.bet_history.under_lost}L.`}
              className="inline-flex items-center gap-1.5 rounded border border-line px-1.5 py-0.5 font-mono text-[9px] font-bold tracking-widest text-mist-200 uppercase transition hover:border-turf-400/40 hover:text-turf-100"
            >
              <FontAwesomeIcon
                icon={faClockRotateLeft}
                className="size-2.5 text-mist-500"
              />
              played {match.bet_history.won}W-{match.bet_history.lost}L
              {match.bet_history.open + match.bet_history.unsettled > 0
                ? ` · ${match.bet_history.open + match.bet_history.unsettled} open`
                : ""}
            </Link>
          ) : null}
          <PaceBadge pace={match.pace} line={line} early={match.early_pace} />
          {match.model_risk && (
            <span
              className="inline-flex items-center gap-1 rounded border border-flag-400/40 px-1.5 py-0.5 font-mono text-[9px] tracking-widest text-flag-400 uppercase"
              title={match.model_risk_reason}
            >
              <FontAwesomeIcon icon={faTriangleExclamation} className="size-2.5" />
              early projection
            </span>
          )}
        </span>
        <span className="flex items-center gap-3 font-mono text-[10px] text-mist-500">
          <Link
            href={`/corners/detail?id=${match.event_id}&line=${line}`}
            className="text-turf-300 hover:text-turf-200"
          >
            details
          </Link>
          <span>
          {watched && threshold !== null ? `alert at ${threshold} corners · ` : ""}
          {match.total_corners === null
            ? "corners not published by provider"
            : match.remaining_minutes !== null
              ? `${match.remaining_minutes} min left`
              : "clock unavailable"}
          </span>
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
  icon,
  hint,
  value,
  note,
}: {
  label: string;
  icon: IconDefinition;
  hint: string;
  value: string;
  note?: string;
}) {
  return (
    <div className="min-w-0 rounded-lg border border-line bg-ink-950/40 px-2 py-1.5 sm:px-2.5">
      <p className="flex items-center gap-1.5 font-mono text-[9px] tracking-wide text-mist-500 uppercase">
        <FontAwesomeIcon icon={icon} className="size-2.5 shrink-0" />
        <span className="truncate">{label}</span>
        <Hint text={hint} />
      </p>
      <p className="font-mono text-sm tabular-nums text-mist-100">{value}</p>
      {note ? (
        <p className="font-mono text-[9px] text-mist-500">{note}</p>
      ) : null}
    </div>
  );
}

/** Small hover/focus explanation attached to a metric label. */
function Hint({ text }: { text: string }) {
  return (
    <span className="group relative inline-flex shrink-0" tabIndex={0} title={text}>
      <FontAwesomeIcon
        icon={faCircleInfo}
        className="size-2.5 text-mist-500 transition group-hover:text-mist-200 group-focus:text-mist-200"
      />
      <span
        role="tooltip"
        className="pointer-events-none absolute bottom-full left-1/2 z-30 mb-2 w-56 -translate-x-1/2 rounded-lg border border-line-strong bg-ink-900 p-2.5 text-[10px] leading-relaxed font-normal tracking-normal text-mist-300 normal-case opacity-0 shadow-2xl transition-opacity group-hover:opacity-100 group-focus:opacity-100"
      >
        {text}
      </span>
    </span>
  );
}

function PaceBadge({
  pace,
  line,
  early,
}: {
  pace: LiveCornerMatch["pace"];
  line: number;
  early: boolean;
}) {
  if (early) {
    return (
      <span className="font-mono text-[10px] tracking-widest text-mist-400 uppercase">
        early pace
      </span>
    );
  }
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
