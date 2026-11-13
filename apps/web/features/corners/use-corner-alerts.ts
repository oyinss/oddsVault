"use client";

import { useCallback, useEffect, useRef, useState } from "react";

import type { LiveCornerMatch } from "@/lib/api";

export type CornerWatch = {
  eventId: number;
  home: string;
  away: string;
  threshold: number;
  /** Set once the alert has fired, so a match never alerts twice. */
  fired: boolean;
};

const STORAGE_KEY = "oddsvault-corner-watches";

function loadWatches(): CornerWatch[] {
  try {
    const raw = window.localStorage.getItem(STORAGE_KEY);
    if (!raw) return [];
    const parsed = JSON.parse(raw) as CornerWatch[];
    if (!Array.isArray(parsed)) return [];
    return parsed.filter(
      (item) =>
        typeof item?.eventId === "number" &&
        typeof item?.threshold === "number" &&
        typeof item?.home === "string",
    );
  } catch {
    return [];
  }
}

function storeWatches(watches: CornerWatch[]) {
  try {
    window.localStorage.setItem(STORAGE_KEY, JSON.stringify(watches));
  } catch {
    // Storage can be unavailable in private mode; alerts still work this session.
  }
}

/**
 * Per-match corner alerts.
 *
 * The check runs against data this page already polls, so an alert fires while
 * the board is open. There is no push server: closing the tab stops alerts.
 */
export function useCornerAlerts(matches: LiveCornerMatch[], line: number) {
  const [watches, setWatches] = useState<CornerWatch[]>([]);
  const [permission, setPermission] = useState<
    NotificationPermission | "unsupported"
  >("default");
  const [notice, setNotice] = useState<string | null>(null);
  const watchesRef = useRef<CornerWatch[]>([]);
  watchesRef.current = watches;

  useEffect(() => {
    setWatches(loadWatches());
    if (typeof Notification === "undefined") {
      setPermission("unsupported");
    } else {
      setPermission(Notification.permission);
    }
  }, []);

  const persist = useCallback((next: CornerWatch[]) => {
    watchesRef.current = next;
    setWatches(next);
    storeWatches(next);
  }, []);

  const requestPermission = useCallback(async () => {
    if (typeof Notification === "undefined") {
      setPermission("unsupported");
      return false;
    }
    const result = await Notification.requestPermission();
    setPermission(result);
    if (result !== "granted") {
      setNotice(
        "Notifications are blocked by the browser. Alerts will show on this page only.",
      );
    }
    return result === "granted";
  }, []);

  const isWatched = useCallback(
    (eventId: number) => watches.some((watch) => watch.eventId === eventId),
    [watches],
  );

  const toggle = useCallback(
    async (match: LiveCornerMatch) => {
      const existing = watchesRef.current.find(
        (watch) => watch.eventId === match.event_id,
      );
      if (existing) {
        persist(
          watchesRef.current.filter(
            (watch) => watch.eventId !== match.event_id,
          ),
        );
        return;
      }

      // A match already past the line alerts on the next corner, not on a
      // threshold it has already cleared.
      const current = match.total_corners ?? 0;
      const threshold = Math.max(line, current + 1);

      if (typeof Notification !== "undefined" && Notification.permission === "default") {
        await requestPermission();
      }

      persist([
        ...watchesRef.current,
        {
          eventId: match.event_id,
          home: match.home.name,
          away: match.away.name,
          threshold,
          fired: false,
        },
      ]);
      setNotice(`Alert set for ${match.home.name} v ${match.away.name} at ${threshold} corners.`);
    },
    [line, persist, requestPermission],
  );

  // Compare watched matches with the latest poll.
  useEffect(() => {
    if (matches.length === 0) return;
    const hits: { watch: CornerWatch; match: LiveCornerMatch }[] = [];
    for (const watch of watchesRef.current) {
      if (watch.fired) continue;
      const match = matches.find((item) => item.event_id === watch.eventId);
      if (!match || match.total_corners === null) continue;
      if (match.total_corners >= watch.threshold) {
        hits.push({ watch, match });
      }
    }
    if (hits.length === 0) return;

    for (const { watch, match } of hits) {
      const body = `${watch.home} v ${watch.away}: ${match.total_corners} corners (alert at ${watch.threshold})`;
      if (
        permission === "granted" &&
        typeof Notification !== "undefined"
      ) {
        try {
          new Notification("Corner alert", {
            body,
            tag: `corner-${watch.eventId}`,
          });
        } catch {
          setNotice(body);
        }
      } else {
        setNotice(body);
      }
    }

    const fired = new Set(hits.map((hit) => hit.watch.eventId));
    persist(
      watchesRef.current.map((watch) =>
        fired.has(watch.eventId) ? { ...watch, fired: true } : watch,
      ),
    );
  }, [matches, permission, persist]);

  return {
    watches,
    permission,
    notice,
    isWatched,
    toggle,
    requestPermission,
    dismissNotice: () => setNotice(null),
  };
}
