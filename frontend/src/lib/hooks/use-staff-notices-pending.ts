"use client";

import { useSession } from "next-auth/react";
import { hasPermission } from "~/lib/auth-utils";
import { useShellAuth } from "~/lib/shell-auth-context";
import { fetchTodaysNotices } from "~/lib/staff-notices-api";
import { useTenantSlugSafe } from "~/lib/tenant-context";
import { useShellSeed } from "~/lib/shell-seed";
import { useUnreadCount } from "./use-unread-count";

/** Event, mit dem eine Kenntnisnahme das Badge sofort nachziehen lässt. */
export const STAFF_NOTICES_REFRESH_EVENT = "staff-notices-refresh";

/**
 * Badge der Tagesinformationen (#2180): heutige Hinweise, deren Kenntnisnahme
 * verlangt ist und noch fehlt. Hinweise ohne Pflicht-Kenntnisnahme zählen
 * nicht — sonst stünde das Badge dauerhaft, und ein Dauer-Badge ist keins.
 */
export function useStaffNoticesPending() {
  const { data: session, status } = useSession();
  const { mode } = useShellAuth();
  const tenantSlug = useTenantSlugSafe();
  const accountId = session?.user?.id ?? "";
  return useUnreadCount({
    enabled:
      status === "authenticated" &&
      mode === "teacher" &&
      hasPermission(session, "users:read"),
    fetcher: async () => {
      const notices = await fetchTodaysNotices();
      return notices.filter(
        (n) => n.requires_acknowledgement && !n.acknowledged_at,
      ).length;
    },
    cacheKey: `staff_notices_pending:${tenantSlug ?? ""}:${accountId}`,
    eventNames: [STAFF_NOTICES_REFRESH_EVENT],
    refetchOnFocus: true,
    initialCount: useShellSeed()?.counts.staffNoticesPending,
  });
}
