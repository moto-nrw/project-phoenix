"use client";

import { useSession } from "next-auth/react";
import { hasPermission } from "~/lib/auth-utils";
import { useShellAuth } from "~/lib/shell-auth-context";
import {
  countPendingAcknowledgements,
  fetchTodaysNotices,
  staffNoticesTodayKey,
  type StaffNotice,
} from "~/lib/staff-notices-api";
import { useSWRAuth } from "~/lib/swr";

/**
 * Badge der Tagesinformationen (#2180): heutige Hinweise, deren Kenntnisnahme
 * verlangt ist und noch fehlt. Hinweise ohne Pflicht-Kenntnisnahme zählen
 * nicht — sonst stünde das Badge dauerhaft, und ein Dauer-Badge ist keins.
 *
 * Gezählt wird im SWR-Eintrag der heutigen Hinweise, den die App-Hülle aus
 * dem Server-Seed anlegt und den Startseite und /tagesinformationen mitlesen
 * (#4989). Eine Kenntnisnahme dort lädt diesen Eintrag neu, das Badge zieht
 * mit. Neu geladen wird außerdem beim Zurückkehren ins Fenster; ein
 * Seitenwechsel lädt nichts.
 */
export function useStaffNoticesPending() {
  const { data: session, status } = useSession();
  const { mode } = useShellAuth();
  const accountId = session?.user?.id;
  const enabled =
    status === "authenticated" &&
    !!accountId &&
    mode === "teacher" &&
    hasPermission(session, "users:read");
  const { data, isLoading, mutate } = useSWRAuth<StaffNotice[]>(
    enabled ? staffNoticesTodayKey(accountId) : null,
    fetchTodaysNotices,
    {
      revalidateOnFocus: true,
      revalidateIfStale: false,
      // Nach einem Konto- oder Schulwechsel nie den Stand des vorigen
      // Schlüssels zeigen, auch nicht bis zur ersten Antwort.
      keepPreviousData: false,
    },
  );
  return {
    unreadCount: enabled && data ? countPendingAcknowledgements(data) : 0,
    isLoading: enabled && isLoading,
    refresh: async () => {
      await mutate();
    },
  };
}
