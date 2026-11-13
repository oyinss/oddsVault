"use client";

import { useEffect, useRef, useState } from "react";
import {
  faBell,
  faBellSlash,
  faCheck,
  faTrashCan,
  faXmark,
} from "@fortawesome/free-solid-svg-icons";
import { FontAwesomeIcon } from "@fortawesome/react-fontawesome";

import {
  ALERT_EVENT,
  clearAlerts,
  loadAlerts,
  markAllSeen,
  type CornerAlert,
} from "@/lib/corner-alerts";

/**
 * Bell and inbox for corner alerts.
 *
 * Alerts are produced by the corners board, which writes them to localStorage
 * and emits an event, so this stays in sync without a server.
 */
export function NotificationCenter({
  variant = "sidebar",
}: {
  variant?: "sidebar" | "mobile";
}) {
  const [alerts, setAlerts] = useState<CornerAlert[]>([]);
  const [open, setOpen] = useState(false);
  const panelRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    setAlerts(loadAlerts());

    const refresh = () => setAlerts(loadAlerts());
    const onStorage = (event: StorageEvent) => {
      if (event.key === "oddsvault-corner-alerts") refresh();
    };
    window.addEventListener(ALERT_EVENT, refresh);
    window.addEventListener("storage", onStorage);
    return () => {
      window.removeEventListener(ALERT_EVENT, refresh);
      window.removeEventListener("storage", onStorage);
    };
  }, []);

  useEffect(() => {
    if (!open) return;
    const onClick = (event: MouseEvent) => {
      if (!panelRef.current?.contains(event.target as Node)) setOpen(false);
    };
    const onKey = (event: KeyboardEvent) => {
      if (event.key === "Escape") setOpen(false);
    };
    document.addEventListener("mousedown", onClick);
    document.addEventListener("keydown", onKey);
    return () => {
      document.removeEventListener("mousedown", onClick);
      document.removeEventListener("keydown", onKey);
    };
  }, [open]);

  const unread = alerts.filter((alert) => !alert.seen).length;

  function toggle() {
    const next = !open;
    setOpen(next);
    if (next && unread > 0) {
      setAlerts(markAllSeen(alerts));
    }
  }

  const panelPosition =
    variant === "sidebar"
      ? "absolute bottom-12 left-3 z-50 w-72"
      : "absolute top-12 right-0 z-50 w-80";

  return (
    <div className="relative" ref={panelRef}>
      <button
        type="button"
        onClick={toggle}
        aria-label={`Corner alerts${unread > 0 ? `, ${unread} unread` : ""}`}
        aria-expanded={open}
        title="Corner alerts"
        className={`flex w-full items-center gap-3 rounded-lg px-3 py-2 text-sm font-semibold transition ${
          open
            ? "bg-turf-500/10 text-turf-300"
            : "text-mist-200 hover:bg-raised hover:text-mist-100"
        } ${variant === "mobile" ? "w-auto border border-line px-2.5" : ""}`}
      >
        <span className="relative">
          <FontAwesomeIcon icon={faBell} className="size-3.5 opacity-80" />
          {unread > 0 && (
            <span className="absolute -top-1.5 -right-1.5 grid min-w-3.5 place-items-center rounded-full bg-turf-400 px-1 font-mono text-[9px] leading-3.5 text-ink-950">
              {unread > 9 ? "9+" : unread}
            </span>
          )}
        </span>
        {variant === "sidebar" && <span>Alerts</span>}
      </button>

      {open && (
        <div
          className={`${panelPosition} overflow-hidden rounded-xl border border-line-strong bg-ink-900 shadow-2xl`}
        >
          <div className="flex items-center justify-between gap-2 border-b border-line px-3 py-2">
            <p className="font-mono text-[10px] tracking-widest text-mist-400 uppercase">
              Corner alerts
            </p>
            <div className="flex items-center gap-1">
              {alerts.length > 0 && (
                <button
                  type="button"
                  onClick={() => setAlerts(clearAlerts())}
                  title="Clear all"
                  className="grid size-6 place-items-center rounded-md text-mist-500 hover:bg-raised hover:text-mist-200"
                >
                  <FontAwesomeIcon icon={faTrashCan} className="size-2.5" />
                </button>
              )}
              <button
                type="button"
                onClick={() => setOpen(false)}
                title="Close"
                className="grid size-6 place-items-center rounded-md text-mist-500 hover:bg-raised hover:text-mist-200"
              >
                <FontAwesomeIcon icon={faXmark} className="size-2.5" />
              </button>
            </div>
          </div>

          {alerts.length === 0 ? (
            <div className="px-3 py-4">
              <p className="flex items-center gap-2 text-xs text-mist-300">
                <FontAwesomeIcon icon={faBellSlash} className="size-3 text-mist-500" />
                No alerts yet.
              </p>
              <p className="mt-1 text-[11px] leading-relaxed text-mist-500">
                Open the corners board and use the bell on a match. Alerts fire
                while that tab is open.
              </p>
            </div>
          ) : (
            <ul className="max-h-80 divide-y divide-white/[0.05] overflow-y-auto">
              {alerts.map((alert) => (
                <li key={alert.id} className="px-3 py-2">
                  <p className="flex items-start gap-2 text-xs text-mist-100">
                    {!alert.seen && (
                      <span className="mt-1 size-1.5 shrink-0 rounded-full bg-turf-400" />
                    )}
                    <span className="min-w-0 flex-1">
                      {alert.home} v {alert.away}
                    </span>
                    <span className="font-mono text-[10px] text-mist-500">
                      {relativeTime(alert.at)}
                    </span>
                  </p>
                  <p className="mt-0.5 font-mono text-[10px] text-mist-500">
                    {alert.corners} corners · alert at {alert.threshold}
                  </p>
                </li>
              ))}
            </ul>
          )}

          <div className="flex items-center gap-1.5 border-t border-line px-3 py-2 font-mono text-[9px] tracking-widest text-mist-500 uppercase">
            <FontAwesomeIcon icon={faCheck} className="size-2" />
            live while a board tab is open
          </div>
        </div>
      )}
    </div>
  );
}

function relativeTime(iso: string): string {
  const at = Date.parse(iso);
  if (Number.isNaN(at)) return "";
  const seconds = Math.max(0, Math.round((Date.now() - at) / 1000));
  if (seconds < 60) return `${seconds}s`;
  const minutes = Math.round(seconds / 60);
  if (minutes < 60) return `${minutes}m`;
  const hours = Math.round(minutes / 60);
  return `${hours}h`;
}
