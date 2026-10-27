"use client";

import { useSession } from "next-auth/react";
import { personalStorageOwner } from "~/lib/personal-storage";

import { canReviewEnrollmentChangeRequests } from "~/lib/change-request-access";
import { fetchPendingEnrollmentChangeRequestCount } from "~/lib/change-request-list-api";
import { useShellAuth } from "~/lib/shell-auth-context";
import { useModuleBooked, useTenantSlugSafe } from "~/lib/tenant-context";
import { useShellSeed } from "~/lib/shell-seed";
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
  const onlineAnmeldungBooked = useModuleBooked("online_anmeldung");
  return useUnreadCount({
    // Änderungen an Anmeldungen gibt es nur mit Online-Anmeldung (#4894).
    enabled:
      status === "authenticated" &&
      mode === "teacher" &&
      onlineAnmeldungBooked &&
      canReviewEnrollmentChangeRequests(session),
    fetcher: fetchPendingEnrollmentChangeRequestCount,
    cacheKey: `enrollment_requests_pending_count:${tenantSlug ?? ""}:${accountId}`,
    cacheOwner: personalStorageOwner(session?.user),
    eventNames: ["change-requests-refresh"],
    eventDebounceMs: 500,
    refetchOnFocus: true,
    initialCount: useShellSeed()?.counts.enrollmentRequestsPending,
  });
}
