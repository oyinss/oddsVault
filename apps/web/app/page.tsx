"use client";

import Link from "next/link";
import { useMemo, useState } from "react";
import {
  faArrowRight,
  faCalendarDay,
  faCalendarDays,
  faRotateRight,
  faTowerBroadcast,
  faTrophy,
  type IconDefinition,
} from "@fortawesome/free-solid-svg-icons";
import { FontAwesomeIcon } from "@fortawesome/react-fontawesome";

import { LoadingNote } from "@/components/loading-note";
import { SiteHeader } from "@/components/site-header";
import { EventList } from "@/features/home/event-list";
import { FilterBar } from "@/features/home/filter-bar";
import {
  localISODate,
  sortEvents,
  type HomeView,
} from "@/features/home/format";
import { HomeSidebar } from "@/features/home/sidebar";
import { SportSwitcher, sportBySlug } from "@/features/home/sport-switcher";
import {
  useHighlights,
  useHomeEvents,
  useHomeSummary,
} from "@/features/home/use-home-data";

export default function HomePage() {
  const today = localISODate();
  const [sport, setSport] = useState("football");
  const [view, setView] = useState<HomeView>("today");
  const [date, setDate] = useState(today);
  const [search, setSearch] = useState("");
  const [competition, setCompetition] = useState("");
  const [sort, setSort] = useState("live_first");

  const { events, loading, error, today: todayDay, tomorrow, refresh } =
    useHomeEvents({ sport, view, date });
  const summary = useHomeSummary(sport);
  const highlights = useHighlights(sport);
  const selectedSport = sportBySlug(sport);

  const competitions = useMemo(
    () =>
      [...new Set(events.map((event) => event.competition_name))].sort((a, b) =>
        a.localeCompare(b),
      ),
    [events],
  );

  const visible = useMemo(() => {
    const needle = search.trim().toLowerCase();
    const filtered = events.filter((event) => {
      if (competition && event.competition_name !== competition) return false;
      if (!needle) return true;
      return [event.home.name, event.away.name, event.competition_name, event.round ?? ""]
        .join(" ")
        .toLowerCase()
        .includes(needle);
    });
    return sortEvents(filtered, sort);
  }, [events, search, competition, sort]);

  function changeSport(next: string) {
    setSport(next);
    setCompetition("");
    setSearch("");
  }

  return (
    <>
      <SiteHeader />
      <main className="mx-auto max-w-7xl px-4 py-6 sm:px-6 lg:py-8">
        <section className="relative overflow-hidden rounded-3xl border border-line">
          <div aria-hidden className="pointer-events-none absolute inset-0">
            <div className="hero-glow absolute inset-0" />
            <div className="pitch-lines absolute inset-0 opacity-70" />
          </div>

          <div className="relative grid gap-6 px-5 py-7 sm:px-8 sm:py-9 lg:grid-cols-[minmax(0,1fr)_auto] lg:items-center">
            <div>
              <p className="flex items-center gap-2 font-mono text-[10px] tracking-[0.2em] text-red-300 uppercase">
                <span className="size-1.5 animate-pulse rounded-full bg-red-400" />
                {summary && summary.live > 0
                  ? `${summary.live} live now`
                  : "Sports intelligence"}
              </p>
              <h1 className="mt-2 max-w-2xl text-3xl leading-tight font-semibold tracking-tight sm:text-4xl">
                Real-time sports odds
                <br />
                <span className="text-mist-400">and insights.</span>
              </h1>
              <p className="mt-3 max-w-xl text-sm leading-relaxed text-mist-300">
                Live scores, captured bookmaker prices and personal betting
                history, built on verified results rather than placeholder
                data.
              </p>
              <div className="mt-5 flex flex-wrap gap-2">
                <Link
                  href="/events"
                  className="inline-flex items-center gap-2 rounded-lg bg-turf-500 px-4 py-2 text-sm font-semibold text-ink-950 transition hover:bg-turf-400"
                >
                  Explore events
                  <FontAwesomeIcon icon={faArrowRight} className="size-3" />
                </Link>
                <Link
                  href="/my-bets"
                  className="inline-flex items-center gap-2 rounded-lg border border-white/10 px-4 py-2 text-sm text-mist-200 transition hover:border-white/25 hover:bg-white/5"
                >
                  My bets
                </Link>
              </div>
            </div>

            <dl className="grid grid-cols-2 gap-2 lg:w-[340px]">
              <HeroStat
                label="Live"
                value={summary?.live ?? 0}
                tone="text-red-300"
                icon={faTowerBroadcast}
              />
              <HeroStat label="Today" value={summary?.today ?? 0} icon={faCalendarDay} />
              <HeroStat label="Upcoming" value={summary?.upcoming ?? 0} icon={faCalendarDays} />
              <HeroStat label="Comps" value={summary?.competitions ?? 0} icon={faTrophy} />
            </dl>
          </div>
        </section>

        <div className="mt-5">
          <SportSwitcher value={sport} onChange={changeSport} />
        </div>

        <div className="mt-5">
          <FilterBar
            view={view}
            onView={setView}
            date={date}
            onDate={setDate}
            search={search}
            onSearch={setSearch}
            competitions={competitions}
            competition={competition}
            onCompetition={setCompetition}
            sort={sort}
            onSort={setSort}
          />
        </div>

        <div className="mt-6 grid gap-6 lg:grid-cols-[minmax(0,1fr)_340px]">
          <section className="min-w-0">
            <div className="mb-3 flex items-center justify-between gap-3">
              <h2 className="flex items-center gap-2 text-lg font-semibold tracking-tight">
                <span className="grid size-8 place-items-center rounded-lg bg-turf-500/10 text-turf-300">
                  <FontAwesomeIcon icon={selectedSport.icon} className="size-3.5" />
                </span>
                {selectedSport.label}
              </h2>
              <div className="flex items-center gap-3">
                <span className="font-mono text-[11px] text-mist-500">
                  {visible.length} {visible.length === 1 ? "game" : "games"}
                </span>
                <button
                  type="button"
                  onClick={refresh}
                  className="grid size-8 place-items-center rounded-lg border border-line text-mist-400 transition hover:border-white/20 hover:text-mist-100"
                  title="Refresh"
                >
                  <FontAwesomeIcon icon={faRotateRight} className="size-3" />
                </button>
              </div>
            </div>

            {error ? (
              <div className="rounded-2xl border border-flag-400/30 bg-flag-400/[0.04] p-6">
                <p className="text-sm font-medium text-flag-400">
                  Unable to load events.
                </p>
                <p className="mt-1 text-xs text-mist-400">{error}</p>
                <button
                  type="button"
                  onClick={refresh}
                  className="mt-3 inline-flex items-center gap-2 rounded-lg border border-flag-400/40 px-3 py-1.5 text-xs font-semibold text-flag-400 transition hover:bg-flag-400/10"
                >
                  <FontAwesomeIcon icon={faRotateRight} className="size-3" />
                  Try again
                </button>
              </div>
            ) : loading && events.length === 0 ? (
              <LoadingNote label={`Loading ${selectedSport.label.toLowerCase()} games...`} />
            ) : (
              <EventList
                events={visible}
                view={view}
                sport={sport}
                today={todayDay}
                tomorrow={tomorrow}
              />
            )}

            {view === "today" && (
              <p className="mt-3 flex items-center gap-1.5 font-mono text-[10px] text-mist-500">
                <FontAwesomeIcon icon={faCalendarDays} className="size-2.5" />
                Times shown in your local timezone
              </p>
            )}
          </section>

          <HomeSidebar
            sport={sport}
            summary={summary}
            highlights={highlights}
          />
        </div>
      </main>
    </>
  );
}

function HeroStat({
  label,
  value,
  tone,
  icon,
}: {
  label: string;
  value: number;
  tone?: string;
  icon: IconDefinition;
}) {
  return (
    <div className="rounded-xl border border-line bg-ink-950/55 px-3 py-2.5 backdrop-blur">
      <dt className="flex items-center gap-1.5 overflow-hidden">
        <FontAwesomeIcon
          icon={icon}
          className="size-2.5 shrink-0 text-mist-500"
        />
        <span className="truncate font-mono text-[9px] tracking-wider text-mist-500 uppercase">
          {label}
        </span>
      </dt>
      <dd className={`mt-0.5 font-mono text-xl tabular-nums ${tone ?? "text-mist-100"}`}>
        {value}
      </dd>
    </div>
  );
}
