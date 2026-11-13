"use client";

import { useCallback, useEffect, useRef, useState } from "react";

import {
  getEvents,
  getHomeSummary,
  type EventItem,
  type HomeSummary,
} from "@/lib/api";
import {
  addDays,
  localISODate,
  type HomeView,
} from "@/features/home/format";

function windowFor(view: HomeView, date: string) {
  const today = localISODate();
  const tomorrow = addDays(today, 1);
  switch (view) {
    case "live":
      return { status: "live", from: undefined, to: undefined };
    case "today":
      return { status: undefined, from: today, to: today };
    case "tomorrow":
      return { status: undefined, from: tomorrow, to: tomorrow };
    case "upcoming":
      return { status: undefined, from: tomorrow, to: addDays(today, 14) };
    case "date":
      return { status: undefined, from: date, to: date };
    default:
      return { status: undefined, from: today, to: today };
  }
}

export function useHomeEvents({
  sport,
  view,
  date,
}: {
  sport: string;
  view: HomeView;
  date: string;
}) {
  const [events, setEvents] = useState<EventItem[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [tick, setTick] = useState(0);
  const today = localISODate();
  const tomorrow = addDays(today, 1);
  const hasLive = events.some((event) => event.status === "live");
  const liveRef = useRef(false);
  liveRef.current = hasLive || view === "live";

  const load = useCallback(async () => {
    const window = windowFor(view, date);
    const result = await getEvents({
      sport,
      status: window.status,
      from: window.from,
      to: window.to,
      sort: "live_first",
      limit: 200,
    });
    if (result.ok) {
      setEvents(result.events);
      setError(null);
    } else {
      setEvents([]);
      setError(result.error);
    }
    setLoading(false);
  }, [sport, view, date]);

  useEffect(() => {
    setLoading(true);
    void load();
  }, [load, tick]);

  useEffect(() => {
    const timer = setInterval(() => {
      if (liveRef.current) setTick((value) => value + 1);
    }, 30000);
    return () => clearInterval(timer);
  }, []);

  return {
    events,
    loading,
    error,
    today,
    tomorrow,
    refresh: () => setTick((value) => value + 1),
  };
}

export function useHomeSummary(sport: string) {
  const [summary, setSummary] = useState<HomeSummary | null>(null);
  const today = localISODate();

  useEffect(() => {
    let active = true;
    void getHomeSummary(sport, today).then((value) => {
      if (active) setSummary(value);
    });
    return () => {
      active = false;
    };
  }, [sport, today]);

  return summary;
}

export function useHighlights(sport: string) {
  const [events, setEvents] = useState<EventItem[]>([]);
  const today = localISODate();

  useEffect(() => {
    let active = true;
    void getEvents({
      sport,
      from: addDays(today, 1),
      to: addDays(today, 7),
      sort: "kickoff_asc",
      limit: 6,
    }).then((result) => {
      if (active && result.ok) setEvents(result.events.slice(0, 4));
    });
    return () => {
      active = false;
    };
  }, [sport, today]);

  return events;
}
