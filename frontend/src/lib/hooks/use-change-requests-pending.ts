"use client";

import { useSession } from "next-auth/react";
import { fetchPendingChangeRequestCount } from "~/lib/change-requests-api";
import { hasRole } from "~/lib/auth-utils";
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
 * cache key). Gated to admins in the tenant-staff shell because the backend
 * endpoint (and both review queues it sums) require UsersManage; a non-admin
 * would only get 403s.
 *
 * Refreshes on messages-unread-refresh (a new request also emits a chat pill, so
 * that fan-out fires) and on change-requests-refresh (dispatched by the review
 * lists after a decision), plus on focus.
 */
export function useChangeRequestsPending() {
  const { data: session, status } = useSession();
  const { mode } = useShellAuth();
  const userIsAdmin = hasRole(session, "admin");
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
      userIsAdmin &&
      messagingEnabled,
    fetcher: fetchPendingChangeRequestCount,
    cacheKey: `change_requests_pending_count:${tenantSlug ?? ""}:${accountId}`,
    eventNames: ["messages-unread-refresh", "change-requests-refresh"],
    eventDebounceMs: 500,
    refetchOnFocus: true,
  });
}
