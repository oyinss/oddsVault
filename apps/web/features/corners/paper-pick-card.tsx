"use client";

import Link from "next/link";
import {
  faBullseye,
  faChartLine,
  faClock,
  faFilePen,
  faCircleCheck,
  faCircleMinus,
  faCircleQuestion,
  faCircleXmark,
  faGaugeHigh,
  faPenToSquare,
  faTrashCan,
  faTriangleExclamation,
  faTrophy,
} from "@fortawesome/free-solid-svg-icons";
import { FontAwesomeIcon } from "@fortawesome/react-fontawesome";
import type { IconDefinition } from "@fortawesome/free-solid-svg-icons";

import { TeamCrest } from "@/components/team-crest";
import { absoluteAsset, formatAge, type PaperPick } from "@/lib/api";
import {
  baseRatePercent,
  impliedPercent,
  priceGapPoints,
} from "@/lib/corner-base-rates";

const STATUS_STYLES: Record<PaperPick["status"], string> = {
  pending: "border-line text-mist-300",
  won: "border-turf-400/40 bg-turf-500/10 text-turf-300",
  lost: "border-loss-400/40 bg-loss-400/10 text-loss-300",
  push: "border-line text-mist-300",
  void: "border-line text-mist-400",
};

// One glyph per outcome so a card can be read at a glance without parsing the
// word, and the icons stay monochrome so the status colour still leads.
export const STATUS_ICONS: Record<PaperPick["status"], IconDefinition> = {
  pending: faClock,
  won: faCircleCheck,
  lost: faCircleXmark,
  push: faCircleMinus,
  void: faCircleQuestion,
};

// PaperPickCard mirrors the live corner card: the same header, corner cells,
// three metric tiles and footer, with the recorded decision pinned to the card
// so progress can be read exactly where it is read on the board.
export function PaperPickCard({
  pick,
  onDelete,
  onEdit,
}: {
  pick: PaperPick;
  onDelete?: (id: number) => void;
  onEdit?: (pick: PaperPick) => void;
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
  const implied = impliedPercent(pick.odds);
  const base = baseRatePercent(pick.side, pick.line);
  const gap = priceGapPoints(pick.side, pick.line, pick.odds);
  // Against the read means the pick sits on the wrong side of the projection
  // frozen with it, not the live one: the decision is judged on what was on
  // screen when it was taken.
  const savedProjection = pick.projected_total;
  const againstRead =
    savedProjection !== null &&
    (pick.side === "over"
      ? savedProjection <= pick.line
      : savedProjection >= pick.line);
  const followedLine =
    (pick.side === "over" && pick.rec_over_line === pick.line) ||
    (pick.side === "under" && pick.rec_under_line === pick.line);
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
          {pick.recovered_at ? (
            <span
              title="Reconstructed from the corner snapshot taken when it was recorded, after a dedupe merge deleted the row."
              className="rounded border border-flag-400/40 bg-flag-400/[0.06] px-1.5 py-0.5 font-mono text-[9px] tracking-widest text-flag-300 uppercase"
            >
              recovered
            </span>
          ) : null}
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
            className="inline-flex items-center gap-1 rounded border border-line px-1.5 py-0.5 text-[9px] tracking-widest text-mist-300"
            title={
              base === null
                ? `The price asks for ${implied ?? "?"}% of the time. No base rate is measured at ${pick.line}.`
                : `The price asks for ${implied}%. The neutral base at ${pick.side} ${pick.line} is ${base}%, so the live read has to find ${gap} points.`
            }
          >
            price {implied !== null ? `${implied}%` : "-"}
            {base !== null ? ` · base ${base}%` : ""}
          </span>
          {againstRead ? (
            <span
              className="inline-flex items-center gap-1 rounded border border-flag-400/40 bg-flag-400/[0.06] px-1.5 py-0.5 text-[9px] tracking-widest text-flag-400"
              title={`The projection at save was ${savedProjection?.toFixed(1)}, on the other side of ${pick.line}.`}
            >
              <FontAwesomeIcon icon={faTriangleExclamation} className="size-2.5" />
              against the read
            </span>
          ) : followedLine ? (
            <span
              className="inline-flex items-center gap-1 rounded border border-turf-400/40 px-1.5 py-0.5 text-[9px] tracking-widest text-turf-300"
              title="This is the line the board recommended when it was recorded."
            >
              followed the card
            </span>
          ) : null}
          <span
            className={`inline-flex items-center gap-1 rounded border px-1.5 py-0.5 text-[9px] tracking-widest ${STATUS_STYLES[pick.status]}`}
          >
            <FontAwesomeIcon icon={STATUS_ICONS[pick.status]} className="size-2.5" />
            {pick.status}
          </span>
          {settled ? (
            <span
              className="flex items-center gap-1.5 text-mist-400"
              title={
                pick.settlement_rule === "total_corners_after_timeout"
                  ? "Settled from the stored corner counts after the feed stopped updating; the provider never published a final status."
                  : undefined
              }
            >
              <FontAwesomeIcon icon={faTrophy} className="size-2.5" />
              final {pick.final_total ?? "-"} corners
              {pick.settlement_rule === "total_corners_after_timeout" ? (
                <span className="rounded border border-line px-1.5 py-0.5 text-[9px] tracking-widest uppercase">
                  after timeout
                </span>
              ) : null}
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
          {onEdit && !settled ? (
            <button
              type="button"
              onClick={() => onEdit(pick)}
              title="Edit the side, line, price or bookmaker"
              className="grid size-6 place-items-center rounded border border-line text-mist-500 transition hover:border-turf-400/40 hover:text-turf-200"
            >
              <FontAwesomeIcon icon={faPenToSquare} className="size-2.5" />
            </button>
          ) : null}
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
