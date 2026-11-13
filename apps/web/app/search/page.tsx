"use client";

import Link from "next/link";
import { useRouter, useSearchParams } from "next/navigation";
import { Suspense, useEffect, useRef, useState } from "react";
import { faMagnifyingGlass } from "@fortawesome/free-solid-svg-icons";
import { FontAwesomeIcon } from "@fortawesome/react-fontawesome";

import { LoadingNote } from "@/components/loading-note";
import { SiteHeader } from "@/components/site-header";
import {
  absoluteAsset,
  searchEvents,
  type EventItem,
  type SearchResult,
} from "@/lib/api";
import { sportBySlug } from "@/features/home/sport-switcher";

const SUGGESTIONS = [
  "Premier League",
  "Nations League",
  "NBA",
  "NFL",
  "Arsenal",
  "Lakers",
];

export default function SearchPage() {
  return (
    <Suspense
      fallback={
        <>
          <SiteHeader />
          <main className="mx-auto max-w-4xl px-6 py-12">
            <LoadingNote label="Loading search…" />
          </main>
        </>
      }
    >
      <SearchView />
    </Suspense>
  );
}

function SearchView() {
  const router = useRouter();
  const params = useSearchParams();
  const initial = params.get("q") ?? "";
  const [input, setInput] = useState(initial);
  const [result, setResult] = useState<SearchResult | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const requestRef = useRef(0);

  // Keep the field in sync when the header link or a suggestion changes q.
  useEffect(() => {
    setInput(initial);
  }, [initial]);

  useEffect(() => {
    const query = input.trim();
    if (query.length < 2) {
      setResult(null);
      setError(null);
      setLoading(false);
      return;
    }

    const id = ++requestRef.current;
    setLoading(true);
    const timer = setTimeout(() => {
      searchEvents(query)
        .then((value) => {
          if (requestRef.current !== id) return;
          setResult(value);
          setError(null);
        })
        .catch((cause: unknown) => {
          if (requestRef.current !== id) return;
          setError(cause instanceof Error ? cause.message : "unreachable");
        })
        .finally(() => {
          if (requestRef.current !== id) return;
          setLoading(false);
          const next = `/search?q=${encodeURIComponent(query)}`;
          router.replace(next);
        });
    }, 250);

    return () => clearTimeout(timer);
  }, [input, router]);

  return (
    <>
      <SiteHeader />
      <main className="mx-auto max-w-4xl px-6 py-12">
        <p className="font-mono text-[11px] tracking-widest text-turf-300 uppercase">
          Search · All sports
        </p>
        <h1 className="mt-2 text-3xl font-bold tracking-tight">
          Find a fixture
        </h1>

        <div className="mt-5 flex items-center gap-2 rounded-xl border border-line bg-ink-900/70 px-4 py-3 focus-within:border-turf-400/40">
          <FontAwesomeIcon
            icon={faMagnifyingGlass}
            className="size-4 shrink-0 text-mist-500"
          />
          <input
            value={input}
            onChange={(event) => setInput(event.target.value)}
            autoFocus
            placeholder="Team, player or competition"
            aria-label="Search fixtures"
            className="w-full bg-transparent text-sm text-mist-100 outline-none placeholder:text-mist-500"
          />
          {loading && (
            <span className="font-mono text-[10px] tracking-widest text-mist-500 uppercase">
              searching
            </span>
          )}
        </div>

        {input.trim().length < 2 ? (
          <div className="mt-6">
            <p className="text-sm text-mist-400">
              Type at least two characters. Try one of these:
            </p>
            <div className="mt-3 flex flex-wrap gap-2">
              {SUGGESTIONS.map((suggestion) => (
                <button
                  key={suggestion}
                  type="button"
                  onClick={() => setInput(suggestion)}
                  className="rounded-lg border border-line px-3 py-1.5 text-xs text-mist-300 transition hover:border-white/20 hover:bg-white/[0.04] hover:text-mist-100"
                >
                  {suggestion}
                </button>
              ))}
            </div>
          </div>
        ) : error ? (
          <div className="mt-6 rounded-2xl border border-flag-400/30 bg-flag-400/[0.04] p-6">
            <p className="text-sm font-medium text-flag-400">
              Search failed.
            </p>
            <p className="mt-1 text-xs text-mist-400">{error}</p>
          </div>
        ) : loading && !result ? (
          <LoadingNote label="Searching fixtures…" />
        ) : result && result.count === 0 ? (
          <div className="mt-6 rounded-2xl border border-line bg-ink-900/60 p-6">
            <p className="text-sm font-medium text-mist-100">
              No fixtures match “{result.query}”.
            </p>
            <p className="mt-1 text-xs leading-relaxed text-mist-400">
              The search covers stored fixtures from the last week forward.
              A competition outside current coverage returns nothing rather
              than a guess.
            </p>
          </div>
        ) : result ? (
          <>
            <p className="mt-6 font-mono text-[11px] text-mist-500">
              {result.count} {result.count === 1 ? "fixture" : "fixtures"} for “
              {result.query}”
            </p>
            <ul className="mt-3 divide-y divide-white/[0.06] overflow-hidden rounded-2xl border border-line bg-ink-900/40">
              {result.events.map((event) => (
                <ResultRow key={event.id} event={event} />
              ))}
            </ul>
          </>
        ) : null}
      </main>
    </>
  );
}

function ResultRow({ event }: { event: EventItem }) {
  const sport = sportBySlug(event.sport);
  const score =
    event.result?.home_score != null && event.result?.away_score != null
      ? `${event.result.home_score} - ${event.result.away_score}`
      : event.live?.home_score != null && event.live?.away_score != null
        ? `${event.live.home_score} - ${event.live.away_score}`
        : null;

  return (
    <li>
      <Link
        href={`/events/detail?id=${event.id}`}
        className="flex items-center gap-3 px-4 py-3 transition hover:bg-white/[0.03]"
      >
        <span className="grid size-8 shrink-0 place-items-center rounded-lg bg-white/[0.05] text-mist-400">
          <FontAwesomeIcon icon={sport.icon} className="size-3.5" />
        </span>

        <span className="min-w-0 flex-1">
          <span className="block truncate text-sm font-semibold text-mist-100">
            {event.home.name} v {event.away.name}
          </span>
          <span className="mt-0.5 flex flex-wrap items-center gap-1.5 font-mono text-[10px] text-mist-500">
            <span className="rounded bg-white/[0.05] px-1.5 py-0.5">
              {event.competition_name}
            </span>
            <span>{kickoffLabel(event)}</span>
          </span>
        </span>

        {event.status === "live" ? (
          <span className="shrink-0 rounded bg-red-500/15 px-2 py-0.5 font-mono text-[10px] tracking-widest text-red-300 uppercase">
            Live
          </span>
        ) : score ? (
          <span className="shrink-0 font-mono text-xs tabular-nums text-mist-300">
            {score}
          </span>
        ) : null}
      </Link>
    </li>
  );
}

function kickoffLabel(event: EventItem): string {
  if (event.start_time) {
    const at = new Date(event.start_time);
    if (!Number.isNaN(at.valueOf())) {
      return at.toLocaleString("en-GB", {
        weekday: "short",
        day: "numeric",
        month: "short",
        hour: "2-digit",
        minute: "2-digit",
      });
    }
  }
  return `${event.start_date}${event.start_time_local ? ` ${event.start_time_local}` : ""}`;
}
