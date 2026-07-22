"use client";

import { useSession } from "next-auth/react";
import { ABSENCES_REFRESH_EVENT } from "~/lib/absence-helpers";
import { hasPermission, isAdmin } from "~/lib/auth-utils";
import { useShellAuth } from "~/lib/shell-auth-context";
import { staffAbsenceService } from "~/lib/staff-api";
import { useTenantSlugSafe } from "~/lib/tenant-context";
import { useUnreadCount } from "./use-unread-count";

async function fetchPendingAbsenceCount(): Promise<number> {
  try {
    const rows = await staffAbsenceService.listPending();
    return rows.length;
  } catch {
    return 0;
  }
}

/**
 * Pending-count badge for the Mitarbeiter sidebar item (#1419): open absence
 * requests (status requested + question) across all staff. Gated on
 * vacation:approve — the same permission the backend endpoint requires. No
 * dedicated count endpoint: the pending payload is one tenant's open requests
 * (small), so counting client-side avoids duplicating the permission gate and
 * status-set logic backend-side.
 *
 * Refreshes on staff-absences-refresh (dispatched by the inbox, the staff
 * detail Abwesenheiten tab, and the MA resubmit form) plus on focus.
 */
export function useStaffAbsencesPending() {
  const { data: session, status } = useSession();
  const { mode } = useShellAuth();
  const canReview =
    isAdmin(session) || hasPermission(session, "vacation:approve");
  const tenantSlug = useTenantSlugSafe();
  const accountId = session?.user?.id ?? "";
  return useUnreadCount({
    enabled: status === "authenticated" && mode === "teacher" && canReview,
    fetcher: fetchPendingAbsenceCount,
    cacheKey: `staff_absences_pending_count:${tenantSlug ?? ""}:${accountId}`,
    eventNames: [ABSENCES_REFRESH_EVENT],
    eventDebounceMs: 500,
    refetchOnFocus: true,
  });
}
