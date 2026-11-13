/**
 * Prefix the browser uses for every API read.
 *
 * The static export is served by Cloudflare Pages, whose advanced-mode worker
 * (see `public/_worker.js`) proxies `/svc/*` to the Go API. Keeping the prefix
 * same-origin means the upstream host can move without rebuilding the site and
 * no CORS exceptions are needed. During `next dev`, `next.config.ts` has no
 * proxy, so set NEXT_PUBLIC_API_BASE_URL=http://localhost:8080 in `.env.local`.
 */
export const API_BASE_URL =
  process.env.NEXT_PUBLIC_API_BASE_URL ?? "/svc";

export type ServiceHealth =
  | { reachable: true; version: string; uptimeSeconds: number }
  | { reachable: false; error: string };

export type Competition = {
  slug: string;
  name: string;
  country: string | null;
  current_season: string | null;
};

export type EventTeam = {
  id: number;
  name: string;
  slug: string;
  crest_url: string | null;
};

export type EventResult = {
  home_score: number | null;
  away_score: number | null;
  home_score_ht: number | null;
  away_score_ht: number | null;
  provider: string;
  captured_at: string | null;
};

export type LiveView = {
  status: string;
  home_score: number | null;
  away_score: number | null;
  detail: string | null;
  provider: string;
  captured_at: string;
};

export type EventOdds = {
  home: number | null;
  draw: number | null;
  away: number | null;
};

export type EventItem = {
  id: number;
  sport: string;
  sport_name: string;
  competition: string;
  competition_name: string;
  season: string | null;
  start_date: string | null;
  start_time: string | null;
  start_time_local: string | null;
  status: string;
  round: string | null;
  identity_status: string;
  home: EventTeam;
  away: EventTeam;
  result: EventResult | null;
  live: LiveView | null;
  odds: EventOdds | null;
  updated_at: string;
};

export type EventList =
  | { ok: true; events: EventItem[] }
  | { ok: false; error: string };

const REQUEST_TIMEOUT_MS = 4000;

async function getJSON<T>(path: string): Promise<T> {
  const res = await fetch(`${API_BASE_URL}${path}`, {
    cache: "no-store",
    signal: AbortSignal.timeout(REQUEST_TIMEOUT_MS),
  });
  if (!res.ok) {
    throw new Error(`HTTP ${res.status}`);
  }
  return (await res.json()) as T;
}

/**
 * Reads the Go API health endpoint from the server. A down API is a normal
 * state, not an exception: the foundation page reports it instead of failing.
 */
export async function getApiHealth(): Promise<ServiceHealth> {
  try {
    const body = await getJSON<{ version?: string; uptime_sec?: number }>(
      "/healthz",
    );
    return {
      reachable: true,
      version: body.version ?? "dev",
      uptimeSeconds: body.uptime_sec ?? 0,
    };
  } catch (error) {
    return {
      reachable: false,
      error: error instanceof Error ? error.message : "unreachable",
    };
  }
}

export async function getCompetitions(): Promise<Competition[]> {
  try {
    const body = await getJSON<{ competitions: Competition[] }>(
      "/api/v1/competitions",
    );
    return body.competitions;
  } catch {
    return [];
  }
}

export async function getEvents(options: {
  sport?: string;
  status?: string;
  competition?: string;
  from?: string;
  to?: string;
  sort?: string;
  limit?: number;
} = {}): Promise<EventList> {
  const params = new URLSearchParams();
  params.set("limit", String(options.limit ?? 60));
  if (options.sport) params.set("sport", options.sport);
  if (options.status) params.set("status", options.status);
  if (options.competition) params.set("competition", options.competition);
  if (options.from) params.set("from", options.from);
  if (options.to) params.set("to", options.to);
  if (options.sort) params.set("sort", options.sort);

  try {
    const body = await getJSON<{ events: EventItem[] }>(
      `/api/v1/events?${params.toString()}`,
    );
    return { ok: true, events: body.events };
  } catch (error) {
    return {
      ok: false,
      error: error instanceof Error ? error.message : "unreachable",
    };
  }
}

export type SelectionView = {
  id: number;
  selection: string;
  decimal_odds: string | null;
  bookmaker: string | null;
  price_kind: string | null;
  odds_captured_at: string | null;
  settlement_status: string | null;
  settlement_reason: string | null;
  settlement_actual: string | null;
};

export type MarketView = {
  id: number;
  type: string;
  period: string;
  line: string | null;
  selections: SelectionView[];
};

export type ProviderLink = {
  provider: string;
  provider_event_id: string;
  match_method: string | null;
  linked_at: string;
};

export type EventDetail = {
  event: EventItem;
  provider_links: ProviderLink[];
  markets: MarketView[];
};

export async function getEvent(id: string): Promise<EventDetail | null> {
  try {
    return await getJSON<EventDetail>(`/api/v1/events/${id}`);
  } catch {
    return null;
  }
}

export async function getLive(): Promise<EventList> {
  try {
    const body = await getJSON<{ events: EventItem[] }>("/api/v1/live");
    return { ok: true, events: body.events };
  } catch (error) {
    return {
      ok: false,
      error: error instanceof Error ? error.message : "unreachable",
    };
  }
}

export type BetTeam = {
  id: number;
  name: string;
  slug: string;
  crest_url: string | null;
};

export type BetLeg = {
  leg_number: number;
  competition: string | null;
  event_raw: string;
  market_raw: string | null;
  selection_raw: string | null;
  market_family: string | null;
  market_side: string | null;
  market_line: number | null;
  odds_taken: number | null;
  leg_status: string;
  identity_status: string;
  resolution_note: string | null;
  event_id: number | null;
  home_team: BetTeam | null;
  away_team: BetTeam | null;
};

export type PersonalBet = {
  id: number;
  external_id: string;
  bookmaker: string;
  currency: string;
  placed_at: string | null;
  placed_at_raw: string | null;
  bet_type: string | null;
  stake: number | null;
  combined_odds: number | null;
  potential_return: number | null;
  status: string;
  actual_return: number | null;
  profit_loss: number | null;
  source: string;
  legs: BetLeg[];
};

export type BetSummary = {
  bets: number;
  staked: number;
  returned: number;
  profit_loss: number;
  won: number;
  lost: number;
  pending: number;
};

export async function getMyBets(): Promise<
  { ok: true; summary: BetSummary; bets: PersonalBet[] } | { ok: false; error: string }
> {
  try {
    const body = await getJSON<{ summary: BetSummary; bets: PersonalBet[] }>(
      "/api/v1/my-bets",
    );
    return { ok: true, summary: body.summary, bets: body.bets };
  } catch (error) {
    return {
      ok: false,
      error: error instanceof Error ? error.message : "unreachable",
    };
  }
}

export type MovementPoint = { at: string; odds: number };

export type MovementSelection = {
  selection: string;
  opening: number;
  current: number;
  high: number;
  low: number;
  samples: number;
  first_at: string;
  last_at: string;
  series: MovementPoint[];
};

export type MovementMarket = {
  type: string;
  line: number | null;
  selections: MovementSelection[];
};

export async function getEventMovement(
  id: string,
): Promise<MovementMarket[]> {
  try {
    const body = await getJSON<{ markets: MovementMarket[] }>(
      `/api/v1/events/${id}/movement`,
    );
    return body.markets;
  } catch {
    return [];
  }
}

export type BookmakerRef = { slug: string; name: string };

export type BookmakerPrice = {
  bookmaker: string;
  bookmaker_name: string;
  odds: number;
  price_kind: string;
  captured_at: string;
};

export type PricedSelection = { selection: string; prices: BookmakerPrice[] };

export type PricedMarket = {
  type: string;
  line: number | null;
  selections: PricedSelection[];
};

export type EventPrices = {
  bookmakers: BookmakerRef[];
  markets: PricedMarket[];
};

export async function getEventPrices(id: string): Promise<EventPrices | null> {
  try {
    return await getJSON<EventPrices>(`/api/v1/events/${id}/prices`);
  } catch {
    return null;
  }
}

export type SnapshotRow = {
  captured_at: string;
  decimal_odds: number;
  price_kind: string;
  bookmaker: string;
  bookmaker_name: string;
  event_id: number;
  competition: string;
  competition_name: string;
  home: string;
  away: string;
  start_date: string | null;
  market: string;
  line: number | null;
  selection: string;
  settlement_status: string | null;
};

export async function getSnapshots(params: {
  kind?: string;
  market?: string;
  bookmaker?: string;
  limit?: number;
}): Promise<SnapshotRow[]> {
  const query = new URLSearchParams();
  if (params.kind) query.set("kind", params.kind);
  if (params.market) query.set("market", params.market);
  if (params.bookmaker) query.set("bookmaker", params.bookmaker);
  query.set("limit", String(params.limit ?? 120));
  try {
    const body = await getJSON<{ snapshots: SnapshotRow[] }>(
      `/api/v1/snapshots?${query.toString()}`,
    );
    return body.snapshots;
  } catch {
    return [];
  }
}

export type BetInsights = {
  min_sample: number;
  overall: {
    bets: number;
    staked: number;
    returned: number;
    profit_loss: number;
    roi: number;
    settled_legs: number;
    verified_legs: number;
    disagreements: number;
  };
  lines: Array<{
    family: string;
    side: string;
    line: number | null;
    legs: number;
    won: number;
    lost: number;
    rate: number;
    verified: number;
    disagreements: number;
  }>;
  competitions: Array<{
    label: string;
    legs: number;
    won: number;
    lost: number;
    rate: number;
    verified: number;
  }>;
  odds_buckets: Array<{
    low: number;
    high: number | null;
    legs: number;
    won: number;
    lost: number;
    rate: number;
  }>;
  stakes: Array<{
    label: string;
    bets: number;
    staked: number;
    returned: number;
    profit_loss: number;
  }>;
  disagreements: Array<{
    leg_id: number;
    event_raw: string;
    side: string;
    line: number | null;
    provider_status: string;
    verified_status: string;
    note: string;
    event_id: number | null;
  }>;
};

export async function getBetInsights(
  minSample: number,
): Promise<BetInsights | null> {
  try {
    return await getJSON<BetInsights>(
      `/api/v1/my-bets/insights?min_sample=${minSample}`,
    );
  } catch {
    return null;
  }
}

export type HomeSummary = {
  sport: string;
  date: string;
  live: number;
  today: number;
  today_scheduled: number;
  today_finished: number;
  upcoming: number;
  competitions: number;
};

export async function getHomeSummary(
  sport: string,
  date: string,
): Promise<HomeSummary | null> {
  try {
    return await getJSON<HomeSummary>(
      `/api/v1/home/summary?sport=${encodeURIComponent(sport)}&date=${encodeURIComponent(date)}`,
    );
  } catch {
    return null;
  }
}

export type SearchResult = {
  query: string;
  count: number;
  events: EventItem[];
};

export async function searchEvents(query: string): Promise<SearchResult | null> {
  try {
    return await getJSON<SearchResult>(
      `/api/v1/search?q=${encodeURIComponent(query)}&limit=30`,
    );
  } catch {
    return null;
  }
}

export type BoardPrice = {
  bookmaker: string;
  bookmaker_name: string;
  odds: number;
  price_kind: string;
  captured_at: string;
};

export type BoardSelection = {
  selection: string;
  best: BoardPrice | null;
  opening: BoardPrice | null;
  movement: number | null;
  prices: BoardPrice[];
};

export type BoardLine = {
  line: number | null;
  selections: BoardSelection[];
};

export type BoardEvent = {
  event_id: number;
  sport: string;
  competition: string;
  competition_slug: string;
  home: { id: number; name: string; crest_url: string | null };
  away: { id: number; name: string; crest_url: string | null };
  kickoff: string;
  status: string;
  lines: BoardLine[];
  last_captured_at: string | null;
};

export type OddsBoard = {
  sport: string;
  market: string;
  generated_at: string;
  count: number;
  events: BoardEvent[];
};

export async function getOddsBoard(params: {
  sport: string;
  market?: string;
  competition?: string;
  limit?: number;
}): Promise<OddsBoard | null> {
  const query = new URLSearchParams({ sport: params.sport });
  if (params.market) query.set("market", params.market);
  if (params.competition) query.set("competition", params.competition);
  if (params.limit) query.set("limit", String(params.limit));
  try {
    return await getJSON<OddsBoard>(`/api/v1/odds/board?${query.toString()}`);
  } catch {
    return null;
  }
}

export type ModelPick = {
  id: number;
  run_id: number;
  event_id: number;
  sport: string;
  competition: string;
  home_name: string;
  away_name: string;
  kickoff: string;
  market_type: string;
  selection: string;
  odds: number;
  status: "pending" | "won" | "lost" | "void" | "partial" | "unknown";
  locked_at: string;
  samples: number;
  hit_rate: number;
  implied: number;
  edge: number;
  strategy_slug: string;
  strategy_version: number;
  data_cutoff_at: string;
};

export type ModelPicks = {
  picks: ModelPick[];
  data_cutoff_at: string | null;
  run_id: number | null;
  strategy: string;
  count: number;
};

export async function getModelPicks(): Promise<ModelPicks | null> {
  try {
    return await getJSON<ModelPicks>("/api/v1/model-picks");
  } catch {
    return null;
  }
}

export type AdminHealth = {
  generated_at: string;
  database: { reachable: boolean; latency_ms: number };
  counts: {
    events: number;
    events_today: number;
    results: number;
    odds_snapshots: number;
    odds_last_hour: number;
    live_states: number;
    personal_bets: number;
  };
  import_runs: Array<{
    provider: string;
    job_type: string;
    status: string;
    started_at: string;
    finished_at: string | null;
    records_received: number;
    records_inserted: number;
    records_updated: number;
    records_skipped: number;
    error_message: string | null;
  }>;
  feeds: Array<{
    provider: string;
    job_type: string;
    last_success: string;
    age_seconds: number;
  }>;
  collectors: Array<{
    bookmaker: string;
    source_kind: string;
    status: string;
    completed_at: string | null;
    offer_count: number;
    age_seconds: number | null;
  }>;
  unmatched: { bookmaker_events: number; events: number; bet_legs: number };
};

export async function getAdminHealth(): Promise<AdminHealth | null> {
  try {
    return await getJSON<AdminHealth>("/api/v1/admin/health");
  } catch {
    return null;
  }
}

export function formatAgeSeconds(seconds: number | null): string {
  if (seconds === null) return "never";
  if (seconds < 60) return `${seconds}s ago`;
  const minutes = Math.round(seconds / 60);
  if (minutes < 60) return `${minutes}m ago`;
  const hours = Math.round(minutes / 60);
  if (hours < 48) return `${hours}h ago`;
  return `${Math.round(hours / 24)}d ago`;
}

export type OddsBucket = {
  low: number;
  high: number;
  samples: number;
  hits: number;
  misses: number;
  excluded: number;
  hit_rate: number;
  avg_odds: number;
};

export type OddsRanges = {
  market: string;
  line: number | null;
  bucket: number;
  settled: number;
  excluded: number;
  total: OddsBucket;
  buckets: OddsBucket[];
};

export async function getOddsRanges(
  market: string,
  line: number | undefined,
  bucket: number,
): Promise<OddsRanges | null> {
  const params = new URLSearchParams({ market, bucket: String(bucket) });
  if (line !== undefined) params.set("line", String(line));
  try {
    return await getJSON<OddsRanges>(
      `/api/v1/analytics/odds-ranges?${params.toString()}`,
    );
  } catch {
    return null;
  }
}

export function formatMoney(value: number | null, currency = "NGN"): string {
  if (value === null || Number.isNaN(value)) return "-";
  return `${currency} ${value.toLocaleString("en-NG", {
    minimumFractionDigits: 2,
    maximumFractionDigits: 2,
  })}`;
}

/** Absolute URL for an API-served asset path (crest proxy, etc.). */
export function absoluteAsset(path: string | null): string | null {
  if (!path) return null;
  return path.startsWith("http") ? path : `${API_BASE_URL}${path}`;
}

export function formatUptime(seconds: number): string {
  if (seconds < 60) return `${seconds}s`;
  const minutes = Math.floor(seconds / 60);
  if (minutes < 60) return `${minutes}m`;
  const hours = Math.floor(minutes / 60);
  const rest = minutes % 60;
  return rest === 0 ? `${hours}h` : `${hours}h ${rest}m`;
}

/** Short age label for a capture timestamp, e.g. "captured 3m ago". */
export function formatAge(iso: string | null): string {
  if (!iso) return "unknown capture time";
  const then = Date.parse(iso);
  if (Number.isNaN(then)) return "unknown capture time";
  const seconds = Math.max(0, Math.round((Date.now() - then) / 1000));
  if (seconds < 60) return `captured ${seconds}s ago`;
  const minutes = Math.round(seconds / 60);
  if (minutes < 60) return `captured ${minutes}m ago`;
  const hours = Math.round(minutes / 60);
  if (hours < 24) return `captured ${hours}h ago`;
  const days = Math.round(hours / 24);
  return `captured ${days}d ago`;
}

export function formatDate(iso: string | null): string {
  if (!iso) return "date unknown";
  const parsed = new Date(`${iso}T00:00:00Z`);
  if (Number.isNaN(parsed.valueOf())) return iso;
  return parsed.toLocaleDateString("en-GB", {
    weekday: "short",
    day: "numeric",
    month: "short",
    timeZone: "UTC",
  });
}

/** Today's date in the app's timezone, as YYYY-MM-DD. */
export function todayInLagos(): string {
  return new Intl.DateTimeFormat("en-CA", {
    timeZone: "Africa/Lagos",
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
  }).format(new Date());
}

export type Kickoff = {
  date: string;
  time: string;
  /** Viewer-timezone abbreviation, e.g. "GMT+1"; null when unavailable. */
  zone: string | null;
  /** Competition-local time when the provider publishes one. */
  venueTime: string | null;
  /** Explanation for a tooltip: never present a converted time as the venue's. */
  title: string;
};

/**
 * Kickoff in the viewer's timezone when the provider publishes a UTC instant.
 * Falls back to the competition-local clock (labelled as such) when it does
 * not, rather than guessing a timezone.
 */
export function kickoff(event: {
  start_date: string | null;
  start_time: string | null;
  start_time_local: string | null;
}): Kickoff {
  if (event.start_time) {
    const instant = new Date(event.start_time);
    if (!Number.isNaN(instant.valueOf())) {
      const zone =
        new Intl.DateTimeFormat("en-GB", { timeZoneName: "short" })
          .formatToParts(instant)
          .find((part) => part.type === "timeZoneName")?.value ?? null;

      return {
        date: instant.toLocaleDateString("en-GB", {
          weekday: "short",
          day: "numeric",
          month: "short",
        }),
        time: instant.toLocaleTimeString("en-GB", {
          hour: "2-digit",
          minute: "2-digit",
          hour12: false,
        }),
        zone,
        venueTime: event.start_time_local,
        title: `Kickoff in your timezone${zone ? ` (${zone})` : ""}.${
          event.start_time_local
            ? ` Competition-local: ${event.start_time_local}.`
            : ""
        }`,
      };
    }
  }

  return {
    date: formatDate(event.start_date),
    time: event.start_time_local ?? "time unknown",
    zone: null,
    venueTime: null,
    title: event.start_time_local
      ? "Competition-local kickoff; the provider did not publish a UTC time."
      : "Kickoff time unknown.",
  };
}
