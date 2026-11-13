"use client";

import { faMagnifyingGlass, faSort } from "@fortawesome/free-solid-svg-icons";
import { FontAwesomeIcon } from "@fortawesome/react-fontawesome";

import type { HomeView } from "@/features/home/format";

const VIEWS: { key: HomeView; label: string }[] = [
  { key: "live", label: "Live" },
  { key: "today", label: "Today" },
  { key: "upcoming", label: "Upcoming" },
  { key: "tomorrow", label: "Tomorrow" },
];

export function FilterBar({
  view,
  onView,
  date,
  onDate,
  search,
  onSearch,
  competitions,
  competition,
  onCompetition,
  sort,
  onSort,
}: {
  view: HomeView;
  onView: (view: HomeView) => void;
  date: string;
  onDate: (date: string) => void;
  search: string;
  onSearch: (value: string) => void;
  competitions: string[];
  competition: string;
  onCompetition: (value: string) => void;
  sort: string;
  onSort: (value: string) => void;
}) {
  return (
    <div className="space-y-3">
      <div className="flex flex-wrap items-center gap-2">
        {VIEWS.map((item) => (
          <button
            key={item.key}
            type="button"
            onClick={() => onView(item.key)}
            className={`rounded-lg border px-3 py-1.5 text-sm transition ${
              view === item.key
                ? "border-turf-400/40 bg-turf-500/10 text-turf-200"
                : "border-line text-mist-300 hover:border-white/20 hover:bg-white/[0.04]"
            }`}
          >
            {item.label}
          </button>
        ))}

        <label
          className={`flex items-center gap-2 rounded-lg border px-3 py-1.5 text-sm ${
            view === "date"
              ? "border-turf-400/40 bg-turf-500/10 text-turf-200"
              : "border-line text-mist-300"
          }`}
        >
          <span className="font-mono text-[10px] tracking-widest uppercase">
            Date
          </span>
          <input
            type="date"
            value={date}
            onChange={(event) => {
              onDate(event.target.value);
              if (event.target.value) onView("date");
            }}
            className="bg-transparent text-sm text-mist-100 outline-none [color-scheme:dark]"
          />
        </label>
      </div>

      <div className="flex flex-col gap-2 sm:flex-row sm:items-center">
        <label className="relative flex-1">
          <span className="pointer-events-none absolute top-1/2 left-3 -translate-y-1/2 text-mist-500">
            <FontAwesomeIcon icon={faMagnifyingGlass} className="size-3.5" />
          </span>
          <input
            value={search}
            onChange={(event) => onSearch(event.target.value)}
            placeholder="Search teams, players, competitions"
            className="w-full rounded-lg border border-line bg-ink-900/70 py-2 pr-3 pl-9 text-sm text-mist-100 placeholder:text-mist-500 focus:border-turf-400/50 focus:outline-none"
          />
        </label>

        <select
          value={competition}
          onChange={(event) => onCompetition(event.target.value)}
          className="rounded-lg border border-line bg-ink-900/70 px-3 py-2 text-sm text-mist-200 focus:border-turf-400/50 focus:outline-none"
        >
          <option value="">All competitions</option>
          {competitions.map((name) => (
            <option key={name} value={name}>
              {name}
            </option>
          ))}
        </select>

        <label className="flex items-center gap-2 rounded-lg border border-line bg-ink-900/70 px-3 py-2 text-sm">
          <FontAwesomeIcon icon={faSort} className="size-3.5 text-mist-500" />
          <select
            value={sort}
            onChange={(event) => onSort(event.target.value)}
            className="bg-transparent text-sm text-mist-200 focus:outline-none"
          >
            <option value="live_first">Live first</option>
            <option value="kickoff_asc">Kickoff - earliest</option>
            <option value="kickoff_desc">Kickoff - latest</option>
            <option value="competition">Competition</option>
            <option value="odds">Highest home odds</option>
          </select>
        </label>
      </div>
    </div>
  );
}
