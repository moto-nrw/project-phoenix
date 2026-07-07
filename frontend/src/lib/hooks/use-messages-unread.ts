"use client";

import { useSession } from "next-auth/react";
import { fetchUnreadCount } from "~/lib/parent-messages-api";
import { useShellAuth } from "~/lib/shell-auth-context";
import { useTenantSafe, useTenantSlugSafe } from "~/lib/tenant-context";
import { useUnreadCount } from "./use-unread-count";

export function useMessagesUnread() {
  const { data: session, status } = useSession();
  // The staff inbox endpoint is tenant-scoped: TenantMiddleware rejects parent
  // and operator (scope != "") tokens with 401. Only fetch in the tenant-staff
  // shell so the parents/operator portals never hit it.
  const { mode } = useShellAuth();
  // Messaging shares the operations.parent_notes_enabled gate, resolved onto the
  // tenant context (TenantInfo.messagingEnabled) and re-resolved whenever the
  // tenant provider sees phoenix:tenant-settings-stale (operator toggles the
  // setting). Tying `enabled` to it means flipping messaging off re-runs the
  // hook (enabled is a refresh dep) and clears the badge to 0 immediately,
  // instead of leaving a cached non-zero count until an unrelated refresh; and
  // re-enabling re-fetches the real count once the flag re-resolves.
  const messagingEnabled = useTenantSafe()?.tenant?.messagingEnabled === true;
  // Per-tenant AND per-account cache key: the unread count is computed for the
  // current account and its student-read scope, so two staff users sharing a
  // browser on the same tenant must not see each other's cached badge. The
  // tenant slug keeps a tenant switch from surfacing the previous school's
  // count. Changing the key also re-runs the fetch (useUnreadCount's refresh
  // depends on cacheKey), so the badge refreshes purely from tenant/account
  // changes.
  const tenantSlug = useTenantSlugSafe();
  const accountId = session?.user?.id ?? "";
  return useUnreadCount({
    enabled:
      status === "authenticated" && mode === "teacher" && messagingEnabled,
    fetcher: fetchUnreadCount,
    cacheKey: `messages_unread_count:${tenantSlug ?? ""}:${accountId}`,
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
