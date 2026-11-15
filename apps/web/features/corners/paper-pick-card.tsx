"use client";

import Link from "next/link";
import {
  faBullseye,
  faChartLine,
  faClock,
  faFilePen,
  faGaugeHigh,
  faTrashCan,
  faTriangleExclamation,
  faTrophy,
} from "@fortawesome/free-solid-svg-icons";
import { FontAwesomeIcon } from "@fortawesome/react-fontawesome";

import { TeamCrest } from "@/components/team-crest";
import { absoluteAsset, formatAge, type PaperPick } from "@/lib/api";

const STATUS_STYLES: Record<PaperPick["status"], string> = {
  pending: "border-line text-mist-300",
  won: "border-turf-400/40 bg-turf-500/10 text-turf-300",
  lost: "border-loss-400/40 bg-loss-400/10 text-loss-300",
  push: "border-line text-mist-300",
  void: "border-line text-mist-400",
};

// PaperPickCard mirrors the live corner card: the same header, corner cells,
// three metric tiles and footer, with the recorded decision pinned to the card
// so progress can be read exactly where it is read on the board.
export function PaperPickCard({
  pick,
  onDelete,
}: {
  pick: PaperPick;
  onDelete?: (id: number) => void;
}) {
  const homeCrest = absoluteAsset(`/api/v1/teams/${pick.home_team_id}/crest`);
  const awayCrest = absoluteAsset(`/api/v1/teams/${pick.away_team_id}/crest`);

  const settled = pick.status !== "pending";
  const current = pick.current ?? null;
  const live = current?.live === true;
  const minute = current?.minute ?? pick.minute;
  const homeCorners = current?.home_corners ?? pick.home_corners;
  const awayCorners = current?.away_corners ?? pick.away_corners;
  const totalCorners = current?.total_corners ?? pick.total_corners;
  const pace = current?.pace_per_minute ?? pick.pace_per_minute;
  const projected = current?.projected_total ?? pick.projected_total;
  const projectionLow = current?.projection_low ?? pick.projection_low;
  const projectionHigh = current?.projection_high ?? pick.projection_high;
  const remaining = current?.remaining_minutes ?? null;
  const toLine = current?.to_line ?? null;
  const homeScore = current?.home_score ?? null;
  const awayScore = current?.away_score ?? null;
  const room = pick.side === "under";
  const risk = pick.model_risk || (projected !== null && projected > pick.line * 1.4 && (minute ?? 99) < 30);

  const tone = settled
    ? pick.status === "won"
      ? "border-turf-400/40 bg-turf-500/[0.06]"
      : pick.status === "lost"
        ? "border-loss-400/25 bg-loss-400/[0.03]"
        : "border-line"
    : "border-line";

  return (
    <article className={`rounded-2xl border ${tone}`}>
      <header className="flex flex-wrap items-center justify-between gap-2 border-b border-line px-4 py-2.5">
        <div className="flex flex-wrap items-center gap-2">
          <span className="rounded bg-white/[0.06] px-1.5 py-0.5 font-mono text-[9px] tracking-wider text-mist-400 uppercase">
            {pick.competition}
          </span>
          {live ? (
            <span className="flex items-center gap-1.5 font-mono text-[11px] text-red-300">
              <span className="size-1.5 animate-pulse rounded-full bg-red-400" />
              {minute !== null ? `${minute} minutes` : "live"}
            </span>
          ) : (
            <span className="font-mono text-[10px] text-mist-500">
              {pick.event_status}
            </span>
          )}
          <span className="font-mono text-[10px] text-mist-500">
            recorded {formatAge(pick.created_at)}
          </span>
        </div>
        <span className="inline-flex items-center gap-1.5 rounded-md border border-turf-400/40 bg-turf-500/10 px-2 py-1 font-mono text-[10px] font-bold tracking-widest text-turf-200 uppercase">
          <FontAwesomeIcon icon={faFilePen} className="size-2.5" />
          {pick.side} {pick.line} @ {pick.odds.toFixed(2)} · {pick.provider}
        </span>
      </header>

      <div className="px-4 py-3">
        <div className="flex flex-wrap items-center gap-2 text-sm font-semibold text-mist-100">
          <TeamCrest src={homeCrest} name={pick.home} size={16} />
          <span>{pick.home}</span>
          {homeScore !== null && awayScore !== null ? (
            <span className="font-mono text-xs text-mist-300">
              {homeScore}-{awayScore}
            </span>
          ) : null}
          <TeamCrest src={awayCrest} name={pick.away} size={16} />
          <span>{pick.away}</span>
        </div>

        <div className="mt-3 grid grid-cols-3 gap-1.5 sm:max-w-md">
          <CornersCell label="Home corners" value={homeCorners} />
          <CornersCell label="Away corners" value={awayCorners} />
          <CornersCell label="Total corners" value={totalCorners} strong />
        </div>

        <div className="mt-3 grid gap-1.5 sm:grid-cols-3">
          <Metric
            icon={faGaugeHigh}
            label="Pace"
            value={pace !== null ? pace.toFixed(2) : "-"}
            note={live ? "live" : pick.early_pace ? "early" : undefined}
          />
          <Metric
            icon={faChartLine}
            label="Proj."
            value={projected !== null ? projected.toFixed(1) : "-"}
            note={
              projectionLow !== null && projectionHigh !== null
                ? `${projectionLow.toFixed(1)}-${projectionHigh.toFixed(1)} likely`
                : pick.early_pace
                  ? "early"
                  : undefined
            }
          />
          <Metric
            icon={faBullseye}
            label={room ? `Room ${pick.line}` : `Need ${pick.line}`}
            value={toLine !== null ? String(toLine) : "-"}
            note={
              toLine !== null
                ? room
                  ? `corners before ${pick.line} is beaten`
                  : `corners still needed${remaining !== null ? ` · ${remaining} min` : ""}`
                : undefined
            }
          />
        </div>
      </div>

      <footer className="flex flex-wrap items-center justify-between gap-2 border-t border-line px-4 py-2">
        <span className="flex flex-wrap items-center gap-2 font-mono text-[10px] uppercase">
          <span
            className={`rounded border px-1.5 py-0.5 text-[9px] tracking-widest ${STATUS_STYLES[pick.status]}`}
          >
            {pick.status}
          </span>
          {settled ? (
            <span className="flex items-center gap-1.5 text-mist-400">
              <FontAwesomeIcon icon={faTrophy} className="size-2.5" />
              final {pick.final_total ?? "-"} corners
            </span>
          ) : live ? (
            <span className="flex items-center gap-1.5 text-mist-400">
              <FontAwesomeIcon icon={faClock} className="size-2.5" />
              live{remaining !== null ? ` · ${remaining} min left` : ""}
              {toLine !== null
                ? room
                  ? ` · ${toLine} corners of room`
                  : ` · ${toLine} needed`
                : ""}
            </span>
          ) : (
            <span className="flex items-center gap-1.5 text-mist-500">
              <FontAwesomeIcon icon={faClock} className="size-2.5" />
              waiting for the final corner count
            </span>
          )}
          {risk ? (
            <span className="flex items-center gap-1 rounded border border-flag-400/40 px-1.5 py-0.5 text-[9px] tracking-widest text-flag-400">
              <FontAwesomeIcon icon={faTriangleExclamation} className="size-2.5" />
              early projection
            </span>
          ) : null}
        </span>
        <span className="flex items-center gap-3">
          <Link
            href={`/corners/detail?id=${pick.event_id}&line=${pick.line}`}
            className="font-mono text-[10px] text-turf-300 hover:text-turf-200"
          >
            details
          </Link>
          {onDelete ? (
            <button
              type="button"
              onClick={() => onDelete(pick.id)}
              title="Delete this recording"
              className="grid size-6 place-items-center rounded border border-line text-mist-500 transition hover:border-loss-400/40 hover:text-loss-300"
            >
              <FontAwesomeIcon icon={faTrashCan} className="size-2.5" />
            </button>
          ) : null}
        </span>
      </footer>
    </article>
  );
}

function CornersCell({
  label,
  value,
  strong = false,
}: {
  label: string;
  value: number | null;
  strong?: boolean;
}) {
  return (
    <div
      className={`rounded-lg border px-2.5 py-1.5 ${
        strong
          ? "border-turf-400/40 bg-turf-500/10"
          : "border-line bg-ink-950/40"
      }`}
    >
      <p className="font-mono text-[8px] tracking-widest text-mist-500 uppercase">
        {label}
      </p>
      <p
        className={`mt-0.5 font-mono text-base tabular-nums ${
          strong ? "text-turf-200" : "text-mist-100"
        }`}
      >
        {value !== null ? value : "-"}
      </p>
    </div>
  );
}

function Metric({
  icon,
  label,
  value,
  note,
}: {
  icon: typeof faGaugeHigh;
  label: string;
  value: string;
  note?: string;
}) {
  return (
    <div className="rounded-xl border border-line bg-ink-950/40 px-3 py-2.5">
      <p className="flex items-center gap-1.5 font-mono text-[9px] tracking-widest text-mist-500 uppercase">
        <FontAwesomeIcon icon={icon} className="size-2.5" />
        {label}
      </p>
      <p className="mt-0.5 font-mono text-lg tabular-nums text-mist-100">
        {value}
      </p>
      {note ? <p className="font-mono text-[10px] text-mist-500">{note}</p> : null}
    </div>
  );
}
