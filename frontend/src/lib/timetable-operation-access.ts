/**
 * Wer eine laufende Aktivität nur über die schulweite Übersicht sieht, darf
 * sie nicht bedienen (#3167). Die Liste sagt das selbst, und eine trotzdem
 * abgelehnte Aktion nennt diesen Grund statt einer allgemeinen Meldung.
 */
export const TIMETABLE_VIEW_ONLY_NOTICE =
  "Sie sind für diese Aktivität nicht eingeplant. Nur eingeplante Betreuungskräfte können hier etwas eintragen oder ändern.";

export const TIMETABLE_OPERATION_FORBIDDEN_MESSAGE = `Das hat leider nicht geklappt. ${TIMETABLE_VIEW_ONLY_NOTICE}`;

/**
 * Erkennt die Ablehnung `timetable operation forbidden` (HTTP 403), mit der
 * das Backend eine nicht eingeplante Person abweist. Geprüft wird die Form von
 * `TimetableOperationsApiError` (`httpStatus` + Meldung), nicht die Klasse:
 * Aufrufer brauchen den API-Client dafür nicht zu importieren. Ein Admin ohne
 * Profil als Betreuungskraft (`… : no staff profile`) ist eingeplant genug und
 * bekommt diesen Grund deshalb nicht.
 */
export function isTimetableOperationForbidden(err: unknown): boolean {
  return (
    err instanceof Error &&
    (err as { httpStatus?: unknown }).httpStatus === 403 &&
    err.message.includes("timetable operation forbidden") &&
    !err.message.includes("no staff profile")
  );
}
