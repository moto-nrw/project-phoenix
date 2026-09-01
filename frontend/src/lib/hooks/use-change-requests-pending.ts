"use client";

import { useSession } from "next-auth/react";
import { fetchPendingChangeRequestCount } from "~/lib/change-requests-api";
import { useShellAuth } from "~/lib/shell-auth-context";
import { useTenantSlugSafe } from "~/lib/tenant-context";
import { useChangeRequestAccess } from "./use-change-request-access";
import { useUnreadCount } from "./use-unread-count";

/**
 * Pending-count badge for the Änderungsanfragen sidebar item — the actionable
 * counterpart to the Nachrichten unread badge, using the same UnreadBadge and
 * the same shared useUnreadCount machinery (cache, focus refetch, tenant/account
 * cache key). Gated on users:update OR users:absence in the tenant-staff shell —
 * the same pair the backend endpoint and the page it links to require. The
 * endpoint scopes the count per child (admin or the child's group supervisor),
 * so a supervising staffer gets a badge for their group's requests, and it
 * counts only the queues the caller may open: with users:absence alone that is
 * the excused-absence queue and nothing else (#2232).
 *
 * Refreshes on messages-unread-refresh (with messaging on, a new request also
 * emits a chat pill, so that fan-out fires) and on change-requests-refresh
 * (dispatched by the review lists after a decision), plus on focus. When
 * messaging is off the pill (and its fan-out) is suppressed, so a freshly
 * submitted master-data request lands on the badge via the next focus refetch.
 */
export function useChangeRequestsPending() {
  const { data: session, status } = useSession();
  const { mode } = useShellAuth();
  const { canReviewParentRequests } = useChangeRequestAccess();
  // Do NOT gate the badge on parent messaging (operations.parent_notes_enabled).
  // Master-data requests (Track B) are gated on their OWN flag
  // (operations.parent_master_data_request_enabled) and are still created — and
  // still actionable in the queue — while messaging is off, so tying `enabled`
  // to messaging would hide a live count from the deciding staffer. And any
  // request already pending stays actionable regardless of the current flag
  // state. The pending-count endpoint (summing the queues this caller may open)
  // is the source of truth: it returns 0 when nothing is pending, so no
  // feature-flag gate is needed here — an empty count clears the badge on its own.
  const tenantSlug = useTenantSlugSafe();
  const accountId = session?.user?.id ?? "";
  return useUnreadCount({
    enabled:
      status === "authenticated" &&
      mode === "teacher" &&
      canReviewParentRequests,
    fetcher: fetchPendingChangeRequestCount,
    cacheKey: `change_requests_pending_count:${tenantSlug ?? ""}:${accountId}`,
    eventNames: ["messages-unread-refresh", "change-requests-refresh"],
    eventDebounceMs: 500,
    refetchOnFocus: true,
  });
}
