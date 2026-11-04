"use client";

import { useSession } from "next-auth/react";

import {
  ENROLLMENTS_UNREAD_SWR_KEY,
  fetchUnreadEnrollmentCount,
} from "~/lib/enrollment-unread-api";
import { hasPermission } from "~/lib/auth-utils";
import { useShellAuth } from "~/lib/shell-auth-context";
import { useSWRAuth } from "~/lib/swr";
import { useModuleBooked } from "~/lib/tenant-context";

/**
 * Zähler der ungelesenen Anmeldungen für den Bereich „Anmeldungen" und die
 * Startseite (#3778). Gelesen gilt pro Person; gezählt wird in der Datenbank.
 * Nur mit config:manage, demselben Recht wie der Bereich und der Endpunkt.
 * Eine Anmeldung ist keine Anfrage: der Zähler fließt nicht in „Anfragen" ein.
 *
 * Seitenleiste, mobile Navigation und Startseite lesen einen SWR-Eintrag je
 * Schule und Konto, also einen Request statt einen je Stelle (#4989). Neu
 * gezählt wird nach dem Markieren und Öffnen (announceEnrollmentReadChange)
 * und beim Zurückkehren ins Fenster.
 */
export function useEnrollmentsUnread() {
  const { data: session, status } = useSession();
  const { mode } = useShellAuth();
  const accountId = session?.user?.id;
  // Anmeldungen gibt es nur mit dem Modul Online-Anmeldung (#4894).
  const onlineAnmeldungBooked = useModuleBooked("online_anmeldung");
  const enabled =
    status === "authenticated" &&
    !!accountId &&
    mode === "teacher" &&
    onlineAnmeldungBooked &&
    hasPermission(session, "config:manage");
  const { data, isLoading, mutate } = useSWRAuth<number>(
    enabled ? `${ENROLLMENTS_UNREAD_SWR_KEY}:${accountId}` : null,
    fetchUnreadEnrollmentCount,
    {
      revalidateOnFocus: true,
      // Nach einem Konto- oder Schulwechsel nie den Zähler des vorigen
      // Schlüssels zeigen, auch nicht bis zur ersten Antwort.
      keepPreviousData: false,
    },
  );
  return {
    unreadCount: enabled ? (data ?? 0) : 0,
    isLoading: enabled && isLoading,
    refresh: async () => {
      await mutate();
    },
  };
}
