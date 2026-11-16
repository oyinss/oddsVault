"use client";

import { useCallback, useEffect, useRef, useState } from "react";

export type ApiState<T> = {
  /** Last successful payload. `null` until the first request resolves. */
  data: T | null;
  /** Message from the most recent failed request, cleared on success. */
  error: string | null;
  /** True while a request is in flight (including background refreshes). */
  loading: boolean;
  /** Re-run the loader now. Stable identity, safe as an effect dependency. */
  reload: () => void;
  /**
   * When the payload on screen came from the browser cache because the live
   * request failed. Null while the data is fresh from the API.
   */
  cachedAt: string | null;
};

const CACHE_PREFIX = "oddsvault:cache:";

type CachedPayload<T> = { at: string; data: T };

function readCache<T>(key: string): CachedPayload<T> | null {
  try {
    const raw = window.localStorage.getItem(CACHE_PREFIX + key);
    if (!raw) return null;
    const parsed = JSON.parse(raw) as CachedPayload<T>;
    if (!parsed || typeof parsed.at !== "string") return null;
    return parsed;
  } catch {
    return null;
  }
}

function writeCache<T>(key: string, data: T) {
  if (data === null || data === undefined) return;
  try {
    window.localStorage.setItem(
      CACHE_PREFIX + key,
      JSON.stringify({ at: new Date().toISOString(), data }),
    );
  } catch {
    // A full or disabled store only costs the fallback, never the page.
  }
}

/**
 * Runs a browser-side API read.
 *
 * Every page is statically rendered, so data is loaded after hydration instead
 * of during the build: a snapshot captured at build time would be stale by the
 * time anyone saw it, which is exactly what this product refuses to show.
 *
 * `key` identifies the request. Changing it clears the previous payload so a
 * different fixture never flashes its neighbour's numbers.
 */
export function useApiData<T>(
  key: string,
  load: () => Promise<T>,
  options: { refreshMs?: number } = {},
): ApiState<T> {
  const loadRef = useRef(load);
  loadRef.current = load;

  const keyRef = useRef(key);
  const [data, setData] = useState<T | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);
  const [nonce, setNonce] = useState(0);
  const [cachedAt, setCachedAt] = useState<string | null>(null);
  const hasDataRef = useRef(false);

  const reload = useCallback(() => setNonce((value) => value + 1), []);

  useEffect(() => {
    if (keyRef.current !== key) {
      keyRef.current = key;
      hasDataRef.current = false;
      setData(null);
      setCachedAt(null);
    }

    // A reload during an outage should still show the last board we had,
    // labelled as cached, instead of an empty error page.
    if (!hasDataRef.current) {
      const cached = readCache<T>(key);
      if (cached) {
        hasDataRef.current = true;
        setData(cached.data);
        setCachedAt(cached.at);
      }
    }

    let cancelled = false;
    setLoading(true);

    loadRef
      .current()
      .then((result) => {
        if (cancelled) return;
        hasDataRef.current = true;
        setData(result);
        setCachedAt(null);
        setError(null);
        writeCache(key, result);
      })
      .catch((cause: unknown) => {
        if (cancelled) return;
        setError(cause instanceof Error ? cause.message : "unreachable");
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });

    return () => {
      cancelled = true;
    };
  }, [key, nonce]);

  useEffect(() => {
    if (!options.refreshMs) return;
    const timer = setInterval(reload, options.refreshMs);
    return () => clearInterval(timer);
  }, [options.refreshMs, reload]);

  return { data, error, loading, reload, cachedAt };
}
