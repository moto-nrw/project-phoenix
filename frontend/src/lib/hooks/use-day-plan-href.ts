"use client";

import { useSession } from "next-auth/react";

import { hasEffectiveAdminScope, isCaregiver } from "~/lib/auth-utils";
import { useTenantAwarePath } from "~/lib/tenant-path";

/**
 * Wohin „der Tag" für diese Person führt.
 *
 * Es gibt zwei Seiten für denselben Gegenstand, und wer welche sieht,
 * entscheidet die Seitenleiste: `/tagesplan` ist der Einstieg der
 * Betreuungskräfte in den laufenden Tag und für reine Adminkonten
 * ausgeblendet (`hideForAdmin`), weil die Leitung den vollen Betreuungsplan
 * im Planungsbereich hat. Ein Konto mit beidem — Admin UND Betreuungskraft —
 * sieht den Tagesplan sehr wohl.
 *
 * Dieselbe Bedingung steht deshalb hier noch einmal: ein Weiterlink der
 * Startseite darf nicht auf eine Seite zeigen, die diese Person in ihrer
 * Navigation gar nicht hat. Ändert sich die Regel in der Seitenleiste, gehört
 * sie hier mit geändert.
 */
export function useDayPlanHref(): string {
  const { data: session } = useSession();
  const tenantPath = useTenantAwarePath();
  const adminOnly = hasEffectiveAdminScope(session) && !isCaregiver(session);
  return tenantPath(adminOnly ? "/betreuungsplan" : "/tagesplan");
}

/** Wie der Weiterlink heißt — die Seite trägt je Rolle einen anderen Namen. */
export function useDayPlanLabel(): string {
  const { data: session } = useSession();
  const adminOnly = hasEffectiveAdminScope(session) && !isCaregiver(session);
  return adminOnly ? "Zum Betreuungsplan" : "Zum Tagesplan";
}
