/** Says out loud that the page re-reads the API on an interval. */
export function RefreshLabel({ seconds }: { seconds: number }) {
  return (
    <span className="font-mono text-[10px] text-mist-500">
      auto-refresh {seconds}s
    </span>
  );
}
