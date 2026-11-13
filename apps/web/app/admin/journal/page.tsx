"use client";

import Link from "next/link";
import { useState } from "react";

import { SiteHeader } from "@/components/site-header";
import { API_BASE_URL } from "@/lib/api";

type ImportResult = {
  ok: boolean;
  bets?: number;
  output?: string;
  error?: string;
};

export default function JournalImportPage() {
  const [files, setFiles] = useState<File[]>([]);
  const [json, setJson] = useState("");
  const [busy, setBusy] = useState(false);
  const [result, setResult] = useState<ImportResult | null>(null);

  async function uploadScreenshots() {
    if (files.length === 0) return;
    setBusy(true);
    setResult(null);
    try {
      const form = new FormData();
      for (const file of files) form.append("files", file);
      const res = await fetch(
        `${API_BASE_URL}/api/v1/admin/journal/import-screenshots`,
        { method: "POST", body: form },
      );
      setResult((await res.json()) as ImportResult);
    } catch (error) {
      setResult({ ok: false, error: String(error) });
    } finally {
      setBusy(false);
    }
  }

  async function importJson() {
    setBusy(true);
    setResult(null);
    try {
      const res = await fetch(`${API_BASE_URL}/api/v1/admin/journal/import-json`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: json,
      });
      setResult((await res.json()) as ImportResult);
    } catch (error) {
      setResult({ ok: false, error: String(error) });
    } finally {
      setBusy(false);
    }
  }

  return (
    <>
      <SiteHeader />
      <main className="mx-auto max-w-3xl px-6 py-12">
        <p className="font-mono text-[11px] tracking-widest text-turf-300 uppercase">
          Journal · Import
        </p>
        <h1 className="mt-2 text-3xl font-bold tracking-tight">
          Add bets
        </h1>
        <p className="mt-2 max-w-xl text-sm leading-relaxed text-mist-400">
          Upload the bookmaker&apos;s bet-detail screenshots and the OCR pass
          will read them. Re-importing is idempotent; nothing is overwritten
          silently and unreadable numbers are skipped, never guessed.
        </p>

        <section className="panel mt-8 p-5">
          <h2 className="text-sm font-semibold">Screenshots</h2>
          <p className="mt-1 text-xs text-mist-500">
            JPG or PNG, up to 32 MiB total. Each screenshot should be a bet
            detail page with the slip number, stake, odds and leg list visible.
          </p>

          <input
            type="file"
            accept="image/jpeg,image/png"
            multiple
            onChange={(event) => setFiles(Array.from(event.target.files ?? []))}
            className="mt-4 block w-full cursor-pointer rounded-lg border border-white/10 bg-ink-950 px-3 py-2 text-sm text-mist-300 file:mr-3 file:cursor-pointer file:rounded file:border-0 file:bg-white/10 file:px-3 file:py-1.5 file:text-xs file:text-mist-100"
          />

          {files.length > 0 && (
            <p className="mt-2 font-mono text-[11px] text-mist-500">
              {files.length} file(s) selected
            </p>
          )}

          <button
            type="button"
            onClick={uploadScreenshots}
            disabled={busy || files.length === 0}
            className="mt-4 rounded-lg bg-turf-500 px-4 py-2 text-sm font-semibold text-ink-950 transition hover:bg-turf-400 disabled:cursor-not-allowed disabled:opacity-40"
          >
            {busy ? "Reading screenshots…" : "Import screenshots"}
          </button>
        </section>

        <section className="panel mt-4 p-5">
          <h2 className="text-sm font-semibold">JSON export</h2>
          <p className="mt-1 text-xs text-mist-500">
            Paste an export produced elsewhere (same schema as{" "}
            <code className="font-mono">make import-bets</code>).
          </p>
          <textarea
            value={json}
            onChange={(event) => setJson(event.target.value)}
            rows={8}
            spellCheck={false}
            className="mt-3 w-full rounded-lg border border-white/10 bg-ink-950 p-3 font-mono text-xs text-mist-200 outline-none focus:border-turf-400/50"
            placeholder='{"bookmaker":"1xbet","currency":"NGN","source":"...","bets":[]}'
          />
          <button
            type="button"
            onClick={importJson}
            disabled={busy || json.trim() === ""}
            className="mt-3 rounded-lg border border-white/10 px-4 py-2 text-sm text-mist-200 transition hover:border-white/25 hover:bg-white/5 disabled:cursor-not-allowed disabled:opacity-40"
          >
            {busy ? "Importing…" : "Import JSON"}
          </button>
        </section>

        {result && (
          <section
            className={`panel mt-4 p-5 ${
              result.ok ? "border-turf-400/30" : "border-flag-400/30"
            }`}
          >
            <p className="text-sm font-medium">
              {result.ok ? "Import finished" : "Import failed"}
            </p>
            {result.bets !== undefined && (
              <p className="mt-1 text-xs text-mist-400">
                {result.bets} slip(s) parsed
              </p>
            )}
            <pre className="mt-3 overflow-x-auto rounded-lg border border-white/[0.06] bg-ink-950 p-3 font-mono text-[11px] whitespace-pre-wrap text-mist-300">
              {result.error ?? result.output ?? "no output"}
            </pre>
            {result.ok && (
              <Link
                href="/my-bets"
                className="mt-4 inline-flex rounded-lg bg-turf-500 px-3 py-1.5 text-xs font-semibold text-ink-950"
              >
                Open My Bets
              </Link>
            )}
          </section>
        )}
      </main>
    </>
  );
}
