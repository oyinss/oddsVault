"use client";

import {
  faBasketball,
  faFootball,
  faFutbol,
  faTableTennisPaddleBall,
  type IconDefinition,
} from "@fortawesome/free-solid-svg-icons";
import { FontAwesomeIcon } from "@fortawesome/react-fontawesome";

export type SportOption = {
  slug: string;
  label: string;
  short: string;
  icon: IconDefinition;
};

export const SPORTS: SportOption[] = [
  { slug: "football", label: "Football", short: "Football", icon: faFutbol },
  { slug: "tennis", label: "Tennis", short: "Tennis", icon: faTableTennisPaddleBall },
  { slug: "basketball", label: "Basketball", short: "Basketball", icon: faBasketball },
  { slug: "american-football", label: "NFL", short: "NFL", icon: faFootball },
];

export function sportBySlug(slug: string): SportOption {
  return SPORTS.find((sport) => sport.slug === slug) ?? SPORTS[0];
}

export function SportSwitcher({
  value,
  onChange,
}: {
  value: string;
  onChange: (slug: string) => void;
}) {
  return (
    <div
      role="tablist"
      aria-label="Sport"
      className="grid grid-cols-2 gap-2 rounded-2xl border border-line bg-ink-900/70 p-2 sm:grid-cols-4"
    >
      {SPORTS.map((sport) => {
        const active = sport.slug === value;
        return (
          <button
            key={sport.slug}
            role="tab"
            aria-selected={active}
            type="button"
            onClick={() => onChange(sport.slug)}
            className={`flex items-center justify-center gap-2.5 rounded-xl border px-3 py-3 text-sm font-medium transition ${
              active
                ? "border-turf-400/50 bg-turf-500/15 text-turf-200"
                : "border-transparent text-mist-300 hover:border-white/10 hover:bg-white/[0.04]"
            }`}
          >
            <span
              className={`grid size-8 place-items-center rounded-lg ${
                active ? "bg-turf-500/25 text-turf-200" : "bg-white/[0.06] text-mist-400"
              }`}
            >
              <FontAwesomeIcon icon={sport.icon} className="size-4" />
            </span>
            {sport.label}
          </button>
        );
      })}
    </div>
  );
}
