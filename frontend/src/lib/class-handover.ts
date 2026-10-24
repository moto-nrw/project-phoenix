// Übergabe nach dem Unterricht (#4794) in den Ansichten der OGS.
//
// Gruppe, Kindersuche und Aufsicht zeigen an jedem Kind, das die Lehrkraft
// beim Übergeben ihrer Klasse als „nicht da“ markiert hat, wann und von wem
// die Klasse übergeben wurde. Die Daten kommen mit der jeweiligen Liste
// (`handover_absent`); ein SSE-Ereignis lädt die Listen nach einer Übergabe
// neu.

import { berlinClockFromISO } from "~/lib/date-helpers";

/** Ein Kind, das bei der heutigen Übergabe seiner Klasse nicht da war. */
export interface ClassHandoverAbsence {
  school_class: string;
  handed_over_at: string; // ISO-Zeitstempel
  // Name der Lehrkraft; fehlt, wenn sie nicht mehr angelegt ist.
  handed_over_by?: string;
}

/**
 * Der Wortlaut vor der Klammer der Zeile am Kind. Bewusst ohne „Übergabe“:
 * im OGS-Portal heißt so schon die Gruppenübergabe an eine Vertretung.
 */
export const HANDOVER_ABSENT_WORDING = "Nach dem Unterricht nicht da";

/** Die Klammer der Zeile: „11:45 Uhr, Petra Lehrkraft“. */
export function handoverAbsenceDetail(absence: ClassHandoverAbsence): string {
  const clock = berlinClockFromISO(absence.handed_over_at);
  const parts = [clock ? `${clock} Uhr` : `Klasse ${absence.school_class}`];
  if (absence.handed_over_by) parts.push(absence.handed_over_by);
  return parts.join(", ");
}
