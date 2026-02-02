/**
 * Global SSE Hook
 *
 * Establishes a single SSE connection for the entire authenticated app.
 * When events arrive, it invalidates relevant SWR caches using targeted
 * keys (per active_group_id) instead of broad pattern matching.
 *
 * Debounces rapid events (e.g. morning check-in burst) so 10 checkins
 * within 500ms trigger a single refetch, not 10.
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
 */

"use client";

import { useCallback, useRef } from "react";
import { mutate } from "swr";
import { useSession } from "next-auth/react";
import { useSSE } from "~/lib/hooks/use-sse";
import type { SSEEvent, SSEHookState } from "~/lib/sse-types";

const DEBOUNCE_MS = 500;

/**
 * Global SSE hook that maintains a single connection for the entire app.
 *
 * Features:
 * - Single SSE connection (shared across all pages)
 * - Targeted cache invalidation based on active_group_id
 * - Debounced invalidation for burst events (morning rush)
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

  // Debounce state: collect affected group IDs, flush once after DEBOUNCE_MS
  const pendingGroupIds = useRef(new Set<string>());
  const pendingStudentIds = useRef(new Set<string>());
  const hasPendingActivityEvent = useRef(false);
  const debounceTimer = useRef<ReturnType<typeof setTimeout> | null>(null);

  const flushInvalidations = useCallback(() => {
    // Invalidate ALL supervision-visits caches for student events.
    // A student checked out of Room A may appear on the Schulhof (catch-all),
    // so we can't limit to just the source group's cache key.
    if (pendingGroupIds.current.size > 0) {
      void mutate(
        (key) =>
          typeof key === "string" && key.startsWith("supervision-visits-"),
        undefined,
        { revalidate: true },
      );
    }

    // Invalidate specific student detail caches
    for (const studentId of pendingStudentIds.current) {
      void mutate(`student-detail-${studentId}`);
    }

    // Invalidate dashboard (student counts changed) — single broad invalidation
    if (pendingGroupIds.current.size > 0 || hasPendingActivityEvent.current) {
      void mutate(
        (key) =>
          typeof key === "string" &&
          (key.startsWith("active-supervision-dashboard") ||
            key.includes("dashboard")),
        undefined,
        { revalidate: true },
      );
    }

    // Activity events also need room/supervision refresh
    if (hasPendingActivityEvent.current) {
      void mutate(
        (key) =>
          typeof key === "string" &&
          (key.includes("supervision") ||
            key.includes("active") ||
            key.includes("rooms")),
        undefined,
        { revalidate: true },
      );
    }

    // Reset pending state
    pendingGroupIds.current.clear();
    pendingStudentIds.current.clear();
    hasPendingActivityEvent.current = false;
  }, []);

  const scheduleFlush = useCallback(() => {
    if (debounceTimer.current) clearTimeout(debounceTimer.current);
    debounceTimer.current = setTimeout(flushInvalidations, DEBOUNCE_MS);
  }, [flushInvalidations]);

  // Handle SSE events by collecting targeted invalidations
  const handleSSEEvent = useCallback(
    (event: SSEEvent) => {
      switch (event.type) {
        case "student_checkin":
        case "student_checkout": {
          // Target the specific group affected
          if (event.active_group_id) {
            pendingGroupIds.current.add(event.active_group_id);
          }
          // Target the specific student detail cache
          if (event.data.student_id) {
            pendingStudentIds.current.add(event.data.student_id);
          }
          scheduleFlush();
          break;
        }

        case "activity_start":
        case "activity_end":
        case "activity_update": {
          if (event.active_group_id) {
            pendingGroupIds.current.add(event.active_group_id);
          }
          hasPendingActivityEvent.current = true;
          scheduleFlush();
          break;
        }
      }
    },
    [scheduleFlush],
  );

  // Use the underlying SSE hook with global event handler
  return useSSE("/api/sse/events", {
    onMessage: handleSSEEvent,
    enabled: isAuthenticated,
  });
}
