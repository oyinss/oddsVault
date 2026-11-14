"use client";

import { useCallback, useEffect, useMemo, useState } from "react";

import type { LiveCornerMatch } from "@/lib/api";

const STORAGE_KEY = "oddsvault.corners.following.v1";

export type FollowedMatch = {
  eventId: number;
  home: string;
  away: string;
  competition: string;
  lastTotal: number | null;
  lastMinute: number | null;
  seenAt: string;
};

type FollowedMap = Record<string, FollowedMatch>;

function readStored(): FollowedMap {
  if (typeof window === "undefined") return {};
  try {
    const raw = window.localStorage.getItem(STORAGE_KEY);
    if (!raw) return {};
    const parsed = JSON.parse(raw) as unknown;
    if (!parsed || typeof parsed !== "object") return {};
    const out: FollowedMap = {};
    for (const [key, value] of Object.entries(parsed as Record<string, unknown>)) {
      const item = value as Partial<FollowedMatch> | null;
      if (!item || typeof item !== "object") continue;
      const eventId = Number(item.eventId ?? key);
      if (!Number.isFinite(eventId) || eventId <= 0) continue;
      if (typeof item.home !== "string" || typeof item.away !== "string") continue;
      out[String(eventId)] = {
        eventId,
        home: item.home,
        away: item.away,
        competition: typeof item.competition === "string" ? item.competition : "",
        lastTotal: typeof item.lastTotal === "number" ? item.lastTotal : null,
        lastMinute: typeof item.lastMinute === "number" ? item.lastMinute : null,
        seenAt: typeof item.seenAt === "string" ? item.seenAt : new Date().toISOString(),
      };
    }
    return out;
  } catch {
    return {};
  }
}

function summary(match: LiveCornerMatch, previous?: FollowedMatch): FollowedMatch {
  return {
    eventId: match.event_id,
    home: match.home.name,
    away: match.away.name,
    competition: match.competition,
    lastTotal: match.total_corners ?? previous?.lastTotal ?? null,
    lastMinute: match.minute ?? previous?.lastMinute ?? null,
    seenAt: new Date().toISOString(),
  };
}

// useCornerFavorites keeps a local list of matches the user wants to track. The
// board reorders and drops matches as they finish, so a followed match keeps its
// last seen count and minute even when it is no longer on the live list.
export function useCornerFavorites(matches: LiveCornerMatch[]) {
  const [followed, setFollowed] = useState<FollowedMap>({});
  const [ready, setReady] = useState(false);

  useEffect(() => {
    setFollowed(readStored());
    setReady(true);
  }, []);

  useEffect(() => {
    if (!ready) return;
    try {
      window.localStorage.setItem(STORAGE_KEY, JSON.stringify(followed));
    } catch {
      // A full or blocked storage must not break the board.
    }
  }, [followed, ready]);

  // Refresh the remembered snapshot while the match is still on the board.
  useEffect(() => {
    if (!matches.length) return;
    setFollowed((previous) => {
      let changed = false;
      const next = { ...previous };
      for (const match of matches) {
        const existing = next[String(match.event_id)];
        if (!existing) continue;
        if (
          existing.lastTotal !== match.total_corners ||
          existing.lastMinute !== match.minute
        ) {
          next[String(match.event_id)] = summary(match, existing);
          changed = true;
        }
      }
      return changed ? next : previous;
    });
  }, [matches]);

  const toggle = useCallback((match: LiveCornerMatch) => {
    setFollowed((previous) => {
      const key = String(match.event_id);
      if (previous[key]) {
        const next = { ...previous };
        delete next[key];
        return next;
      }
      return { ...previous, [key]: summary(match) };
    });
  }, []);

  const unfollow = useCallback((eventId: number) => {
    setFollowed((previous) => {
      const key = String(eventId);
      if (!previous[key]) return previous;
      const next = { ...previous };
      delete next[key];
      return next;
    });
  }, []);

  const list = useMemo(
    () =>
      Object.values(followed).sort((a, b) => b.seenAt.localeCompare(a.seenAt)),
    [followed],
  );

  const isFollowed = useCallback(
    (eventId: number) => Boolean(followed[String(eventId)]),
    [followed],
  );

  return { ready, followed, list, count: list.length, isFollowed, toggle, unfollow };
}
