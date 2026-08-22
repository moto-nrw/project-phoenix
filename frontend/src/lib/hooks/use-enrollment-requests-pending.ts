"use client";

import { useSession } from "next-auth/react";

import { canReviewEnrollmentChangeRequests } from "~/lib/change-request-access";
import { fetchPendingEnrollmentChangeRequestCount } from "~/lib/change-request-list-api";
import { useShellAuth } from "~/lib/shell-auth-context";
import { useTenantSlugSafe } from "~/lib/tenant-context";
import { useUnreadCount } from "./use-unread-count";

/**
 * Zähler der offenen Anmeldungsänderungen für das Badge am Anfragen-Eintrag
 * (#2435). Er kommt zu den vier Kinderdaten-Arten und den Abwesenheiten hinzu,
 * hat aber ein eigenes Recht (config:manage) und deshalb einen eigenen Hook —
 * wer die Art nicht entscheiden darf, zählt sie auch nicht mit.
 *
 * Gezählt wird in der Datenbank, nicht die Länge einer geladenen Seite: sonst
 * stünde ab der Seitengröße dauerhaft dieselbe Zahl im Badge, egal wie viel
 * wirklich wartet.
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
    fetcher: fetchPendingEnrollmentChangeRequestCount,
    cacheKey: `enrollment_requests_pending_count:${tenantSlug ?? ""}:${accountId}`,
    eventNames: ["change-requests-refresh"],
    eventDebounceMs: 500,
    refetchOnFocus: true,
  });
}
