"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { useSearchParams } from "next/navigation";

export interface UseUrlParamsOptions {
  /**
   * Registers a `popstate` listener that re-reads the snapshot from
   * `window.location.search` (browser back/forward, or any history change
   * `useSearchParams()` itself doesn't pick up). Off by default — the two
   * current consumers (Vertretung, Dienstplan) never navigate via the
   * browser's back/forward buttons within the view, only via in-page
   * controls that go through `updateParams`. Planned for the Betreuungsplan
   * (docs/planung-redesign 06 §2.1).
   */
  syncPopstate?: boolean;
}

/** Read-only snapshot of the allowed keys; a key not present in the URL is `null`. */
type UrlParamsSnapshot<K extends string> = Readonly<Record<K, string | null>>;

export interface UseUrlParamsResult<K extends string> {
  params: UrlParamsSnapshot<K>;
  /**
   * Rebuilds the URL from the allowlist and applies `patch` on top, then
   * writes it via `window.history.replaceState` (no history entry per
   * change, no server roundtrip). A key is deleted when its patch value is
   * `null` or `""`; any query param outside `allowedKeys` — present before
   * the call — does NOT survive (allowlist rebuild, not a merge).
   */
  updateParams: (patch: Partial<Record<K, string | null>>) => void;
}

function readSnapshot<K extends string>(
  allowedKeys: readonly K[],
  source: URLSearchParams,
): UrlParamsSnapshot<K> {
  const result = {} as Record<K, string | null>;
  for (const key of allowedKeys) {
    result[key] = source.get(key);
  }
  return result;
}

/**
 * Shared URL query-param state for the Planung views (Vertretung, Dienstplan,
 * Betreuungsplan): a small allowlist of keys, read as a snapshot and written
 * via an allowlist rebuild so params outside the list never survive an
 * update. Extracted from the identical `ALLOWED_URL_PARAMS` /
 * `updateUrlParams` pattern in `vertretung-view.tsx` and
 * `dienstplan-view.tsx` — keep it behaviorally identical to both.
 *
 * SSR-safe: `updateParams` no-ops when `window` is undefined; reading falls
 * back to `useSearchParams()` (which is itself SSR-safe) unless
 * `syncPopstate` is on, in which case the popstate-driven re-read only ever
 * runs client-side (the effect that registers the listener never runs on
 * the server).
 */
export function useUrlParams<K extends string>(
  allowedKeys: readonly K[],
  options: UseUrlParamsOptions = {},
): UseUrlParamsResult<K> {
  const { syncPopstate = false } = options;
  const searchParams = useSearchParams();

  // Bumped on every popstate so the memo below re-reads window.location.search
  // even though `searchParams` itself may not have changed identity.
  const [popstateTick, setPopstateTick] = useState(0);

  useEffect(() => {
    if (!syncPopstate || typeof window === "undefined") return;
    const onPopstate = () => setPopstateTick((tick) => tick + 1);
    window.addEventListener("popstate", onPopstate);
    return () => window.removeEventListener("popstate", onPopstate);
  }, [syncPopstate]);

  const params = useMemo(() => {
    if (syncPopstate && typeof window !== "undefined") {
      return readSnapshot(
        allowedKeys,
        new URLSearchParams(window.location.search),
      );
    }
    return readSnapshot(allowedKeys, searchParams);
    // popstateTick is a deliberate re-read trigger, not a value read here.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [allowedKeys, searchParams, syncPopstate, popstateTick]);

  const updateParams = useCallback(
    (patch: Partial<Record<K, string | null>>) => {
      if (typeof window === "undefined") return;
      const current = new URLSearchParams(window.location.search);
      const next = new URLSearchParams();
      for (const key of allowedKeys) {
        const value = current.get(key);
        if (value !== null) next.set(key, value);
      }
      for (const [key, value] of Object.entries(patch) as Array<
        [string, string | null | undefined]
      >) {
        if (value === null || value === undefined || value === "")
          next.delete(key);
        else next.set(key, value);
      }
      const qs = next.toString();
      window.history.replaceState(
        null,
        "",
        qs ? `?${qs}` : window.location.pathname,
      );
    },
    [allowedKeys],
  );

  return { params, updateParams };
}
