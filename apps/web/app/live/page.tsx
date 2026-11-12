"use client";

import Link from "next/link";

import { LoadingNote } from "@/components/loading-note";
import { RefreshLabel } from "@/components/refresh-label";
import { SiteHeader } from "@/components/site-header";
import { TeamCrest } from "@/components/team-crest";
import {
  absoluteAsset,
  formatAge,
  formatDate,
  getLive,
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
            <h1 className="mt-2 text-3xl font-semibold tracking-tight">
              Live &amp; today
            </h1>
            <p className="mt-2 max-w-xl text-sm leading-relaxed text-mist-400">
              Provider status and scores, polled by the worker and cached. The
              browser talks only to our API.
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
              The live job syncs yesterday, today and tomorrow. Run{" "}
              <code className="font-mono text-mist-200">make sync</code> then{" "}
              <code className="font-mono text-mist-200">
                JOB_RUN_ON_START=true make workers
              </code>{" "}
              to populate it.
            </p>
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
  const score = event.result
    ? `${event.result.home_score ?? "—"} – ${event.result.away_score ?? "—"}`
    : event.live
      ? `${event.live.home_score ?? "—"} – ${event.live.away_score ?? "—"}`
      : null;

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
              event.status === "live"
                ? "border-flag-400/30 bg-flag-400/10 text-flag-400"
                : "border-line-strong text-mist-400"
            }`}
          >
            {event.live?.detail ?? event.status}
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
            {score ?? formatDate(event.start_date)}
          </p>
        </div>

        <p className="mt-4 font-mono text-[10px] text-mist-500">
          {event.live
            ? `${event.live.provider} · ${formatAge(event.live.captured_at)}`
            : `${formatDate(event.start_date)} · ${event.start_time_local ?? "time unknown"}`}
        </p>
      </Link>
    </li>
  );
}
