"use client";

import { useState } from "react";
import { faFloppyDisk, faSpinner, faXmark } from "@fortawesome/free-solid-svg-icons";
import { FontAwesomeIcon } from "@fortawesome/react-fontawesome";

import { createPaperPick, type PaperPick } from "@/lib/api";

// PaperPickForm records a corner selection by hand. No stake is involved: the
// API freezes the board context with the pick so it can be scored and used as
// training data later.
export function PaperPickForm({
  eventId,
  defaultLine,
  onSaved,
  onCancel,
}: {
  eventId: number;
  defaultLine: number;
  onSaved?: (pick: PaperPick) => void;
  onCancel?: () => void;
}) {
  const [side, setSide] = useState<"over" | "under">("under");
  const [line, setLine] = useState(String(defaultLine));
  const [odds, setOdds] = useState("");
  const [saving, setSaving] = useState(false);
  const [message, setMessage] = useState<string | null>(null);
  const [failed, setFailed] = useState(false);

  async function save() {
    const lineValue = Number(line);
    const oddsValue = Number(odds);
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
      const pick = await createPaperPick({
        eventId,
        side,
        line: lineValue,
        odds: oddsValue,
      });
      setMessage(`Recorded ${pick.side} ${pick.line} at ${pick.odds.toFixed(2)}.`);
      onSaved?.(pick);
    } catch (error) {
      setFailed(true);
      setMessage(error instanceof Error ? error.message : "Could not save the pick.");
    } finally {
      setSaving(false);
    }
  }

  return (
    <div className="flex flex-wrap items-center gap-2">
      <div className="flex overflow-hidden rounded-md border border-line">
        {(["under", "over"] as const).map((option) => (
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
        <input
          type="number"
          step="0.5"
          min="0.5"
          max="40"
          value={line}
          onChange={(event) => setLine(event.target.value)}
          aria-label="Corner line"
          className="w-16 rounded-md border border-line bg-ink-900 px-2 py-1 font-mono text-sm text-mist-100 outline-none focus:border-turf-400/50"
        />
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
        {saving ? "saving" : "record"}
      </button>
      {onCancel ? (
        <button
          type="button"
          onClick={onCancel}
          className="grid size-6 place-items-center rounded-md border border-line text-mist-500 hover:text-mist-200"
          aria-label="Cancel recording"
        >
          <FontAwesomeIcon icon={faXmark} className="size-2.5" />
        </button>
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
