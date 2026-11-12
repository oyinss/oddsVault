"use client";

import Link from "next/link";
import { useSearchParams } from "next/navigation";
import { Suspense } from "react";

import { LoadingNote } from "@/components/loading-note";
import { SiteHeader } from "@/components/site-header";
import { TeamCrest } from "@/components/team-crest";
import {
  absoluteAsset,
  formatAge,
  formatDate,
  getCompetitions,
  getEvents,
  kickoff,
  todayInLagos,
  type EventItem,
  type EventList,
} from "@/lib/api";
import { useApiData } from "@/lib/use-api";

const FILTERS = [
  { label: "All", value: "" },
  { label: "Today", value: "today" },
  { label: "Finished", value: "finished" },
  { label: "Scheduled", value: "scheduled" },
];

const VALID_STATUSES = new Set(["finished", "scheduled", "live"]);

export default function EventsPage() {
  return (
    <Suspense
      fallback={
        <>
          <SiteHeader />
          <main className="mx-auto max-w-6xl px-6 py-12">
            <LoadingNote label="Loading fixtures…" />
          </main>
        </>
      }
    >
      <EventsView />
    </Suspense>
  );
}

function EventsView() {
  const searchParams = useSearchParams();
  const dayParam = searchParams.get("day");
  const requested = searchParams.get("status") ?? "";
  const status = VALID_STATUSES.has(requested) ? requested : "";
  const today = todayInLagos();
  // Today is the default: no parameters at all renders today's fixtures.
  const isToday =
    dayParam === "today" || (dayParam === null && status === "");

  const { data, loading } = useApiData(
    `events:${isToday ? `day:${today}` : status}`,
    () =>
      Promise.all([
        getCompetitions(),
        isToday
          ? getEvents({ from: today, to: today, limit: 200 })
          : getEvents({ status, limit: 80 }),
      ]).then(([competitions, list]) => ({ competitions, list })),
  );

  const competitions = data?.competitions ?? [];
  const list: EventList = data?.list ?? { ok: true, events: [] };
  const premierLeague = competitions.find(
    (competition) => competition.slug === "premier-league",
  );
  const events = list.ok ? list.events : [];
  const finished = events.filter((event) => event.status === "finished").length;
  const todayCount = isToday
    ? events.length
    : events.filter((event) => event.start_date === today).length;
  // Only the unfiltered list reliably contains every fixture today (the API
  // orders by proximity to today). On a status view the count would describe
  // that slice, not the day.
  const showTodayCount = isToday || status === "";

  return (
    <>
      <SiteHeader />

      <main className="mx-auto max-w-6xl px-6 py-12">
        <div className="flex flex-col gap-6 sm:flex-row sm:items-end sm:justify-between">
          <div>
            <p className="font-mono text-[11px] tracking-widest text-turf-300 uppercase">
              Phase 1A · Football{isToday ? ` · Today, ${formatDate(today)}` : ""}
            </p>
            <h1 className="mt-2 text-3xl font-semibold tracking-tight sm:text-4xl">
              {isToday ? "Today's fixtures" : "Fixtures & results"}
            </h1>
            <p className="mt-2 max-w-xl text-sm leading-relaxed text-mist-400">
              Imported football fixtures and results. Kickoff times are shown in
              your timezone when the provider publishes a UTC time; otherwise
              the competition-local time is labelled local.
            </p>
          </div>

          {premierLeague && (
            <div className="rounded-xl border border-line bg-ink-900/70 px-4 py-3">
              <p className="font-mono text-[10px] tracking-widest text-mist-500 uppercase">
                Season
              </p>
              <p className="mt-1 text-sm font-medium">
                {premierLeague.current_season ?? "not synced"}
              </p>
            </div>
          )}
        </div>

        {loading && <LoadingNote label="Loading fixtures…" />}

        <div className="mt-8 flex flex-wrap items-center gap-2">
          {FILTERS.map((filter) => {
            const active =
              filter.value === "today"
                ? isToday
                : filter.value === ""
                  ? !isToday && status === ""
                  : !isToday && status === filter.value;
            const href =
              filter.value === "today"
                ? "/events"
                : filter.value === ""
                  ? "/events?day=all"
                  : `/events?status=${filter.value}`;
            return (
              <Link
                key={filter.label}
                href={href}
                className={`rounded-lg border px-3 py-1.5 text-sm transition ${
                  active
                    ? "border-turf-400/30 bg-turf-500/10 text-turf-300"
                    : "border-line-strong text-mist-300 hover:border-line-strong hover:bg-raised"
                }`}
              >
                {filter.value === "today"
                  ? showTodayCount
                    ? `Today (${todayCount})`
                    : "Today"
                  : filter.label}
              </Link>
            );
          })}
          <span className="ml-auto font-mono text-[11px] text-mist-500">
            {events.length} shown · {finished} finished
          </span>
        </div>

        {!list.ok && (
          <div className="panel mt-6 border-flag-400/30 p-5">
            <p className="text-sm font-medium text-flag-400">
              Events API unavailable
            </p>
            <p className="mt-1 text-xs text-mist-400">
              {list.error}. Start the Go API with{" "}
              <code className="font-mono text-mist-200">make api</code>.
            </p>
          </div>
        )}

        {!loading && list.ok && events.length === 0 && isToday && (
          <div className="panel mt-6 p-6">
            <p className="text-sm font-medium">No fixtures today</p>
            <p className="mt-1 text-xs leading-relaxed text-mist-400">
              Nothing is scheduled for {formatDate(today)} in the imported
              competitions.
            </p>
            <Link
              href="/events?day=all"
              className="mt-3 inline-block text-xs text-turf-300 underline decoration-turf-400/30 underline-offset-4"
            >
              Show all fixtures
            </Link>
          </div>
        )}

        {!loading && list.ok && events.length === 0 && !isToday && (
          <div className="panel mt-6 p-6">
            <p className="text-sm font-medium">No events stored yet</p>
            <p className="mt-1 text-xs leading-relaxed text-mist-400">
              Run the sync once to import the current season:
            </p>
            <pre className="mt-3 overflow-x-auto rounded-lg border border-line bg-ink-950 p-3 font-mono text-[11px] text-mist-200">
              JOB_RUN_ON_START=true make workers
            </pre>
          </div>
        )}

        {events.length > 0 && (
          <ul className="mt-6 divide-y divide-line overflow-hidden rounded-2xl border border-line bg-ink-900/50">
            {events.map((event) => (
              <EventRow key={event.id} event={event} />
            ))}
          </ul>
        )}

        <p className="mt-6 max-w-3xl text-[11px] leading-relaxed text-mist-500">
          Kickoff times are converted from the provider&apos;s UTC instant to
          your device timezone; venue-local time is labelled local when no UTC
          time exists. Missing scores are shown as “—”, never as 0. Events
          matched only by competition, participants and date are marked
          provisional until a provider with real ids confirms them.
        </p>
      </main>
    </>
  );
}

function EventRow({ event }: { event: EventItem }) {
  const scoreline = event.result
    ? `${event.result.home_score ?? "—"} – ${event.result.away_score ?? "—"}`
    : null;
  const when = kickoff(event);
  const today = new Date().toISOString().slice(0, 10);
  const awaitingResult =
    !scoreline && event.status === "scheduled" && (event.start_date ?? "") < today;

  return (
    <li>
      <Link
        href={`/events/detail?id=${event.id}`}
        className="flex flex-col gap-3 px-4 py-4 transition hover:bg-hover sm:flex-row sm:items-center sm:gap-6 sm:px-5"
      >
      <div className="w-28 shrink-0">
        <p className="text-sm font-medium text-mist-200">{when.date}</p>
        <p
          className="font-mono text-[11px] text-mist-500"
          title={when.title}
        >
          {when.time}
          {when.zone ? (
            <span className="ml-1 text-[9px]">{when.zone}</span>
          ) : event.start_time_local ? (
            <span className="ml-1 text-[9px]">local</span>
          ) : null}
        </p>
      </div>

      <div className="min-w-0 flex-1">
        <div className="flex min-w-0 items-center gap-2 text-sm text-mist-100">
          <TeamCrest src={absoluteAsset(event.home.crest_url)} name={event.home.name} size={18} />
          <span className="truncate">{event.home.name}</span>
          <span className="shrink-0 text-mist-500">v</span>
          <TeamCrest src={absoluteAsset(event.away.crest_url)} name={event.away.name} size={18} />
          <span className="truncate">{event.away.name}</span>
        </div>
        <p className="mt-0.5 truncate font-mono text-[11px] text-mist-500">
          {event.competition_name}
          {event.round ? ` · ${event.round}` : ""}
          {event.identity_status === "provisional" ? " · provisional" : ""}
        </p>
      </div>

      <div className="flex items-center gap-4 sm:justify-end">
        {scoreline ? (
          <div className="text-right">
            <p className="font-mono text-base font-semibold text-mist-100">
              {scoreline}
            </p>
            <p className="font-mono text-[10px] text-mist-500">
              {formatAge(event.result?.captured_at ?? null)}
            </p>
          </div>
        ) : awaitingResult ? (
          <StatusPill status="awaiting result" />
        ) : (
          <StatusPill status={event.status} />
        )}
      </div>
      </Link>
    </li>
  );
}

function StatusPill({ status }: { status: string }) {
  const styles: Record<string, string> = {
    finished: "border-line-strong text-mist-300",
    scheduled: "border-turf-400/25 bg-turf-500/10 text-turf-300",
    live: "border-flag-400/30 bg-flag-400/10 text-flag-400",
  };
  return (
    <span
      className={`rounded-full border px-3 py-1 font-mono text-[10px] tracking-widest uppercase ${
        styles[status] ?? "border-line-strong text-mist-400"
      }`}
    >
      {status}
    </span>
  );
}
