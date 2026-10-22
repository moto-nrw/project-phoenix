import type { Session } from "next-auth";
import { hasPermission, isCaregiver } from "~/lib/auth-utils";

/**
 * Wer „Aktuelle Aufsicht“ öffnen darf, am Recht festgemacht und nicht am
 * Rollennamen: eine Schule benennt ihre Betreuungsrolle frei (#3469), und die
 * Rolle „Extern“ (#4573) hält nur `schedules:read`.
 *
 * Sitzungen, die vor dem Rechte-Claim ausgestellt wurden, tragen keine
 * Rechteliste; für sie bleibt der Rollenweg, wie im SupervisionProvider.
 */
function isLegacyCaregiver(session: Session | null): boolean {
  return !Array.isArray(session?.user?.permissions) && isCaregiver(session);
}

/**
 * Die volle Seite: GET /api/active/supervision-dashboard verlangt
 * `groups:read`. Das Recht zeigt die Anwesenheit der ganzen Schule.
 */
export function canReadSupervisionDashboard(session: Session | null): boolean {
  return hasPermission(session, "groups:read") || isLegacyCaregiver(session);
}

/**
 * Die Seite überhaupt: mit `groups:read` voll, mit `schedules:read` allein
 * die eigene laufende Aufsicht über die Betriebs-API des Betreuungsplans
 * (Kinderliste, an- und abmelden, beenden). Dasselbe Gate gilt für die
 * Einträge in Seitenleiste und unterer Leiste.
 */
export function canOpenActiveSupervision(session: Session | null): boolean {
  return (
    canReadSupervisionDashboard(session) ||
    hasPermission(session, "schedules:read")
  );
}
