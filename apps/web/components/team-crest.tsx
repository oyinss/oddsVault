"use client";

import { useState } from "react";

/**
 * Team badge with an honest fallback: initials in a neutral tile, never a
 * placeholder logo that pretends to be the club.
 */
export function TeamCrest({
  src,
  name,
  size = 20,
}: {
  src: string | null;
  name: string;
  size?: number;
}) {
  const [failed, setFailed] = useState(false);
  const initials = name
    .split(/\s+/)
    .filter(Boolean)
    .slice(0, 2)
    .map((word) => word[0]?.toUpperCase() ?? "")
    .join("");

  if (!src || failed) {
    return (
      <span
        aria-hidden
        className="inline-flex shrink-0 items-center justify-center rounded-md border border-line-strong bg-raised font-mono text-mist-400"
        style={{ width: size, height: size, fontSize: Math.max(8, size * 0.36) }}
      >
        {initials || "?"}
      </span>
    );
  }

  return (
    // eslint-disable-next-line @next/next/no-img-element
    <img
      src={src}
      alt=""
      width={size}
      height={size}
      onError={() => setFailed(true)}
      className="shrink-0 object-contain"
      style={{ width: size, height: size }}
    />
  );
}
