"use client";

import { useCallback, useState } from "react";
import { useSession } from "next-auth/react";
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
 * Both values are bound to the authenticated school session (school + account).
 * SchoolShell stays mounted across a session change, so an unscoped state would
 * keep showing the previous account's badge — and its availability — until a
 * refresh lands, or forever if that refresh fails. The count is cached under
 * the same scope, which is what makes useUnreadCount clear the badge the moment
 * the scope changes; the probe is stamped with the scope it was measured for,
 * so a reply from the previous session cannot revive it.
 */
export interface SchoolTeamChatUnread {
  unreadCount: number;
  available: boolean | null;
}

export function useSchoolTeamChatUnread(): SchoolTeamChatUnread {
  const { data: session, status } = useSession();
  const scope = `${session?.user.tenantId ?? ""}:${session?.user.id ?? ""}`;
  const [probe, setProbe] = useState<{
    scope: string;
    available: boolean;
  } | null>(null);

  const fetcher = useCallback(async () => {
    const count = await schoolStaffMessagesApi.fetchUnreadCount();
    setProbe({ scope, available: true });
    return count;
  }, [scope]);
  const onError = useCallback(
    (err: unknown) => {
      if (isStaffMessagingDisabled(err)) setProbe({ scope, available: false });
    },
    [scope],
  );

  const { unreadCount } = useUnreadCount({
    enabled: status === "authenticated",
    fetcher,
    cacheKey: `school_team_chat_unread:${scope}`,
    eventNames: ["team-messages-unread-refresh"],
    eventDebounceMs: 500,
    refetchOnFocus: true,
    onError,
  });

  return {
    unreadCount,
    available: probe && probe.scope === scope ? probe.available : null,
  };
}
