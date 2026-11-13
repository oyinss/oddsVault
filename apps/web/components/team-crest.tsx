"use client";

import { useState } from "react";

/**
 * Team badge with an honest fallback.
 *
 * When no artwork exists the row still needs a readable identifier, so the
 * initials sit on a colour derived from the team name. It is a monogram, not a
 * pretend club logo: the same team always gets the same colour, and two teams in
 * one match never look alike by accident.
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
    const hue = hueFor(name);
    return (
      <span
        aria-hidden
        className="inline-flex shrink-0 items-center justify-center rounded-md font-semibold tracking-wide text-white"
        style={{
          width: size,
          height: size,
          fontSize: Math.max(8, size * 0.36),
          backgroundColor: `hsl(${hue} 42% 34%)`,
          boxShadow: `inset 0 0 0 1px hsl(${hue} 45% 58% / 0.35)`,
        }}
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

/** Stable hue per team name, so the colour is part of the identity. */
function hueFor(name: string): number {
  let hash = 0;
  for (let index = 0; index < name.length; index += 1) {
    hash = (hash * 31 + name.charCodeAt(index)) % 360;
  }
  return hash;
}
