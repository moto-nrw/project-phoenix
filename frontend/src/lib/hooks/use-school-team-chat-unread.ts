"use client";

import { useCallback, useEffect, useRef, useState } from "react";
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
 * A transient failure (offline, 500, a request that raced a token refresh) is
 * NOT an answer: it would otherwise leave `available` at null and hide the
 * Team-Chat for the rest of the session, because nothing re-probes on its own
 * before the next focus or SSE event. Such a failure schedules its own retry
 * with a growing delay, so the entry appears as soon as the backend answers
 * again. `staff_messaging_disabled` is a real answer and stops the retries.
 *
 * Both values are bound to the authenticated school session (school + account).
 * SchoolShell stays mounted across a session change, so an unscoped state would
 * keep showing the previous account's badge — and its availability — until a
 * refresh lands, or forever if that refresh fails. The count is cached under
 * the same scope, which is what makes useUnreadCount clear the badge the moment
 * the scope changes; the probe is stamped with the scope it was measured for,
 * so a reply from the previous session cannot revive it.
 */

const RETRY_DELAYS_MS = [2_000, 5_000, 15_000, 30_000, 60_000] as const;

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

  // Latest refresh() of the count hook, so the transient-error retry below can
  // re-probe without the hook depending on its own result.
  const refreshRef = useRef<((skipCache?: boolean) => Promise<void>) | null>(
    null,
  );
  const retryTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const retryCountRef = useRef(0);
  const retryScopeRef = useRef(scope);

  const cancelRetry = useCallback(() => {
    if (retryTimerRef.current) {
      clearTimeout(retryTimerRef.current);
      retryTimerRef.current = null;
    }
  }, []);

  // A new session starts a fresh probe: drop the pending retry and its backoff.
  useEffect(() => {
    if (retryScopeRef.current !== scope) {
      retryScopeRef.current = scope;
      retryCountRef.current = 0;
      cancelRetry();
    }
  }, [scope, cancelRetry]);

  useEffect(() => cancelRetry, [cancelRetry]);

  const fetcher = useCallback(async () => {
    const count = await schoolStaffMessagesApi.fetchUnreadCount();
    retryCountRef.current = 0;
    cancelRetry();
    setProbe({ scope, available: true });
    return count;
  }, [scope, cancelRetry]);

  const onError = useCallback(
    (err: unknown) => {
      if (isStaffMessagingDisabled(err)) {
        retryCountRef.current = 0;
        cancelRetry();
        setProbe({ scope, available: false });
        return;
      }
      // Transient: keep asking, with a growing delay, until the backend answers
      // one way or the other. The final delay repeats for the rest of the
      // session rather than giving up on the navigation entry.
      const attempt = retryCountRef.current;
      retryCountRef.current = Math.min(attempt + 1, RETRY_DELAYS_MS.length - 1);
      cancelRetry();
      retryTimerRef.current = setTimeout(
        () => {
          retryTimerRef.current = null;
          if (retryScopeRef.current !== scope) return;
          void refreshRef.current?.(true);
        },
        RETRY_DELAYS_MS[attempt] ?? RETRY_DELAYS_MS[RETRY_DELAYS_MS.length - 1],
      );
    },
    [scope, cancelRetry],
  );

  const { unreadCount, refresh } = useUnreadCount({
    enabled: status === "authenticated",
    fetcher,
    cacheKey: `school_team_chat_unread:${scope}`,
    eventNames: ["team-messages-unread-refresh"],
    eventDebounceMs: 500,
    refetchOnFocus: true,
    onError,
  });

  refreshRef.current = refresh;

  return {
    unreadCount,
    available: probe && probe.scope === scope ? probe.available : null,
  };
}
