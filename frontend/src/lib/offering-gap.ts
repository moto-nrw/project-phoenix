import { schoolClassesLabel } from "~/lib/school-class-label";

/**
 * An offering whose booked children no Regeltermin of the planning period
 * takes over (#5140). The backend compares bookings and Regeltermine; the
 * school creates or connects a Regeltermin, or decides that these children
 * need none of their own.
 */
export interface OfferingGap {
  offering: { id: string; name: string };
  /** Children with at least one booked school day without a Regeltermin. */
  studentCount: number;
  /** Their classes, sorted; "" stands for children without a class. */
  schoolClasses: string[];
  /** The uncovered ISO weekdays, 1 = Monday. */
  weekdays: number[];
  /** The school decided that every listed class needs no Regeltermin. */
  acknowledged: boolean;
  /** The classes of the stored decision, also when a new class came up. */
  acknowledgedClasses: string[];
}

interface BackendOfferingGap {
  offering: { id: number; name: string };
  student_count: number;
  school_classes?: string[] | null;
  weekdays?: number[] | null;
  acknowledged: boolean;
  acknowledged_classes?: string[] | null;
}

export interface BackendOfferingGapsResponse {
  gaps?: BackendOfferingGap[] | null;
}

export function mapOfferingGaps(
  raw: BackendOfferingGapsResponse,
): OfferingGap[] {
  return (raw.gaps ?? []).map((gap) => ({
    offering: { id: String(gap.offering.id), name: gap.offering.name },
    studentCount: gap.student_count,
    schoolClasses: gap.school_classes ?? [],
    weekdays: gap.weekdays ?? [],
    acknowledged: gap.acknowledged,
    acknowledgedClasses: gap.acknowledged_classes ?? [],
  }));
}

const WEEKDAY_SHORT = ["", "Mo", "Di", "Mi", "Do", "Fr", "Sa", "So"];

/** „1 Kind“ or „46 Kinder“. */
export function describeGapChildren(count: number): string {
  return count === 1 ? "1 Kind" : `${count} Kinder`;
}

/**
 * The classes in short and in full. Up to three classes stand as they are
 * („Klassen 3a, 3b, 4a“); more read „Klassen 1a, 1b und 6 weitere“, the full
 * list goes into the tooltip. Children without a class are named last.
 */
export function summarizeGapClasses(schoolClasses: string[]): {
  short: string;
  full: string;
} {
  const named = schoolClasses.filter((c) => c.trim() !== "");
  const withoutClass = named.length < schoolClasses.length;
  if (named.length === 0) {
    return { short: "Kinder ohne Klasse", full: "Kinder ohne Klasse" };
  }
  const suffix = withoutClass ? " und Kinder ohne Klasse" : "";
  const full = schoolClassesLabel(named) + suffix;
  const short =
    named.length > 3
      ? schoolClassesLabel(
          named,
          (names) =>
            `${names.slice(0, 2).join(", ")} und ${names.length - 2} weitere`,
        ) + suffix
      : full;
  return { short, full };
}

/** „Mo–Fr“ for a run of days, else „Mo, Mi“. */
export function describeGapWeekdays(weekdays: number[]): string {
  const sorted = [...weekdays].sort((a, b) => a - b);
  const contiguous = sorted.every(
    (day, i) => i === 0 || day === (sorted[i - 1] ?? 0) + 1,
  );
  if (sorted.length >= 3 && contiguous) {
    return `${WEEKDAY_SHORT[sorted[0] ?? 0]}–${WEEKDAY_SHORT[sorted[sorted.length - 1] ?? 0]}`;
  }
  return sorted.map((day) => WEEKDAY_SHORT[day] ?? "").join(", ");
}
