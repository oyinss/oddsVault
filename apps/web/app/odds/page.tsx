"use client";

import Link from "next/link";
import { useMemo, useState } from "react";
import {
  faArrowRight,
  faArrowsRotate,
  faCircleInfo,
  faClock,
} from "@fortawesome/free-solid-svg-icons";
import { FontAwesomeIcon } from "@fortawesome/react-fontawesome";

import { LoadingNote } from "@/components/loading-note";
import { SiteHeader } from "@/components/site-header";
import { TeamCrest } from "@/components/team-crest";
import {
  absoluteAsset,
  flagUrl,
  formatAge,
  getOddsBoard,
  type BoardEvent,
  type BoardLine,
  type BoardSelection,
  type OddsBoard,
} from "@/lib/api";
import { useApiData } from "@/lib/use-api";
import { SportSwitcher, sportBySlug } from "@/features/home/sport-switcher";

type MarketOption = { slug: string; label: string };

const FOOTBALL_MARKETS: MarketOption[] = [
  { slug: "match_result", label: "1X2" },
  { slug: "double_chance", label: "Double chance" },
  { slug: "total_goals", label: "Total goals" },
  { slug: "both_teams_to_score", label: "BTTS" },
];

const TWO_WAY_MARKETS: MarketOption[] = [
  { slug: "moneyline", label: "Moneyline" },
  { slug: "total_goals", label: "Total points" },
];

function marketsFor(sport: string): MarketOption[] {
  return sport === "football" ? FOOTBALL_MARKETS : TWO_WAY_MARKETS;
}

export default function OddsPage() {
  const [sport, setSport] = useState("football");
  const [market, setMarket] = useState("match_result");
  const [competition, setCompetition] = useState("");

  function changeSport(next: string) {
    setSport(next);
    setMarket(marketsFor(next)[0].slug);
    setCompetition("");
  }

  const markets = marketsFor(sport);
  const { data, loading, error, reload } = useApiData<OddsBoard | null>(
    `odds-board:${sport}:${market}:${competition}`,
    () => getOddsBoard({ sport, market, competition, limit: 30 }),
    { refreshMs: 60000 },
  );

  const competitions = useMemo(() => {
    const names = new Set((data?.events ?? []).map((event) => event.competition));
    return [...names].sort((a, b) => a.localeCompare(b));
  }, [data]);

  const competitionSlugs = useMemo(() => {
    const map = new Map<string, string>();
    for (const event of data?.events ?? []) {
      map.set(event.competition, event.competition_slug);
    }
    return map;
  }, [data]);

  return (
    <>
      <SiteHeader />
      <main className="mx-auto max-w-6xl px-6 py-12">
        <div className="flex flex-wrap items-end justify-between gap-4">
          <div>
            <p className="font-mono text-[11px] tracking-widest text-turf-300 uppercase">
              Odds · Captured prices
            </p>
            <h1 className="mt-2 text-3xl font-bold tracking-tight">
              Bookmaker comparison
            </h1>
            <p className="mt-2 max-w-2xl text-sm leading-relaxed text-mist-300">
              Every price below was captured from a bookmaker and never
              overwritten. The best price is the highest captured one, not an
              average, and each cell shows when it was taken.
            </p>
          </div>
          <div className="flex items-center gap-3">
            <p className="text-right font-mono text-[10px] text-mist-500">
              <span className="block tracking-widest uppercase">Newest price</span>
              {formatAge(newestCapture(data))}
            </p>
            <button
              type="button"
              onClick={reload}
              className="grid size-9 place-items-center rounded-lg border border-line text-mist-400 transition hover:border-white/20 hover:text-mist-100"
              title="Refresh prices"
            >
              <FontAwesomeIcon icon={faArrowsRotate} className="size-3.5" />
            </button>
          </div>
        </div>

        <div className="mt-6">
          <SportSwitcher value={sport} onChange={changeSport} />
        </div>

        <div className="mt-4 flex flex-wrap items-center gap-2">
          {markets.map((option) => (
            <button
              key={option.slug}
              type="button"
              onClick={() => setMarket(option.slug)}
              className={`rounded-lg border px-3 py-1.5 font-mono text-[11px] tracking-wide uppercase transition ${
                option.slug === market
                  ? "border-turf-400/40 bg-turf-500/10 text-turf-200"
                  : "border-line text-mist-400 hover:border-white/20 hover:text-mist-200"
              }`}
            >
              {option.label}
            </button>
          ))}

          {competitions.length > 1 && (
            <select
              value={competition}
              onChange={(event) => setCompetition(event.target.value)}
              className="ml-auto rounded-lg border border-line bg-ink-900 px-3 py-1.5 text-xs text-mist-200"
            >
              <option value="">All competitions</option>
              {competitions.map((name) => (
                <option key={name} value={competitionSlugs.get(name) ?? ""}>
                  {name}
                </option>
              ))}
            </select>
          )}
        </div>

        {error ? (
          <div className="mt-6 rounded-2xl border border-flag-400/30 bg-flag-400/[0.04] p-6">
            <p className="text-sm font-medium text-flag-400">
              Unable to load prices.
            </p>
            <p className="mt-1 text-xs text-mist-400">{error}</p>
          </div>
        ) : loading && !data ? (
          <LoadingNote label="Reading captured prices…" />
        ) : !data || data.events.length === 0 ? (
          <div className="mt-6 rounded-2xl border border-line bg-ink-900/60 p-6">
            <p className="text-sm font-medium text-mist-100">
              No captured prices for {sportBySlug(sport).label} in this market.
            </p>
            <p className="mt-1 text-xs text-mist-400">
              Prices appear once the collector has seen a fixture and recorded a
              bookmaker quote. Nothing is filled in to avoid an empty screen.
            </p>
          </div>
        ) : (
          <div className="mt-6 space-y-4">
            {data.events.map((event) => (
              <EventCard key={event.event_id} event={event} market={market} />
            ))}
          </div>
        )}

        <section className="mt-8 rounded-2xl border border-line bg-ink-900/50 p-5">
          <h2 className="flex items-center gap-2 text-sm font-semibold text-mist-100">
            <FontAwesomeIcon icon={faCircleInfo} className="size-3 text-mist-500" />
            How to read this
          </h2>
          <ul className="mt-3 grid gap-2 text-xs leading-relaxed text-mist-300 sm:grid-cols-2">
            <li>
              A movement figure compares the newest captured price with the
              first one on record for that selection.
            </li>
            <li>
              A highlight marks the best price only when more than one bookmaker
              priced the same selection.
            </li>
            <li>
              A missing price stays missing: no cell is filled with a neighbour
              or an estimate.
            </li>
            <li>
              A dash means the bookmaker had not priced that selection when it
              was captured.
            </li>
          </ul>
        </section>
      </main>
    </>
  );
}

function EventCard({ event, market }: { event: BoardEvent; market: string }) {
  return (
    <article className="overflow-hidden rounded-2xl border border-line bg-ink-900/40">
      <header className="flex flex-wrap items-center justify-between gap-2 border-b border-line px-4 py-2.5">
        <div className="flex min-w-0 items-center gap-2">
          <span className="flex items-center gap-1.5 rounded bg-white/[0.06] px-1.5 py-0.5 font-mono text-[9px] tracking-wider text-mist-400 uppercase">
            {flagUrl(event.competition_country) ? (
              // eslint-disable-next-line @next/next/no-img-element
              <img
                src={flagUrl(event.competition_country) ?? undefined}
                alt=""
                width={16}
                height={11}
                className="h-[11px] w-[16px] shrink-0 rounded-[2px] object-cover"
              />
            ) : null}
            {event.competition}
          </span>
          <span className="font-mono text-[11px] text-mist-400">
            {kickoffLabel(event.kickoff)}
          </span>
        </div>
        <span className="flex items-center gap-1.5 font-mono text-[10px] text-mist-500">
          <FontAwesomeIcon icon={faClock} className="size-2.5" />
          {formatAge(event.last_captured_at)}
        </span>
      </header>

      <div className="px-4 py-3">
        <Link
          href={`/events/detail?id=${event.event_id}`}
          className="group flex items-center gap-2"
        >
          <span className="flex min-w-0 items-center gap-2 text-sm font-semibold text-mist-100">
            <TeamCrest
              src={absoluteAsset(event.home.crest_url)}
              name={event.home.name}
              size={16}
            />
            <span className="truncate">{event.home.name}</span>
            <span className="text-mist-500">v</span>
            <TeamCrest
              src={absoluteAsset(event.away.crest_url)}
              name={event.away.name}
              size={16}
            />
            <span className="truncate">{event.away.name}</span>
          </span>
          <FontAwesomeIcon
            icon={faArrowRight}
            className="size-2.5 shrink-0 text-mist-600 transition group-hover:text-mist-200"
          />
        </Link>

        <div className="mt-3 space-y-2">
          {event.lines.map((line, index) => (
            <LineRow
              key={`${line.line ?? "main"}-${index}`}
              line={line}
              event={event}
              market={market}
            />
          ))}
        </div>
      </div>
    </article>
  );
}

function LineRow({
  line,
  event,
  market,
}: {
  line: BoardLine;
  event: BoardEvent;
  market: string;
}) {
  return (
    <div className="flex flex-wrap items-stretch gap-2">
      {line.line !== null && (
        <span className="grid min-w-[54px] place-items-center rounded-lg border border-line bg-ink-950/40 px-2 font-mono text-[11px] text-mist-400">
          {line.line}
        </span>
      )}
      {line.selections.map((selection) => (
        <PriceCell
          key={selection.selection}
          selection={selection}
          label={selectionLabel(selection.selection, event, market)}
        />
      ))}
    </div>
  );
}

function PriceCell({
  selection,
  label,
}: {
  selection: BoardSelection;
  label: string;
}) {
  const best = selection.best;
  const multipleBooks = selection.prices.length > 1;

  return (
    <div
      className={`min-w-[132px] flex-1 rounded-lg border px-3 py-2 ${
        best && multipleBooks
          ? "border-turf-400/40 bg-turf-500/[0.07]"
          : "border-line bg-white/[0.02]"
      }`}
    >
      <div className="flex items-baseline justify-between gap-2">
        <span className="truncate text-[11px] text-mist-400">{label}</span>
        <span className="font-mono text-sm font-semibold tabular-nums text-mist-100">
          {best ? best.odds.toFixed(2) : "-"}
        </span>
      </div>
      <div className="mt-1 flex items-center justify-between gap-2">
        <span className="truncate font-mono text-[9px] tracking-wide text-mist-500 uppercase">
          {best ? best.bookmaker_name : "no price"}
        </span>
        <Movement value={selection.movement} />
      </div>
    </div>
  );
}

function Movement({ value }: { value: number | null }) {
  if (value === null || Math.abs(value) < 0.001) {
    return <span className="font-mono text-[9px] text-mist-600">flat</span>;
  }
  const up = value > 0;
  return (
    <span
      className={`font-mono text-[9px] tabular-nums ${
        up ? "text-turf-300" : "text-loss-400"
      }`}
      title={up ? "price drifted out" : "price shortened"}
    >
      {up ? "+" : ""}
      {(value * 100).toFixed(1)}%
    </span>
  );
}

function selectionLabel(
  selection: string,
  event: BoardEvent,
  market: string,
): string {
  switch (selection) {
    case "home":
      return shortName(event.home.name);
    case "away":
      return shortName(event.away.name);
    case "draw":
      return "Draw";
    case "over":
      return "Over";
    case "under":
      return "Under";
    case "yes":
      return "Yes";
    case "no":
      return "No";
    case "home_or_draw":
      return `${shortName(event.home.name)} / Draw`;
    case "home_or_away":
      return "Either team";
    case "draw_or_away":
      return `Draw / ${shortName(event.away.name)}`;
    default:
      return selection;
  }
}

function shortName(name: string): string {
  const words = name.split(" ");
  if (words.length === 1) return name;
  // Keep the last word for city + nickname clubs, which is the distinctive part.
  return words.slice(-1)[0];
}

function newestCapture(board: OddsBoard | null): string | null {
  let newest: string | null = null;
  for (const event of board?.events ?? []) {
    if (!event.last_captured_at) continue;
    if (!newest || Date.parse(event.last_captured_at) > Date.parse(newest)) {
      newest = event.last_captured_at;
    }
  }
  return newest;
}

function kickoffLabel(iso: string): string {
  const at = new Date(iso);
  if (Number.isNaN(at.valueOf())) return "kickoff unknown";
  return at.toLocaleString("en-GB", {
    weekday: "short",
    day: "numeric",
    month: "short",
    hour: "2-digit",
    minute: "2-digit",
  });
}
