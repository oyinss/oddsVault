"use client";

import Link from "next/link";
import { useEffect, useState } from "react";
import {
  faArrowDownWideShort,
  faArrowTrendUp,
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
  faFutbol,
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
  type BoardTeamForm,
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
  const { data, loading, error, reload, cachedAt } = useApiData<LiveCorners | null>(
    `live-corners:${line}:${sortParam}`,
    () => getLiveCorners(line, sortParam),
    { refreshMs: 30000 },
  );

  const matches = data?.matches ?? [];
  const alerts = useCornerAlerts(matches, line);
  const favorites = useCornerFavorites(matches);
  const [onlyFollowed, setOnlyFollowed] = useState(false);
  // Storage keeps a follow after the match ends, so count only the ones still
  // on the live board; the board itself no longer lists finished games.
  const liveFollowed = matches.filter((match) =>
    favorites.isFollowed(match.event_id),
  ).length;
  const [onlyRecorded, setOnlyRecorded] = useState(false);
  const [onlyPlayed, setOnlyPlayed] = useState(false);
  const [pickNotice, setPickNotice] = useState<string | null>(null);
  const [onlySide, setOnlySide] = useState<"none" | "over" | "under">("none");
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
  const overLineCount = matches.filter(
    (match) => match.rec_over_line !== null,
  ).length;
  const underLineCount = matches.filter(
    (match) => match.rec_under_line !== null,
  ).length;
  const playedCount = matches.filter(
    (match) => (match.bet_history?.legs ?? 0) > 0,
  ).length;
  const visible = searched
    .filter((match) => !onlyFollowed || favorites.isFollowed(match.event_id))
    .filter(
      (match) => !onlyRecorded || (match.paper_picks?.length ?? 0) > 0,
    )
    .filter(
      (match) => !onlyPlayed || (match.bet_history?.legs ?? 0) > 0,
    )
    .filter((match) =>
      onlySide === "over"
        ? match.rec_over_line !== null
        : onlySide === "under"
          ? match.rec_under_line !== null
          : true,
    );
  const [editingId, setEditingId] = useState<number | null>(null);
  const [draft, setDraft] = useState("10");
  const [recordingId, setRecordingId] = useState<number | null>(null);

  useEffect(() => {
    if (!pickNotice) return;
    const timer = setTimeout(() => setPickNotice(null), 6000);
    return () => clearTimeout(timer);
  }, [pickNotice]);

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
      <main className="w-full px-4 py-8 sm:px-6 lg:px-8">
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
              are looking at on the bookmaker. Matches whose provider publishes
              no corner count are not listed.
            </p>
          </div>
          <div className="flex w-full flex-wrap items-center gap-2 sm:w-auto">
            <label className="flex items-center gap-2 rounded-xl border border-line bg-ink-900 px-3 py-2">
              <span className="flex items-center gap-1.5 font-mono text-[10px] tracking-widest text-mist-500 uppercase">
                <FontAwesomeIcon icon={faRulerHorizontal} className="size-3" />
                Board line
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
          <span className="font-mono text-[10px] text-mist-500">
            {alerts.watches.length} watched · use the bell on a match to add one
          </span>
        </div>

        <div className="mt-3 grid grid-cols-2 gap-2 sm:grid-cols-3 lg:grid-cols-5">
          <FilterCard
            label="Recorded"
            value={recordedCount}
            icon={faFilePen}
            active={onlyRecorded}
            hint={
              onlyRecorded
                ? "Show every live match again"
                : "Show only matches with a recorded paper pick"
            }
            onToggle={() => setOnlyRecorded((value) => !value)}
          />
          <FilterCard
            label="Over lines"
            value={overLineCount}
            icon={faFlag}
            active={onlySide === "over"}
            hint="Show only matches where the range clears an over line"
            onToggle={() =>
              setOnlySide((value) => (value === "over" ? "none" : "over"))
            }
          />
          <FilterCard
            label="Under lines"
            value={underLineCount}
            icon={faBullseye}
            active={onlySide === "under"}
            hint="Show only matches where the range clears an under line"
            onToggle={() =>
              setOnlySide((value) => (value === "under" ? "none" : "under"))
            }
          />
          <FilterCard
            label="Played"
            value={playedCount}
            icon={faClockRotateLeft}
            active={onlyPlayed}
            hint="Show only matches where your journal has a corner leg on this league or either team"
            onToggle={() => setOnlyPlayed((value) => !value)}
          />
          <FilterCard
            label="Following"
            value={liveFollowed}
            icon={faStar}
            active={onlyFollowed}
            hint={
              onlyFollowed
                ? "Show every live match again"
                : "Show only the matches you follow"
            }
            onToggle={() => setOnlyFollowed((value) => !value)}
          />
        </div>

        {error && cachedAt ? (
          <div className="mt-3 flex flex-wrap items-center gap-3 rounded-xl border border-flag-400/30 bg-flag-400/[0.04] px-3 py-2">
            <FontAwesomeIcon
              icon={faCircleInfo}
              className="size-3 text-flag-400"
            />
            <p className="flex-1 text-xs leading-relaxed text-mist-300">
              The API is not answering, so this is the last board we saved in
              this browser, {formatAge(cachedAt)}. Counts and prices in it are
              as old as that stamp.
            </p>
            <button
              type="button"
              onClick={reload}
              className="rounded-lg border border-flag-400/40 px-2.5 py-1 font-mono text-[10px] tracking-widest text-flag-400 uppercase transition hover:bg-flag-400/10"
            >
              retry
            </button>
          </div>
        ) : null}

        {(!cachedAt && error) || (!loading && data === null && !cachedAt) ? (
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
        ) : onlySide !== "none" && visible.length === 0 ? (
          <div className="mt-6 rounded-2xl border border-line bg-ink-900/60 p-6">
            <p className="text-sm font-medium text-mist-100">
              No live match clears an {onlySide} line right now.
            </p>
            <p className="mt-1 max-w-2xl text-xs leading-relaxed text-mist-400">
              Every projection range is crossing the standard {onlySide} lines.
              That is a pass, not a gap: widen the range only when the minute and
              corner count justify it.
            </p>
            <button
              type="button"
              onClick={() => setOnlySide("none")}
              className="mt-3 rounded-lg border border-line px-3 py-1.5 text-xs font-semibold text-mist-200 hover:border-white/20 hover:text-mist-100"
            >
              Show all live matches
            </button>
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
        ) : onlyPlayed && visible.length === 0 ? (
          <div className="mt-6 rounded-2xl border border-line bg-ink-900/60 p-6">
            <p className="text-sm font-medium text-mist-100">
              No live match has a corner leg from your journal.
            </p>
            <p className="mt-1 max-w-2xl text-xs leading-relaxed text-mist-400">
              Nothing is linked to a live league or team right now. The board
              shows only legs already imported into the journal, not games you
              watched.
            </p>
            <button
              type="button"
              onClick={() => setOnlyPlayed(false)}
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
              Followed matches are stored in this browser and only appear while
              the match is live. Once it leaves the live board it drops off the
              card list.
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
                onPickSaved={(message) => {
                  setPickNotice(message);
                  reload();
                }}
              />
            ))}
          </div>
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
        {pickNotice && (
          <div className="fixed inset-x-4 bottom-4 z-50 mx-auto flex max-w-md items-start gap-3 rounded-xl border border-turf-400/40 bg-ink-900 px-4 py-3 shadow-2xl">
            <FontAwesomeIcon
              icon={faFilePen}
              className="mt-0.5 size-3.5 text-turf-300"
            />
            <p className="flex-1 text-xs leading-relaxed text-mist-200">
              {pickNotice}
            </p>
            <button
              type="button"
              onClick={() => setPickNotice(null)}
              className="font-mono text-[10px] tracking-widest text-mist-500 uppercase hover:text-mist-200"
            >
              close
            </button>
          </div>
        )}
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
  onPickSaved,
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
  onPickSaved: (message: string) => void;
}) {
  const [editingPickId, setEditingPickId] = useState<number | null>(null);
  const editingPick =
    match.paper_picks?.find((pick) => pick.id === editingPickId) ?? null;
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
            onClick={() => {
              setEditingPickId(null);
              onOpenRecord();
            }}
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

      {(recording || editingPick) && (
        <div className="border-b border-line bg-ink-950/40 px-4 py-2">
          <PaperPickForm
            eventId={match.event_id}
            defaultLine={line}
            recOverLine={match.rec_over_line}
            recUnderLine={match.rec_under_line}
            projectedTotal={match.projected_total}
            editing={
              editingPick
                ? {
                    id: editingPick.id,
                    provider_slug: editingPick.provider_slug,
                    side: editingPick.side,
                    line: editingPick.line,
                    odds: editingPick.odds,
                  }
                : null
            }
            onCancel={() => {
              setEditingPickId(null);
              onCloseRecord();
            }}
            onSaved={(_pick, message) => {
              // Close the form so the chip and the toast carry the result,
              // instead of leaving a recorder open that has already saved.
              setEditingPickId(null);
              onCloseRecord();
              onPickSaved(message);
            }}
          />
        </div>
      )}

      <div className="grid min-w-0 gap-4 px-4 py-3 lg:grid-cols-[minmax(0,1fr)_460px_360px] lg:items-start lg:gap-5">
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
              {cornerEdge(match) === "home" ? (
                <span
                  className="inline-flex shrink-0"
                  title={`Corner edge: ${match.home.name} wins ${match.home_form?.avg_for.toFixed(1)} corners a game, ${match.away.name} ${match.away_form?.avg_for.toFixed(1)}`}
                >
                  <FontAwesomeIcon
                    icon={faArrowTrendUp}
                    className="size-3 text-turf-300"
                  />
                </span>
              ) : null}
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
              {cornerEdge(match) === "away" ? (
                <span
                  className="inline-flex shrink-0"
                  title={`Corner edge: ${match.away.name} wins ${match.away_form?.avg_for.toFixed(1)} corners a game, ${match.home.name} ${match.home_form?.avg_for.toFixed(1)}`}
                >
                  <FontAwesomeIcon
                    icon={faArrowTrendUp}
                    className="size-3 text-turf-300"
                  />
                </span>
              ) : null}
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

          {match.minute !== null &&
          match.minute > 45 &&
          match.first_half_corners !== null &&
          match.first_half_corners !== undefined &&
          match.second_half_corners !== null &&
          match.second_half_corners !== undefined ? (
            <p
              className="mt-1.5 font-mono text-[10px] text-mist-500"
              title="Corners in each half, counted from the snapshots captured while the match was live. The second half figure is the current total minus the count at the break."
            >
              1H {match.first_half_corners} · 2H {match.second_half_corners}
            </p>
          ) : null}
        </div>

        <div className="grid min-w-0 grid-cols-2 gap-1.5 sm:gap-2">
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
            hint="Highest over line that still sits below the low end of the likely range, so the whole range clears it. When the range still spans a line there is no edge and it reads pass, with the fair line shown underneath."
            value={
              match.rec_over_line !== null
                ? match.rec_over_line.toFixed(1)
                : match.projected_total !== null
                  ? "pass"
                  : "-"
            }
            small={match.rec_over_line === null && match.projected_total !== null}
            note={
              match.rec_over_line !== null && match.total_corners !== null
                ? `${Math.max(0, Math.ceil(match.rec_over_line) - match.total_corners)} needed`
                : match.rec_over_line === null && match.projected_total !== null
                  ? `fair ${fairLine(match.projected_total)} · no edge`
                  : match.projected_total === null
                    ? "before 15 min"
                    : undefined
            }
          />
          <Metric
            label="Under line"
            icon={faFlag}
            hint="Lowest under line that still sits above the high end of the likely range, so the whole range stays under it. When the range still spans a line there is no edge and it reads pass, with the fair line shown underneath."
            value={
              match.rec_under_line !== null
                ? match.rec_under_line.toFixed(1)
                : match.projected_total !== null
                  ? "pass"
                  : "-"
            }
            small={match.rec_under_line === null && match.projected_total !== null}
            note={
              match.rec_under_line !== null && match.total_corners !== null
                ? `${Math.max(0, Math.floor(match.rec_under_line) - match.total_corners)} room`
                : match.rec_under_line === null && match.projected_total !== null
                  ? `fair ${fairLine(match.projected_total)} · no edge`
                  : match.projected_total === null
                    ? "before 15 min"
                    : undefined
            }
          />

        </div>

        {match.home_form ||
        match.away_form ||
        (match.bet_history?.recent?.length ?? 0) > 0 ? (
          <div className="min-w-0 space-y-2.5 lg:border-l lg:border-line lg:pl-4">
            <FormPanel
              title={`${match.home.name} · last 5`}
              form={match.home_form ?? null}
            />
            <FormPanel
              title={`${match.away.name} · last 5`}
              form={match.away_form ?? null}
            />
            <PlayedPanel recent={match.bet_history?.recent ?? []} />
          </div>
        ) : (
          <p className="font-mono text-[9px] leading-4 tracking-widest text-mist-500 uppercase lg:border-l lg:border-line lg:pl-4">
            no stored corner form for these teams, and no linked journal legs
          </p>
        )}
      </div>

      <footer className="flex flex-wrap items-center justify-between gap-2 border-t border-line px-4 py-2">
        <span className="flex flex-wrap items-center gap-2">
          {match.paper_picks?.map((pick) => (
            <button
              key={pick.id}
              type="button"
              onClick={() => {
                onCloseRecord();
                setEditingPickId((current) =>
                  current === pick.id ? null : pick.id,
                );
              }}
              aria-pressed={editingPickId === pick.id}
              title={`${pick.provider} ${pick.side} ${pick.line} at ${pick.odds}. Click to edit this recording.`}
              className={`inline-flex items-center gap-1.5 rounded border px-1.5 py-0.5 font-mono text-[9px] font-bold tracking-widest uppercase transition ${
                editingPickId === pick.id
                  ? "border-turf-400/70 bg-turf-500/20 text-turf-100"
                  : "border-turf-400/40 bg-turf-500/10 text-turf-200 hover:border-turf-400/70"
              }`}
            >
              <FontAwesomeIcon icon={faFilePen} className="size-2.5" />
              {pick.side === "under" ? "U" : "O"} {pick.line}
            </button>
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
  if (src) {
    return (
      // eslint-disable-next-line @next/next/no-img-element
      <img
        src={src}
        alt=""
        width={18}
        height={12}
        title={`${name}${iso ? ` · ${iso}` : ""}`}
        className="h-3 w-[18px] shrink-0 rounded-[2px] object-cover ring-1 ring-white/20"
      />
    );
  }
  // No crest and no country on file: still leave a marker so a league chip can
  // never look like plain text, and the card is scannable at a glance.
  return (
    <FontAwesomeIcon
      icon={faFutbol}
      title={name}
      className="size-3 shrink-0 text-mist-500"
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
  small = false,
}: {
  label: string;
  icon: IconDefinition;
  hint: string;
  value: string;
  note?: string;
  small?: boolean;
}) {
  return (
    <div className="min-w-0 rounded-lg border border-line bg-ink-950/40 px-2 py-1.5 sm:px-2.5">
      <p className="flex items-center gap-1.5 font-mono text-[9px] tracking-wide text-mist-500 uppercase">
        <FontAwesomeIcon icon={icon} className="size-2.5 shrink-0" />
        <span className="truncate">{label}</span>
        <Hint text={hint} />
      </p>
      <p
        className={`font-mono tabular-nums ${
          small ? "text-xs text-mist-300" : "text-sm text-mist-100"
        }`}
      >
        {value}
      </p>
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

// FilterCard is the board filter as a stat tile: the label names the filter and
// the number is how many matches it would leave, so the count explains itself
// before the click.
function FilterCard({
  label,
  value,
  icon,
  active,
  hint,
  onToggle,
}: {
  label: string;
  value: number;
  icon: IconDefinition;
  active: boolean;
  hint: string;
  onToggle: () => void;
}) {
  return (
    <button
      type="button"
      onClick={onToggle}
      aria-pressed={active}
      title={hint}
      className={`rounded-xl border px-3 py-2.5 text-left transition ${
        active
          ? "border-turf-400/60 bg-turf-500/10"
          : "border-line bg-ink-950/40 hover:border-turf-400/40"
      }`}
    >
      <span className="flex items-center justify-between font-mono text-[9px] tracking-widest text-mist-500 uppercase">
        {label}
        <FontAwesomeIcon
          icon={icon}
          style={{ width: "1rem", height: "1rem" }}
          className={active ? "text-turf-300" : "text-mist-500"}
        />
      </span>
      <span
        className={`mt-0.5 block font-mono text-lg tabular-nums ${
          active ? "text-turf-100" : "text-mist-100"
        }`}
      >
        {value}
      </span>
    </button>
  );
}

// fairLine is the standard line nearest the projection, the one a book would
// hang at even money. It is shown only to explain why no line qualified.
function fairLine(projected: number): string {
  return (Math.round(projected * 2) / 2).toFixed(1);
}

// cornerEdge marks the side that wins more corners per game, but only when both
// teams have a real sample and the gap is at least one corner. It is a corner
// edge, not a claim about who wins the match.
function cornerEdge(match: LiveCornerMatch): "home" | "away" | null {
  const home = match.home_form;
  const away = match.away_form;
  if (!home || !away || home.sample < 3 || away.sample < 3) return null;
  const gap = home.avg_for - away.avg_for;
  if (Math.abs(gap) < 1) return null;
  return gap > 0 ? "home" : "away";
}

function shortDay(value: string | null) {
  if (!value) return "-";
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value;
  return date.toLocaleDateString("en-GB", { day: "numeric", month: "short" });
}

const PLAYED_STATUS: Record<string, string> = {
  won: "text-turf-300",
  lost: "text-loss-300",
  void: "text-mist-500",
  pending: "text-flag-400",
};

// Who played the leg: the journal stores a bettor per slip, a paper pick is
// recorded on this board. Unknown stays unknown rather than being attributed.
const PLAYED_BY: Record<string, { label: string; cls: string }> = {
  emife: { label: "Emife", cls: "border-flag-400/40 text-flag-400" },
  oyins: { label: "Oyins", cls: "border-turf-400/40 text-turf-300" },
  agent: { label: "Agent", cls: "border-line text-mist-300" },
  paper: { label: "Paper", cls: "border-line text-mist-300" },
  unknown: { label: "Unlabelled", cls: "border-line text-mist-500" },
};

// FormPanel shows the last five finished matches that carry a corner count for
// one team. A missing sample prints as missing, never as zero corners.
function FormPanel({
  title,
  form,
}: {
  title: string;
  form: BoardTeamForm | null;
}) {
  const matches = form?.matches ?? [];
  return (
    <div className="min-w-0">
      <p className="truncate font-mono text-[9px] tracking-widest text-mist-500 uppercase">
        {title}
      </p>
      {matches.length === 0 ? (
        <p className="mt-1 text-[10px] text-mist-500">
          no finished match with corner data
        </p>
      ) : (
        <>
          <ul className="mt-1 space-y-0.5 font-mono text-[10px]">
            {matches.map((item) => (
              <li key={item.event_id} className="flex items-baseline gap-1.5">
                <span className="w-11 shrink-0 text-mist-500">
                  {shortDay(item.date)}
                </span>
                <span
                  className="min-w-0 flex-1 truncate text-mist-300"
                  title={item.opponent}
                >
                  {item.home ? "v" : "@"} {item.opponent}
                </span>
                <span className="shrink-0 text-mist-200">
                  {item.corners_for}-{item.corners_against}
                </span>
                <span
                  className="shrink-0 text-right text-mist-400"
                  title={
                    item.first_half_corners != null &&
                    item.second_half_corners != null
                      ? `${item.total_corners} corners: ${item.first_half_corners} in the first half, ${item.second_half_corners} in the second`
                      : `${item.total_corners} corners. No first or second half split was captured for this match.`
                  }
                >
                  {item.total_corners}
                  {item.first_half_corners != null &&
                  item.second_half_corners != null ? (
                    <span className="text-mist-500">
                      {" "}
                      ({item.first_half_corners}+{item.second_half_corners})
                    </span>
                  ) : null}
                </span>
              </li>
            ))}
          </ul>
          <p className="mt-1 font-mono text-[9px] text-mist-500">
            avg {form?.avg_for.toFixed(1)} for · {form?.avg_total.toFixed(1)} total
            · {form?.sample} matches
          </p>
        </>
      )}
    </div>
  );
}

// PlayedPanel lists the corner legs already settled or open for this league or
// either team, so the card shows the selection itself, not just a tally.
function PlayedPanel({
  recent,
}: {
  recent: Array<{
    side: "over" | "under";
    line: number | null;
    odds: number | null;
    status: string;
    date: string | null;
    scope: "league" | "team" | "match";
    by?: string;
    total_corners?: number;
    first_half_corners?: number;
    second_half_corners?: number;
  }>;
}) {
  return (
    <div className="min-w-0">
      <p className="truncate font-mono text-[9px] tracking-widest text-mist-500 uppercase">
        your last 5 legs
      </p>
      {recent.length === 0 ? (
        <p className="mt-1 text-[10px] text-mist-500">
          no linked corner legs on record
        </p>
      ) : (
        <ul className="mt-1 space-y-0.5 font-mono text-[10px]">
          {recent.map((leg, index) => {
            const source =
              PLAYED_BY[leg.by ?? "unknown"] ?? PLAYED_BY.unknown;
            return (
              <li
                key={`${leg.side}-${leg.line}-${index}`}
                className="space-y-0.5"
                title={`${leg.side} ${leg.line ?? "-"} · ${leg.status} · ${leg.scope} · played by ${leg.by ?? "unknown"}`}
              >
                <div className="flex items-baseline gap-1.5">
                  <span className="w-14 shrink-0 font-bold text-mist-100 uppercase">
                    {leg.side} {leg.line ?? "-"}
                  </span>
                  {leg.odds !== null ? (
                    <span className="shrink-0 text-mist-300">@{leg.odds}</span>
                  ) : null}
                  <span
                    className={`shrink-0 ${PLAYED_STATUS[leg.status] ?? "text-mist-500"}`}
                  >
                    {leg.status}
                  </span>
                  <span
                    className={`ml-auto shrink-0 rounded border px-1 py-0.5 text-[8px] tracking-wider uppercase ${source.cls}`}
                  >
                    {source.label}
                  </span>
                  <span className="shrink-0 text-mist-500">
                    {shortDay(leg.date)}
                  </span>
                </div>
                {leg.total_corners !== undefined &&
                (leg.status === "won" ||
                  leg.status === "lost" ||
                  leg.status === "push" ||
                  leg.status === "void") ? (
                  <p className="pl-14 font-mono text-[9px] text-mist-500">
                    final {leg.total_corners} corners
                    {leg.first_half_corners !== undefined &&
                    leg.second_half_corners !== undefined
                      ? ` · 1H ${leg.first_half_corners} · 2H ${leg.second_half_corners}`
                      : ""}
                  </p>
                ) : leg.status === "won" ||
                  leg.status === "lost" ||
                  leg.status === "push" ||
                  leg.status === "void" ? (
                  <p className="pl-14 font-mono text-[9px] text-mist-500">
                    no corner count stored for that match
                  </p>
                ) : null}
              </li>
            );
          })}
        </ul>
      )}
    </div>
  );
}
