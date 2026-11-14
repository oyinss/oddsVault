"use client";

import Link from "next/link";
import { useSearchParams } from "next/navigation";
import { Suspense } from "react";
import {
  faArrowLeft,
  faCircleInfo,
  faClock,
  faFlag,
} from "@fortawesome/free-solid-svg-icons";
import { FontAwesomeIcon } from "@fortawesome/react-fontawesome";

import { LoadingNote } from "@/components/loading-note";
import { SiteHeader } from "@/components/site-header";
import { TeamCrest } from "@/components/team-crest";
import {
  absoluteAsset,
  formatAge,
  getCornerContext,
  type CornerBetHistory,
  type CornerBetRecord,
  type CornerBetSide,
  type CornerContext,
  type CornerMetrics,
  type CornerTeamForm,
} from "@/lib/api";
import { useApiData } from "@/lib/use-api";

const OVER_LINES = ["8.5", "9.5", "10.5", "11.5"];

export default function CornerDetailPage() {
  return (
    <Suspense
      fallback={
        <>
          <SiteHeader />
          <main className="mx-auto max-w-4xl px-6 py-12">
            <LoadingNote label="Loading match detail..." />
          </main>
        </>
      }
    >
      <DetailView />
    </Suspense>
  );
}

function DetailView() {
  const params = useSearchParams();
  const id = params.get("id") ?? "";
  const line = Number(params.get("line") ?? "10") || 10;
  const { data, loading } = useApiData<CornerContext | null>(
    `corner-context:${id}:${line}`,
    () => getCornerContext(id, line),
  );

  if (loading && !data) {
    return (
      <>
        <SiteHeader />
        <main className="mx-auto max-w-4xl px-6 py-12">
          <LoadingNote label="Reading match detail..." />
        </main>
      </>
    );
  }

  if (!data) {
    return (
      <>
        <SiteHeader />
        <main className="mx-auto max-w-4xl px-6 py-12">
          <p className="text-sm text-mist-300">Match detail is unavailable.</p>
          <Link href="/corners" className="mt-3 inline-block text-xs text-turf-300">
            Back to the board
          </Link>
        </main>
      </>
    );
  }

  const event = data.event;
  const latest = data.points.at(-1);
  const total = data.metrics?.total_corners ?? latest?.total_corners ?? null;
  const minute = data.metrics?.minute ?? latest?.minute ?? null;

  return (
    <>
      <SiteHeader />
      <main className="mx-auto max-w-4xl px-6 py-12">
        <Link
          href="/corners"
          className="inline-flex items-center gap-2 font-mono text-[10px] tracking-widest text-mist-500 uppercase hover:text-mist-200"
        >
          <FontAwesomeIcon icon={faArrowLeft} className="size-2.5" />
          Corner board
        </Link>

        <p className="mt-4 font-mono text-[11px] tracking-widest text-turf-300 uppercase">
          {event.competition_name}
        </p>
        <h1 className="mt-1 flex flex-wrap items-center gap-3 text-2xl font-bold tracking-tight">
          <span className="flex items-center gap-2">
            <TeamCrest
              src={absoluteAsset(event.home.crest_url)}
              name={event.home.name}
              size={24}
            />
            {event.home.name}
          </span>
          <span className="font-mono text-lg text-mist-300">
            {event.live?.home_score ?? event.result?.home_score ?? "-"} -{" "}
            {event.live?.away_score ?? event.result?.away_score ?? "-"}
          </span>
          <span className="flex items-center gap-2">
            <TeamCrest
              src={absoluteAsset(event.away.crest_url)}
              name={event.away.name}
              size={24}
            />
            {event.away.name}
          </span>
        </h1>
        <p className="mt-2 flex flex-wrap items-center gap-3 font-mono text-[11px] text-mist-500">
          <span className="flex items-center gap-1.5">
            <FontAwesomeIcon icon={faClock} className="size-2.5" />
            {event.live?.detail ?? event.status}
          </span>
          <span>kickoff {event.start_time_local ?? event.start_date}</span>
          <span className="flex items-center gap-1.5">
            <FontAwesomeIcon icon={faFlag} className="size-2.5" />
            line {data.line}
          </span>
        </p>

        <div className="mt-6">
          <DecisionPanel metrics={data.metrics} line={data.line} />
        </div>

        <section className="mt-4 grid gap-3 sm:grid-cols-4">
          <Metric label="Total corners" value={total !== null ? String(total) : "-"} />
          <Metric label="Minute" value={minute !== null ? `${minute}'` : "-"} />
          <Metric
            label="Pace"
            value={
              data.metrics?.corners_per_minute != null
                ? `${data.metrics.corners_per_minute.toFixed(2)}/min`
                : "-"
            }
          />
          <Metric
            label="Corners updated"
            value={latest ? formatAge(latest.captured_at) : "no snapshot"}
          />
        </section>

        <section className="mt-8 rounded-2xl border border-line bg-ink-900/50 p-5">
          <h2 className="text-sm font-semibold text-mist-100">Corner timeline</h2>
          <p className="mt-1 text-xs text-mist-500">
            One point each time the count changed. Providers do not publish the
            exact minute of each corner, so this is the closest record we have.
          </p>
          <Timeline points={data.points} line={data.line} />
        </section>

        <section className="mt-6 grid gap-4 md:grid-cols-2">
          <FormCard form={data.home_form} />
          <FormCard form={data.away_form} />
        </section>

        <section className="mt-6 rounded-2xl border border-line bg-ink-900/50 p-5">
          <h2 className="text-sm font-semibold text-mist-100">Your record here</h2>
          <p className="mt-1 text-xs text-mist-500">
            Corner-total bets from your own journal on this competition and
            these two teams. Won and lost come from the legs themselves.
          </p>
          <YourBets history={data.bet_history ?? null} />
        </section>

        <p className="mt-4 flex items-start gap-2 rounded-xl border border-line bg-ink-900/40 p-3 text-[11px] leading-relaxed text-mist-400">
          <FontAwesomeIcon icon={faCircleInfo} className="mt-0.5 size-3 shrink-0 text-mist-500" />
          {data.coverage_note}
        </p>

        <section className="mt-6 rounded-2xl border border-line bg-ink-900/50 p-5">
          <h2 className="text-sm font-semibold text-mist-100">How to use this</h2>
          <ul className="mt-2 space-y-1.5 text-xs leading-relaxed text-mist-300">
            <li>
              A small sample is a weak prior. With one or two matches, the
              average describes those matches, not the team.
            </li>
            <li>
              Compare the team's recent total with the current projection range.
              If both point the same way, the read is stronger.
            </li>
            <li>
              The price decides the bet. A range clearly under the line is worth
              playing only when the bookmaker pays more than fair odds for it.
            </li>
            <li>
              The over rate at a line is simply how many of the listed matches
              finished above it. It is not a probability.
            </li>
          </ul>
        </section>
      </main>
    </>
  );
}

function YourBets({ history }: { history: CornerBetHistory | null }) {
  if (!history) {
    return (
      <p className="mt-3 text-xs text-mist-500">
        Your bet journal could not be read for this match.
      </p>
    );
  }

  const rows: Array<{ label: string; record: CornerBetRecord | null }> = [
    { label: "Competition", record: history.competition },
    { label: "Home team", record: history.home },
    { label: "Away team", record: history.away },
  ];
  const anyLegs = rows.some(
    (row) =>
      (row.record?.over.legs ?? 0) + (row.record?.under.legs ?? 0) > 0,
  );

  const attached = history.journal
    ? history.journal.linked_legs + (history.journal.league_legs ?? 0)
    : 0;
  const coverage =
    history.journal && history.journal.corner_legs > 0
      ? `${attached} of ${history.journal.corner_legs} corner legs are attached to a match or an exactly named league. A slip whose fixture could not be identified cannot appear under a team.`
      : null;

  if (!anyLegs) {
    return (
      <>
        <p className="mt-3 text-xs text-mist-500">
          No linked corner-total bets on this competition or these teams yet.
        </p>
        {coverage ? (
          <p className="mt-2 rounded-lg border border-line bg-ink-950/40 p-2.5 text-[10px] leading-relaxed text-mist-400">
            {coverage}
          </p>
        ) : null}
      </>
    );
  }

  return (
    <div className="mt-3 overflow-x-auto">
      <table className="w-full text-left text-[11px]">
        <thead className="font-mono text-[9px] tracking-widest text-mist-500 uppercase">
          <tr>
            <th className="py-1 font-normal">Where</th>
            <th className="py-1 font-normal">Over</th>
            <th className="py-1 font-normal">Under</th>
            <th className="py-1 text-right font-normal">One-leg P/L</th>
          </tr>
        </thead>
        <tbody className="divide-y divide-white/[0.05]">
          {rows.map(({ label, record }) => (
            <BetRow key={label} label={label} record={record} />
          ))}
        </tbody>
      </table>
      <p className="mt-2 text-[10px] leading-relaxed text-mist-500">
        {history.note}
      </p>
      {coverage ? (
        <p className="mt-1 text-[10px] leading-relaxed text-mist-500">
          {coverage}
        </p>
      ) : null}
    </div>
  );
}

function BetRow({
  label,
  record,
}: {
  label: string;
  record: CornerBetRecord | null;
}) {
  const legs = (record?.over.legs ?? 0) + (record?.under.legs ?? 0);
  if (!record || legs === 0) {
    return (
      <tr>
        <td className="py-1.5 text-mist-200">{label}</td>
        <td className="py-1.5 text-mist-500" colSpan={3}>
          no bets
        </td>
      </tr>
    );
  }

  const money = record.single_bet;
  return (
    <tr>
      <td className="py-1.5 text-mist-200">{label}</td>
      <td className="py-1.5 text-mist-200">
        <SideLine side={record.over} />
      </td>
      <td className="py-1.5 text-mist-200">
        <SideLine side={record.under} />
      </td>
      <td className="py-1.5 text-right">
        {money.bets > 0 ? (
          <span className="font-mono text-mist-100">
            {naira(money.profit_loss)}{" "}
            <span className="text-mist-500">on {naira(money.stake)}</span>
          </span>
        ) : (
          <span className="font-mono text-mist-500">accumulators only</span>
        )}
      </td>
    </tr>
  );
}

function SideLine({ side }: { side: CornerBetSide }) {
  if (side.legs === 0) {
    return <span className="font-mono text-mist-500">-</span>;
  }
  const extra: string[] = [];
  if (side.void > 0) extra.push(`${side.void} void`);
  if (side.pending > 0) extra.push(`${side.pending} open`);
  if (side.unknown > 0) extra.push(`${side.unknown} unsettled`);
  return (
    <span className="font-mono">
      {side.won}W {side.lost}L
      {extra.length > 0 ? (
        <span className="text-mist-500"> · {extra.join(" · ")}</span>
      ) : null}
    </span>
  );
}

function naira(value: number) {
  const formatted = new Intl.NumberFormat("en-NG", {
    style: "currency",
    currency: "NGN",
    maximumFractionDigits: 0,
  }).format(Math.abs(value));
  if (value > 0) return `+${formatted}`;
  if (value < 0) return `-${formatted}`;
  return formatted;
}

type Tone = "over" | "under" | "pass" | "early";

function DecisionPanel({
  metrics,
  line,
}: {
  metrics: CornerMetrics | null;
  line: number;
}) {
  if (!metrics || metrics.total_corners === null) {
    return (
      <div className="rounded-2xl border border-line bg-ink-900/50 p-4">
        <h2 className="text-sm font-semibold text-mist-100">Decision read</h2>
        <p className="mt-1.5 text-xs leading-relaxed text-mist-400">
          No corner count has been published for this match yet. Without a count
          and a minute there is no pace, no projection and no read to show.
        </p>
      </div>
    );
  }

  const m = metrics;
  const underClear = m.projection_high !== null && m.projection_high < line;
  const overClear = m.projection_low !== null && m.projection_low > line;

  let tone: Tone;
  let read: string;
  if (m.minute !== null && m.minute < 15) {
    tone = "early";
    read =
      "Too early. Before the 15th minute the rate is noise, so the range is information, not a signal.";
  } else if (m.model_risk) {
    tone = "early";
    read =
      "Early projection risk. The sample is small and the extrapolation sits far above the line, which usually means a burst that regresses.";
  } else if (underClear) {
    tone = "under";
    read = `The whole projected range is below ${line}. Under ${line} only needs the match to stay normal, not to collapse.`;
  } else if (overClear) {
    tone = "over";
    read = `The whole projected range is above ${line}. Over ${line} is the side the current pace supports.`;
  } else {
    tone = "pass";
    read = `The range crosses ${line}. There is no edge at this line: move the line or pass.`;
  }

  const toneBox: Record<Tone, string> = {
    under: "border-sky-400/40 bg-sky-400/[0.06]",
    over: "border-turf-400/40 bg-turf-400/[0.06]",
    pass: "border-line bg-ink-900/50",
    early: "border-flag-400/40 bg-flag-400/[0.06]",
  };
  const toneBadge: Record<Tone, string> = {
    under: "border-sky-400/40 text-sky-200",
    over: "border-turf-400/40 text-turf-200",
    pass: "border-line text-mist-300",
    early: "border-flag-400/40 text-flag-200",
  };
  const toneLabel: Record<Tone, string> = {
    under: "Under lean",
    over: "Over lean",
    pass: "No edge",
    early: "Too early",
  };

  const range =
    m.projection_low !== null && m.projection_high !== null
      ? `${m.projection_low.toFixed(1)} to ${m.projection_high.toFixed(1)}`
      : "-";

  return (
    <div className={`rounded-2xl border p-4 ${toneBox[tone]}`}>
      <div className="flex flex-wrap items-center justify-between gap-2">
        <h2 className="text-sm font-semibold text-mist-100">
          Decision read at line {line}
        </h2>
        <span
          className={`rounded-md border px-2 py-0.5 font-mono text-[10px] tracking-widest uppercase ${toneBadge[tone]}`}
        >
          {toneLabel[tone]}
        </span>
      </div>
      <p className="mt-1.5 text-xs leading-relaxed text-mist-300">{read}</p>

      <dl className="mt-3 grid grid-cols-2 gap-2 sm:grid-cols-4">
        <DecisionCell
          label="Projected"
          value={m.projected_total !== null ? m.projected_total.toFixed(1) : "-"}
        />
        <DecisionCell label="Likely range" value={range} />
        <DecisionCell
          label={`Need for over ${line}`}
          value={
            m.needed !== null
              ? `${m.needed} in ${m.remaining_minutes ?? "?"}'`
              : "-"
          }
        />
        <DecisionCell
          label="Rate needed"
          value={
            m.needed_per_minute !== null
              ? `${m.needed_per_minute.toFixed(2)}/min`
              : "-"
          }
        />
      </dl>

      {m.early_pace ? (
        <p className="mt-2 font-mono text-[10px] text-mist-400">
          early pace at {m.corners_per_minute?.toFixed(2)} corners per minute,
          before the 15th minute
        </p>
      ) : m.pace !== "unknown" ? (
        <p className="mt-2 font-mono text-[10px] text-mist-400">
          pace is {m.pace.replace("_", " ")} versus the rate the line needs
        </p>
      ) : null}
    </div>
  );
}

function DecisionCell({ label, value }: { label: string; value: string }) {
  return (
    <div className="rounded-lg border border-white/10 bg-ink-950/40 px-2 py-1.5">
      <dt className="font-mono text-[9px] tracking-widest text-mist-500 uppercase">
        {label}
      </dt>
      <dd className="font-mono text-sm tabular-nums text-mist-100">{value}</dd>
    </div>
  );
}

function Metric({ label, value }: { label: string; value: string }) {
  return (
    <div className="rounded-xl border border-line bg-ink-950/40 px-3 py-2.5">
      <p className="font-mono text-[9px] tracking-widest text-mist-500 uppercase">
        {label}
      </p>
      <p className="mt-0.5 font-mono text-lg tabular-nums text-mist-100">
        {value}
      </p>
    </div>
  );
}

function Timeline({
  points,
  line,
}: {
  points: CornerContext["points"];
  line: number;
}) {
  if (points.length < 2) {
    return (
      <p className="mt-3 text-xs text-mist-500">
        No timeline yet. Points appear as the count changes while the match is
        live.
      </p>
    );
  }

  const width = 640;
  const height = 180;
  const maxMinute = 90;
  const maxTotal = Math.max(line + 2, ...points.map((p) => p.total_corners));
  const x = (minute: number) => 16 + (minute / maxMinute) * (width - 32);
  const y = (value: number) => height - 16 - (value / maxTotal) * (height - 32);
  const path = points
    .map((p, index) => {
      const px = x(p.minute ?? 0);
      const py = y(p.total_corners);
      return `${index === 0 ? "M" : "L"}${px.toFixed(1)},${py.toFixed(1)}`;
    })
    .join(" ");

  return (
    <svg
      viewBox={`0 0 ${width} ${height}`}
      className="mt-3 w-full"
      role="img"
      aria-label="Corner count over match time"
    >
      <line
        x1={16}
        y1={y(line)}
        x2={width - 16}
        y2={y(line)}
        stroke="currentColor"
        className="text-flag-400/50"
        strokeDasharray="4 4"
      />
      <text
        x={width - 16}
        y={y(line) - 4}
        textAnchor="end"
        className="fill-flag-400 text-[9px]"
      >
        line {line}
      </text>
      <path
        d={path}
        fill="none"
        stroke="currentColor"
        className="text-turf-400"
        strokeWidth="2"
      />
      {points.map((p, index) => (
        <circle
          key={`${p.captured_at}-${index}`}
          cx={x(p.minute ?? 0)}
          cy={y(p.total_corners)}
          r="2.5"
          fill="currentColor"
          className="text-turf-300"
        />
      ))}
      <text x={16} y={height - 2} className="fill-mist-500 text-[9px]">
        0'
      </text>
      <text
        x={width / 2}
        y={height - 2}
        textAnchor="middle"
        className="fill-mist-500 text-[9px]"
      >
        45'
      </text>
      <text
        x={width - 16}
        y={height - 2}
        textAnchor="end"
        className="fill-mist-500 text-[9px]"
      >
        90'
      </text>
    </svg>
  );
}

function FormCard({ form }: { form: CornerTeamForm }) {
  return (
    <div className="rounded-2xl border border-line bg-ink-900/50 p-4">
      <header className="flex items-center justify-between gap-2">
        <h2 className="flex items-center gap-2 text-sm font-semibold text-mist-100">
          <TeamCrest
            src={absoluteAsset(form.team.crest_url)}
            name={form.team.name}
            size={18}
          />
          {form.team.name}
        </h2>
        <span className="font-mono text-[10px] text-mist-500">
          {form.sample} match{form.sample === 1 ? "" : "es"}
        </span>
      </header>

      {form.sample === 0 ? (
        <p className="mt-3 text-xs text-mist-500">
          No finished matches with corner statistics in our coverage.
        </p>
      ) : (
        <>
          <dl className="mt-3 grid grid-cols-3 gap-2 text-center">
            <Stat label="For" value={form.avg_for.toFixed(1)} />
            <Stat label="Against" value={form.avg_against.toFixed(1)} />
            <Stat label="Total" value={form.avg_total.toFixed(1)} />
          </dl>

          <table className="mt-3 w-full text-left text-[11px]">
            <thead className="font-mono text-[9px] tracking-widest text-mist-500 uppercase">
              <tr>
                <th className="py-1 font-normal">Date</th>
                <th className="py-1 font-normal">Opponent</th>
                <th className="py-1 text-right font-normal">For</th>
                <th className="py-1 text-right font-normal">Ag</th>
                <th className="py-1 text-right font-normal">Tot</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-white/[0.05]">
              {form.matches.map((match) => (
                <tr key={match.event_id}>
                  <td className="py-1.5 font-mono text-mist-500">
                    {match.date.slice(5)}
                  </td>
                  <td className="truncate py-1.5 text-mist-200">{match.opponent}</td>
                  <td className="py-1.5 text-right font-mono text-mist-200">
                    {match.corners_for}
                  </td>
                  <td className="py-1.5 text-right font-mono text-mist-300">
                    {match.corners_against}
                  </td>
                  <td className="py-1.5 text-right font-mono text-mist-100">
                    {match.total_corners}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>

          <div className="mt-3 border-t border-line pt-3">
            <p className="font-mono text-[9px] tracking-widest text-mist-500 uppercase">
              Over rate in these matches
            </p>
            <div className="mt-1.5 flex flex-wrap gap-2">
              {OVER_LINES.map((key) => (
                <span
                  key={key}
                  className="rounded-md border border-line px-2 py-1 font-mono text-[10px] text-mist-300"
                >
                  {key}:{" "}
                  {form.over_rates[key] !== undefined
                    ? `${Math.round(form.over_rates[key] * 100)}%`
                    : "-"}
                </span>
              ))}
            </div>
          </div>
        </>
      )}
    </div>
  );
}

function Stat({ label, value }: { label: string; value: string }) {
  return (
    <div className="rounded-lg border border-line bg-ink-950/40 py-1.5">
      <dt className="font-mono text-[9px] tracking-widest text-mist-500 uppercase">
        {label}
      </dt>
      <dd className="font-mono text-sm tabular-nums text-mist-100">{value}</dd>
    </div>
  );
}
