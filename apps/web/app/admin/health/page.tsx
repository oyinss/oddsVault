"use client";

import { SiteHeader } from "@/components/site-header";
import { LoadingNote } from "@/components/loading-note";
import {
  formatAgeSeconds,
  getAdminHealth,
  type AdminHealth,
} from "@/lib/api";
import { useApiData } from "@/lib/use-api";

function stale(seconds: number, threshold: number) {
  return seconds > threshold;
}

export default function HealthPage() {
  const { data, loading } = useApiData<AdminHealth | null>(
    "admin-health",
    getAdminHealth,
  );

  return (
    <>
      <SiteHeader />
      <main className="mx-auto max-w-6xl px-6 py-12">
        <div className="flex flex-wrap items-end justify-between gap-4">
          <div>
            <p className="font-mono text-[11px] tracking-widest text-turf-300 uppercase">
              Admin · Data health
            </p>
            <h1 className="mt-2 text-3xl font-bold tracking-tight">
              Is the data fresh?
            </h1>
            <p className="mt-2 max-w-2xl text-sm leading-relaxed text-mist-400">
              One screen for the question that matters: are odds, results and
              live state still arriving, and what is waiting on a human?
            </p>
          </div>
          {data && (
            <button
              type="button"
              onClick={() => window.location.reload()}
              className="rounded-lg border border-white/10 px-3 py-1.5 text-xs text-mist-300 transition hover:border-white/25 hover:bg-white/5"
            >
              Refresh
            </button>
          )}
        </div>

        {loading && <LoadingNote label="Reading health…" />}

        {!loading && !data && (
          <div className="panel mt-8 border-flag-400/30 p-5">
            <p className="text-sm font-medium text-flag-400">
              Health endpoint unreachable
            </p>
            <p className="mt-1 text-xs text-mist-400">
              The API is down, or this deployment is not proxying{" "}
              <code className="font-mono">/api/v1/admin/health</code>.
            </p>
          </div>
        )}

        {data && (
          <>
            <dl className="mt-8 grid grid-cols-2 gap-3 sm:grid-cols-3 lg:grid-cols-6">
              <Stat
                label="Database"
                value={data.database.reachable ? "ok" : "down"}
                hint={`${data.database.latency_ms}ms`}
                tone={data.database.reachable ? "good" : "bad"}
              />
              <Stat label="Events" value={String(data.counts.events)} />
              <Stat label="Today" value={String(data.counts.events_today)} />
              <Stat
                label="Odds / hour"
                value={String(data.counts.odds_last_hour)}
                tone={data.counts.odds_last_hour > 0 ? "good" : "bad"}
              />
              <Stat label="Bets" value={String(data.counts.personal_bets)} />
              <Stat
                label="Unlinked legs"
                value={String(data.unmatched.bet_legs)}
                tone={data.unmatched.bet_legs > 0 ? "warn" : "good"}
              />
            </dl>

            <Section title="Feeds" note="Last successful run per provider job.">
              <Table
                head={["Provider", "Job", "Last success", "Age"]}
                rows={data.feeds.map((feed) => [
                  feed.provider,
                  feed.job_type,
                  new Date(feed.last_success).toLocaleString("en-GB"),
                  <Fresh
                    key={`${feed.provider}-${feed.job_type}`}
                    seconds={feed.age_seconds}
                    threshold={feed.job_type.includes("live") ? 900 : 26 * 3600}
                  />,
                ])}
              />
            </Section>

            <Section title="Collectors" note="Newest snapshot batch per bookmaker.">
              <Table
                head={["Bookmaker", "Kind", "Status", "Offers", "Last batch"]}
                rows={data.collectors.map((row) => [
                  row.bookmaker,
                  row.source_kind,
                  row.status,
                  String(row.offer_count),
                  <Fresh
                    key={row.bookmaker}
                    seconds={row.age_seconds}
                    threshold={15 * 60}
                  />,
                ])}
              />
            </Section>

            <Section
              title="Waiting on a human"
              note="Unresolved identities are stored, never guessed."
            >
              <dl className="grid grid-cols-1 gap-3 sm:grid-cols-3">
                <Stat
                  label="Unmatched bookmaker events"
                  value={String(data.unmatched.bookmaker_events)}
                />
                <Stat
                  label="Unresolved events"
                  value={String(data.unmatched.events)}
                />
                <Stat
                  label="Journal legs without a fixture"
                  value={String(data.unmatched.bet_legs)}
                />
              </dl>
            </Section>

            <Section title="Recent import runs" note="Newest first.">
              <Table
                head={["Provider", "Job", "Status", "Received", "Inserted", "Error"]}
                rows={data.import_runs.map((run) => [
                  run.provider,
                  run.job_type,
                  <span
                    key={`${run.provider}-${run.started_at}`}
                    className={
                      run.status === "success"
                        ? "text-turf-300"
                        : run.status === "running"
                          ? "text-mist-200"
                          : "text-flag-400"
                    }
                  >
                    {run.status}
                  </span>,
                  String(run.records_received),
                  String(run.records_inserted),
                  run.error_message ?? "-",
                ])}
              />
            </Section>

            <p className="mt-8 font-mono text-[10px] text-mist-500">
              generated {new Date(data.generated_at).toLocaleString("en-GB")}
            </p>
          </>
        )}
      </main>
    </>
  );
}

function Section({
  title,
  note,
  children,
}: {
  title: string;
  note: string;
  children: React.ReactNode;
}) {
  return (
    <section className="mt-10">
      <h2 className="text-sm font-semibold tracking-tight">{title}</h2>
      <p className="mt-1 text-xs text-mist-500">{note}</p>
      <div className="mt-4 overflow-hidden rounded-xl border border-white/[0.07] bg-ink-900/50">
        {children}
      </div>
    </section>
  );
}

function Table({
  head,
  rows,
}: {
  head: string[];
  rows: React.ReactNode[][];
}) {
  if (rows.length === 0) {
    return <p className="px-4 py-4 text-xs text-mist-500">Nothing recorded yet.</p>;
  }
  return (
    <table className="w-full text-left text-sm">
      <thead className="border-b border-white/[0.06] font-mono text-[10px] tracking-widest text-mist-500 uppercase">
        <tr>
          {head.map((label) => (
            <th key={label} className="px-4 py-2.5">
              {label}
            </th>
          ))}
        </tr>
      </thead>
      <tbody className="divide-y divide-white/[0.05]">
        {rows.map((row, index) => (
          <tr key={index}>
            {row.map((cell, cellIndex) => (
              <td
                key={cellIndex}
                className="max-w-[22rem] truncate px-4 py-2 text-xs text-mist-300"
              >
                {cell}
              </td>
            ))}
          </tr>
        ))}
      </tbody>
    </table>
  );
}

function Stat({
  label,
  value,
  hint,
  tone,
}: {
  label: string;
  value: string;
  hint?: string;
  tone?: "good" | "bad" | "warn";
}) {
  const colour =
    tone === "good"
      ? "text-turf-300"
      : tone === "bad"
        ? "text-flag-400"
        : tone === "warn"
          ? "text-flag-400"
          : "";
  return (
    <div className="rounded-xl border border-white/[0.06] bg-ink-900/60 p-4">
      <dt className="font-mono text-[10px] tracking-widest text-mist-500 uppercase">
        {label}
      </dt>
      <dd className={`mt-1 text-lg font-semibold tracking-tight ${colour}`}>
        {value}
      </dd>
      {hint && <p className="text-[11px] text-mist-500">{hint}</p>}
    </div>
  );
}

function Fresh({ seconds, threshold }: { seconds: number | null; threshold: number }) {
  const old = seconds !== null && stale(seconds, threshold);
  return (
    <span className={old ? "font-mono text-flag-400" : "font-mono text-mist-300"}>
      {formatAgeSeconds(seconds)}
    </span>
  );
}
