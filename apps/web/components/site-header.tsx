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
          {/* eslint-disable-next-line @next/next/no-img-element */}
          <img
            src="/brand/logo-64.png"
            alt=""
            width={32}
            height={32}
            className="size-8 shrink-0 select-none"
          />
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
