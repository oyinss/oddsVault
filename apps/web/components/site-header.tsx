"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";

import { ThemeToggle } from "@/components/theme-toggle";

const NAV = [
  { label: "Events", href: "/events", ready: true },
  { label: "Live", href: "/live", ready: true },
  { label: "Odds", href: null, ready: false },
  { label: "Snapshots", href: "/snapshots", ready: true },
  { label: "My Bets", href: "/my-bets", ready: true },
  { label: "Analytics", href: "/analytics", ready: true },
];

export function SiteHeader() {
  const pathname = usePathname();
  const isActive = (href: string) =>
    href === "/" ? pathname === "/" : pathname === href || pathname.startsWith(`${href}/`);

  return (
    <header className="sticky top-0 z-20 border-b border-line bg-ink-950/80 backdrop-blur">
      <div className="mx-auto flex h-16 max-w-6xl items-center justify-between gap-6 px-6">
        <Link href="/" className="flex items-center gap-2.5">
          <span
            aria-hidden
            className="grid size-8 place-items-center rounded-lg bg-turf-500/15 ring-1 ring-turf-400/30"
          >
            <svg viewBox="0 0 24 24" className="size-4 text-turf-300" fill="none">
              <path
                d="M12 3.5 20 7v5.2c0 4.4-3.2 7.6-8 8.3-4.8-.7-8-3.9-8-8.3V7l8-3.5Z"
                stroke="currentColor"
                strokeWidth="1.6"
              />
              <path
                d="M8.4 12.2l2.5 2.5 4.7-5"
                stroke="currentColor"
                strokeWidth="1.6"
                strokeLinecap="round"
                strokeLinejoin="round"
              />
            </svg>
          </span>
          <span className="text-[15px] font-semibold tracking-tight">
            Odds<span className="text-turf-400">Vault</span>
          </span>
        </Link>

        <nav aria-label="Primary" className="hidden items-center gap-1 md:flex">
          {NAV.map((item) =>
            item.href ? (
              <Link
                key={item.label}
                href={item.href}
                className={`relative rounded-md px-3 py-1.5 text-sm transition hover:bg-raised hover:text-mist-100 ${
                  isActive(item.href) ? "text-mist-100" : "text-mist-200"
                }`}
              >
                {item.label}
                {isActive(item.href) && (
                  <span
                    aria-hidden
                    className="absolute inset-x-2 -bottom-1.5 h-px bg-turf-400"
                  />
                )}
              </Link>
            ) : (
              <span
                key={item.label}
                className="flex cursor-not-allowed items-center gap-1.5 rounded-md px-3 py-1.5 text-sm text-mist-500"
                title="Available in a later phase"
              >
                {item.label}
              </span>
            ),
          )}
        </nav>

        <div className="flex items-center gap-2">
          <ThemeToggle />
          <span className="rounded-full border border-line-strong px-3 py-1 font-mono text-[11px] tracking-widest text-mist-400 uppercase">
            Phase 7
          </span>
        </div>
      </div>

      <div className="border-t border-line md:hidden">
        <nav
          aria-label="Primary"
          className="mx-auto flex max-w-6xl items-center gap-1 overflow-x-auto px-4 py-2"
        >
          {NAV.map((item) =>
            item.href ? (
              <Link
                key={item.label}
                href={item.href}
                className={`shrink-0 rounded-md px-2.5 py-1 text-[12px] transition ${
                  isActive(item.href)
                    ? "bg-turf-500/10 text-turf-300"
                    : "text-mist-300 hover:bg-raised"
                }`}
              >
                {item.label}
              </Link>
            ) : (
              <span
                key={item.label}
                className="shrink-0 cursor-not-allowed px-2.5 py-1 text-[12px] text-mist-500"
                title="Available in a later phase"
              >
                {item.label}
              </span>
            ),
          )}
        </nav>
      </div>
    </header>
  );
}
