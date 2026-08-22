"use client";

import {
  useCallback,
  useEffect,
  useLayoutEffect,
  useRef,
  useState,
} from "react";

/**
 * Shared unread-badge hook: a single localStorage-cache + concurrent-fetch +
 * refresh-event implementation parameterized by a fetcher, so every sidebar
 * badge stops forking the same ~90 lines. Used by the staff-messages, parent-
 * messages badges. (The operator badge keeps its own hook: it
 * caches a two-field breakdown and listens to two events, a contract this
 * single-count hook deliberately does not model.)
 */

const CACHE_DURATION_MS = 60 * 1000; // 1 minute cache

// Client components can still render on the server. Use a layout effect in the
// browser so tenant guards advance before paint, without triggering the server
// warning from useLayoutEffect.
const useCommitEffect =
  typeof window !== "undefined" ? useLayoutEffect : useEffect;

function useCommittedLatest<T>(value: T) {
  const ref = useRef(value);

  useCommitEffect(() => {
    ref.current = value;
  }, [value]);

  return ref;
}

interface CachedData {
  count: number;
  timestamp: number;
}

function readCache(key: string): number | null {
  if (typeof window === "undefined") return null;
  try {
    const cached = localStorage.getItem(key);
    if (!cached) return null;
    const data = JSON.parse(cached) as CachedData;
    if (Date.now() - data.timestamp > CACHE_DURATION_MS) {
      localStorage.removeItem(key);
      return null;
    }
    return data.count;
  } catch {
    return null;
  }
}

function writeCache(key: string, count: number): void {
  if (typeof window === "undefined") return;
  try {
    const data: CachedData = { count, timestamp: Date.now() };
    localStorage.setItem(key, JSON.stringify(data));
  } catch {
    // Ignore localStorage errors
  }
}

export interface UseUnreadCountOptions {
  /** Only fetch and attach listeners when true (auth/portal-mode gate). */
  enabled: boolean;
  /** Returns the current unread count. May be a fresh closure each render. */
  fetcher: () => Promise<number>;
  /** localStorage cache key; omit to skip the 1-minute cache layer entirely. */
  cacheKey?: string;
  /** Custom window events that trigger a cache-busting refetch. */
  eventNames?: readonly string[];
  /**
   * Debounce event-driven (SSE) refetches by N ms so a burst collapses into one
   * count refresh. The messaging fan-out wakes every badge once per message
   * tenant-wide; without this, each event does removeItem(cacheKey) +
   * refresh(skipCache) per badge, defeating the cache exactly when traffic is
   * highest. Omit (or 0) to refresh immediately on every event (default).
   */
  eventDebounceMs?: number;
  /** Also refetch on window focus. */
  refetchOnFocus?: boolean;
  /** Error handler; defaults to swallowing — a badge must never break nav. */
  onError?: (error: unknown) => void;
}

export interface UseUnreadCountResult {
  unreadCount: number;
  isLoading: boolean;
  refresh: (skipCache?: boolean) => Promise<void>;
}

export function useUnreadCount({
  enabled,
  fetcher,
  cacheKey,
  eventNames,
  refetchOnFocus = false,
  eventDebounceMs,
  onError,
}: UseUnreadCountOptions): UseUnreadCountResult {
  const [unreadCount, setUnreadCount] = useState(0);
  const [isLoading, setIsLoading] = useState(true);
  const isFetchingRef = useRef(false);
  // Set when a forced refresh (skipCache) arrives while a fetch is already in
  // flight. The in-flight fetch drains it after it resolves, so the force is
  // honored instead of dropped — otherwise the in-flight original would
  // writeCache its now-stale pre-event count with a fresh timestamp and later
  // refreshes would treat it as valid for the full cache window (stale badge).
  const pendingForceRef = useRef(false);
  // Update the async request inputs during commit, before the browser paints a
  // new tenant/session. Passive updates leave a window where an old request can
  // publish after the UI has switched. Updating the fetcher at the same time
  // also prevents a retry from pairing a new cache key with an old closure.
  const fetcherRef = useCommittedLatest(fetcher);
  const onErrorRef = useCommittedLatest(onError);
  const enabledRef = useCommittedLatest(enabled);
  const cacheKeyRef = useCommittedLatest(cacheKey);
  // Tracks the cacheKey whose count is currently shown in React state. Unlike
  // the refs above it is NOT overwritten every render — it advances only when we
  // actually set a count, so refresh() can detect a tenant switch (cacheKey
  // changed) that has no cached entry for the new tenant and clear the badge
  // instead of leaving the previous tenant's count rendered (a cross-tenant leak
  // that would otherwise persist forever if the new tenant's fetch fails).
  const displayedKeyRef = useRef(cacheKey);
  // Latest refresh(), so the error-path reconciliation below can re-fetch for the
  // CURRENT tenant without listing refresh in its own deps (self-reference).
  const refreshRef = useRef<((skipCache?: boolean) => Promise<void>) | null>(
    null,
  );

  useCommitEffect(() => {
    if (!enabled) {
      setUnreadCount(0);
      setIsLoading(false);
      return;
    }
    if (displayedKeyRef.current !== cacheKey) {
      setUnreadCount(0);
      setIsLoading(true);
    }
  }, [enabled, cacheKey]);

  const refresh = useCallback(
    async (skipCache = false) => {
      if (!enabled) {
        setUnreadCount(0);
        setIsLoading(false);
        return;
      }

      // Prevent concurrent fetches. A forced refresh that lands mid-flight is
      // not dropped: it queues so the running fetch re-runs once it resolves,
      // ensuring the freshest count wins over the in-flight (pre-event) one.
      if (isFetchingRef.current) {
        if (skipCache) pendingForceRef.current = true;
        return;
      }

      // Show the cached value immediately, then refresh in the background.
      if (cacheKey && !skipCache) {
        const cached = readCache(cacheKey);
        if (cached !== null) {
          setUnreadCount(cached);
          displayedKeyRef.current = cacheKey;
          setIsLoading(false);
        } else if (displayedKeyRef.current !== cacheKey) {
          // Tenant switched and the new tenant has no cached count: clear the
          // stale previous-tenant count NOW rather than after the network
          // resolves. Without this, a failed fetch leaves the old school's badge
          // number in the sidebar indefinitely (cross-tenant metadata leak).
          setUnreadCount(0);
          displayedKeyRef.current = cacheKey;
          setIsLoading(true);
        }
      }

      let fetchKey = cacheKeyRef.current;
      try {
        isFetchingRef.current = true;
        // Drain any forced refresh that arrives while a fetch is in flight, so
        // the last write reflects the newest count rather than a stale one.
        do {
          pendingForceRef.current = false;
          // The key this iteration is fetching for. A tenant switch mid-flight
          // changes cacheKeyRef; the resolved count then belongs to the previous
          // tenant and must not be shown or cached under the new key.
          fetchKey = cacheKeyRef.current;
          const count = await fetcherRef.current();
          // The gate may have flipped off while this fetch was in flight; the
          // layout effect already reset the count to 0, so a stale non-zero count
          // here must NOT win (and must not be cached).
          if (!enabledRef.current) return;
          // Tenant switched mid-flight (cacheKey changed): discard this stale
          // result and re-fetch for the new key, so the badge never retains the
          // previous tenant's count. The refresh() the switch triggered hit the
          // in-flight guard and returned; this re-loop is what honors it.
          if (cacheKeyRef.current !== fetchKey) {
            pendingForceRef.current = true;
            continue;
          }
          setUnreadCount(count);
          displayedKeyRef.current = fetchKey;
          if (fetchKey) writeCache(fetchKey, count);
        } while (pendingForceRef.current);
      } catch (error) {
        // Do not report an error from a request that no longer belongs to the
        // committed tenant/session. The reconciliation below retries the new
        // tenant, while logout simply discards the rejected request.
        if (enabledRef.current && cacheKeyRef.current === fetchKey) {
          onErrorRef.current?.(error);
        }
      } finally {
        isFetchingRef.current = false;
        pendingForceRef.current = false;
        setIsLoading(false);
        // A tenant switch that landed while this fetch was in flight made the
        // switch's own refresh() return early on the in-flight guard above. On the
        // SUCCESS path the do-while re-loop already re-fetched for the new key; but
        // if this fetch FAILED (e.g. the JWT was re-scoped by the switch and the
        // request 401'd), nothing else reconciles the badge. Re-fetch for the
        // current tenant when the displayed count still belongs to the old key.
        if (
          enabledRef.current &&
          displayedKeyRef.current !== cacheKeyRef.current
        ) {
          setUnreadCount(0);
          displayedKeyRef.current = cacheKeyRef.current;
          void refreshRef.current?.();
        }
      }
    },
    [cacheKey, cacheKeyRef, enabled, enabledRef, fetcherRef, onErrorRef],
  );
  useCommitEffect(() => {
    refreshRef.current = refresh;
  }, [refresh]);

  useEffect(() => {
    void refresh();
  }, [refresh]);

  // Stable string key so the listener effect only re-runs when the set of
  // event names actually changes, not on every render's fresh array.
  const eventKey = (eventNames ?? []).join("|");
  useEffect(() => {
    if (!enabled) return;
    let timer: ReturnType<typeof setTimeout> | null = null;
    const runRefresh = () => {
      if (cacheKey && typeof window !== "undefined") {
        localStorage.removeItem(cacheKey);
      }
      void refresh(true);
    };
    // Debounce bursts (the messaging fan-out fires one event per message,
    // tenant-wide) into a single refetch; fire immediately when no debounce set.
    const handleEvent = () => {
      if (eventDebounceMs && eventDebounceMs > 0) {
        if (timer) clearTimeout(timer);
        timer = setTimeout(runRefresh, eventDebounceMs);
      } else {
        runRefresh();
      }
    };
    const names = eventKey ? eventKey.split("|") : [];
    names.forEach((name) => window.addEventListener(name, handleEvent));
    if (refetchOnFocus) window.addEventListener("focus", handleEvent);
    return () => {
      names.forEach((name) => window.removeEventListener(name, handleEvent));
      if (refetchOnFocus) window.removeEventListener("focus", handleEvent);
      if (timer) clearTimeout(timer);
    };
  }, [enabled, cacheKey, eventKey, refetchOnFocus, eventDebounceMs, refresh]);

  return { unreadCount, isLoading, refresh };
}
