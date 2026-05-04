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
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "GlobalSSE" });

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

  // Enable SSE for staff (has "user" role) and admins.
  // Pure admins without a staff record connect with zero supervised groups
  // but still receive BroadcastToAll events (e.g. dashboard_counts_changed).
  const isStaff = session?.user?.roles?.includes("user") ?? false;
  const isAdmin = session?.user?.roles?.includes("admin") ?? false;
  const isAuthenticated =
    sessionStatus === "authenticated" &&
    !!session?.user?.token &&
    (isStaff || isAdmin);

  // Debounce state: collect affected group IDs, flush once after DEBOUNCE_MS
  const pendingGroupIds = useRef(new Set<string>());
  const pendingStudentIds = useRef(new Set<string>());
  const hasPendingActivityEvent = useRef(false);
  const hasPendingDashboardEvent = useRef(false);
  const hasPendingDailyCheckoutDashboardEvent = useRef(false);
  const hasPendingArrivalScheduleEvent = useRef(false);
  const hasPendingStudentUpdateEvent = useRef(false);
  const debounceTimer = useRef<ReturnType<typeof setTimeout> | null>(null);

  // SWR cache keys are tenant-prefixed by useSWRAuth (e.g. "tenant-slug:ogs-students-2").
  // All matchers must use includes() instead of startsWith() to match regardless of prefix.
  const flushInvalidations = useCallback(() => {
    // Invalidate ALL supervision-visits caches for student/dashboard events.
    // A student checked out of Room A may appear on the Schulhof (catch-all),
    // so we can't limit to just the source group's cache key.
    // Zero-topic clients only receive dashboard_counts_changed, so include
    // that flag to keep their detail views in sync.
    if (pendingGroupIds.current.size > 0 || hasPendingDashboardEvent.current) {
      mutate(
        (key) => typeof key === "string" && key.includes("supervision-visits-"),
      ).catch((err) => {
        logger.debug("swr_revalidation_failed", {
          error: err instanceof Error ? err.message : String(err),
          scope: "supervision_visits",
        });
      });
    }

    // Invalidate student list caches so "Meine Gruppe" and students/search
    // pick up location changes (e.g. Zuhause) without a manual refresh.
    // Also invalidate tracking indicator caches on active-supervisions
    // (tracking-supervisions-*) and students/search (tracking-indicators-*).
    // Triggered by pendingGroupIds (room-level events), pendingStudentIds
    // (daily checkout sends student_checkout without an active_group_id),
    // or hasPendingDashboardEvent (dashboard_counts_changed is broadcast
    // to ALL clients on every check-in/out — ensures search page updates
    // even when the user doesn't supervise the affected room/group).
    if (
      pendingGroupIds.current.size > 0 ||
      pendingStudentIds.current.size > 0 ||
      hasPendingDashboardEvent.current ||
      hasPendingArrivalScheduleEvent.current ||
      hasPendingStudentUpdateEvent.current
    ) {
      mutate(
        (key) =>
          typeof key === "string" &&
          (key.includes("ogs-students-") ||
            key.includes("database-students-list") ||
            key.includes("search-students-") ||
            key.includes("tracking-supervisions-") ||
            key.includes("tracking-indicators-") ||
            // Live "Kinder im Raum" view on /rooms/{id}. Cache key shape is
            // "room-students-{roomId}" — see
            // components/rooms/students-in-room-section.tsx. Student
            // checkin/checkout events do not carry room_id, so we cannot
            // narrow further here; refetching the section's SWR key is
            // cheap and gives the page live data without polling.
            key.includes("room-students-")),
      ).catch((err) => {
        logger.debug("swr_revalidation_failed", {
          error: err instanceof Error ? err.message : String(err),
          scope: "ogs_students",
        });
      });
    }

    // Invalidate specific student detail caches
    for (const studentId of pendingStudentIds.current) {
      mutate(
        (key) =>
          typeof key === "string" &&
          key.includes(`student-detail-${studentId}`),
      ).catch((err) => {
        logger.debug("swr_revalidation_failed", {
          error: err instanceof Error ? err.message : String(err),
          scope: "student_detail",
        });
      });
    }

    if (hasPendingStudentUpdateEvent.current) {
      mutate(
        (key) => typeof key === "string" && key.includes("student-detail-"),
      ).catch((err) => {
        logger.debug("swr_revalidation_failed", {
          error: err instanceof Error ? err.message : String(err),
          scope: "student_detail",
        });
      });
    }

    // Arrival schedule changes affect derived "Kommt heute nicht" badges and
    // arrival rows. These keys are independent from attendance/location caches.
    if (hasPendingArrivalScheduleEvent.current) {
      mutate(
        (key) =>
          typeof key === "string" &&
          (key.includes("arrival-search-") ||
            key.includes("arrival-supervisions-") ||
            key.includes("arrival-ogs-groups-") ||
            key.includes("arrival-data-")),
      ).catch((err) => {
        logger.debug("swr_revalidation_failed", {
          error: err instanceof Error ? err.message : String(err),
          scope: "arrival_schedule",
        });
      });
    }

    // Invalidate dashboard for activity events, explicit dashboard broadcasts,
    // and student movement fallbacks. BroadcastToAll is best-effort, so a
    // delivered student_checkin/student_checkout may be the only signal that
    // counts changed if dashboard_counts_changed gets dropped under backpressure.
    if (
      pendingGroupIds.current.size > 0 ||
      pendingStudentIds.current.size > 0 ||
      hasPendingActivityEvent.current ||
      hasPendingDashboardEvent.current ||
      hasPendingDailyCheckoutDashboardEvent.current ||
      hasPendingArrivalScheduleEvent.current ||
      hasPendingStudentUpdateEvent.current
    ) {
      mutate(
        (key) => typeof key === "string" && key.includes("dashboard"),
      ).catch((err) => {
        logger.debug("swr_revalidation_failed", {
          error: err instanceof Error ? err.message : String(err),
          scope: "dashboard",
        });
      });
    }

    // Activity events also need room/supervision refresh
    if (hasPendingActivityEvent.current) {
      mutate(
        (key) =>
          typeof key === "string" &&
          (key.includes("supervision") ||
            key.includes("active") ||
            key.includes("rooms")),
      ).catch((err) => {
        logger.debug("swr_revalidation_failed", {
          error: err instanceof Error ? err.message : String(err),
          scope: "activity_supervision",
        });
      });
    }

    // Reset pending state
    pendingGroupIds.current.clear();
    pendingStudentIds.current.clear();
    hasPendingActivityEvent.current = false;
    hasPendingDashboardEvent.current = false;
    hasPendingDailyCheckoutDashboardEvent.current = false;
    hasPendingArrivalScheduleEvent.current = false;
    hasPendingStudentUpdateEvent.current = false;
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
          // Daily "nach Hause" checkout emits only student_checkout on the
          // educational group topic without a companion dashboard event.
          if (event.type === "student_checkout" && !event.active_group_id) {
            hasPendingDailyCheckoutDashboardEvent.current = true;
          }
          scheduleFlush();
          break;
        }

        case "student_updated": {
          hasPendingStudentUpdateEvent.current = true;
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

        case "dashboard_counts_changed": {
          // Global event from BroadcastToAll — only refresh dashboard counts,
          // NOT room/supervision/active caches (those are for activity events).
          hasPendingDashboardEvent.current = true;
          scheduleFlush();
          break;
        }

        case "arrival_schedule_changed": {
          hasPendingArrivalScheduleEvent.current = true;
          scheduleFlush();
          break;
        }
      }
    },
    [scheduleFlush],
  );

  // Use the underlying SSE hook with global event handler.
  // reconnectKey ensures the EventSource tears down and reconnects with a
  // fresh JWT whenever the user switches tenant.
  return useSSE("/api/sse/events", {
    onMessage: handleSSEEvent,
    enabled: isAuthenticated,
    reconnectKey: session?.user?.tenantId,
  });
}
