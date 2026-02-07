"use client";

import { useState, useEffect, useCallback, useRef } from "react";
import { useShellAuth } from "~/lib/shell-auth-context";
import { operatorSuggestionsService } from "~/lib/operator/suggestions-api";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "useOperatorSuggestionsUnread" });

const CACHE_KEY = "operator_suggestions_unread_count";
const CACHE_DURATION_MS = 60 * 1000; // 1 minute cache
const UNREAD_REFRESH_EVENT = "operator-suggestions-unread-refresh";
const UNVIEWED_REFRESH_EVENT = "operator-suggestions-unviewed-refresh";

interface CachedData {
  unreadComments: number;
  unviewedPosts: number;
  timestamp: number;
}

function getCachedCounts(): {
  unreadComments: number;
  unviewedPosts: number;
} | null {
  if (typeof window === "undefined") return null;
  try {
    const cached = localStorage.getItem(CACHE_KEY);
    if (!cached) return null;
    const data = JSON.parse(cached) as CachedData;
    if (Date.now() - data.timestamp > CACHE_DURATION_MS) {
      localStorage.removeItem(CACHE_KEY);
      return null;
    }
    return {
      unreadComments: data.unreadComments,
      unviewedPosts: data.unviewedPosts,
    };
  } catch {
    return null;
  }
}

function setCachedCounts(unreadComments: number, unviewedPosts: number): void {
  if (typeof window === "undefined") return;
  try {
    const data: CachedData = {
      unreadComments,
      unviewedPosts,
      timestamp: Date.now(),
    };
    localStorage.setItem(CACHE_KEY, JSON.stringify(data));
  } catch {
    // Ignore localStorage errors
  }
}

export function useOperatorSuggestionsUnread() {
  const { mode } = useShellAuth();
  const isOperatorMode = mode === "operator";

  const [unreadCount, setUnreadCount] = useState<number>(0);
  const [isLoading, setIsLoading] = useState(true);
  const isFetchingRef = useRef(false);

  const refresh = useCallback(
    async (skipCache = false) => {
      // Only fetch when in operator mode
      if (!isOperatorMode) {
        setUnreadCount(0);
        setIsLoading(false);
        return;
      }

      // Prevent concurrent fetches
      if (isFetchingRef.current) return;

      // Check cache first (unless skipCache is true)
      if (!skipCache) {
        const cached = getCachedCounts();
        if (cached !== null) {
          setUnreadCount(cached.unreadComments + cached.unviewedPosts);
          setIsLoading(false);
        }
      }

      try {
        isFetchingRef.current = true;
        // Fetch both counts in parallel
        const [unreadComments, unviewedPosts] = await Promise.all([
          operatorSuggestionsService.fetchUnreadCount(),
          operatorSuggestionsService.fetchUnviewedCount(),
        ]);
        const total = unreadComments + unviewedPosts;
        setUnreadCount(total);
        setCachedCounts(unreadComments, unviewedPosts);
      } catch (error) {
        // Log error for debugging but don't crash
        logger.error("operator_unread_count_fetch_failed", {
          error: error instanceof Error ? error.message : String(error),
        });
      } finally {
        isFetchingRef.current = false;
        setIsLoading(false);
      }
    },
    [isOperatorMode],
  );

  useEffect(() => {
    void refresh();
  }, [refresh]);

  // Listen for refresh events from accordions (both unread comments and unviewed posts)
  useEffect(() => {
    if (!isOperatorMode) return;

    const handleRefreshEvent = () => {
      // Clear cache and refetch
      if (typeof window !== "undefined") {
        localStorage.removeItem(CACHE_KEY);
      }
      void refresh(true);
    };

    window.addEventListener(UNREAD_REFRESH_EVENT, handleRefreshEvent);
    window.addEventListener(UNVIEWED_REFRESH_EVENT, handleRefreshEvent);
    return () => {
      window.removeEventListener(UNREAD_REFRESH_EVENT, handleRefreshEvent);
      window.removeEventListener(UNVIEWED_REFRESH_EVENT, handleRefreshEvent);
    };
  }, [refresh, isOperatorMode]);

  return { unreadCount, isLoading, refresh };
}
