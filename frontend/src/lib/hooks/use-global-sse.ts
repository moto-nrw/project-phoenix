/**
 * Global SSE Hook
 *
 * Establishes a single SSE connection for the entire authenticated app.
 * When events arrive, it invalidates relevant SWR caches using pattern matching.
 *
 * This hook should be called ONCE in the auth layout to:
 * 1. Maintain a single SSE connection (not per-page)
 * 2. Automatically invalidate caches when events arrive
 * 3. Provide connection status for debugging/UI indicators
 *
 * @example
 * ```tsx
 * // In app/layout.tsx or auth layout wrapper
 * const { status } = useGlobalSSE();
 * ```
 *
 * Cache Invalidation Strategy:
 * - student_checkin/checkout → Invalidates student*, dashboard*, supervision* caches
 * - activity_start/end/update → Invalidates supervision*, active*, dashboard* caches
 */

"use client";

import { useCallback } from "react";
import { mutate } from "swr";
import { useSession } from "next-auth/react";
import { useSSE } from "~/lib/hooks/use-sse";
import type { SSEEvent, SSEHookState } from "~/lib/sse-types";

/**
 * Pattern matcher for SWR cache keys.
 * Returns true if the key matches any of the provided patterns.
 */
function matchesCachePattern(key: unknown, patterns: string[]): boolean {
  if (typeof key !== "string") return false;
  return patterns.some((pattern) =>
    key.toLowerCase().includes(pattern.toLowerCase()),
  );
}

/**
 * Invalidates SWR caches matching the given patterns.
 * Uses SWR's mutate() with a filter function for pattern-based invalidation.
 */
function invalidateCaches(patterns: string[]): void {
  void mutate((key) => matchesCachePattern(key, patterns), undefined, {
    revalidate: true,
  });
}

// Cache patterns for different event types
// NOTE: These patterns must match the SWR cache keys used in pages.
// If a page uses a cache key like "rooms-list", "rooms" must be in the patterns.
const STUDENT_EVENT_PATTERNS = ["student", "dashboard", "supervision", "visit"];
const ACTIVITY_EVENT_PATTERNS = [
  "supervision",
  "active",
  "dashboard",
  "visit",
  "rooms", // Rooms page uses "rooms-list" key and needs invalidation on activity events
];

/**
 * Global SSE hook that maintains a single connection for the entire app.
 *
 * Features:
 * - Single SSE connection (shared across all pages)
 * - Automatic cache invalidation based on event type
 * - Only connects when authenticated (session has token)
 * - Exposes connection status for debugging/UI
 *
 * @returns SSE connection state (status, isConnected, error, reconnectAttempts)
 */
export function useGlobalSSE(): SSEHookState {
  const { data: session, status: sessionStatus } = useSession();

  // Only enable SSE when authenticated
  const isAuthenticated =
    sessionStatus === "authenticated" && !!session?.user?.token;

  // Handle SSE events by invalidating relevant caches
  const handleSSEEvent = useCallback((event: SSEEvent) => {
    console.log(
      "🔴 [Global SSE] Event received:",
      event.type,
      "group:",
      event.active_group_id,
    );

    // Determine which caches to invalidate based on event type
    switch (event.type) {
      case "student_checkin":
      case "student_checkout":
        // Student movement events - invalidate student and dashboard caches
        console.log(
          "🔴 [Global SSE] Invalidating student-related caches:",
          STUDENT_EVENT_PATTERNS,
        );
        invalidateCaches(STUDENT_EVENT_PATTERNS);
        break;

      case "activity_start":
      case "activity_end":
      case "activity_update":
        // Activity events - invalidate supervision and active caches
        console.log(
          "🔴 [Global SSE] Invalidating activity-related caches:",
          ACTIVITY_EVENT_PATTERNS,
        );
        invalidateCaches(ACTIVITY_EVENT_PATTERNS);
        break;

      default:
        // Unknown event type - log for debugging
        console.warn("🔴 [Global SSE] Unknown event type:", event.type);
    }
  }, []);

  // Use the underlying SSE hook with global event handler
  return useSSE("/api/sse/events", {
    onMessage: handleSSEEvent,
    enabled: isAuthenticated,
  });
}
