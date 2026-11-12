import type { Student } from "~/lib/student-helpers";
import { countedDayStatus } from "~/lib/location-helper";
import {
  getDayPlanningNotComingLabel,
  hasActualAttendance,
} from "~/lib/day-planning-helper";

/**
 * Whether „Alle auswählen“ in the check-in mode leaves a child out (#4762):
 * sick, excused (class trip included) or not expected today. Checking such a
 * child in by one tap on „Alle“ would book a child who is not there. A child
 * that came anyway is present, so neither its status (place before status,
 * #6371) nor the not-expected rule applies.
 */
export function isLeftOutOfSelectAll(student: Student): boolean {
  if (countedDayStatus(student) !== null) return true;
  if (getDayPlanningNotComingLabel(student) !== null) return true;
  return (
    (student.arrival_is_exception ?? false) &&
    !student.arrival_time &&
    !hasActualAttendance(student)
  );
}

interface SelectAllPlan {
  /** The visible children „Alle auswählen“ marks. */
  readonly ids: readonly string[];
  /** Visible children left out because they are absent today. */
  readonly leftOutCount: number;
}

/** Splits the children on screen into those „Alle“ marks and those it leaves out. */
export function planSelectAll(
  visibleStudents: readonly Student[],
): SelectAllPlan {
  const ids: string[] = [];
  let leftOutCount = 0;
  for (const student of visibleStudents) {
    if (isLeftOutOfSelectAll(student)) leftOutCount += 1;
    else ids.push(student.id.toString());
  }
  return { ids, leftOutCount };
}

/** Line under the bar's „Alle auswählen“, or null when nobody is left out. */
export function leftOutLabel(leftOutCount: number): string | null {
  if (leftOutCount === 0) return null;
  return `${leftOutCount} ausgelassen: krank, entschuldigt, auf Klassenfahrt oder „Kommt heute nicht“`;
}

/** What the check-in bars need to offer „Alle N auswählen“ (#4762). */
export interface SelectAllControl {
  /** Children the button marks: the visible ones minus those left out. */
  readonly count: number;
  readonly leftOutCount: number;
  /** Every child the button would mark is marked already. */
  readonly allSelected: boolean;
  readonly onSelectAll: () => void;
}

/**
 * Was ein Klick im An- und Abmelde-Modus tut (#6766). Leiste am Desktop,
 * Leiste am Handy und runder Knopf nutzen dieselben Sätze.
 */
export const CHECKIN_HINTS = {
  start: "Startet den Modus, in dem jeder Tipp ein Kind an- oder abmeldet.",
  finish: "Beendet den Modus, die bisherigen An- und Abmeldungen bleiben.",
  selectAll: "Wählt alle angezeigten Kinder aus, gemeldet wird noch nichts.",
  clearSelection: "Hebt die Auswahl auf, an den Kindern ändert sich nichts.",
  checkIn: "Meldet die ausgewählten Kinder in der Schule an.",
  checkOut:
    "Meldet die ausgewählten Kinder ab, bei Raumbesuchen fragt moto vorher nach.",
  noneSelected: "Wählen Sie zuerst mindestens ein Kind aus.",
} as const;

/** Warum „Alle N auswählen“ gerade nicht geht, sonst `undefined`. */
export function selectAllBlockedReason(
  control: Pick<SelectAllControl, "count" | "allSelected">,
): string | undefined {
  if (control.count === 0) return "In der Liste steht gerade kein Kind.";
  if (control.allSelected)
    return "Alle angezeigten Kinder sind schon ausgewählt.";
  return undefined;
}
