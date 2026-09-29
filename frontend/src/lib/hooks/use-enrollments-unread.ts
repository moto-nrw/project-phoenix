"use client";

import { useSession } from "next-auth/react";

import {
  ENROLLMENTS_UNREAD_REFRESH_EVENT,
  fetchUnreadEnrollmentCount,
} from "~/lib/enrollment-admin-api";
import { hasPermission } from "~/lib/auth-utils";
import { useShellAuth } from "~/lib/shell-auth-context";
import { useTenantSlugSafe } from "~/lib/tenant-context";
import { useUnreadCount } from "./use-unread-count";

/**
 * Zähler der ungelesenen Anmeldungen für den Bereich „Anmeldungen" und die
 * Startseite (#3778). Gelesen gilt pro Person; gezählt wird in der Datenbank.
 * Nur mit config:manage, demselben Recht wie der Bereich und der Endpunkt.
 * Eine Anmeldung ist keine Anfrage: der Zähler fließt nicht in „Anfragen" ein.
 *
 * Neu gezählt wird nach dem Markieren und Öffnen (Fenster-Ereignis) und beim
 * Zurückkehren ins Fenster.
 */
export function useEnrollmentsUnread() {
  const { data: session, status } = useSession();
  const { mode } = useShellAuth();
  const tenantSlug = useTenantSlugSafe();
  const accountId = session?.user?.id ?? "";
  return useUnreadCount({
    enabled:
      status === "authenticated" &&
      mode === "teacher" &&
      hasPermission(session, "config:manage"),
    fetcher: fetchUnreadEnrollmentCount,
    cacheKey: `enrollments_unread_count:${tenantSlug ?? ""}:${accountId}`,
    eventNames: [ENROLLMENTS_UNREAD_REFRESH_EVENT],
    eventDebounceMs: 300,
    refetchOnFocus: true,
  });
}
