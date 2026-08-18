"use client";

import { fetchAnnouncementsUnreadCount } from "~/lib/parent-api";
import { useUnreadCount } from "./use-unread-count";

/**
 * Number of parent announcements that still need attention in the sidebar.
 * Only fetches when `enabled` (parent mode).
 * Refreshes on mount, on window focus, and on the parent-news-unread-refresh
 * event (dispatched after opening a news item marks it read, so the badge
 * drops without a full reload).
 */
export function useParentNewsUnread(enabled: boolean) {
  const { unreadCount, refresh } = useUnreadCount({
    enabled,
    fetcher: fetchAnnouncementsUnreadCount,
    eventNames: ["parent-news-unread-refresh"],
    refetchOnFocus: true,
    eventDebounceMs: 500,
  });
  return { unreadCount, refresh };
}
