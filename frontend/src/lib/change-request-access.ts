import type { Session } from "next-auth";

import { hasPermission, isAdmin } from "~/lib/auth-utils";

/**
 * Zugriffsregeln der Seite "Änderungsanfragen" an einer Stelle, weil sie an
 * vier Orten gleich lauten müssen: Seite, Sidebar-Eintrag, Eltern-Übersicht und
 * das Zähler-Badge. Auseinanderlaufen heißt entweder ein Badge ohne erreichbare
 * Seite oder eine Warteschlange, die niemand findet.
 */

/**
 * Darf die Person die Stammdaten-, Betreuungszeiten- und Angebots-Warteschlange
 * bearbeiten? Das ist users:update — dasselbe Recht, das auch das direkte
 * Bearbeiten eines Kindes verlangt, und exakt der Gate der drei zugehörigen
 * Backend-Routen.
 */
export function canReviewStudentDataRequests(session: Session | null): boolean {
  return isAdmin(session) || hasPermission(session, "users:update");
}

/**
 * Darf die Person die Seite überhaupt öffnen? Zusätzlich zu users:update reicht
 * users:absence: Wer Abwesenheiten schreiben darf, entscheidet auch elterliche
 * Entschuldigungsanträge (#2232), sieht dort aber nur diese eine
 * Warteschlange. Backend-Queue und Zähler-Endpunkt tragen dasselbe Paar.
 */
export function canReviewChangeRequests(session: Session | null): boolean {
  return (
    canReviewStudentDataRequests(session) ||
    hasPermission(session, "users:absence")
  );
}
