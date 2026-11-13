"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { useEffect, useState } from "react";
import { config, type IconDefinition } from "@fortawesome/fontawesome-svg-core";
import {
  faBars,
  faBullseye,
  faCalendarDays,
  faChartColumn,
  faChartLine,
  faClockRotateLeft,
  faLock,
  faMagnifyingGlass,
  faTicket,
  faTowerBroadcast,
  faHeartPulse,
  faWandMagicSparkles,
  faXmark,
} from "@fortawesome/free-solid-svg-icons";
import { FontAwesomeIcon } from "@fortawesome/react-fontawesome";

import { ThemeToggle } from "@/components/theme-toggle";

// The stylesheet is imported in the root layout; do not inject it twice.
config.autoAddCss = false;

const NAV: {
  label: string;
  href: string | null;
  ready: boolean;
  locked?: boolean;
  icon: IconDefinition;
}[] = [
  { label: "Events", href: "/events", ready: true, icon: faCalendarDays },
  { label: "Live", href: "/live", ready: true, icon: faTowerBroadcast },
  { label: "Corners", href: "/corners", ready: true, icon: faBullseye },
  { label: "Odds", href: "/odds", ready: true, icon: faChartLine },
  { label: "Snapshots", href: "/snapshots", ready: true, icon: faClockRotateLeft },
  { label: "My Bets", href: "/my-bets", ready: true, locked: true, icon: faTicket },
  { label: "Picks", href: "/model-picks", ready: true, icon: faWandMagicSparkles },
  { label: "Analytics", href: "/analytics", ready: true, icon: faChartColumn },
  { label: "Health", href: "/admin/health", ready: true, icon: faHeartPulse },
];

export function SiteHeader() {
  const pathname = usePathname();
  const [menuOpen, setMenuOpen] = useState(false);

  const isActive = (href: string) =>
    href === "/" ? pathname === "/" : pathname === href || pathname.startsWith(`${href}/`);

  // Navigating closes the drawer.
  useEffect(() => {
    setMenuOpen(false);
  }, [pathname]);

  // Lock the page behind the drawer and close it with Escape.
  useEffect(() => {
    if (!menuOpen) return;

    const previousOverflow = document.body.style.overflow;
    document.body.style.overflow = "hidden";

    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key === "Escape") setMenuOpen(false);
    };
    window.addEventListener("keydown", onKeyDown);

    return () => {
      document.body.style.overflow = previousOverflow;
      window.removeEventListener("keydown", onKeyDown);
    };
  }, [menuOpen]);

  // Rotating to a tablet should not leave the mobile drawer open.
  useEffect(() => {
    const query = window.matchMedia("(min-width: 768px)");
    const onChange = () => {
      if (query.matches) setMenuOpen(false);
    };
    query.addEventListener("change", onChange);
    return () => query.removeEventListener("change", onChange);
  }, []);

  return (
    <>
      <header className="sticky top-0 z-50 border-b border-line bg-ink-950/80 backdrop-blur">
        <div className="mx-auto flex h-16 max-w-6xl items-center justify-between gap-4 px-4 sm:px-6">
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
                  aria-label={item.label}
                  title={item.label}
                  className={`relative flex items-center gap-1.5 whitespace-nowrap rounded-md px-2 py-1.5 text-[13px] font-semibold transition hover:bg-raised hover:text-mist-100 xl:px-2.5 xl:text-sm ${
                    isActive(item.href) ? "text-mist-100" : "text-mist-200"
                  }`}
                >
                  <FontAwesomeIcon
                    icon={item.icon}
                    style={{ width: 12, height: 12 }}
                    className="shrink-0 opacity-80"
                  />
                  <span className="hidden xl:inline">{item.label}</span>
                  {item.locked && (
                    <FontAwesomeIcon
                      icon={faLock}
                      style={{ width: 9, height: 9 }}
                      className="shrink-0 opacity-60"
                    />
                  )}
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
                  className="flex cursor-not-allowed items-center gap-1.5 rounded-md px-2 py-1.5 text-[13px] font-semibold whitespace-nowrap text-mist-500 xl:px-2.5 xl:text-sm"
                  title="Available in a later phase"
                >
                  <FontAwesomeIcon
                    icon={item.icon}
                    style={{ width: 12, height: 12 }}
                    className="shrink-0 opacity-60"
                  />
                  {item.label}
                </span>
              ),
            )}
          </nav>

          <div className="flex items-center gap-2">
            <Link
              href="/search"
              aria-label="Search fixtures"
              title="Search fixtures"
              className={`grid size-9 place-items-center rounded-lg border text-mist-300 transition hover:border-line-strong hover:bg-raised hover:text-mist-100 ${
                isActive("/search")
                  ? "border-turf-400/40 bg-turf-500/10 text-turf-300"
                  : "border-line"
              }`}
            >
              <FontAwesomeIcon
                icon={faMagnifyingGlass}
                style={{ width: 13, height: 13 }}
              />
            </Link>
            <ThemeToggle />
            <span className="hidden rounded-full border border-line-strong px-3 py-1 font-mono text-[11px] tracking-widest whitespace-nowrap text-mist-400 uppercase 2xl:inline-flex">
              Phase 7
            </span>
            <button
              type="button"
              aria-label={menuOpen ? "Close menu" : "Open menu"}
              aria-expanded={menuOpen}
              aria-controls="mobile-menu"
              onClick={() => setMenuOpen((open) => !open)}
              className="grid size-10 place-items-center rounded-lg border border-line-strong text-mist-300 transition hover:bg-raised hover:text-mist-100 md:hidden"
            >
              <FontAwesomeIcon
                icon={menuOpen ? faXmark : faBars}
                style={{ width: 15, height: 15 }}
              />
            </button>
          </div>
        </div>
      </header>

      {menuOpen && (
        <div className="md:hidden">
          <button
            type="button"
            aria-label="Close menu"
            tabIndex={-1}
            onClick={() => setMenuOpen(false)}
            className="fixed inset-0 z-40 cursor-default bg-ink-950/60 backdrop-blur-sm"
          />
          <nav
            id="mobile-menu"
            aria-label="Primary"
            className="fixed inset-x-0 top-16 z-40 max-h-[calc(100dvh-4rem)] overflow-y-auto border-b border-line bg-ink-900 shadow-2xl"
          >
            <ul className="mx-auto max-w-6xl space-y-1 p-3">
              {NAV.map((item) => {
                const active = item.href !== null && isActive(item.href);
                const tile = (
                  <span
                    className={`grid size-8 shrink-0 place-items-center rounded-md border ${
                      active
                        ? "border-turf-400/25 bg-turf-500/10 text-turf-300"
                        : "border-line bg-raised text-mist-400"
                    }`}
                  >
                    <FontAwesomeIcon
                      icon={item.icon}
                      style={{ width: 13, height: 13 }}
                    />
                  </span>
                );

                return (
                  <li key={item.label}>
                    {item.href ? (
                      <Link
                        href={item.href}
                        onClick={() => setMenuOpen(false)}
                        className={`flex items-center gap-3 rounded-lg px-3 py-2.5 text-sm transition ${
                          active
                            ? "bg-turf-500/10 text-turf-300"
                            : "text-mist-200 hover:bg-raised"
                        }`}
                      >
                        {tile}
                        <span className="flex-1">{item.label}</span>
                        {active ? (
                          <span className="font-mono text-[9px] tracking-widest text-turf-300 uppercase">
                            here
                          </span>
                        ) : item.locked ? (
                          <span className="flex items-center gap-1 font-mono text-[9px] tracking-widest text-mist-500 uppercase">
                            <FontAwesomeIcon
                              icon={faLock}
                              style={{ width: 8, height: 8 }}
                            />
                            private
                          </span>
                        ) : null}
                      </Link>
                    ) : (
                      <span
                        className="flex cursor-not-allowed items-center gap-3 rounded-lg px-3 py-2.5 text-sm text-mist-500"
                        title="Available in a later phase"
                      >
                        {tile}
                        <span className="flex-1">{item.label}</span>
                        <span className="font-mono text-[9px] tracking-widest uppercase">
                          soon
                        </span>
                      </span>
                    )}
                  </li>
                );
              })}
            </ul>

            <div className="mx-auto flex max-w-6xl items-center justify-between border-t border-line px-5 py-3">
              <span className="font-mono text-[10px] tracking-widest text-mist-500 uppercase">
                Phase 7
              </span>
              <span className="font-mono text-[10px] tracking-widest text-mist-500 uppercase">
                OddsVault
              </span>
            </div>
          </nav>
        </div>
      )}
    </>
  );
}
