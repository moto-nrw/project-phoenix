import type { Session } from "next-auth";
import { canReadSupervisionData, hasPermission } from "~/lib/auth-utils";

/**
 * Wer „Aktuelle Aufsicht“ öffnen darf, am Recht festgemacht und nicht am
 * Rollennamen: mit `canReadSupervisionData` voll, mit `schedules:read` allein
 * (#4573) die eigene laufende Aufsicht über die Betriebs-API des
 * Betreuungsplans (Kinderliste, an- und abmelden, beenden). Dasselbe Gate gilt
 * für die Einträge in Seitenleiste und unterer Leiste.
 */
export function canOpenActiveSupervision(session: Session | null): boolean {
  return (
    canReadSupervisionData(session) || hasPermission(session, "schedules:read")
  );
}
