import { ApiError, wireErrorCode } from "~/lib/api-error";

/**
 * Wer eine laufende Aktivität nur über die schulweite Übersicht sieht, darf
 * sie nicht bedienen (#3167). Die Liste sagt das selbst, und eine trotzdem
 * abgelehnte Aktion nennt diesen Grund über ihren Code
 * `timetable.operation_not_planned` (Katalogtext).
 */
export const TIMETABLE_VIEW_ONLY_NOTICE =
  "Sie sind für diese Aktivität nicht eingeplant. Nur eingeplante Betreuungskräfte können hier etwas eintragen oder ändern.";

/**
 * Erkennt die Ablehnung einer nicht eingeplanten Person an ihrem Code
 * (ADR 0006), nie am Text. Ein Admin ohne Profil als Betreuungskraft hat einen
 * eigenen Code (`timetable.no_staff_profile`) und fällt nicht hierunter.
 */
export function isTimetableOperationForbidden(err: unknown): boolean {
  return (
    err instanceof ApiError &&
    wireErrorCode(err.code) === "timetable.operation_not_planned"
  );
}
