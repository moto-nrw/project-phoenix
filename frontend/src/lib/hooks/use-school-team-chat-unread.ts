"use client";

import { useCallback, useState } from "react";
import { schoolStaffMessagesApi } from "~/lib/school-staff-messages-api";
import { isStaffMessagingDisabled } from "~/lib/staff-messages-api";
import { useUnreadCount } from "./use-unread-count";

/**
 * Unread badge AND availability of the Team-Chat in the school portal
 * (#2208). The OGS portal reads the feature flag from its cached tenant
 * metadata; the school portal has no such cache, so the first count fetch
 * doubles as the probe: a `staff_messaging_disabled` answer hides the
 * navigation entry instead of leaving a dead link.
 *
 * `available` is null until the probe answered — the navigation renders the
 * entry only once it knows, so a switched-off school never sees it flash.
 *
 * No session hook and no localStorage cache here: the school shell renders
 * only behind SchoolAuthGuard (always authenticated, one school per session),
 * and a per-account cache would need the session just to build its key. The
 * count is cheap; it is simply fetched on mount, on the refresh event and on
 * focus.
 */
export interface SchoolTeamChatUnread {
  unreadCount: number;
  available: boolean | null;
}

export function useSchoolTeamChatUnread(): SchoolTeamChatUnread {
  const [available, setAvailable] = useState<boolean | null>(null);

  const fetcher = useCallback(async () => {
    const count = await schoolStaffMessagesApi.fetchUnreadCount();
    setAvailable(true);
    return count;
  }, []);
  const onError = useCallback((err: unknown) => {
    if (isStaffMessagingDisabled(err)) setAvailable(false);
  }, []);

  const { unreadCount } = useUnreadCount({
    enabled: true,
    fetcher,
    eventNames: ["team-messages-unread-refresh"],
    eventDebounceMs: 500,
    refetchOnFocus: true,
    onError,
  });

  return { unreadCount, available };
}
