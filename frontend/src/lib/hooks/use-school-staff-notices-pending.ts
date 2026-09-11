"use client";

import { useSession } from "next-auth/react";
import { schoolStaffNoticesApi } from "~/lib/school-staff-notices-api";
import { STAFF_NOTICES_REFRESH_EVENT } from "~/lib/staff-notices-api";
import { useUnreadCount } from "./use-unread-count";

export interface SchoolStaffNoticesPending {
  pendingCount: number;
}

/**
 * Zähler der Tagesinformationen in moto schule (#2208): heutige Hinweise der
 * OGS-Leitung, deren Kenntnisnahme verlangt ist und noch fehlt. Dasselbe Maß
 * wie im OGS-Portal — ein Hinweis ohne Pflicht-Kenntnisnahme zählt nicht,
 * sonst stünde die Zahl dauerhaft.
 *
 * Gebunden an die angemeldete Schul-Sitzung (Schule + Konto): die Hülle
 * bleibt über einen Kontowechsel hinweg montiert, ein ungebundener Zähler
 * zeigte sonst die Zahl der vorigen Person.
 */
export function useSchoolStaffNoticesPending(): SchoolStaffNoticesPending {
  const { data: session, status } = useSession();
  const scope = `${session?.user.tenantId ?? ""}:${session?.user.id ?? ""}`;
  const { unreadCount } = useUnreadCount({
    enabled: status === "authenticated",
    fetcher: async () => {
      const notices = await schoolStaffNoticesApi.fetchTodaysNotices();
      return notices.filter(
        (n) => n.requires_acknowledgement && !n.acknowledged_at,
      ).length;
    },
    cacheKey: `school_staff_notices_pending:${scope}`,
    eventNames: [STAFF_NOTICES_REFRESH_EVENT],
    refetchOnFocus: true,
  });
  return { pendingCount: unreadCount };
}
