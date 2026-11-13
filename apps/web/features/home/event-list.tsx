"use client";

import Link from "next/link";
import { faChartLine } from "@fortawesome/free-solid-svg-icons";
import { FontAwesomeIcon } from "@fortawesome/react-fontawesome";

import { TeamCrest } from "@/components/team-crest";
import {
  absoluteAsset,
  type EventItem,
  type EventTeam,
} from "@/lib/api";
import {
  eventDayLabel,
  eventLocalDate,
  eventTimeLabel,
  groupByCompetition,
  scoreLine,
  statusLabel,
  type DisplayState,
  type HomeView,
} from "@/features/home/format";
import { sportBySlug } from "@/features/home/sport-switcher";

export function EventList({
  events,
  view,
  sport,
  today,
  tomorrow,
}: {
  events: EventItem[];
  view: HomeView;
  sport: string;
  today: string;
  tomorrow: string;
}) {
  if (events.length === 0) {
    return <EmptyState view={view} sport={sport} />;
  }

  if (view === "upcoming") {
    const byDay = new Map<string, EventItem[]>();
    for (const event of events) {
      const day = eventLocalDate(event) ?? "unknown";
      const list = byDay.get(day) ?? [];
      list.push(event);
      byDay.set(day, list);
    }
    const days = [...byDay.keys()].sort();
    return (
      <div className="space-y-8">
        {days.map((day) => (
          <section key={day}>
            <h2 className="mb-3 flex items-baseline gap-2">
              <span className="text-sm font-semibold text-mist-100">
                {day === tomorrow ? "Tomorrow" : dayLabel(day)}
              </span>
              <span className="font-mono text-[11px] text-mist-500">
                {day}
              </span>
            </h2>
            <CompetitionGroups
              events={byDay.get(day) ?? []}
              sport={sport}
              today={today}
              tomorrow={tomorrow}
            />
          </section>
        ))}
      </div>
    );
  }

  return (
    <CompetitionGroups
      events={events}
      sport={sport}
      today={today}
      tomorrow={tomorrow}
    />
  );
}

function dayLabel(day: string): string {
  const parsed = new Date(`${day}T12:00:00`);
  if (Number.isNaN(parsed.valueOf())) return day;
  return parsed.toLocaleDateString("en-GB", {
    weekday: "long",
    day: "numeric",
    month: "short",
  });
}

function CompetitionGroups({
  events,
  sport,
  today,
  tomorrow,
}: {
  events: EventItem[];
  sport: string;
  today: string;
  tomorrow: string;
}) {
  const groups = groupByCompetition(events);
  return (
    <div className="space-y-5">
      {groups.map((group) => (
        <section
          key={group.competition}
          className="overflow-hidden rounded-2xl border border-line bg-ink-900/40"
        >
          <header className="flex items-center justify-between gap-3 border-b border-line px-4 py-3">
            <div className="flex min-w-0 items-center gap-2">
              <span className="grid size-6 shrink-0 place-items-center rounded bg-white/[0.06] text-[10px] font-semibold text-mist-400">
                {group.competition.slice(0, 1)}
              </span>
              <h3 className="truncate text-sm font-semibold text-mist-100">
                {group.competition}
              </h3>
            </div>
            <span className="font-mono text-[11px] text-mist-500">
              {group.events.length}{" "}
              {sport === "football"
                ? group.events.length === 1
                  ? "match"
                  : "matches"
                : group.events.length === 1
                  ? "game"
                  : "games"}
            </span>
          </header>

          <ul className="divide-y divide-white/[0.05]">
            {group.events.map((event) => (
              <EventRow
                key={event.id}
                event={event}
                sport={sport}
                today={today}
                tomorrow={tomorrow}
              />
            ))}
          </ul>
        </section>
      ))}
    </div>
  );
}

function EventRow({
  event,
  sport,
  today,
  tomorrow,
}: {
  event: EventItem;
  sport: string;
  today: string;
  tomorrow: string;
}) {
  const state: DisplayState =
    event.status === "live"
      ? "live"
      : event.status === "finished"
        ? "finished"
        : event.status === "postponed"
          ? "postponed"
          : event.status === "cancelled" || event.status === "abandoned"
            ? "cancelled"
            : "today-upcoming";

  const score = scoreLine(event);

  return (
    <li>
      <Link
        href={`/events/detail?id=${event.id}`}
        className="grid items-center gap-3 px-4 py-3 transition hover:bg-white/[0.03] sm:grid-cols-[76px_minmax(0,1fr)_auto]"
      >
        <StatusCell event={event} state={state} sport={sport} />

        <div className="min-w-0">
          <TeamLine
            team={event.home}
            score={event.result?.home_score ?? event.live?.home_score}
            state={state}
            winner={
              score !== null &&
              (event.result?.home_score ?? event.live?.home_score ?? 0) >
                (event.result?.away_score ?? event.live?.away_score ?? 0)
            }
          />
          <TeamLine
            team={event.away}
            score={event.result?.away_score ?? event.live?.away_score}
            state={state}
            winner={
              score !== null &&
              (event.result?.away_score ?? event.live?.away_score ?? 0) >
                (event.result?.home_score ?? event.live?.home_score ?? 0)
            }
          />
        </div>

        <div className="flex items-center justify-between gap-3 sm:justify-end">
          {hasAnyPrice(event) ? (
            sport === "football" ? (
              <OddsGroup
                values={[
                  { label: "1", value: event.odds?.home ?? null },
                  { label: "X", value: event.odds?.draw ?? null },
                  { label: "2", value: event.odds?.away ?? null },
                ]}
              />
            ) : (
              <OddsGroup
                values={[
                  { label: "1", value: event.odds?.home ?? null },
                  { label: "2", value: event.odds?.away ?? null },
                ]}
              />
            )
          ) : null}

          <span
            className="hidden size-7 shrink-0 place-items-center rounded-md border border-line text-mist-500 transition group-hover:text-mist-200 sm:grid"
            title="Price history"
          >
            <FontAwesomeIcon icon={faChartLine} className="size-3" />
          </span>
        </div>
      </Link>
    </li>
  );
}

/** True when at least one captured price exists, so no column of dashes shows. */
function hasAnyPrice(event: EventItem): boolean {
  return (
    event.odds?.home != null ||
    event.odds?.draw != null ||
    event.odds?.away != null
  );
}

function TeamLine({
  team,
  score,
  winner,
  state,
}: {
  team: EventTeam;
  score: number | null | undefined;
  winner: boolean;
  state: DisplayState;
}) {
  return (
    <div className="flex items-center gap-2">
      <TeamCrest src={absoluteAsset(team.crest_url)} name={team.name} size={18} />
      <span
        className={`truncate text-sm ${
          state === "finished" && !winner ? "text-mist-400" : "text-mist-100"
        }`}
      >
        {team.name}
      </span>
      {state === "live" || state === "finished" ? (
        <span
          className={`ml-auto shrink-0 font-mono text-sm font-semibold tabular-nums ${
            winner ? "text-mist-100" : "text-mist-400"
          }`}
        >
          {score ?? "-"}
        </span>
      ) : null}
    </div>
  );
}

function StatusCell({
  event,
  state,
  sport,
}: {
  event: EventItem;
  state: DisplayState;
  sport: string;
}) {
  if (state === "live") {
    return (
      <div className="flex items-center gap-2 sm:block">
        <span className="inline-flex items-center gap-1.5 rounded bg-red-500/15 px-5 py-0.5 font-mono text-[10px] font-semibold tracking-widest text-red-300 uppercase">
          <span className="size-1.5 animate-pulse rounded-full bg-red-400" />
          Live
        </span>
        <p className="font-mono text-[11px] text-mist-400">
          {event.live?.detail ?? statusLabel(event, state)}
        </p>
      </div>
    );
  }

  if (state === "finished") {
    return (
      <span className="font-mono text-[11px] tracking-widest text-mist-500 uppercase">
        FT
      </span>
    );
  }

  if (state === "postponed" || state === "cancelled") {
    return (
      <span className="font-mono text-[10px] tracking-widest text-flag-400 uppercase">
        {state === "postponed" ? "Postponed" : "Cancelled"}
      </span>
    );
  }

  return (
    <div>
      <p className="font-mono text-sm text-mist-200">{eventTimeLabel(event)}</p>
      {sport !== "football" && (
        <p className="font-mono text-[10px] text-mist-500">
          {eventDayLabel(event)}
        </p>
      )}
    </div>
  );
}

function OddsGroup({
  values,
}: {
  values: { label: string; value: number | null }[];
}) {
  return (
    <div className="flex items-center gap-1">
      {values.map(({ label, value }) => (
        <span
          key={label}
          className="flex min-w-[54px] items-center justify-between gap-2 rounded-md border border-line bg-white/[0.03] px-2 py-1"
        >
          <span className="font-mono text-[9px] tracking-wider text-mist-500">
            {label}
          </span>
          <span className="font-mono text-[12px] font-semibold text-mist-100 tabular-nums">
            {value !== null ? value.toFixed(2) : "-"}
          </span>
        </span>
      ))}
    </div>
  );
}

function EmptyState({ view, sport }: { view: HomeView; sport: string }) {
  const label = sportBySlug(sport).label;
  const message =
    view === "live"
      ? `No live ${label} games right now.`
      : view === "today"
        ? `No ${label} games scheduled today.`
        : view === "tomorrow"
          ? `No ${label} games scheduled tomorrow.`
          : view === "upcoming"
            ? `No upcoming ${label} games in the next two weeks.`
            : `No ${label} games on that date.`;
  return (
    <div className="rounded-2xl border border-line bg-ink-900/40 p-8 text-center">
      <p className="text-sm text-mist-300">{message}</p>
      <p className="mt-1 text-xs text-mist-500">
        Nothing is invented to fill this space.
      </p>
    </div>
  );
}
