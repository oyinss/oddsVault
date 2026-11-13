"use client";

import { useId } from "react";

import { formatDayShort, type DayPoint } from "./derive";

/* ------------------------------------------------------------- sparkline */

export function Sparkline({
  values,
  className = "h-11 w-32",
}: {
  values: number[];
  className?: string;
}) {
  const gradientId = `spark-${useId().replace(/:/g, "")}`;
  if (values.length < 2) return <span aria-hidden className={className} />;

  const width = 132;
  const height = 46;
  const pad = 4;
  const min = Math.min(...values, 0);
  const max = Math.max(...values, 0);
  const span = max - min || 1;
  const stepX = (width - pad * 2) / (values.length - 1);
  const x = (index: number) => pad + index * stepX;
  const y = (value: number) =>
    height - pad - ((value - min) / span) * (height - pad * 2);

  const line = values
    .map((value, index) => `${index === 0 ? "M" : "L"}${x(index)},${y(value)}`)
    .join(" ");
  const area = `${line} L${x(values.length - 1)},${height - pad} L${pad},${height - pad} Z`;
  const rising = (values[values.length - 1] ?? 0) >= 0;

  return (
    <svg
      viewBox={`0 0 ${width} ${height}`}
      aria-hidden
      className={className}
      preserveAspectRatio="none"
    >
      <defs>
        <linearGradient id={gradientId} x1="0" y1="0" x2="0" y2="1">
          <stop offset="0%" stopColor="var(--turf-400)" stopOpacity="0.35" />
          <stop offset="100%" stopColor="var(--turf-400)" stopOpacity="0" />
        </linearGradient>
      </defs>
      <path d={area} fill={`url(#${gradientId})`} />
      <path
        d={line}
        fill="none"
        stroke={rising ? "var(--turf-400)" : "var(--loss-400)"}
        strokeWidth="1.8"
        strokeLinecap="round"
        strokeLinejoin="round"
      />
    </svg>
  );
}

/* ------------------------------------------------------- performance chart */

const WIDTH = 560;
const HEIGHT = 220;
const PAD = { top: 14, right: 14, bottom: 26, left: 46 };

function compact(value: number): string {
  const abs = Math.abs(value);
  const sign = value < 0 ? "-" : "";
  if (abs >= 1_000_000) return `${sign}${(abs / 1_000_000).toFixed(1)}M`;
  if (abs >= 1_000) return `${sign}${Math.round(abs / 1_000)}K`;
  return `${sign}${Math.round(abs)}`;
}

function smooth(points: { x: number; y: number }[]): string {
  if (points.length === 0) return "";
  if (points.length === 1) return `M${points[0].x},${points[0].y}`;
  let path = `M${points[0].x},${points[0].y}`;
  for (let index = 0; index < points.length - 1; index += 1) {
    const from = points[index];
    const to = points[index + 1];
    const midX = (from.x + to.x) / 2;
    path += ` C${midX},${from.y} ${midX},${to.y} ${to.x},${to.y}`;
  }
  return path;
}

export function PerformanceChart({
  points,
  mode,
}: {
  points: DayPoint[];
  mode: "pl" | "rate";
}) {
  const gradientId = `trend-${useId().replace(/:/g, "")}`;

  const series = points
    .map((point) => ({
      day: point.day,
      value:
        mode === "pl"
          ? point.cumulativeProfit
          : point.cumulativeWinRate === null
            ? null
            : point.cumulativeWinRate * 100,
    }))
    .filter((point): point is { day: string; value: number } => point.value !== null);

  if (series.length < 2) {
    return (
      <div className="grid h-[220px] place-items-center rounded-lg border border-line bg-ink-950/40 text-xs text-mist-500">
        Not enough settled days to draw a trend yet.
      </div>
    );
  }

  const values = series.map((point) => point.value);
  const rawMin = Math.min(...values, mode === "pl" ? 0 : 0);
  const rawMax = Math.max(...values, mode === "pl" ? 0 : 100);
  const headroom = mode === "pl" ? Math.max((rawMax - rawMin) * 0.12, 1) : 0;
  const min = mode === "pl" ? rawMin - headroom : 0;
  const max = mode === "pl" ? rawMax + headroom : 100;
  const span = max - min || 1;

  const plotWidth = WIDTH - PAD.left - PAD.right;
  const plotHeight = HEIGHT - PAD.top - PAD.bottom;
  const times = series.map((point) => Date.parse(`${point.day}T00:00:00Z`));
  const timeMin = Math.min(...times);
  const timeMax = Math.max(...times);

  const x = (time: number) =>
    timeMax === timeMin
      ? PAD.left + plotWidth / 2
      : PAD.left + ((time - timeMin) / (timeMax - timeMin)) * plotWidth;
  const y = (value: number) =>
    PAD.top + (1 - (value - min) / span) * plotHeight;

  const coords = series.map((point, index) => ({
    x: x(times[index]),
    y: y(point.value),
  }));
  const line = smooth(coords);
  const baseline = mode === "pl" ? y(0) : y(0);
  const area = `${line} L${coords[coords.length - 1].x},${baseline} L${coords[0].x},${baseline} Z`;

  const ticks = 4;
  const tickValues = Array.from({ length: ticks + 1 }, (_, index) =>
    min + (index * span) / ticks,
  );

  const labelIndexes = [0, Math.floor((series.length - 1) / 2), series.length - 1]
    .filter((value, index, all) => all.indexOf(value) === index);

  return (
    <svg
      viewBox={`0 0 ${WIDTH} ${HEIGHT}`}
      role="img"
      aria-label={
        mode === "pl"
          ? "Cumulative profit and loss over time"
          : "Cumulative win rate over time"
      }
      className="w-full"
    >
      <defs>
        <linearGradient id={gradientId} x1="0" y1="0" x2="0" y2="1">
          <stop offset="0%" stopColor="var(--turf-400)" stopOpacity="0.28" />
          <stop offset="100%" stopColor="var(--turf-400)" stopOpacity="0" />
        </linearGradient>
      </defs>

      {tickValues.map((value) => (
        <g key={value}>
          <line
            x1={PAD.left}
            x2={WIDTH - PAD.right}
            y1={y(value)}
            y2={y(value)}
            stroke="var(--line)"
            strokeWidth="1"
          />
          <text
            x={PAD.left - 8}
            y={y(value) + 3}
            textAnchor="end"
            fontSize="9"
            fill="var(--mist-500)"
            fontFamily="var(--font-mono)"
          >
            {mode === "pl" ? compact(value) : `${Math.round(value)}%`}
          </text>
        </g>
      ))}

      {mode === "pl" && min < 0 && max > 0 && (
        <line
          x1={PAD.left}
          x2={WIDTH - PAD.right}
          y1={y(0)}
          y2={y(0)}
          stroke="var(--line-strong)"
          strokeWidth="1"
          strokeDasharray="3 4"
        />
      )}

      <path d={area} fill={`url(#${gradientId})`} />
      <path
        d={line}
        fill="none"
        stroke="var(--turf-400)"
        strokeWidth="2"
        strokeLinecap="round"
        strokeLinejoin="round"
      />
      <circle
        cx={coords[coords.length - 1].x}
        cy={coords[coords.length - 1].y}
        r="3"
        fill="var(--ink-950)"
        stroke="var(--turf-300)"
        strokeWidth="2"
      />

      {labelIndexes.map((index) => (
        <text
          key={series[index].day}
          x={coords[index].x}
          y={HEIGHT - 8}
          textAnchor={index === 0 ? "start" : index === series.length - 1 ? "end" : "middle"}
          fontSize="9"
          fill="var(--mist-500)"
          fontFamily="var(--font-mono)"
        >
          {formatDayShort(series[index].day)}
        </text>
      ))}
    </svg>
  );
}

/* ------------------------------------------------------------ win rate ring */

export function WinRateGauge({
  value,
  size = 62,
}: {
  value: number | null;
  size?: number;
}) {
  const stroke = 5;
  const radius = (size - stroke * 2) / 2;
  const circumference = 2 * Math.PI * radius;
  const fraction = value === null ? 0 : Math.min(1, Math.max(0, value));

  return (
    <div className="relative shrink-0" style={{ width: size, height: size }}>
      <svg viewBox={`0 0 ${size} ${size}`} className="size-full -rotate-90">
        <circle
          cx={size / 2}
          cy={size / 2}
          r={radius}
          fill="none"
          stroke="var(--line-strong)"
          strokeWidth={stroke}
        />
        <circle
          cx={size / 2}
          cy={size / 2}
          r={radius}
          fill="none"
          stroke="var(--turf-400)"
          strokeWidth={stroke}
          strokeLinecap="round"
          strokeDasharray={`${circumference * fraction} ${circumference}`}
        />
      </svg>
      <div className="absolute inset-0 grid place-items-center">
        <span className="font-mono text-[11px] font-semibold text-mist-100">
          {value === null ? "-" : `${(value * 100).toFixed(0)}%`}
        </span>
      </div>
    </div>
  );
}
