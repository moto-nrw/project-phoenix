"use client";

import {
  fetchFeedbackUnreadCount,
  PARENT_FEEDBACK_UNREAD_EVENT,
} from "~/lib/parent-feedback-api";
import { useUnreadCount } from "./use-unread-count";

/**
 * Number of unread product-team replies on the guardian's feedback board
 * (#1678), summed across every school they have a board at. Only fetches when
 * `enabled` (parent mode). Refreshes on mount, on window focus, and when a
 * thread marks itself read.
 */
export function useParentFeedbackUnread(enabled: boolean) {
  const { unreadCount, refresh } = useUnreadCount({
    enabled,
    fetcher: fetchFeedbackUnreadCount,
    eventNames: [PARENT_FEEDBACK_UNREAD_EVENT],
    refetchOnFocus: true,
    eventDebounceMs: 500,
  });
  return { unreadCount, refresh };
}
