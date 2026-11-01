import { schoolClassesLabel } from "~/lib/school-class-label";

/**
 * An offering whose booked children no Regeltermin of the planning period
 * takes over (#5140). The backend compares bookings with the Regeltermine
 * and the Klassentag (#5343): a child already planned at the time, or whose
 * class arrives only at the pickup time, is not listed. The school creates
 * or connects a Regeltermin, or decides that these children need none of
 * their own.
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
  /** Klassenfilter of a Regeltermin started from the hint; empty = none. */
  newSeriesClasses: string[];
  /**
   * Such a Regeltermin would plan children a second time, or on days their
   * class is still at school: the hint offers no creation then.
   */
  newSeriesPlansCovered: boolean;
}

interface BackendOfferingGap {
  offering: { id: number; name: string };
  student_count: number;
  school_classes?: string[] | null;
  weekdays?: number[] | null;
  acknowledged: boolean;
  acknowledged_classes?: string[] | null;
  new_series_classes?: string[] | null;
  new_series_plans_covered?: boolean | null;
}

export interface BackendOfferingGapsResponse {
  gaps?: BackendOfferingGap[] | null;
  phases_outside_period?: BackendOfferingPhaseOutsidePeriod[] | null;
}

/**
 * An enrollment phase whose service window reaches beyond the planning
 * period (#5344). Its active offerings are left out of the period's source
 * choice and its booking checks, so the screen names it instead of showing
 * an empty list that reads as „alles in Ordnung“.
 */
export interface OfferingPhaseOutsidePeriod {
  phaseId: string;
  phaseName: string;
  /** Service window of the phase, YYYY-MM-DD. */
  serviceStart: string;
  serviceEnd: string;
  /** Active offerings of the phase the period leaves out. */
  offeringCount: number;
  /** The planning period's own dates, YYYY-MM-DD. */
  periodStart: string;
  periodEnd: string;
}

export interface BackendOfferingPhaseOutsidePeriod {
  phase_id: number;
  phase_name: string;
  service_start: string;
  service_end: string;
  offering_count: number;
  period_start: string;
  period_end: string;
}

export function mapPhasesOutsidePeriod(
  raw: BackendOfferingPhaseOutsidePeriod[] | null | undefined,
): OfferingPhaseOutsidePeriod[] {
  return (raw ?? []).map((phase) => ({
    phaseId: String(phase.phase_id),
    phaseName: phase.phase_name,
    serviceStart: phase.service_start,
    serviceEnd: phase.service_end,
    offeringCount: phase.offering_count,
    periodStart: phase.period_start,
    periodEnd: phase.period_end,
  }));
}

/** The gap check of a period with the phases it could not check (#5344). */
export interface OfferingGapReport {
  gaps: OfferingGap[];
  phasesOutsidePeriod: OfferingPhaseOutsidePeriod[];
}

export function mapOfferingGapReport(
  raw: BackendOfferingGapsResponse,
): OfferingGapReport {
  return {
    gaps: mapOfferingGaps(raw),
    phasesOutsidePeriod: mapPhasesOutsidePeriod(raw.phases_outside_period),
  };
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
    newSeriesClasses: gap.new_series_classes ?? [],
    // An unknown answer offers no creation rather than a second list.
    newSeriesPlansCovered: gap.new_series_plans_covered ?? true,
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
