import type { ReactNode } from "react";

export type IconName =
  | "wallet"
  | "refresh"
  | "trend"
  | "trophy"
  | "calendar"
  | "book"
  | "ball"
  | "layers"
  | "search"
  | "sort"
  | "check"
  | "x"
  | "clock"
  | "chevron"
  | "info"
  | "spark"
  | "chart"
  | "arrow-up"
  | "arrow-down";

const PATHS: Record<IconName, ReactNode> = {
  wallet: (
    <>
      <path d="M3.5 7.5A2.5 2.5 0 0 1 6 5h11.5A1.5 1.5 0 0 1 19 6.5v11a1.5 1.5 0 0 1-1.5 1.5H6a2.5 2.5 0 0 1-2.5-2.5v-9Z" />
      <path d="M3.5 8h13" />
      <circle cx="15.5" cy="12.5" r="1" />
    </>
  ),
  refresh: (
    <>
      <path d="M4.5 9A7.5 7.5 0 0 1 17 5.8L19 8" />
      <path d="M19 4.5V8h-3.5" />
      <path d="M19.5 15A7.5 7.5 0 0 1 7 18.2L5 16" />
      <path d="M5 19.5V16h3.5" />
    </>
  ),
  trend: (
    <>
      <path d="M4 16.5l4.5-4.5 3 3L20 7" />
      <path d="M20 7h-4.5M20 7v4.5" />
    </>
  ),
  trophy: (
    <>
      <path d="M8 4.5h8V9a4 4 0 0 1-8 0V4.5Z" />
      <path d="M8 5.5H5.5V7a3 3 0 0 0 2.5 3" />
      <path d="M16 5.5h2.5V7a3 3 0 0 1-2.5 3" />
      <path d="M12 13v3.5M9 20h6M10 16.5h4V20h-4z" />
    </>
  ),
  calendar: (
    <>
      <rect x="3.5" y="5" width="17" height="15.5" rx="2" />
      <path d="M3.5 9.5h17M8 3.5v3M16 3.5v3" />
    </>
  ),
  book: (
    <>
      <path d="M5 4.5A1.5 1.5 0 0 1 6.5 3H19v18H6.5A1.5 1.5 0 0 1 5 19.5v-15Z" />
      <path d="M8 3v18M11 7.5h5M11 11h5" />
    </>
  ),
  ball: (
    <>
      <circle cx="12" cy="12" r="8.5" />
      <path d="M12 7.2l2.8 2-1 3.3h-3.6l-1-3.3 2.8-2Z" />
      <path d="M12 3.5v3.7M4.4 9.6l4.3 2.9M19.6 9.6l-4.3 2.9M7.7 20.3l1.9-5.2M16.3 20.3l-1.9-5.2" />
    </>
  ),
  layers: (
    <>
      <path d="M12 3.5 20 8l-8 4.5L4 8l8-4.5Z" />
      <path d="M4 12l8 4.5 8-4.5" />
      <path d="M4 16l8 4.5 8-4.5" />
    </>
  ),
  search: (
    <>
      <circle cx="11" cy="11" r="6.5" />
      <path d="M16 16l4.5 4.5" />
    </>
  ),
  sort: (
    <>
      <path d="M8 5v14M8 19l-3-3M8 19l3-3" />
      <path d="M16 19V5M16 5l-3 3M16 5l3 3" />
    </>
  ),
  check: (
    <>
      <circle cx="12" cy="12" r="8.5" />
      <path d="M8.2 12.3l2.6 2.6 5-5.4" />
    </>
  ),
  x: (
    <>
      <circle cx="12" cy="12" r="8.5" />
      <path d="M9 9l6 6M15 9l-6 6" />
    </>
  ),
  clock: (
    <>
      <circle cx="12" cy="12" r="8.5" />
      <path d="M12 7.5V12l3 2" />
    </>
  ),
  chevron: <path d="M6 9l6 6 6-6" />,
  info: (
    <>
      <circle cx="12" cy="12" r="8.5" />
      <path d="M12 11v5M12 8h.01" />
    </>
  ),
  spark: (
    <path d="M12 3.5l1.7 4.8L18.5 10l-4.8 1.7L12 16.5l-1.7-4.8L5.5 10l4.8-1.7L12 3.5Z" />
  ),
  chart: (
    <>
      <path d="M4 19V5M4 19h16" />
      <path d="M8 15l3-4 3 2 5-6" />
    </>
  ),
  "arrow-up": <path d="M12 19V5M12 5l-5 5M12 5l5 5" />,
  "arrow-down": <path d="M12 5v14M12 19l-5-5M12 19l5-5" />,
};

export function Icon({
  name,
  className = "size-4",
  strokeWidth = 1.6,
}: {
  name: IconName;
  className?: string;
  strokeWidth?: number;
}) {
  return (
    <svg
      viewBox="0 0 24 24"
      aria-hidden
      fill="none"
      stroke="currentColor"
      strokeWidth={strokeWidth}
      strokeLinecap="round"
      strokeLinejoin="round"
      className={className}
    >
      {PATHS[name]}
    </svg>
  );
}
