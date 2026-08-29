"use client";

import { useSession } from "next-auth/react";
import { fetchStaffUnreadCount } from "~/lib/staff-messages-api";
import { useShellAuth } from "~/lib/shell-auth-context";
import { useTenantSafe, useTenantSlugSafe } from "~/lib/tenant-context";
import { useUnreadCount } from "./use-unread-count";

/**
 * Unread badge for the OGS-internal Team-Chat (#2598). Mirrors
 * useMessagesUnread, with two differences: it reads the staff-messaging feature
 * flag rather than the parent one, and it listens on its own refresh event so a
 * parent message never nudges the team badge (or vice versa).
 */
export function useStaffMessagesUnread() {
  const { data: session, status } = useSession();
  // The endpoint is tenant-scoped: TenantMiddleware rejects parent and operator
  // tokens with 401. Only fetch in the tenant-staff shell.
  const { mode } = useShellAuth();
  // Tying `enabled` to the flag means switching the chat off re-runs the hook
  // and clears the badge immediately, instead of leaving a cached non-zero
  // count until an unrelated refresh.
  const chatEnabled = useTenantSafe()?.tenant?.staffMessagingEnabled === true;
  // Per-tenant AND per-account cache key: the count belongs to this account, so
  // two staff users sharing a browser must not see each other's badge, and a
  // tenant switch must not surface the previous school's count.
  const tenantSlug = useTenantSlugSafe();
  const accountId = session?.user?.id ?? "";
  return useUnreadCount({
    enabled: status === "authenticated" && mode === "teacher" && chatEnabled,
    fetcher: fetchStaffUnreadCount,
    cacheKey: `team_chat_unread_count:${tenantSlug ?? ""}:${accountId}`,
    eventNames: ["team-messages-unread-refresh"],
    // Collapse a burst of messages into one count refetch.
    eventDebounceMs: 500,
    // SSE stops reconnecting after maxReconnectAttempts and stays "failed"; if
    // that happens while the tab is backgrounded no refresh event fires and the
    // badge would stay stale until a full reload. A focus refetch heals it.
    refetchOnFocus: true,
  });
}
