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
import { usePathname } from "next/navigation";
import { mutate } from "swr";
import { useSession } from "~/lib/auth-client";
import { useSSE } from "~/lib/hooks/use-sse";
import type { SSEEvent, SSEHookState } from "~/lib/sse-types";

// Paths where SSE should be disabled (no staff context required)
const SSE_DISABLED_PATHS = ["/saas-admin"];

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
 * - Only connects when authenticated AND has an active organization
 * - Exposes connection status for debugging/UI
 *
 * Note: SSE is disabled for:
 * - SaaS platform admin pages (/saas-admin/*) - no staff context
 * - Users without an active organization
 *
 * @returns SSE connection state (status, isConnected, error, reconnectAttempts)
 */
export function useGlobalSSE(): SSEHookState {
  const { data: session, isPending } = useSession();
  const pathname = usePathname();

  // Check if current path should have SSE disabled
  // SaaS admin pages don't have staff context, so SSE would fail with 403
  const isDisabledPath = SSE_DISABLED_PATHS.some(
    (path) => pathname === path || pathname?.startsWith(`${path}/`),
  );

  // Only enable SSE when:
  // 1. User is authenticated
  // 2. User has an active organization
  // 3. Not on a path where SSE is disabled (e.g., SaaS admin)
  const hasActiveOrg = !!session?.session?.activeOrganizationId;
  const isAuthenticated =
    !isPending && !!session?.user && hasActiveOrg && !isDisabledPath;

  // Handle SSE events by invalidating relevant caches
  const handleSSEEvent = useCallback((event: SSEEvent) => {
    switch (event.type) {
      case "student_checkin":
      case "student_checkout":
        invalidateCaches(STUDENT_EVENT_PATTERNS);
        break;

      case "activity_start":
      case "activity_end":
      case "activity_update":
        invalidateCaches(ACTIVITY_EVENT_PATTERNS);
        break;
    }
  }, []);

  // Use the underlying SSE hook with global event handler
  return useSSE("/api/sse/events", {
    onMessage: handleSSEEvent,
    enabled: isAuthenticated,
  });
}
