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
  faHeartPulse,
  faLock,
  faMagnifyingGlass,
  faTicket,
  faTowerBroadcast,
  faWandMagicSparkles,
  faXmark,
} from "@fortawesome/free-solid-svg-icons";
import { FontAwesomeIcon } from "@fortawesome/react-fontawesome";

import { NotificationCenter } from "@/components/notification-center";
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
    const query = window.matchMedia("(min-width: 1024px)");
    const onChange = () => {
      if (query.matches) setMenuOpen(false);
    };
    query.addEventListener("change", onChange);
    return () => query.removeEventListener("change", onChange);
  }, []);

  return (
    <>
      {/* Desktop: a permanent rail keeps the growing menu out of the content. */}
      <aside
        aria-label="Primary"
        className="fixed inset-y-0 left-0 z-40 hidden w-60 flex-col border-r border-line bg-ink-950/95 backdrop-blur lg:flex"
      >
        <Link href="/" className="flex h-16 items-center gap-2.5 px-5">
          {/* eslint-disable-next-line @next/next/no-img-element */}
          <img
            src="/brand/logo-64.png"
            alt=""
            width={30}
            height={30}
            className="size-[30px] shrink-0 select-none"
          />
          <span className="text-[15px] font-semibold tracking-tight">
            Odds<span className="text-turf-400">Vault</span>
          </span>
        </Link>

        <nav className="flex-1 space-y-0.5 overflow-y-auto px-3 py-3">
          {NAV.map((item) => {
            const active = item.href !== null && isActive(item.href);
            if (!item.href) {
              return (
                <span
                  key={item.label}
                  title="Available in a later phase"
                  className="flex cursor-not-allowed items-center gap-3 rounded-lg px-3 py-2 text-sm font-semibold text-mist-500"
                >
                  <FontAwesomeIcon icon={item.icon} className="size-3.5 opacity-70" />
                  {item.label}
                  <span className="ml-auto font-mono text-[9px] tracking-widest uppercase">
                    soon
                  </span>
                </span>
              );
            }
            return (
              <Link
                key={item.label}
                href={item.href}
                className={`relative flex items-center gap-3 rounded-lg px-3 py-2 text-sm font-semibold transition ${
                  active
                    ? "bg-turf-500/10 text-turf-300"
                    : "text-mist-200 hover:bg-raised hover:text-mist-100"
                }`}
              >
                {active && (
                  <span
                    aria-hidden
                    className="absolute inset-y-1.5 left-0 w-0.5 rounded-full bg-turf-400"
                  />
                )}
                <FontAwesomeIcon icon={item.icon} className="size-3.5 opacity-80" />
                {item.label}
                {item.locked && (
                  <FontAwesomeIcon
                    icon={faLock}
                    className="ml-auto size-2.5 opacity-60"
                  />
                )}
              </Link>
            );
          })}
        </nav>

        <div className="space-y-1 border-t border-line p-3">
          <NotificationCenter variant="sidebar" />
          <Link
            href="/search"
            className={`flex items-center gap-3 rounded-lg px-3 py-2 text-sm font-semibold transition ${
              isActive("/search")
                ? "bg-turf-500/10 text-turf-300"
                : "text-mist-200 hover:bg-raised hover:text-mist-100"
            }`}
          >
            <FontAwesomeIcon icon={faMagnifyingGlass} className="size-3.5 opacity-80" />
            Search
          </Link>
          <div className="flex items-center justify-between rounded-lg px-3 py-1.5">
            <span className="text-sm font-semibold text-mist-200">Theme</span>
            <ThemeToggle />
          </div>
        </div>
      </aside>

      {/* Mobile: the top bar only holds what the rail would hold. */}
      <header className="sticky top-0 z-50 border-b border-line bg-ink-950/80 backdrop-blur lg:hidden">
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

          <div className="flex items-center gap-2">
            <NotificationCenter variant="mobile" />
            <Link
              href="/search"
              aria-label="Search fixtures"
              title="Search fixtures"
              className="grid size-10 place-items-center rounded-lg border border-line text-mist-300 transition hover:bg-raised hover:text-mist-100"
            >
              <FontAwesomeIcon icon={faMagnifyingGlass} style={{ width: 14, height: 14 }} />
            </Link>
            <ThemeToggle />
            <button
              type="button"
              aria-label={menuOpen ? "Close menu" : "Open menu"}
              aria-expanded={menuOpen}
              aria-controls="mobile-menu"
              onClick={() => setMenuOpen((open) => !open)}
              className="grid size-10 place-items-center rounded-lg border border-line-strong text-mist-300 transition hover:bg-raised hover:text-mist-100"
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
        <div className="lg:hidden">
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
                        className={`flex items-center gap-3 rounded-lg px-3 py-2.5 text-sm font-semibold transition ${
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
                        className="flex cursor-not-allowed items-center gap-3 rounded-lg px-3 py-2.5 text-sm font-semibold text-mist-500"
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
              <li>
                <Link
                  href="/search"
                  onClick={() => setMenuOpen(false)}
                  className="flex items-center gap-3 rounded-lg px-3 py-2.5 text-sm font-semibold text-mist-200 hover:bg-raised"
                >
                  <span className="grid size-8 shrink-0 place-items-center rounded-md border border-line bg-raised text-mist-400">
                    <FontAwesomeIcon
                      icon={faMagnifyingGlass}
                      style={{ width: 13, height: 13 }}
                    />
                  </span>
                  <span className="flex-1">Search</span>
                </Link>
              </li>
            </ul>
          </nav>
        </div>
      )}
    </>
  );
}
