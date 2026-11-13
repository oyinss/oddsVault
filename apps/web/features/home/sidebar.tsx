"use client";

import Link from "next/link";
import { faArrowRight } from "@fortawesome/free-solid-svg-icons";
import { FontAwesomeIcon } from "@fortawesome/react-fontawesome";

import {
  formatMoney,
  getMyBets,
  type BetSummary,
  type EventItem,
  type HomeSummary,
} from "@/lib/api";
import { useEffect, useState } from "react";
import { eventTimeLabel, eventDayLabel } from "@/features/home/format";
import { sportBySlug } from "@/features/home/sport-switcher";

export function HomeSidebar({
  sport,
  summary,
  highlights,
}: {
  sport: string;
  summary: HomeSummary | null;
  highlights: EventItem[];
}) {
  return (
    <aside className="space-y-4">
      <Glance summary={summary} />
      <MyBets />
      <ModelPicks />
      <Highlights sport={sport} events={highlights} />
    </aside>
  );
}

function Panel({
  title,
  href,
  children,
}: {
  title: string;
  href?: string;
  children: React.ReactNode;
}) {
  return (
    <section className="rounded-2xl border border-line bg-ink-900/60 p-4">
      <header className="mb-3 flex items-center justify-between gap-3">
        <h2 className="text-sm font-semibold text-mist-100">{title}</h2>
        {href && (
          <Link
            href={href}
            className="flex items-center gap-1 font-mono text-[10px] tracking-widest text-mist-500 uppercase hover:text-mist-200"
          >
            View all
            <FontAwesomeIcon icon={faArrowRight} className="size-2.5" />
          </Link>
        )}
      </header>
      {children}
    </section>
  );
}

function Glance({ summary }: { summary: HomeSummary | null }) {
  const stats = [
    { label: "Live games", value: summary?.live ?? 0, tone: "text-red-300" },
    { label: "Today", value: summary?.today ?? 0, tone: "text-mist-100" },
    { label: "Upcoming", value: summary?.upcoming ?? 0, tone: "text-mist-100" },
    { label: "Competitions", value: summary?.competitions ?? 0, tone: "text-mist-100" },
  ];

  return (
    <Panel title="Today at a glance" href="/events">
      <dl className="grid grid-cols-2 gap-2">
        {stats.map((stat) => (
          <div
            key={stat.label}
            className="rounded-xl border border-line bg-ink-950/50 px-3 py-2.5"
          >
            <dt className="font-mono text-[9px] tracking-widest text-mist-500 uppercase">
              {stat.label}
            </dt>
            <dd className={`mt-0.5 font-mono text-xl tabular-nums ${stat.tone}`}>
              {stat.value}
            </dd>
          </div>
        ))}
      </dl>
    </Panel>
  );
}

function MyBets() {
  const [summary, setSummary] = useState<BetSummary | null>(null);
  const [loaded, setLoaded] = useState(false);

  useEffect(() => {
    void getMyBets().then((result) => {
      if (result.ok) setSummary(result.summary);
      setLoaded(true);
    });
  }, []);

  const roi =
    summary && summary.staked > 0 ? summary.profit_loss / summary.staked : 0;
  const settled = summary ? summary.won + summary.lost : 0;
  const winRate = settled > 0 && summary ? summary.won / settled : 0;

  return (
    <Panel title="My bets" href="/my-bets">
      {!loaded ? (
        <p className="text-xs text-mist-500">Loading journal...</p>
      ) : !summary || summary.bets === 0 ? (
        <div>
          <p className="text-sm text-mist-300">No bets recorded yet.</p>
          <Link
            href="/admin/journal"
            className="mt-2 inline-block font-mono text-[11px] text-turf-300"
          >
            Import a bet slip
          </Link>
        </div>
      ) : (
        <>
          <dl className="grid grid-cols-3 gap-2 text-xs">
            <Metric label="Staked" value={formatMoney(summary.staked)} />
            <Metric label="Returned" value={formatMoney(summary.returned)} />
            <Metric
              label="P / L"
              value={formatMoney(summary.profit_loss)}
              tone={summary.profit_loss >= 0 ? "good" : "bad"}
            />
          </dl>
          <div className="mt-3 grid grid-cols-3 gap-2 border-t border-line pt-3 text-xs">
            <Metric
              label="Record"
              value={`${summary.won}W / ${summary.lost}L`}
            />
            <Metric
              label="ROI"
              value={`${(roi * 100).toFixed(1)}%`}
              tone={roi >= 0 ? "good" : "bad"}
            />
            <Metric label="Win rate" value={`${(winRate * 100).toFixed(0)}%`} />
          </div>
        </>
      )}
    </Panel>
  );
}

function ModelPicks() {
  return (
    <Panel title="Model picks">
      <p className="text-sm text-mist-300">No model picks yet.</p>
      <p className="mt-1 text-xs leading-relaxed text-mist-500">
        Picks will appear here once a strategy has frozen pre-kickoff
        recommendations. Nothing is generated to fill the space.
      </p>
    </Panel>
  );
}

function Highlights({ sport, events }: { sport: string; events: EventItem[] }) {
  const icon = sportBySlug(sport).icon;
  return (
    <Panel title="Upcoming highlights" href="/events">
      {events.length === 0 ? (
        <p className="text-xs text-mist-500">No upcoming fixtures found.</p>
      ) : (
        <ul className="space-y-2.5">
          {events.map((event) => (
            <li key={event.id}>
              <Link
                href={`/events/detail?id=${event.id}`}
                className="flex items-center gap-2.5 rounded-lg px-1 py-1 transition hover:bg-white/[0.03]"
              >
                <span className="grid size-7 shrink-0 place-items-center rounded-md bg-white/[0.05] text-mist-400">
                  <FontAwesomeIcon icon={icon} className="size-3" />
                </span>
                <span className="min-w-0 flex-1">
                  <span className="block truncate text-xs text-mist-200">
                    {event.home.name} v {event.away.name}
                  </span>
                  <span className="block font-mono text-[10px] text-mist-500">
                    {eventDayLabel(event)} · {eventTimeLabel(event)}
                  </span>
                </span>
              </Link>
            </li>
          ))}
        </ul>
      )}
    </Panel>
  );
}

function Metric({
  label,
  value,
  tone,
}: {
  label: string;
  value: string;
  tone?: "good" | "bad";
}) {
  return (
    <div className="min-w-0">
      <dt className="font-mono text-[9px] tracking-widest text-mist-500 uppercase">
        {label}
      </dt>
      <dd
        className={`mt-0.5 truncate font-mono text-[11px] tabular-nums ${
          tone === "good"
            ? "text-turf-300"
            : tone === "bad"
              ? "text-flag-400"
              : "text-mist-100"
        }`}
      >
        {value}
      </dd>
    </div>
  );
}
