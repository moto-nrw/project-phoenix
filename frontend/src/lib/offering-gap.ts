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

/** „3a, 3b, ohne Klasse“: the classes as the school names them. */
export function describeGapClasses(schoolClasses: string[]): string {
  const named = schoolClasses
    .filter((schoolClass) => schoolClass !== "")
    .map((schoolClass) => schoolClass.replace(/^klasse\s+/i, ""));
  if (schoolClasses.includes("")) named.push("ohne Klasse");
  const label = named.length === 1 ? "Klasse" : "Klassen";
  return schoolClasses.length === 1 && schoolClasses[0] === ""
    ? "Kinder ohne Klasse"
    : `${label} ${named.join(", ")}`;
}

/** The hint in two short sentences: who is affected, then where and when. */
export function describeOfferingGap(gap: OfferingGap): string {
  const children =
    gap.studentCount === 1 ? "1 Kind hat" : `${gap.studentCount} Kinder haben`;
  const days = gap.weekdays
    .map((weekday) => WEEKDAY_SHORT[weekday] ?? "")
    .join(", ");
  return `${children} „${gap.offering.name}“ gebucht, aber keinen Regeltermin. ${describeGapClasses(gap.schoolClasses)} · ${days}`;
}
