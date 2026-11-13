"use client";

import Link from "next/link";

import { LoadingNote } from "@/components/loading-note";
import { RefreshLabel } from "@/components/refresh-label";
import { SiteHeader } from "@/components/site-header";
import { TeamCrest } from "@/components/team-crest";
import {
  absoluteAsset,
  formatAge,
  getLive,
  kickoff,
  type EventItem,
  type EventList,
} from "@/lib/api";
import { useApiData } from "@/lib/use-api";

const REFRESH_SECONDS = 30;

export default function LivePage() {
  const { data, loading } = useApiData("live", getLive, {
    refreshMs: REFRESH_SECONDS * 1000,
  });
  const list: EventList = data ?? { ok: true, events: [] };
  const events = list.ok ? list.events : [];
  const live = events.filter(
    (event) => event.status === "live" || event.live?.status === "live",
  ).length;

  return (
    <>
      <SiteHeader />
      <main className="mx-auto max-w-5xl px-6 py-12">
        <div className="flex flex-wrap items-end justify-between gap-4">
          <div>
            <p className="font-mono text-[11px] tracking-widest text-turf-300 uppercase">
              Phase 1B · Live
            </p>
            <h1 className="mt-2 text-3xl font-bold tracking-tight">
              Live &amp; today
            </h1>
            <p className="mt-2 max-w-xl text-sm leading-relaxed text-mist-400">
              Live matches and today&apos;s fixtures. Scores come from the
              provider, polled by the worker and cached; the browser talks only
              to our API.
            </p>
          </div>
          <div className="flex items-center gap-4">
            <span className="rounded-full border border-line-strong px-3 py-1 font-mono text-[10px] tracking-widest text-mist-400 uppercase">
              {live} live
            </span>
            <RefreshLabel seconds={REFRESH_SECONDS} />
          </div>
        </div>

        {loading && <LoadingNote label="Loading live feed…" />}

        {!list.ok && (
          <div className="panel mt-8 border-flag-400/30 p-5">
            <p className="text-sm font-medium text-flag-400">
              Live feed unavailable
            </p>
            <p className="mt-1 text-xs text-mist-400">{list.error}</p>
          </div>
        )}

        {!loading && list.ok && events.length === 0 && (
          <div className="panel mt-8 p-6">
            <p className="text-sm font-medium">Nothing live right now</p>
            <p className="mt-1 text-xs leading-relaxed text-mist-400">
              No match is in progress and nothing else is scheduled today.
              Results and upcoming days are on the fixtures page.
            </p>
            <Link
              href="/events?day=all"
              className="mt-3 inline-block text-xs text-turf-300 underline decoration-turf-400/30 underline-offset-4"
            >
              Open all fixtures
            </Link>
          </div>
        )}

        {events.length > 0 && (
          <ul className="mt-8 grid gap-3 sm:grid-cols-2">
            {events.map((event) => (
              <LiveCard key={event.id} event={event} />
            ))}
          </ul>
        )}
      </main>
    </>
  );
}

function LiveCard({ event }: { event: EventItem }) {
  const when = kickoff(event);
  const isLive = event.status === "live" || event.live?.status === "live";
  const finished = event.status === "finished" || Boolean(event.result);
  const score = event.result
    ? `${event.result.home_score ?? "-"} - ${event.result.away_score ?? "-"}`
    : isLive && event.live
      ? `${event.live.home_score ?? "-"} - ${event.live.away_score ?? "-"}`
      : null;
  const statusLabel = isLive
    ? (event.live?.detail ?? "LIVE")
    : finished
      ? "FT"
      : "SCHEDULED";
  const footer =
    finished || isLive
      ? `${event.live?.provider ?? "provider"} · ${formatAge(
          event.live?.captured_at ?? event.result?.captured_at ?? null,
        )}`
      : `${when.date} · ${when.time}${
          when.zone ? ` ${when.zone}` : event.start_time_local ? " local" : ""
        }`;

  return (
    <li>
      <Link
        href={`/events/detail?id=${event.id}`}
        className="block rounded-xl border border-line bg-ink-900/50 p-5 transition hover:border-line-strong hover:bg-ink-900"
      >
        <div className="flex items-center justify-between gap-3">
          <p className="font-mono text-[10px] tracking-widest text-mist-500 uppercase">
            {event.competition_name}
          </p>
          <span
            className={`rounded-full border px-2 py-0.5 font-mono text-[10px] uppercase ${
              isLive
                ? "border-flag-400/30 bg-flag-400/10 text-flag-400"
                : "border-line-strong text-mist-400"
            }`}
          >
            {statusLabel}
          </span>
        </div>

        <div className="mt-4 flex items-center justify-between gap-4">
          <div className="min-w-0 space-y-1.5">
            <div className="flex items-center gap-2">
              <TeamCrest src={absoluteAsset(event.home.crest_url)} name={event.home.name} size={20} />
              <p className="truncate text-sm text-mist-100">{event.home.name}</p>
            </div>
            <div className="flex items-center gap-2">
              <TeamCrest src={absoluteAsset(event.away.crest_url)} name={event.away.name} size={20} />
              <p className="truncate text-sm text-mist-300">{event.away.name}</p>
            </div>
          </div>
          <p className="font-mono text-xl font-semibold">
            {score ?? when.time}
          </p>
        </div>

        <p
          className="mt-4 font-mono text-[10px] text-mist-500"
          title={finished || isLive ? undefined : when.title}
        >
          {footer}
        </p>
      </Link>
    </li>
  );
}
