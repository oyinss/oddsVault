"use client";

import { useEffect, useState } from "react";
import { faFloppyDisk, faSpinner, faXmark } from "@fortawesome/free-solid-svg-icons";
import { FontAwesomeIcon } from "@fortawesome/react-fontawesome";

import {
  baseRatePercent,
  impliedPercent,
  priceGapPoints,
} from "@/lib/corner-base-rates";
import {
  createPaperPick,
  getPaperPickProviders,
  updatePaperPick,
  type BookmakerOption,
  type PaperPick,
} from "@/lib/api";

// Corner lines the books actually hang. A free number box invited typos like
// 9.4, which settlement would then grade against a line that never existed.
const LINE_OPTIONS = Array.from({ length: 24 }, (_, index) =>
  (5.5 + index * 0.5).toFixed(1),
);

// PaperPickForm records a corner selection by hand. No stake is involved: the
// API freezes the board context with the pick so it can be scored and used as
// training data later.
export function PaperPickForm({
  eventId,
  defaultLine,
  recOverLine,
  recUnderLine,
  projectedTotal,
  editing,
  onSaved,
  onCancel,
}: {
  eventId: number;
  defaultLine: number;
  // The board's own recommendation, so the form opens on the read the card is
  // showing instead of the generic board line.
  recOverLine?: number | null;
  recUnderLine?: number | null;
  projectedTotal?: number | null;
  editing?: {
    id: number;
    provider_slug: string;
    side: "over" | "under";
    line: number;
    odds: number;
  } | null;
  onSaved?: (pick: PaperPick, message: string) => void;
  onCancel?: () => void;
}) {
  const [providers, setProviders] = useState<BookmakerOption[]>([]);
  const [provider, setProvider] = useState(editing?.provider_slug ?? "1xbet");
  const [side, setSide] = useState<"over" | "under">(
    editing?.side ??
      (recOverLine != null ? "over" : recUnderLine != null ? "under" : "under"),
  );
  const [line, setLine] = useState(
    (
      editing?.line ??
      recOverLine ??
      recUnderLine ??
      defaultLine
    ).toFixed(1),
  );
  const [odds, setOdds] = useState(
    editing?.odds != null ? String(editing.odds) : "",
  );
  const [saving, setSaving] = useState(false);
  const [message, setMessage] = useState<string | null>(null);
  const [failed, setFailed] = useState(false);

  useEffect(() => {
    let active = true;
    void getPaperPickProviders().then((list) => {
      if (!active || list.length === 0) return;
      setProviders(list);
      setProvider((current) =>
        list.some((option) => option.slug === current) ? current : list[0].slug,
      );
    });
    return () => {
      active = false;
    };
  }, []);

  async function save() {
    const lineValue = Number(line);
    const oddsValue = Number(odds);
    if (!provider) {
      setFailed(true);
      setMessage("Choose the bookmaker that offered the price.");
      return;
    }
    if (!Number.isFinite(lineValue) || lineValue <= 0) {
      setFailed(true);
      setMessage("Enter the corner line.");
      return;
    }
    if (!Number.isFinite(oddsValue) || oddsValue <= 1) {
      setFailed(true);
      setMessage("Enter the odds you would have taken (above 1).");
      return;
    }
    setSaving(true);
    setFailed(false);
    setMessage(null);
    try {
      const pick = editing
        ? await updatePaperPick(editing.id, {
            provider,
            side,
            line: lineValue,
            odds: oddsValue,
          })
        : await createPaperPick({
            eventId,
            provider,
            side,
            line: lineValue,
            odds: oddsValue,
          });
      const confirmation = `${editing ? "Updated" : "Recorded"} ${pick.side} ${pick.line} at ${pick.odds.toFixed(2)} on ${pick.provider}.`;
      setMessage(confirmation);
      onSaved?.(pick, confirmation);
    } catch (error) {
      setFailed(true);
      setMessage(error instanceof Error ? error.message : "Could not save the pick.");
    } finally {
      setSaving(false);
    }
  }

  const oddsValue = Number(odds);
  const lineValue = Number(line);
  const implied = impliedPercent(oddsValue);
  const base = Number.isFinite(lineValue)
    ? baseRatePercent(side, lineValue)
    : null;
  const gap =
    Number.isFinite(lineValue) && Number.isFinite(oddsValue)
      ? priceGapPoints(side, lineValue, oddsValue)
      : null;
  // The same test the card uses: a pick is against the read when it sits on the
  // wrong side of the live projection.
  const againstRead =
    projectedTotal != null &&
    Number.isFinite(lineValue) &&
    (side === "over" ? projectedTotal <= lineValue : projectedTotal >= lineValue);
  const followedLine =
    (side === "over" && recOverLine != null && lineValue === recOverLine) ||
    (side === "under" && recUnderLine != null && lineValue === recUnderLine);

  return (
    <div className="flex flex-wrap items-center gap-2">
      <label className="flex items-center gap-1.5 font-mono text-[10px] text-mist-500 uppercase">
        book
        <select
          value={provider}
          onChange={(event) => setProvider(event.target.value)}
          aria-label="Bookmaker"
          className="rounded-md border border-line bg-ink-900 px-2 py-1 font-mono text-xs text-mist-100 outline-none focus:border-turf-400/50"
        >
          {providers.length === 0 ? (
            <option value="">loading…</option>
          ) : (
            providers.map((option) => (
              <option key={option.slug} value={option.slug}>
                {option.name}
              </option>
            ))
          )}
        </select>
      </label>
      <div className="flex overflow-hidden rounded-md border border-line">
        {(["over", "under"] as const).map((option) => (
          <button
            key={option}
            type="button"
            onClick={() => setSide(option)}
            className={`px-2.5 py-1 font-mono text-[10px] tracking-widest uppercase transition ${
              side === option
                ? "bg-turf-500/15 text-turf-200"
                : "text-mist-400 hover:text-mist-100"
            }`}
          >
            {option}
          </button>
        ))}
      </div>
      <label className="flex items-center gap-1.5 font-mono text-[10px] text-mist-500 uppercase">
        line
        <select
          value={line}
          onChange={(event) => setLine(event.target.value)}
          aria-label="Corner line"
          className="rounded-md border border-line bg-ink-900 px-2 py-1 font-mono text-xs text-mist-100 outline-none focus:border-turf-400/50"
        >
          {!LINE_OPTIONS.includes(line) ? (
            <option value={line}>{line}</option>
          ) : null}
          {LINE_OPTIONS.map((option) => (
            <option key={option} value={option}>
              {option}
            </option>
          ))}
        </select>
      </label>
      <label className="flex items-center gap-1.5 font-mono text-[10px] text-mist-500 uppercase">
        odds
        <input
          type="number"
          step="0.01"
          min="1.01"
          value={odds}
          onChange={(event) => setOdds(event.target.value)}
          placeholder="1.85"
          aria-label="Odds taken"
          className="w-20 rounded-md border border-line bg-ink-900 px-2 py-1 font-mono text-sm text-mist-100 outline-none focus:border-turf-400/50"
        />
      </label>
      <button
        type="button"
        onClick={() => void save()}
        disabled={saving}
        className="inline-flex items-center gap-1.5 rounded-md border border-turf-400/40 bg-turf-500/10 px-2.5 py-1 font-mono text-[10px] tracking-widest text-turf-200 uppercase transition hover:bg-turf-500/20 disabled:opacity-60"
      >
        <FontAwesomeIcon
          icon={saving ? faSpinner : faFloppyDisk}
          className={`size-2.5 ${saving ? "animate-spin" : ""}`}
        />
        {saving ? "saving" : editing ? "save changes" : "record"}
      </button>
      {onCancel ? (
        <button
          type="button"
          onClick={onCancel}
          className="grid size-6 place-items-center rounded-md border border-line text-mist-500 hover:text-mist-200"
          aria-label={editing ? "Cancel editing" : "Cancel recording"}
        >
          <FontAwesomeIcon icon={faXmark} className="size-2.5" />
        </button>
      ) : null}
      <span className="basis-full text-[10px] text-mist-400">
        {implied !== null ? (
          <>
            <span className="text-mist-200">price asks {implied}%</span>
            {base !== null ? (
              <>
                {" "}
                · neutral base at {side} {line} is{" "}
                <span className="text-mist-200">{base}%</span> ·
                {gap !== null && gap > 0 ? (
                  <span className="text-flag-400">
                    {" "}
                    the read must find {gap} points
                  </span>
                ) : (
                  <span className="text-turf-300"> the price is on your side</span>
                )}
              </>
            ) : (
              <span> · no base rate measured at {line}</span>
            )}
          </>
        ) : (
          <>Enter the odds you can get to see what the price asks for.</>
        )}
      </span>
      {againstRead ? (
        <span className="basis-full text-[10px] text-flag-400">
          This is against the read: the projection is {projectedTotal?.toFixed(1)}
          , on the other side of {line}. Two of your four losses came from picks
          like this.
        </span>
      ) : followedLine ? (
        <span className="basis-full text-[10px] text-turf-300">
          This is the line the card recommends.
        </span>
      ) : null}
      {message ? (
        <span
          className={`text-[10px] ${failed ? "text-flag-400" : "text-turf-300"}`}
        >
          {message}
        </span>
      ) : null}
    </div>
  );
}
