"use client";

import { useSession } from "next-auth/react";
import { fetchUnreadCount } from "~/lib/parent-messages-api";
import { useShellAuth } from "~/lib/shell-auth-context";
import { useTenantSlugSafe } from "~/components/tenant/tenant-provider";
import { useUnreadCount } from "./use-unread-count";

export function useMessagesUnread() {
  const { status } = useSession();
  // The staff inbox endpoint is tenant-scoped: TenantMiddleware rejects parent
  // and operator (scope != "") tokens with 401. Only fetch in the tenant-staff
  // shell so the parents/operator portals never hit it.
  const { mode } = useShellAuth();
  // Per-tenant cache key: the unread count is tenant-scoped metadata, so a tenant
  // switch must not show the previous school's count from localStorage. Changing
  // the key also re-runs the fetch (useUnreadCount's refresh depends on cacheKey),
  // so the badge refreshes purely from the tenant change.
  const tenantSlug = useTenantSlugSafe();
  return useUnreadCount({
    enabled: status === "authenticated" && mode === "teacher",
    fetcher: fetchUnreadCount,
    cacheKey: `messages_unread_count:${tenantSlug ?? ""}`,
    eventNames: ["messages-unread-refresh"],
    // Collapse the tenant-wide message fan-out into one count refetch per burst.
    eventDebounceMs: 500,
    // Refetch on focus, matching the parent badge. SSE stops reconnecting after
    // maxReconnectAttempts and stays "failed"; if that happens while the tab is
    // backgrounded, no messages-unread-refresh event fires for a new message and
    // the badge would stay stale until a full reload. A focus refetch heals it
    // when the staffer returns to the tab.
    refetchOnFocus: true,
  });
}
