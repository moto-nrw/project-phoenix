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
 * users:absence zusammen mit users:read: Wer Abwesenheiten schreiben darf,
 * entscheidet auch elterliche Entschuldigungsanträge (#2232), sieht dort aber
 * nur diese eine Warteschlange.
 *
 * users:read gehört zwingend dazu, weil das Backend es genauso verlangt
 * (authorize.CanManageStudentAbsence): users:absence ist ein Schreibrecht auf
 * die Kinder, die jemand ohnehin sehen darf, und schaltet allein nichts frei.
 * Ohne dieses Paar hier landete eine Person auf einer Seite, deren Liste und
 * Zähler-Endpunkt ihr dauerhaft leer antworten.
 */
export function canReviewChangeRequests(session: Session | null): boolean {
  return (
    canReviewStudentDataRequests(session) ||
    (hasPermission(session, "users:absence") &&
      hasPermission(session, "users:read"))
  );
}
