import type { Student } from "~/lib/api";
import {
  getDayPlanningNotComingLabel,
  hasActualAttendance,
} from "~/lib/day-planning-helper";

/**
 * Whether „Alle auswählen“ in the check-in mode leaves a child out (#4762):
 * sick, excused (class trip included) or not expected today. Checking such a
 * child in by one tap on „Alle“ would book a child who is not there. A child
 * that came anyway is present, so the not-expected rule no longer applies.
 */
export function isLeftOutOfSelectAll(student: Student): boolean {
  if (student.sick || student.excused || student.class_trip) return true;
  if (getDayPlanningNotComingLabel(student) !== null) return true;
  return (
    (student.arrival_is_exception ?? false) &&
    !student.arrival_time &&
    !hasActualAttendance(student)
  );
}

export interface SelectAllPlan {
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
  return `${leftOutCount} ausgelassen: krank, entschuldigt oder „Kommt heute nicht“`;
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
