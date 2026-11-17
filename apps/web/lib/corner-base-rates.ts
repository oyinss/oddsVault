// Neutral corner base rates from 338 finished matches with final counts, the
// same numbers the line guide prints on the board.
//
// These are match-level rates, not in-play probabilities: they are the anchor a
// live read has to beat, and the price tells you by how much. A line with no
// measured base returns null rather than an interpolated guess.
const BASE_OVER_PERCENT: Record<string, number> = {
  "8.5": 61,
  "9.5": 48,
  "10.5": 38,
  "11.5": 30,
  "12.5": 22,
  "13.5": 16,
};

/** Neutral rate for the side and line being played, or null when unmeasured. */
export function baseRatePercent(
  side: "over" | "under",
  line: number,
): number | null {
  const over = BASE_OVER_PERCENT[line.toFixed(1)];
  if (over === undefined) return null;
  return side === "under" ? 100 - over : over;
}

/** What the price demands: the break-even strike rate for this odds. */
export function impliedPercent(odds: number): number | null {
  if (!Number.isFinite(odds) || odds <= 1) return null;
  return Math.round((100 / odds) * 10) / 10;
}

/**
 * Points the price asks for above the neutral base. Positive means the price is
 * demanding more than the base rate, so the live read has to make up the gap.
 */
export function priceGapPoints(
  side: "over" | "under",
  line: number,
  odds: number,
): number | null {
  const base = baseRatePercent(side, line);
  const implied = impliedPercent(odds);
  if (base === null || implied === null) return null;
  return Math.round((implied - base) * 10) / 10;
}
