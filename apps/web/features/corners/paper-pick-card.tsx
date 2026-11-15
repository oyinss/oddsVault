"use client";

import Link from "next/link";
import {
  faBullseye,
  faChartLine,
  faClock,
  faFilePen,
  faGaugeHigh,
  faTrashCan,
  faTrophy,
} from "@fortawesome/free-solid-svg-icons";
import { FontAwesomeIcon } from "@fortawesome/react-fontawesome";

import { TeamCrest } from "@/components/team-crest";
import {
  absoluteAsset,
  formatAge,
  type PaperPick,
} from "@/lib/api";

const STATUS_STYLES: Record<PaperPick["status"], string> = {
  pending: "border-line text-mist-300",
  won: "border-turf-400/40 bg-turf-500/10 text-turf-300",
  lost: "border-loss-400/40 bg-loss-400/10 text-loss-300",
  push: "border-line text-mist-300",
  void: "border-line text-mist-400",
};

// PaperPickCard mirrors the corner board card so the view is the one already
// understood: competition, teams, corners, pace, projection and the line, with
// the recorded pick sitting in the same place on every card.
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
  // While the match is still being played, show its real progress. Once it is
  // over, the saved snapshot plus the final count tells the whole story.
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
  const room = pick.side === "under";

  return (
    <article
      className={`rounded-2xl border ${
        pick.status === "won"
          ? "border-turf-400/30 bg-turf-500/[0.04]"
          : pick.status === "lost"
            ? "border-loss-400/25 bg-loss-400/[0.03]"
            : "border-line"
      }`}
    >
      <header className="flex flex-wrap items-center justify-between gap-2 border-b border-line px-4 py-2.5">
        <div className="flex flex-wrap items-center gap-2">
          <span className="rounded bg-white/[0.06] px-1.5 py-0.5 font-mono text-[9px] tracking-wider text-mist-400 uppercase">
            {pick.competition}
          </span>
          <span className="flex items-center gap-1.5 font-mono text-[10px] text-mist-500">
            <FontAwesomeIcon icon={faClock} className="size-2.5" />
            recorded {formatAge(pick.created_at)}
          </span>
          <span
            className={`rounded border px-1.5 py-0.5 font-mono text-[9px] tracking-widest uppercase ${live ? "border-flag-400/40 text-flag-300" : STATUS_STYLES[pick.status]}`}
          >
            {live ? "live" : pick.status}
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
          <span className="font-mono text-xs text-mist-500">v</span>
          <TeamCrest src={awayCrest} name={pick.away} size={16} />
          <span>{pick.away}</span>
          {pick.event_status ? (
            <span className="rounded bg-white/[0.06] px-1.5 py-0.5 font-mono text-[9px] tracking-wider text-mist-400 uppercase">
              {pick.event_status}
            </span>
          ) : null}
        </div>

        <div className="mt-3 grid grid-cols-2 gap-1.5 sm:grid-cols-4">
          <Cell
            icon={faBullseye}
            label="Corners"
            value={totalCorners !== null ? String(totalCorners) : "-"}
            note={
              homeCorners !== null && awayCorners !== null
                ? `${homeCorners}-${awayCorners}${minute !== null ? ` · ${minute}'` : ""}`
                : minute !== null
                  ? `${minute}'`
                  : undefined
            }
          />
          <Cell
            icon={faGaugeHigh}
            label="Pace"
            value={pace !== null ? pace.toFixed(2) : "-"}
            note={live ? "live" : pick.early_pace ? "early" : undefined}
          />
          <Cell
            icon={faChartLine}
            label="Proj."
            value={projected !== null ? projected.toFixed(1) : "-"}
            note={
              projectionLow !== null && projectionHigh !== null
                ? `${projectionLow.toFixed(1)}-${projectionHigh.toFixed(1)}`
                : pick.early_pace
                  ? "early"
                  : undefined
            }
          />
          <Cell
            icon={faClock}
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
        <span className="flex flex-wrap items-center gap-2 font-mono text-[10px] text-mist-500">
          {settled ? (
            <>
              <FontAwesomeIcon icon={faTrophy} className="size-2.5" />
              final {pick.final_total ?? "-"} corners
            </>
          ) : live ? (
            <>
              <FontAwesomeIcon icon={faClock} className="size-2.5" />
              live{remaining !== null ? ` · ${remaining} min left` : ""}
              {toLine !== null
                ? room
                  ? ` · ${toLine} corners of room`
                  : ` · ${toLine} needed`
                : ""}
            </>
          ) : (
            <>
              <FontAwesomeIcon icon={faClock} className="size-2.5" />
              waiting for the final corner count
            </>
          )}
          {pick.model_risk ? (
            <span className="rounded border border-flag-400/40 px-1.5 py-0.5 text-[9px] tracking-widest text-flag-400 uppercase">
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

function Cell({
  icon,
  label,
  value,
  note,
}: {
  icon: typeof faBullseye;
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
      {note ? (
        <p className="font-mono text-[10px] text-mist-500">{note}</p>
      ) : null}
    </div>
  );
}
