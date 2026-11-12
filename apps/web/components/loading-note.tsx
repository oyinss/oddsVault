/**
 * Honest placeholder while a browser-side read is in flight. It never renders
 * a zero or an empty list that could be mistaken for real data.
 */
export function LoadingNote({
  label = "Loading…",
  className = "mt-6",
}: {
  label?: string;
  className?: string;
}) {
  return (
    <p
      role="status"
      className={`${className} flex items-center gap-2 font-mono text-[11px] tracking-widest text-mist-500 uppercase`}
    >
      <span aria-hidden className="size-1.5 animate-pulse rounded-full bg-turf-400" />
      {label}
    </p>
  );
}
