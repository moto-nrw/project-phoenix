// Übergabe nach dem Unterricht (#4794) in den Ansichten der OGS.
//
// Gruppe, Kindersuche und Aufsicht zeigen an jedem Kind, das die Lehrkraft
// mit ihrer Klasse an die OGS übergeben hat, ob es „da“ oder „nicht da“ war
// und wann und von wem die Klasse übergeben wurde. Die Daten kommen mit der
// jeweiligen Liste (`class_handover`); ein SSE-Ereignis lädt die Listen nach
// einer Übergabe neu.

import { berlinClockFromISO } from "~/lib/date-helpers";

/** Die heutige Übergabe eines Kindes mit seiner Klasse. */
export interface ClassHandover {
  school_class: string;
  handed_over_at: string; // ISO-Zeitstempel
  // Name der Lehrkraft; fehlt, wenn sie nicht mehr angelegt ist.
  handed_over_by?: string;
  // Die Lehrkraft hat das Kind als „nicht da“ markiert.
  absent: boolean;
}

/**
 * Der Wortlaut vor der Klammer der Zeile am Kind. Bewusst ohne „Übergabe“:
 * im OGS-Portal heißt so schon die Gruppenübergabe an eine Vertretung.
 */
export const HANDOVER_ABSENT_WORDING = "Nach dem Unterricht nicht da";
export const HANDOVER_PRESENT_WORDING = "Nach dem Unterricht da";

/** „Nach dem Unterricht da“ oder „Nach dem Unterricht nicht da“. */
export function handoverWording(handover: ClassHandover): string {
  return handover.absent ? HANDOVER_ABSENT_WORDING : HANDOVER_PRESENT_WORDING;
}

/** Die Klammer der Zeile: „11:45 Uhr, Petra Lehrkraft“. */
export function handoverDetail(handover: ClassHandover): string {
  const clock = berlinClockFromISO(handover.handed_over_at);
  const parts = [clock ? `${clock} Uhr` : `Klasse ${handover.school_class}`];
  if (handover.handed_over_by) parts.push(handover.handed_over_by);
  return parts.join(", ");
}

/** Die ganze Zeile: „Nach dem Unterricht da (11:45 Uhr, Petra Lehrkraft)“. */
export function handoverLine(handover: ClassHandover): string {
  return `${handoverWording(handover)} (${handoverDetail(handover)})`;
}
