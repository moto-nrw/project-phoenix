"use client";

import { fetchMessagesUnreadCount } from "~/lib/parent-api";
import { useUnreadCount } from "./use-unread-count";

/**
 * Number of conversations with unread staff-side activity for a guardian, for
 * the parents-portal sidebar badge. Only fetches when `enabled` (parent mode) so
 * the staff/operator portals never hit the parent endpoint. Uses the dedicated
 * COUNT endpoint instead of fetching every thread's full projection. Refreshes
 * on mount, on window focus, and on the parent-messages-unread-refresh event
 * (dispatched by the parent chat after it loads/sends — reading marks a thread
 * read server-side — so the badge updates without a full reload).
 */
export function useParentMessagesUnread(enabled: boolean) {
  const { unreadCount, refresh } = useUnreadCount({
    enabled,
    fetcher: fetchMessagesUnreadCount,
    eventNames: ["parent-messages-unread-refresh"],
    refetchOnFocus: true,
    // Collapse a burst of parent-message events into one count refetch.
    eventDebounceMs: 500,
  });
  return { unreadCount, refresh };
}
