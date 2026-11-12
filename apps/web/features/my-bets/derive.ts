import type { BetLeg, PersonalBet } from "@/lib/api";

/**
 * Derived bet-journal figures.
 *
 * Every number here comes from rows the API already returned; nothing is
 * estimated. Rates only ever divide by settled slips and callers must render
 * the sample size next to them. Profit is never split across accumulator legs:
 * a slip's result cannot be attributed to one selection.
 */

const LAGOS_TZ = "Africa/Lagos";
const dayFormat = new Intl.DateTimeFormat("en-CA", {
  timeZone: LAGOS_TZ,
  year: "numeric",
  month: "2-digit",
  day: "2-digit",
});

/** Calendar day a bet was placed, in the journal's timezone (Africa/Lagos). */
export function lagosDay(iso: string | null | undefined): string | null {
  if (!iso) return null;
  const ms = Date.parse(iso);
  if (Number.isNaN(ms)) return null;
  return dayFormat.format(new Date(ms));
}

export function todayLagos(): string {
  return dayFormat.format(new Date());
}

export function shiftDay(day: string, delta: number): string {
  const date = new Date(`${day}T12:00:00Z`);
  date.setUTCDate(date.getUTCDate() + delta);
  return date.toISOString().slice(0, 10);
}

export function formatDay(day: string | null): string {
  if (!day) return "unknown date";
  const date = new Date(`${day}T12:00:00Z`);
  if (Number.isNaN(date.valueOf())) return day;
  return date.toLocaleDateString("en-GB", {
    day: "numeric",
    month: "short",
    year: "numeric",
    timeZone: "UTC",
  });
}

export function formatDayShort(day: string | null): string {
  if (!day) return "unknown";
  const date = new Date(`${day}T12:00:00Z`);
  if (Number.isNaN(date.valueOf())) return day;
  return date.toLocaleDateString("en-GB", {
    day: "numeric",
    month: "short",
    timeZone: "UTC",
  });
}

/* ------------------------------------------------------------------ filters */

export type RangeKey = "all" | "7d" | "30d" | "month";
export type StatusKey = "all" | "won" | "lost" | "pending";
export type TypeKey = "all" | "accumulator" | "single";
export type SortKey = "date" | "stake" | "profit" | "odds";

export type Filters = {
  range: RangeKey;
  bookmaker: string;
  competition: string;
  status: StatusKey;
  betType: TypeKey;
  query: string;
  sort: SortKey;
};

export const DEFAULT_FILTERS: Filters = {
  range: "all",
  bookmaker: "",
  competition: "",
  status: "all",
  betType: "all",
  query: "",
  sort: "date",
};

export const RANGE_LABELS: Record<RangeKey, string> = {
  all: "All time",
  "7d": "Last 7 days",
  "30d": "Last 30 days",
  month: "This month",
};

function inRange(day: string | null, range: RangeKey, today: string): boolean {
  if (range === "all") return true;
  if (!day) return false;
  if (range === "7d") return day >= shiftDay(today, -6);
  if (range === "30d") return day >= shiftDay(today, -29);
  return day.slice(0, 7) === today.slice(0, 7);
}

function competitionOf(bet: PersonalBet): string | null {
  for (const leg of bet.legs) {
    if (leg.competition) return leg.competition;
  }
  return null;
}

function matchesQuery(bet: PersonalBet, query: string): boolean {
  if (!query) return true;
  const needle = query.trim().toLowerCase();
  if (!needle) return true;
  const haystack = [
    bet.external_id,
    bet.bookmaker,
    bet.bet_type ?? "",
    ...bet.legs.flatMap((leg) => [
      leg.event_raw,
      leg.market_raw ?? "",
      leg.selection_raw ?? "",
      leg.competition ?? "",
    ]),
  ]
    .join(" ")
    .toLowerCase();
  return haystack.includes(needle);
}

/** Bets matching the current filters, newest first unless sorted otherwise. */
export function filterBets(
  bets: PersonalBet[],
  filters: Filters,
  today: string = todayLagos(),
): PersonalBet[] {
  const filtered = bets.filter((bet) => {
    if (filters.status !== "all" && bet.status !== filters.status) return false;
    if (filters.bookmaker && bet.bookmaker !== filters.bookmaker) return false;
    if (filters.betType !== "all") {
      const type = (bet.bet_type ?? "").toLowerCase();
      if (type !== filters.betType) return false;
    }
    if (filters.competition && competitionOf(bet) !== filters.competition) {
      return false;
    }
    if (!inRange(lagosDay(bet.placed_at), filters.range, today)) return false;
    if (!matchesQuery(bet, filters.query)) return false;
    return true;
  });

  const placedMs = (bet: PersonalBet) =>
    bet.placed_at ? Date.parse(bet.placed_at) || 0 : 0;

  return filtered.sort((a, b) => {
    switch (filters.sort) {
      case "stake":
        return (b.stake ?? 0) - (a.stake ?? 0);
      case "profit":
        return (b.profit_loss ?? 0) - (a.profit_loss ?? 0);
      case "odds":
        return (b.combined_odds ?? 0) - (a.combined_odds ?? 0);
      default:
        return placedMs(b) - placedMs(a);
    }
  });
}

export function bookmakers(bets: PersonalBet[]): string[] {
  return [...new Set(bets.map((bet) => bet.bookmaker))].sort();
}

export function competitions(bets: PersonalBet[], limit = 24): string[] {
  const counts = new Map<string, number>();
  for (const bet of bets) {
    for (const leg of bet.legs) {
      if (!leg.competition) continue;
      counts.set(leg.competition, (counts.get(leg.competition) ?? 0) + 1);
    }
  }
  return [...counts.entries()]
    .sort((a, b) => b[1] - a[1] || a[0].localeCompare(b[0]))
    .slice(0, limit)
    .map(([name]) => name)
    .sort((a, b) => a.localeCompare(b));
}

/* ------------------------------------------------------------------ totals */

export type Totals = {
  bets: number;
  staked: number;
  returned: number;
  profit: number;
  roi: number | null;
  won: number;
  lost: number;
  pending: number;
  settled: number;
  /** Won / settled, as a fraction. `null` when nothing settled. */
  winRate: number | null;
  currency: string;
};

export function totals(bets: PersonalBet[]): Totals {
  let staked = 0;
  let returned = 0;
  let profit = 0;
  let won = 0;
  let lost = 0;
  let pending = 0;

  for (const bet of bets) {
    staked += bet.stake ?? 0;
    returned += bet.actual_return ?? 0;
    profit += bet.profit_loss ?? 0;
    if (bet.status === "won") won += 1;
    else if (bet.status === "lost") lost += 1;
    else pending += 1;
  }

  const settled = won + lost;
  return {
    bets: bets.length,
    staked,
    returned,
    profit,
    roi: staked > 0 ? (profit / staked) * 100 : null,
    won,
    lost,
    pending,
    settled,
    winRate: settled > 0 ? won / settled : null,
    currency: bets[0]?.currency ?? "NGN",
  };
}

/* ------------------------------------------------------------- performance */

export type DayPerformance = {
  day: string;
  isToday: boolean;
  bets: number;
  won: number;
  lost: number;
  pending: number;
  settled: number;
  winRate: number | null;
  profit: number;
  currency: string;
};

/**
 * Today's results, or the most recent day that actually had bets. The card
 * says which one it is showing; it never presents an older day as today.
 */
export function dayPerformance(
  bets: PersonalBet[],
  today: string = todayLagos(),
): DayPerformance | null {
  if (bets.length === 0) return null;

  const days = bets
    .map((bet) => lagosDay(bet.placed_at))
    .filter((day): day is string => day !== null)
    .sort();

  const chosen = days.includes(today) ? today : days[days.length - 1];
  if (!chosen) return null;

  const todays = bets.filter((bet) => lagosDay(bet.placed_at) === chosen);
  const t = totals(todays);

  return {
    day: chosen,
    isToday: chosen === today,
    bets: t.bets,
    won: t.won,
    lost: t.lost,
    pending: t.pending,
    settled: t.settled,
    winRate: t.winRate,
    profit: t.profit,
    currency: t.currency,
  };
}

export type DayPoint = {
  day: string;
  bets: number;
  won: number;
  lost: number;
  pending: number;
  settled: number;
  /** Profit/loss on that day alone. */
  profit: number;
  /** Profit/loss from the first day through this one. */
  cumulativeProfit: number;
  /** Won / settled for this day, as a fraction. */
  winRate: number | null;
  /** Won / settled across all days up to and including this one. */
  cumulativeWinRate: number | null;
};

export function dailySeries(bets: PersonalBet[]): DayPoint[] {
  const byDay = new Map<string, PersonalBet[]>();
  for (const bet of bets) {
    const day = lagosDay(bet.placed_at);
    if (!day) continue;
    const bucket = byDay.get(day) ?? [];
    bucket.push(bet);
    byDay.set(day, bucket);
  }

  const days = [...byDay.keys()].sort();
  let cumulativeProfit = 0;
  let cumulativeWon = 0;
  let cumulativeSettled = 0;

  return days.map((day) => {
    const t = totals(byDay.get(day) ?? []);
    cumulativeProfit += t.profit;
    cumulativeWon += t.won;
    cumulativeSettled += t.settled;
    return {
      day,
      bets: t.bets,
      won: t.won,
      lost: t.lost,
      pending: t.pending,
      settled: t.settled,
      profit: t.profit,
      cumulativeProfit,
      winRate: t.winRate,
      cumulativeWinRate:
        cumulativeSettled > 0 ? cumulativeWon / cumulativeSettled : null,
    };
  });
}

/** Daily profit/loss for the most recent `count` days that had bets. */
export function recentDailyProfit(bets: PersonalBet[], count = 7): number[] {
  return dailySeries(bets)
    .slice(-count)
    .map((point) => point.profit);
}

/* ----------------------------------------------------------------- markets */

const FAMILY_LABELS: Record<string, string> = {
  corners_total: "Corners total",
  corners_handicap: "Corners handicap",
  total_goals: "Total goals",
  both_teams_to_score: "Both teams to score",
  correct_score: "Correct score",
  match_result: "Match result",
  handicap: "Handicap",
};

/** Human label for a leg's market, or null when the family is not verified. */
export function marketLabel(leg: BetLeg): string | null {
  if (!leg.market_family || leg.market_family === "unknown") {
    return null;
  }
  const family = FAMILY_LABELS[leg.market_family] ?? leg.market_family;
  const side = leg.market_side
    ? leg.market_side.charAt(0).toUpperCase() + leg.market_side.slice(1)
    : "";
  const line = leg.market_line !== null ? ` ${leg.market_line}` : "";
  return side ? `${family} · ${side}${line}` : `${family}${line}`;
}

export type MarketRow = {
  key: string;
  label: string;
  legs: number;
  won: number;
  lost: number;
  settled: number;
  winRate: number | null;
  /** Profit from single-leg slips only; null when none exist for this market. */
  singleProfit: number | null;
  singleBets: number;
  lastPlayed: string | null;
  currency: string;
};

export function marketRows(bets: PersonalBet[]): MarketRow[] {
  const rows = new Map<string, MarketRow>();

  for (const bet of bets) {
    const day = lagosDay(bet.placed_at);
    const single = bet.legs.length === 1;

    for (const leg of bet.legs) {
      const label = marketLabel(leg);
      if (!label) continue;

      const row =
        rows.get(label) ??
        ({
          key: label,
          label,
          legs: 0,
          won: 0,
          lost: 0,
          settled: 0,
          winRate: null,
          singleProfit: null,
          singleBets: 0,
          lastPlayed: null,
          currency: bet.currency,
        } satisfies MarketRow);

      row.legs += 1;
      if (leg.leg_status === "won") row.won += 1;
      if (leg.leg_status === "lost") row.lost += 1;

      if (single) {
        row.singleBets += 1;
        row.singleProfit = (row.singleProfit ?? 0) + (bet.profit_loss ?? 0);
      }

      if (day && (!row.lastPlayed || day > row.lastPlayed)) {
        row.lastPlayed = day;
      }

      rows.set(label, row);
    }
  }

  return [...rows.values()]
    .map((row) => {
      const settled = row.won + row.lost;
      return {
        ...row,
        settled,
        winRate: settled > 0 ? row.won / settled : null,
      };
    })
    .sort((a, b) => b.legs - a.legs || a.label.localeCompare(b.label));
}

/* -------------------------------------------------------------------- legs */

export function legNames(leg: BetLeg): { home: string; away: string } {
  const split = leg.event_raw.split(" - ");
  if (split.length !== 2) return { home: leg.event_raw, away: "" };
  const clean = (part: string) => part.replace(/^corners\s+/i, "").trim();
  return { home: clean(split[0]), away: clean(split[1]) };
}

export function placedLabel(iso: string | null): string {
  if (!iso) return "date unknown";
  const ms = Date.parse(iso);
  if (Number.isNaN(ms)) return "date unknown";
  return new Date(ms).toLocaleString("en-GB", {
    day: "numeric",
    month: "short",
    year: "numeric",
    hour: "2-digit",
    minute: "2-digit",
    timeZone: LAGOS_TZ,
  });
}
