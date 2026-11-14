"use client";

import Link from "next/link";

import { TeamCrest } from "@/components/team-crest";
import { absoluteAsset, formatMoney, type BetLeg, type PersonalBet } from "@/lib/api";

import { legNames, marketLabel, placedLabel } from "./derive";
import { Icon } from "./icons";

export function BetCard({ bet }: { bet: PersonalBet }) {
  const settled = bet.status === "won" || bet.status === "lost";
  const profit = bet.profit_loss;
  const returnValue = bet.actual_return ?? bet.potential_return;

  return (
    <li className="rounded-2xl border border-line bg-ink-900/50 p-4 transition hover:border-line-strong">
      <header className="flex flex-wrap items-start justify-between gap-3">
        <div className="flex min-w-0 items-start gap-3">
          <span className="grid size-9 shrink-0 place-items-center rounded-lg border border-turf-400/25 bg-turf-500/10 text-turf-300">
            <Icon name="spark" className="size-4" />
          </span>
          <div className="min-w-0">
            <p className="truncate text-sm font-medium text-mist-100">
              {bet.bet_type ?? "Bet"} · #{bet.external_id}
              {bet.placed_by ? (
                <span className="ml-2 rounded bg-white/[0.08] px-1.5 py-0.5 font-mono text-[9px] tracking-widest text-mist-400 uppercase">
                  {bet.placed_by}
                </span>
              ) : null}
            </p>
            <p className="mt-0.5 font-mono text-[11px] text-mist-500">
              {placedLabel(bet.placed_at)} · {bet.bookmaker}
            </p>
          </div>
        </div>
        <StatusPill status={bet.status} />
      </header>

      <dl className="mt-3 grid grid-cols-2 gap-3 rounded-xl border border-line bg-ink-950/30 px-3 py-2.5 sm:grid-cols-4">
        <Metric label="Stake" value={formatMoney(bet.stake, bet.currency)} />
        <Metric
          label="Odds"
          value={bet.combined_odds !== null ? bet.combined_odds.toFixed(3) : "-"}
        />
        <Metric
          label={settled ? "Return" : "Potential"}
          value={formatMoney(returnValue, bet.currency)}
        />
        <Metric
          label="P/L"
          value={profit === null ? "-" : formatMoney(profit, bet.currency)}
          tone={profit === null ? "muted" : profit >= 0 ? "good" : "bad"}
        />
      </dl>

      <div className="mt-2 overflow-hidden rounded-xl border border-line bg-ink-950/30">
        <div className="flex items-center gap-2 border-b border-line px-3 py-2">
          <span className="rounded-full border border-line-strong px-2 py-0.5 font-mono text-[9px] tracking-widest text-mist-400 uppercase">
            {bet.legs.length} {bet.legs.length === 1 ? "leg" : "legs"}
          </span>
          <span className="truncate font-mono text-[10px] text-mist-500">
            {bet.source}
          </span>
        </div>
        <ul className="divide-y divide-line">
          {bet.legs.map((leg) => (
            <LegRow key={leg.leg_number} leg={leg} />
          ))}
        </ul>
      </div>
    </li>
  );
}

function LegRow({ leg }: { leg: BetLeg }) {
  const names = legNames(leg);
  const market = marketLabel(leg);
  const unresolved = !leg.event_id
    ? leg.resolution_note === "no_matching_fixture"
      ? "not in coverage"
      : leg.resolution_note === "unparsed_event_name"
        ? "name unparsed"
        : "fixture unresolved"
    : null;

  return (
    <li className="flex flex-col gap-2 px-3 py-2.5 sm:grid sm:grid-cols-[minmax(0,1fr)_minmax(0,14rem)_3.5rem] sm:items-center sm:gap-3">
      <div className="flex min-w-0 items-start gap-2">
        <LegStatus status={leg.leg_status} />
        <span className="flex shrink-0 items-center gap-0.5 pt-0.5">
          <TeamCrest
            src={absoluteAsset(leg.home_team?.crest_url ?? null)}
            name={leg.home_team?.name ?? names.home}
            size={16}
          />
          <TeamCrest
            src={absoluteAsset(leg.away_team?.crest_url ?? null)}
            name={leg.away_team?.name ?? names.away}
            size={16}
          />
        </span>
        <div className="min-w-0">
          {leg.event_id ? (
            <Link
              href={`/events/detail?id=${leg.event_id}`}
              className="block truncate text-[13px] text-mist-100 underline decoration-line-strong underline-offset-4 transition hover:text-turf-300"
            >
              {leg.event_raw}
            </Link>
          ) : (
            <p className="truncate text-[13px] text-mist-200">{leg.event_raw}</p>
          )}
          <p className="truncate font-mono text-[10px] text-mist-500">
            {leg.competition ?? "competition unknown"}
            {unresolved ? ` · ${unresolved}` : ""}
          </p>
        </div>
      </div>

      <p className="truncate pl-7 text-[11px] text-mist-300 sm:pl-0 sm:text-right">
        {market ?? leg.market_raw ?? "market unparsed"}
      </p>

      <p className="pl-7 font-mono text-[11px] text-mist-200 sm:pl-0 sm:text-right">
        {leg.odds_taken !== null ? leg.odds_taken.toFixed(2) : "-"}
      </p>
    </li>
  );
}

function LegStatus({ status }: { status: string }) {
  if (status === "won") {
    return <Icon name="check" className="mt-0.5 size-[18px] shrink-0 text-turf-400" />;
  }
  if (status === "lost") {
    return <Icon name="x" className="mt-0.5 size-[18px] shrink-0 text-loss-400" />;
  }
  return <Icon name="clock" className="mt-0.5 size-[18px] shrink-0 text-flag-400" />;
}

function StatusPill({ status }: { status: string }) {
  const style =
    status === "won"
      ? "border-turf-400/30 bg-turf-500/10 text-turf-300"
      : status === "lost"
        ? "border-loss-400/25 bg-loss-500/10 text-loss-300"
        : "border-flag-400/25 bg-flag-400/[0.08] text-flag-400";
  const icon = status === "won" ? "check" : status === "lost" ? "x" : "clock";
  return (
    <span
      className={`inline-flex items-center gap-1.5 rounded-full border px-2.5 py-0.5 font-mono text-[9px] tracking-widest uppercase ${style}`}
    >
      <Icon name={icon} className="size-3" />
      {status}
    </span>
  );
}

function Metric({
  label,
  value,
  tone = "plain",
}: {
  label: string;
  value: string;
  tone?: "plain" | "good" | "bad" | "muted";
}) {
  const color =
    tone === "good"
      ? "text-turf-300"
      : tone === "bad"
        ? "text-loss-300"
        : tone === "muted"
          ? "text-mist-500"
          : "text-mist-100";
  return (
    <div className="min-w-0">
      <dt className="font-mono text-[9px] tracking-widest text-mist-500 uppercase">
        {label}
      </dt>
      <dd className={`mt-0.5 truncate font-mono text-[13px] ${color}`}>{value}</dd>
    </div>
  );
}
