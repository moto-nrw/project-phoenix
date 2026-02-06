"use client";

import { useState, useEffect, useCallback, useRef } from "react";
import { useSession } from "next-auth/react";
import { fetchUnreadCount } from "~/lib/suggestions-api";

const CACHE_KEY = "suggestions_unread_count";
const CACHE_DURATION_MS = 60 * 1000; // 1 minute cache
const REFRESH_EVENT = "suggestions-unread-refresh";

interface CachedData {
  count: number;
  timestamp: number;
}

function getCachedCount(): number | null {
  if (typeof window === "undefined") return null;
  try {
    const cached = localStorage.getItem(CACHE_KEY);
    if (!cached) return null;
    const data = JSON.parse(cached) as CachedData;
    if (Date.now() - data.timestamp > CACHE_DURATION_MS) {
      localStorage.removeItem(CACHE_KEY);
      return null;
    }
    return data.count;
  } catch {
    return null;
  }
}

function setCachedCount(count: number): void {
  if (typeof window === "undefined") return;
  try {
    const data: CachedData = { count, timestamp: Date.now() };
    localStorage.setItem(CACHE_KEY, JSON.stringify(data));
  } catch {
    // Ignore localStorage errors
  }
}

export function useSuggestionsUnread() {
  const { status } = useSession();
  const [unreadCount, setUnreadCount] = useState<number>(0);
  const [isLoading, setIsLoading] = useState(true);
  const isFetchingRef = useRef(false);

  const refresh = useCallback(
    async (skipCache = false) => {
      if (status !== "authenticated") {
        setUnreadCount(0);
        setIsLoading(false);
        return;
      }

      // Prevent concurrent fetches
      if (isFetchingRef.current) return;

      // Check cache first (unless skipCache is true)
      if (!skipCache) {
        const cached = getCachedCount();
        if (cached !== null) {
          setUnreadCount(cached);
          setIsLoading(false);
          // Still fetch in background to refresh
        }
      }

      try {
        isFetchingRef.current = true;
        const count = await fetchUnreadCount();
        setUnreadCount(count);
        setCachedCount(count);
      } catch {
        // Silently ignore errors
      } finally {
        isFetchingRef.current = false;
        setIsLoading(false);
      }
    },
    [status],
  );

  useEffect(() => {
    void refresh();
  }, [refresh]);

  // Listen for refresh events from accordions
  useEffect(() => {
    const handleRefreshEvent = () => {
      // Clear cache and refetch
      if (typeof window !== "undefined") {
        localStorage.removeItem(CACHE_KEY);
      }
      void refresh(true);
    };

    window.addEventListener(REFRESH_EVENT, handleRefreshEvent);
    return () => {
      window.removeEventListener(REFRESH_EVENT, handleRefreshEvent);
    };
  }, [refresh]);

  return { unreadCount, isLoading, refresh };
}
