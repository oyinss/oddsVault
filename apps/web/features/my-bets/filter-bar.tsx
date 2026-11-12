"use client";

import {
  RANGE_LABELS,
  type Filters,
  type RangeKey,
  type SortKey,
  type StatusKey,
  type TypeKey,
} from "./derive";
import { Icon, type IconName } from "./icons";

export function FilterBar({
  filters,
  onChange,
  onReset,
  dirty,
  bookmakers,
  competitions,
}: {
  filters: Filters;
  onChange: (patch: Partial<Filters>) => void;
  onReset: () => void;
  dirty: boolean;
  bookmakers: string[];
  competitions: string[];
}) {
  return (
    <div className="mt-4 rounded-xl border border-line bg-ink-900/50 p-2">
      <div className="flex flex-wrap items-center gap-1">
        <Select
          icon="calendar"
          label="Date range"
          value={filters.range}
          onChange={(value) => onChange({ range: value as RangeKey })}
        >
          {(Object.keys(RANGE_LABELS) as RangeKey[]).map((key) => (
            <option key={key} value={key}>
              {RANGE_LABELS[key]}
            </option>
          ))}
        </Select>

        <Divider />

        <Select
          icon="book"
          label="Bookmaker"
          value={filters.bookmaker}
          onChange={(value) => onChange({ bookmaker: value })}
        >
          <option value="">All bookmakers</option>
          {bookmakers.map((name) => (
            <option key={name} value={name}>
              {name}
            </option>
          ))}
        </Select>

        <Divider />

        <Select
          icon="ball"
          label="Competition"
          value={filters.competition}
          onChange={(value) => onChange({ competition: value })}
        >
          <option value="">All competitions</option>
          {competitions.map((name) => (
            <option key={name} value={name}>
              {name}
            </option>
          ))}
        </Select>

        <Divider />

        <Select
          icon="layers"
          label="Status"
          value={filters.status}
          onChange={(value) => onChange({ status: value as StatusKey })}
        >
          <option value="all">All statuses</option>
          <option value="won">Won</option>
          <option value="lost">Lost</option>
          <option value="pending">Pending</option>
        </Select>

        <div className="ml-auto flex w-full flex-1 items-center gap-2 sm:w-auto sm:min-w-[320px]">
          <label className="flex min-w-0 flex-1 items-center gap-2 rounded-lg border border-line-strong bg-ink-950/40 px-2.5 py-2">
            <Icon name="search" className="size-3.5 shrink-0 text-mist-500" />
            <input
              type="search"
              value={filters.query}
              onChange={(event) => onChange({ query: event.target.value })}
              placeholder="Search bets, markets, teams…"
              className="w-full bg-transparent text-[12px] text-mist-100 outline-none placeholder:text-mist-500"
            />
          </label>

          <Select
            icon="sort"
            label="Sort by"
            value={filters.sort}
            onChange={(value) => onChange({ sort: value as SortKey })}
          >
            <option value="date">Newest</option>
            <option value="stake">Stake (high → low)</option>
            <option value="profit">Profit (high → low)</option>
            <option value="odds">Odds (high → low)</option>
          </Select>
        </div>
      </div>

      <div className="flex flex-wrap items-center gap-2 px-2 pt-2 pb-1">
        <TypePill
          active={filters.betType === "all"}
          onClick={() => onChange({ betType: "all" })}
        >
          All types
        </TypePill>
        <TypePill
          active={filters.betType === "accumulator"}
          onClick={() => onChange({ betType: "accumulator" })}
        >
          Accumulators
        </TypePill>
        <TypePill
          active={filters.betType === "single"}
          onClick={() => onChange({ betType: "single" })}
        >
          Singles
        </TypePill>

        {dirty && (
          <button
            type="button"
            onClick={onReset}
            className="ml-auto font-mono text-[10px] tracking-widest text-mist-500 uppercase transition hover:text-mist-200"
          >
            Clear filters
          </button>
        )}
      </div>
    </div>
  );
}

function TypePill({
  active,
  onClick,
  children,
}: {
  active: boolean;
  onClick: () => void;
  children: React.ReactNode;
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      className={`rounded-full border px-3 py-1 text-[11px] transition ${
        active
          ? "border-turf-400/30 bg-turf-500/10 text-turf-300"
          : "border-line-strong text-mist-400 hover:border-line-strong hover:text-mist-200"
      }`}
    >
      {children}
    </button>
  );
}

function Divider() {
  return <span className="mx-1 hidden h-8 w-px bg-raised sm:block" />;
}

function Select({
  icon,
  label,
  value,
  onChange,
  children,
}: {
  icon: IconName;
  label: string;
  value: string;
  onChange: (value: string) => void;
  children: React.ReactNode;
}) {
  return (
    <label className="flex min-w-0 items-center gap-2 rounded-lg px-2 py-1.5 transition hover:bg-hover">
      <Icon name={icon} className="size-4 shrink-0 text-mist-500" />
      <span className="min-w-0">
        <span className="block font-mono text-[8px] tracking-widest text-mist-500 uppercase">
          {label}
        </span>
        <span className="relative block">
          <select
            value={value}
            onChange={(event) => onChange(event.target.value)}
            className="w-full max-w-[10rem] cursor-pointer appearance-none truncate bg-transparent pr-4 text-[12px] text-mist-100 outline-none"
          >
            {children}
          </select>
          <Icon
            name="chevron"
            className="pointer-events-none absolute top-1/2 right-0 size-3 -translate-y-1/2 text-mist-500"
          />
        </span>
      </span>
    </label>
  );
}
