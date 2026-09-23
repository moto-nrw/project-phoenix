import type { Session } from "next-auth";

import { hasPermission, isAdmin } from "~/lib/auth-utils";

/**
 * Zugriffsregel der Elternzugänge-Warteschlange an einer Stelle, weil sie an
 * drei Orten gleich lauten muss: Seite, Sidebar-Eintrag und mobiles
 * Mehr-Menü. Auseinanderlaufen heißt eine Warteschlange, die jemand öffnet,
 * aber nicht entscheiden kann.
 *
 * Das Backend liest die Liste mit `users:manage` (sie zeigt schulweit
 * E-Mail-Adressen und Kindernamen), entscheidet aber mit `users:update`
 * (`GuardianResource.mountRelatedAccounts`). Beide Rechte gehören deshalb
 * zusammen: wer nur `users:manage` hält, sähe eine gefüllte Warteschlange,
 * in der jedes Annehmen und Ablehnen mit 403 endet. Der Adminzuschnitt hält
 * ohnehin beides.
 */
export function canReviewGuardianApprovals(session: Session | null): boolean {
  return (
    isAdmin(session) ||
    (hasPermission(session, "users:manage") &&
      hasPermission(session, "users:update"))
  );
}
