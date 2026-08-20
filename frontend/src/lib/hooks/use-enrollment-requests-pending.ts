"use client";

import { useSession } from "next-auth/react";

import { listEnrollmentChangeRequests } from "~/lib/change-request-list-api";
import { canReviewEnrollmentChangeRequests } from "~/lib/change-request-access";
import { useShellAuth } from "~/lib/shell-auth-context";
import { useTenantSlugSafe } from "~/lib/tenant-context";
import { useUnreadCount } from "./use-unread-count";

async function fetchPendingEnrollmentRequestCount(): Promise<number> {
  try {
    const page = await listEnrollmentChangeRequests("open", { limit: 100 });
    return page.items.length;
  } catch {
    return 0;
  }
}

/**
 * Zähler der offenen Anmeldungsänderungen für das Badge am Anfragen-Eintrag
 * (#2435). Er kommt zu den vier Kinderdaten-Arten und den Abwesenheiten hinzu,
 * hat aber ein eigenes Recht (config:manage) und deshalb einen eigenen Hook —
 * wer die Art nicht entscheiden darf, zählt sie auch nicht mit.
 *
 * Kein eigener Zähl-Endpunkt: die offene Liste einer Schule ist kurz, und
 * clientseitig zu zählen erspart es, Rechte- und Statuslogik ein zweites Mal
 * im Backend zu führen — dasselbe Vorgehen wie bei den Abwesenheitsanträgen.
 * Bewusste Grenze: gezählt wird eine Seite von 100. Wer mehr als 100 offene
 * Anmeldungsänderungen hat, sieht 100 im Badge und die vollständige Liste auf
 * der Seite; ab dieser Größenordnung wäre ein Zähl-Endpunkt fällig.
 */
export function useEnrollmentRequestsPending() {
  const { data: session, status } = useSession();
  const { mode } = useShellAuth();
  const tenantSlug = useTenantSlugSafe();
  const accountId = session?.user?.id ?? "";
  return useUnreadCount({
    enabled:
      status === "authenticated" &&
      mode === "teacher" &&
      canReviewEnrollmentChangeRequests(session),
    fetcher: fetchPendingEnrollmentRequestCount,
    cacheKey: `enrollment_requests_pending_count:${tenantSlug ?? ""}:${accountId}`,
    eventNames: ["change-requests-refresh"],
    eventDebounceMs: 500,
    refetchOnFocus: true,
  });
}
