"use client";

import Link from "next/link";

import { SiteHeader } from "@/components/site-header";
import { API_BASE_URL, formatUptime, getApiHealth } from "@/lib/api";
import { useApiData } from "@/lib/use-api";

const FOUNDATION = [
  {
    title: "Go backend",
    detail: "One modular module: API plus four worker binaries, structured logs, graceful shutdown.",
    tag: "cmd/",
  },
  {
    title: "PostgreSQL + TimescaleDB",
    detail: "Forward-only migrations with checksums and an advisory lock. Reference catalogue only.",
    tag: "migrations/",
  },
  {
    title: "Premier League data",
    detail: "380 fixtures imported from OpenFootball, 90 results from football-data.co.uk, matched to one event.",
    tag: "events/",
  },
  {
    title: "Operations surface",
    detail: "Health, readiness and version endpoints; import and quota tables ready for Phase 1.",
    tag: "api/",
  },
];

const PHASES = [
  { name: "Phase 0 — Foundation", state: "Done", active: false },
  { name: "Phase 1A — Football fixtures & results", state: "Done", active: false },
  { name: "Phase 1B — Live scores", state: "Active", active: true },
  { name: "Phase 2 — Odds capture", state: "Partial", active: false },
  { name: "Phase 3 — Football settlement", state: "Planned", active: false },
  { name: "Phase 4 — Sports visual UI", state: "Planned", active: false },
];

const RULES = [
  {
    title: "No mock application data",
    body: "Games, odds, results and bets enter the database only from a real provider or collector.",
  },
  {
    title: "Missing stays missing",
    body: "Unavailable values are NULL or an explicit state such as needs_data — never a fabricated zero.",
  },
  {
    title: "Snapshots are immutable",
    body: "1.92 → 1.88 → 1.81 → 1.76 keeps every observation. The newest price never overwrites history.",
  },
];

export default function HomePage() {
  const { data: health, loading } = useApiData("api-health", getApiHealth, {
    refreshMs: 30_000,
  });

  return (
    <>
      <SiteHeader />

      <main>
        <section className="relative overflow-hidden border-b border-line">
          <div
            aria-hidden
            className="pitch-lines pointer-events-none absolute inset-0"
          />
          <div className="relative mx-auto max-w-6xl px-6 pt-20 pb-24">
            <p className="mb-5 inline-flex items-center gap-2 rounded-full border border-turf-400/25 bg-turf-500/10 px-3 py-1 font-mono text-[11px] tracking-widest text-turf-300 uppercase">
              <span className="size-1.5 rounded-full bg-turf-400" />
              Football data live
            </p>

            <h1 className="max-w-3xl text-4xl leading-[1.05] font-semibold tracking-tight text-balance sm:text-5xl md:text-6xl">
              Every price, timestamped.
              <br />
              <span className="text-mist-400">Every result, verified.</span>
            </h1>

            <p className="mt-6 max-w-2xl text-base leading-relaxed text-mist-300 sm:text-lg">
              OddsVault captures real bookmaker odds as immutable snapshots,
              settles supported markets deterministically once events finish,
              and keeps the whole history queryable — so analysis can only ever
              move forward.
            </p>

            <div className="mt-10 flex flex-wrap items-center gap-3">
              <a
                href={`${API_BASE_URL}/healthz`}
                className="inline-flex items-center gap-2 rounded-lg bg-turf-500 px-4 py-2.5 text-sm font-semibold text-accent-ink transition hover:bg-turf-400"
              >
                API health
                <span aria-hidden>↗</span>
              </a>
              <Link
                href="/events"
                className="inline-flex items-center gap-2 rounded-lg border border-line-strong px-4 py-2.5 text-sm font-medium text-mist-200 transition hover:border-line-strong hover:bg-raised"
              >
                Premier League fixtures
                <span aria-hidden>→</span>
              </Link>
              <a
                href={`${API_BASE_URL}/readyz`}
                className="inline-flex items-center gap-2 rounded-lg px-4 py-2.5 text-sm font-medium text-mist-400 transition hover:text-mist-200"
              >
                Readiness
              </a>
            </div>

            <dl className="mt-14 grid max-w-3xl grid-cols-2 gap-x-6 gap-y-8 sm:grid-cols-4">
              <Stat label="Sports modelled" value="5" hint="football first" />
              <Stat label="Mock rows" value="0" hint="by policy" />
              <Stat label="Odds tables" value="hypertable" hint="Phase 2" />
              <Stat label="Settlement" value="versioned" hint="Phase 3" />
            </dl>
          </div>
        </section>

        <section className="mx-auto max-w-6xl px-6 py-16">
          <div className="grid gap-5 lg:grid-cols-3">
            <div className="panel p-6 lg:col-span-1">
              <h2 className="text-sm font-semibold tracking-tight text-mist-100">
                Go API
              </h2>
              <p className="mt-1 font-mono text-[11px] text-mist-500">
                {API_BASE_URL}
              </p>

              <div className="mt-6 flex items-center gap-3">
                <span
                  className={`size-2.5 rounded-full ${
                    loading
                      ? "bg-mist-500"
                      : health?.reachable
                        ? "bg-turf-400"
                        : "bg-flag-400"
                  }`}
                />
                <span className="text-sm font-medium">
                  {loading
                    ? "Checking…"
                    : health?.reachable
                      ? "Responding"
                      : "Not reachable"}
                </span>
              </div>

              <dl className="mt-5 space-y-2.5 text-sm">
                <div className="flex items-baseline justify-between gap-4">
                  <dt className="text-mist-500">Build</dt>
                  <dd className="font-mono text-[12px] text-mist-200">
                    {health?.reachable ? health.version : "—"}
                  </dd>
                </div>
                <div className="flex items-baseline justify-between gap-4">
                  <dt className="text-mist-500">Uptime</dt>
                  <dd className="font-mono text-[12px] text-mist-200">
                    {health?.reachable ? formatUptime(health.uptimeSeconds) : "—"}
                  </dd>
                </div>
              </dl>

              {!loading && health && !health.reachable && (
                <p className="mt-5 rounded-lg border border-line bg-ink-850 p-3 text-xs leading-relaxed text-mist-400">
                  Start the backend with{" "}
                  <code className="font-mono text-mist-200">make api</code>. The
                  page stays usable while the API is down.
                </p>
              )}
            </div>

            <div className="panel p-6 lg:col-span-2">
              <h2 className="text-sm font-semibold tracking-tight">
                What exists in this foundation
              </h2>
              <p className="mt-1 text-sm text-mist-400">
                Everything below boots from this repository; nothing below
                pretends to have data yet.
              </p>

              <ul className="mt-6 grid gap-4 sm:grid-cols-2">
                {FOUNDATION.map((item) => (
                  <li
                    key={item.title}
                    className="rounded-xl border border-line bg-ink-850/60 p-4"
                  >
                    <div className="flex items-center justify-between gap-3">
                      <h3 className="text-sm font-medium">{item.title}</h3>
                      <code className="font-mono text-[10px] text-mist-500">
                        {item.tag}
                      </code>
                    </div>
                    <p className="mt-2 text-xs leading-relaxed text-mist-400">
                      {item.detail}
                    </p>
                  </li>
                ))}
              </ul>
            </div>
          </div>
        </section>

        <section className="border-y border-line bg-ink-900/40">
          <div className="mx-auto grid max-w-6xl gap-10 px-6 py-16 lg:grid-cols-2">
            <div>
              <h2 className="text-2xl font-semibold tracking-tight">
                The build order
              </h2>
              <p className="mt-2 max-w-md text-sm leading-relaxed text-mist-400">
                One proven pipeline before the next. Football ingestion, then a
                single collector, then settlement — not twelve half-features.
              </p>

              <ol className="mt-8 space-y-3">
                {PHASES.map((phase) => (
                  <li
                    key={phase.name}
                    className={`flex items-center justify-between gap-4 rounded-xl border px-4 py-3 ${
                      phase.active
                        ? "border-turf-400/25 bg-turf-500/[0.07]"
                        : "border-line bg-ink-950/40"
                    }`}
                  >
                    <span className="text-sm text-mist-200">{phase.name}</span>
                    <span
                      className={`shrink-0 font-mono text-[10px] tracking-widest uppercase ${
                        phase.active ? "text-turf-300" : "text-mist-500"
                      }`}
                    >
                      {phase.state}
                    </span>
                  </li>
                ))}
              </ol>
            </div>

            <div>
              <h2 className="text-2xl font-semibold tracking-tight">
                Non-negotiable rules
              </h2>
              <p className="mt-2 max-w-md text-sm leading-relaxed text-mist-400">
                The dataset is the product. Convenience never outranks
                historical truth.
              </p>

              <div className="mt-8 space-y-3">
                {RULES.map((rule, index) => (
                  <div
                    key={rule.title}
                    className="rounded-xl border border-line bg-ink-950/40 p-4"
                  >
                    <div className="flex items-baseline gap-3">
                      <span className="font-mono text-[11px] text-turf-400">
                        0{index + 1}
                      </span>
                      <h3 className="text-sm font-medium">{rule.title}</h3>
                    </div>
                    <p className="mt-2 pl-8 text-xs leading-relaxed text-mist-400">
                      {rule.body}
                    </p>
                  </div>
                ))}
              </div>
            </div>
          </div>
        </section>
      </main>

      <footer className="mx-auto flex max-w-6xl flex-col gap-3 px-6 py-10 text-xs text-mist-500 sm:flex-row sm:items-center sm:justify-between">
        <p>OddsVault — sports odds intelligence platform.</p>
        <p className="font-mono">Go · PostgreSQL + TimescaleDB · Next.js</p>
      </footer>
    </>
  );
}

function Stat({
  label,
  value,
  hint,
}: {
  label: string;
  value: string;
  hint: string;
}) {
  return (
    <div>
      <dt className="font-mono text-[10px] tracking-widest text-mist-500 uppercase">
        {label}
      </dt>
      <dd className="mt-1 text-lg font-semibold tracking-tight">{value}</dd>
      <p className="text-[11px] text-mist-500">{hint}</p>
    </div>
  );
}
