"use client";

import { useEffect, useState } from "react";

type Mode = "light" | "dark" | "system";

const STORAGE_KEY = "oddsvault-theme";

const OPTIONS: { mode: Mode; label: string; icon: React.ReactNode }[] = [
  {
    mode: "light",
    label: "Light",
    icon: (
      <>
        <circle cx="12" cy="12" r="3.6" />
        <path d="M12 2.6v2.1M12 19.3v2.1M4.3 4.3l1.5 1.5M18.2 18.2l1.5 1.5M2.6 12h2.1M19.3 12h2.1M4.3 19.7l1.5-1.5M18.2 5.8l1.5-1.5" />
      </>
    ),
  },
  {
    mode: "dark",
    label: "Dark",
    icon: <path d="M20 13.4A8.1 8.1 0 1 1 10.6 4a6.6 6.6 0 0 0 9.4 9.4Z" />,
  },
  {
    mode: "system",
    label: "System",
    icon: (
      <>
        <rect x="3.4" y="4.6" width="17.2" height="11.6" rx="1.6" />
        <path d="M9 19.8h6M12 16.2v3.6" />
      </>
    ),
  },
];

function prefersDark(): boolean {
  return window.matchMedia("(prefers-color-scheme: dark)").matches;
}

/** Applies a mode to <html>. The pre-paint script in the layout does the same. */
export function applyThemeMode(mode: Mode) {
  const dark = mode === "dark" || (mode === "system" && prefersDark());
  const root = document.documentElement;
  root.classList.toggle("light", !dark);
  root.style.colorScheme = dark ? "dark" : "light";
}

export function ThemeToggle() {
  const [mode, setMode] = useState<Mode | null>(null);

  // The server cannot know the stored choice; adopt it once hydrated. Until
  // then no button is marked active and the pre-paint script's choice stands.
  useEffect(() => {
    const override = new URLSearchParams(window.location.search).get("theme");
    if (override === "light" || override === "dark") {
      setMode(override);
      return;
    }
    const stored = window.localStorage.getItem(STORAGE_KEY);
    if (stored === "light" || stored === "dark" || stored === "system") {
      setMode(stored);
    } else {
      setMode("system");
    }
  }, []);

  useEffect(() => {
    if (mode === null) return;
    applyThemeMode(mode);
    if (mode !== "system") return;

    const query = window.matchMedia("(prefers-color-scheme: dark)");
    const onChange = () => applyThemeMode("system");
    query.addEventListener("change", onChange);
    return () => query.removeEventListener("change", onChange);
  }, [mode]);

  const choose = (next: Mode) => {
    window.localStorage.setItem(STORAGE_KEY, next);
    setMode(next);
    applyThemeMode(next);
  };

  return (
    <div
      role="group"
      aria-label="Colour theme"
      className="flex items-center gap-0.5 rounded-full border border-line bg-ink-950/40 p-0.5"
    >
      {OPTIONS.map((option) => {
        const active = mode === option.mode;
        return (
          <button
            key={option.mode}
            type="button"
            onClick={() => choose(option.mode)}
            aria-pressed={active}
            title={option.label}
            aria-label={`${option.label} theme`}
            className={`grid size-6 place-items-center rounded-full transition ${
              active
                ? "bg-turf-500/15 text-turf-300"
                : "text-mist-500 hover:text-mist-200"
            }`}
          >
            <svg
              viewBox="0 0 24 24"
              aria-hidden
              fill="none"
              stroke="currentColor"
              strokeWidth="1.6"
              strokeLinecap="round"
              strokeLinejoin="round"
              className="size-3.5"
            >
              {option.icon}
            </svg>
          </button>
        );
      })}
    </div>
  );
}
