"use client";

/** Alerts already delivered, kept for the notification centre. */
export type CornerAlert = {
  id: string;
  eventId: number;
  home: string;
  away: string;
  corners: number;
  threshold: number;
  at: string;
  seen: boolean;
};

const ALERTS_KEY = "oddsvault-corner-alerts";
const MAX_ALERTS = 50;

export const ALERT_EVENT = "oddsvault:corner-alert";

export function loadAlerts(): CornerAlert[] {
  try {
    const raw = window.localStorage.getItem(ALERTS_KEY);
    if (!raw) return [];
    const parsed = JSON.parse(raw) as CornerAlert[];
    if (!Array.isArray(parsed)) return [];
    return parsed.filter(
      (item) =>
        typeof item?.id === "string" &&
        typeof item?.eventId === "number" &&
        typeof item?.at === "string",
    );
  } catch {
    return [];
  }
}

export function saveAlerts(alerts: CornerAlert[]) {
  try {
    window.localStorage.setItem(
      ALERTS_KEY,
      JSON.stringify(alerts.slice(0, MAX_ALERTS)),
    );
  } catch {
    // Private mode can refuse storage; the in-memory list still works.
  }
}

export function pushAlert(alert: Omit<CornerAlert, "id" | "at" | "seen">) {
  const item: CornerAlert = {
    ...alert,
    id: `${alert.eventId}-${Date.now()}`,
    at: new Date().toISOString(),
    seen: false,
  };
  const next = [item, ...loadAlerts()].slice(0, MAX_ALERTS);
  saveAlerts(next);
  window.dispatchEvent(new CustomEvent(ALERT_EVENT, { detail: item }));
}

export function markAllSeen(alerts: CornerAlert[]): CornerAlert[] {
  const next = alerts.map((alert) => ({ ...alert, seen: true }));
  saveAlerts(next);
  return next;
}

export function clearAlerts(): CornerAlert[] {
  saveAlerts([]);
  return [];
}
