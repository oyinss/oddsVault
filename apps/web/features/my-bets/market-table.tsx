"use client";

import { useState } from "react";

import { formatMoney } from "@/lib/api";

import { formatDayShort, type MarketRow } from "./derive";
import { Icon } from "./icons";

type Column = "label" | "legs" | "won" | "rate" | "profit" | "last";

export function MarketTable({ rows }: { rows: MarketRow[] }) {
  const [sort, setSort] = useState<{ column: Column; dir: 1 | -1 }>({
    column: "legs",
    dir: -1,
  });
  const [expanded, setExpanded] = useState(false);

  const toggle = (column: Column) => {
    setSort((current) =>
      current.column === column
        ? { column, dir: current.dir === 1 ? -1 : 1 }
        : { column, dir: column === "label" ? 1 : -1 },
    );
  };

  const sorted = [...rows].sort((a, b) => {
    const dir = sort.dir;
    switch (sort.column) {
      case "label":
        return a.label.localeCompare(b.label) * dir;
      case "won":
        return (a.won - b.won) * dir;
      case "rate":
        return ((a.winRate ?? -1) - (b.winRate ?? -1)) * dir;
      case "profit":
        return ((a.singleProfit ?? -1e15) - (b.singleProfit ?? -1e15)) * dir;
      case "last":
        return ((a.lastPlayed ?? "") < (b.lastPlayed ?? "") ? -1 : 1) * dir;
      default:
        return (a.legs - b.legs) * dir;
    }
  });

  return (
    <section className="flex h-full flex-col rounded-2xl border border-line bg-ink-900/40">
      <header className="flex items-center justify-between gap-3 border-b border-line px-4 py-3">
        <div className="flex items-center gap-2">
          <Icon name="chart" className="size-4 text-turf-300" />
          <h2 className="text-sm font-semibold tracking-tight">Market patterns</h2>
          <span
            className="text-mist-500"
            title="What you actually played, by line. Rates count settled legs only and always show the sample size."
          >
            <Icon name="info" className="size-3.5" />
          </span>
        </div>
        <p className="hidden font-mono text-[10px] tracking-widest text-mist-500 uppercase sm:block">
          {rows.length} lines
        </p>
      </header>

      {rows.length === 0 ? (
        <p className="px-4 py-8 text-sm text-mist-400">
          No verified market lines in this selection. Unparsed provider text is
          never grouped as if it were a market.
        </p>
      ) : (
        <div className="overflow-x-auto">
          <table className="w-full min-w-[560px] text-left text-sm">
            <thead className="font-mono text-[9px] tracking-widest text-mist-500 uppercase">
              <tr className="border-b border-line">
                <Th column="label" sort={sort} onSort={toggle} align="left">
                  Market
                </Th>
                <Th column="legs" sort={sort} onSort={toggle}>
                  Legs
                </Th>
                <Th column="won" sort={sort} onSort={toggle}>
                  Won
                </Th>
                <Th column="rate" sort={sort} onSort={toggle}>
                  Win rate
                </Th>
                <Th column="profit" sort={sort} onSort={toggle}>
                  P/L <span className="normal-case">(singles)</span>
                </Th>
                <Th column="last" sort={sort} onSort={toggle} align="right">
                  Last played
                </Th>
              </tr>
            </thead>
            <tbody className="divide-y divide-line">
              {sorted.slice(0, expanded ? sorted.length : 8).map((row) => (
                <tr key={row.key} className="transition hover:bg-hover">
                  <td className="px-4 py-2.5 text-xs text-mist-200">{row.label}</td>
                  <td className="px-3 py-2.5 text-right font-mono text-xs text-mist-300">
                    {row.legs}
                  </td>
                  <td className="px-3 py-2.5 text-right font-mono text-xs text-mist-300">
                    {row.won}
                  </td>
                  <td className="px-3 py-2.5 text-right">
                    <span
                      className={`rounded-full px-1.5 py-0.5 font-mono text-[10px] ${
                        row.winRate === null
                          ? "text-mist-500"
                          : row.winRate >= 0.7
                            ? "bg-turf-500/10 text-turf-300"
                            : row.winRate >= 0.5
                              ? "bg-raised text-mist-200"
                              : "bg-loss-500/10 text-loss-300"
                      }`}
                    >
                      {row.winRate === null
                        ? "-"
                        : `${(row.winRate * 100).toFixed(1)}% (${row.settled})`}
                    </span>
                  </td>
                  <td className="px-3 py-2.5 text-right font-mono text-[11px]">
                    {row.singleProfit === null ? (
                      <span
                        className="text-mist-500"
                        title="No single-leg slips on this line; an accumulator's result cannot be split across its legs."
                      >
                        -
                      </span>
                    ) : (
                      <span
                        className={
                          row.singleProfit >= 0 ? "text-turf-300" : "text-loss-300"
                        }
                      >
                        {formatMoney(row.singleProfit, row.currency)}
                      </span>
                    )}
                  </td>
                  <td className="px-4 py-2.5 text-right font-mono text-[10px] text-mist-500">
                    {formatDayShort(row.lastPlayed)}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      <div className="mt-auto border-t border-line px-4 py-3">
        {sorted.length > 8 && (
          <button
            type="button"
            onClick={() => setExpanded((value) => !value)}
            className="mb-2 font-mono text-[10px] tracking-widest text-turf-300/80 uppercase transition hover:text-turf-300"
          >
            {expanded ? "Show top 8" : `Show all ${sorted.length} lines`}
          </button>
        )}
        <p className="text-[10px] leading-relaxed text-mist-500">
          Rates divide by settled legs only. P/L appears for single-leg slips;
          an accumulator&apos;s result cannot be attributed to one leg.
        </p>
      </div>
    </section>
  );
}

function Th({
  column,
  sort,
  onSort,
  children,
  align = "right",
}: {
  column: Column;
  sort: { column: Column; dir: 1 | -1 };
  onSort: (column: Column) => void;
  children: React.ReactNode;
  align?: "left" | "right";
}) {
  const active = sort.column === column;
  return (
    <th
      scope="col"
      className={`px-3 py-2 font-medium ${align === "left" ? "text-left" : "text-right"}`}
    >
      <button
        type="button"
        onClick={() => onSort(column)}
        className={`inline-flex items-center gap-1 uppercase transition hover:text-mist-200 ${
          active ? "text-mist-200" : "text-mist-500"
        }`}
      >
        {children}
        <Icon
          name="chevron"
          className={`size-2.5 transition ${
            active ? (sort.dir === 1 ? "rotate-180" : "") : "opacity-30"
          }`}
        />
      </button>
    </th>
  );
}
