"use client";

import { useSession } from "next-auth/react";
import { fetchPendingChangeRequestCount } from "~/lib/change-requests-api";
import { hasPermission, isAdmin } from "~/lib/auth-utils";
import { useShellAuth } from "~/lib/shell-auth-context";
import {
  useTenantSafe,
  useTenantSlugSafe,
} from "~/components/tenant/tenant-provider";
import { useUnreadCount } from "./use-unread-count";

/**
 * Pending-count badge for the Änderungsanfragen sidebar item — the actionable
 * counterpart to the Nachrichten unread badge, using the same UnreadBadge and
 * the same shared useUnreadCount machinery (cache, focus refetch, tenant/account
 * cache key). Gated on users:update in the tenant-staff shell — the same
 * permission the backend endpoint (and both review queues it sums) now require.
 * The endpoint scopes the count per child (admin or the child's group
 * supervisor), so a supervising staffer gets a badge for their group's requests.
 *
 * Refreshes on messages-unread-refresh (a new request also emits a chat pill, so
 * that fan-out fires) and on change-requests-refresh (dispatched by the review
 * lists after a decision), plus on focus.
 */
export function useChangeRequestsPending() {
  const { data: session, status } = useSession();
  const { mode } = useShellAuth();
  const canReviewRequests =
    isAdmin(session) || hasPermission(session, "users:update");
  // Re-resolve on the same messaging gate the review queues live behind: care /
  // master-data requests only exist where parent messaging is enabled, and
  // tying `enabled` to it clears the badge immediately when an operator flips
  // messaging off (enabled is a refresh dep) instead of leaving a stale count.
  const messagingEnabled = useTenantSafe()?.tenant?.messagingEnabled === true;
  const tenantSlug = useTenantSlugSafe();
  const accountId = session?.user?.id ?? "";
  return useUnreadCount({
    enabled:
      status === "authenticated" &&
      mode === "teacher" &&
      canReviewRequests &&
      messagingEnabled,
    fetcher: fetchPendingChangeRequestCount,
    cacheKey: `change_requests_pending_count:${tenantSlug ?? ""}:${accountId}`,
    eventNames: ["messages-unread-refresh", "change-requests-refresh"],
    eventDebounceMs: 500,
    refetchOnFocus: true,
  });
}
