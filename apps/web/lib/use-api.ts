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
};

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

  const reload = useCallback(() => setNonce((value) => value + 1), []);

  useEffect(() => {
    if (keyRef.current !== key) {
      keyRef.current = key;
      setData(null);
    }

    let cancelled = false;
    setLoading(true);

    loadRef
      .current()
      .then((result) => {
        if (cancelled) return;
        setData(result);
        setError(null);
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

  return { data, error, loading, reload };
}
