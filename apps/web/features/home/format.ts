import type { EventItem } from "@/lib/api";

export type HomeView = "live" | "today" | "upcoming" | "tomorrow" | "date";

export type DisplayState =
  | "live"
  | "today-upcoming"
  | "finished"
  | "tomorrow"
  | "future"
  | "postponed"
  | "cancelled"
  | "past";

export function localISODate(date = new Date()): string {
  const year = date.getFullYear();
  const month = String(date.getMonth() + 1).padStart(2, "0");
  const day = String(date.getDate()).padStart(2, "0");
  return `${year}-${month}-${day}`;
}

export function addDays(iso: string, days: number): string {
  const date = new Date(`${iso}T12:00:00`);
  date.setDate(date.getDate() + days);
  return localISODate(date);
}

export function eventLocalDate(event: EventItem): string | null {
  if (event.start_time) {
    const parsed = new Date(event.start_time);
    if (!Number.isNaN(parsed.valueOf())) return localISODate(parsed);
  }
  return event.start_date;
}

export function eventTimeLabel(event: EventItem): string {
  if (event.start_time) {
    const parsed = new Date(event.start_time);
    if (!Number.isNaN(parsed.valueOf())) {
      return parsed.toLocaleTimeString([], {
        hour: "2-digit",
        minute: "2-digit",
      });
    }
  }
  return event.start_time_local ?? "TBC";
}

export function eventDayLabel(event: EventItem): string {
  const day = eventLocalDate(event);
  if (!day) return "date unknown";
  const parsed = new Date(`${day}T12:00:00`);
  return parsed.toLocaleDateString("en-GB", {
    weekday: "long",
    day: "numeric",
    month: "short",
  });
}

export function displayState(
  event: EventItem,
  today: string,
  tomorrow: string,
): DisplayState {
  if (event.status === "live") return "live";
  if (event.status === "finished") return "finished";
  if (event.status === "postponed") return "postponed";
  if (event.status === "cancelled" || event.status === "abandoned") {
    return "cancelled";
  }
  const day = eventLocalDate(event);
  if (!day) return "past";
  if (day === today) return "today-upcoming";
  if (day === tomorrow) return "tomorrow";
  if (day > today) return "future";
  return "past";
}

export function scoreLine(event: EventItem): string | null {
  const home = event.result?.home_score ?? event.live?.home_score;
  const away = event.result?.away_score ?? event.live?.away_score;
  if (home === null || home === undefined) return null;
  if (away === null || away === undefined) return null;
  return `${home} - ${away}`;
}

export function statusLabel(event: EventItem, state: DisplayState): string {
  if (state === "live") {
    return event.live?.detail?.trim() || "LIVE";
  }
  if (state === "finished") return "FT";
  if (state === "postponed") return "PP";
  if (state === "cancelled") return "CANC";
  return eventTimeLabel(event);
}

export function sortEvents(events: EventItem[], sort: string): EventItem[] {
  const copy = [...events];
  const kickoff = (event: EventItem) =>
    event.start_time
      ? new Date(event.start_time).valueOf()
      : new Date(`${event.start_date ?? "9999-12-31"}T${event.start_time_local ?? "23:59"}`).valueOf();

  switch (sort) {
    case "kickoff_desc":
      copy.sort((a, b) => kickoff(b) - kickoff(a));
      break;
    case "competition":
      copy.sort(
        (a, b) =>
          a.competition_name.localeCompare(b.competition_name) ||
          kickoff(a) - kickoff(b),
      );
      break;
    case "odds":
      copy.sort((a, b) => (b.odds?.home ?? 0) - (a.odds?.home ?? 0));
      break;
    case "kickoff_asc":
      copy.sort((a, b) => kickoff(a) - kickoff(b));
      break;
    default:
      copy.sort((a, b) => {
        const liveA = a.status === "live" ? 0 : 1;
        const liveB = b.status === "live" ? 0 : 1;
        return liveA - liveB || kickoff(a) - kickoff(b);
      });
  }
  return copy;
}

export function groupByCompetition(
  events: EventItem[],
): { competition: string; events: EventItem[] }[] {
  const groups = new Map<string, EventItem[]>();
  for (const event of events) {
    const key = event.competition_name;
    const list = groups.get(key) ?? [];
    list.push(event);
    groups.set(key, list);
  }
  return [...groups.entries()].map(([competition, list]) => ({
    competition,
    events: list,
  }));
}
