// Calendar period (Kalenderperiode) — domain types and mappers.
//
// Calendar periods anchor the materialization service: without an active
// period covering a candidate date, materialization silently no-ops. They
// also drive A/B week resolution via week_cycle_length + week_cycle_anchor.
//
// Backend reference: backend/models/schedule/calendar_period.go,
// backend/api/timetable/api.go (CalendarPeriodRequest / Response).

export type PeriodType = "school_year" | "semester" | "holiday" | "custom";

export const PERIOD_TYPES: PeriodType[] = [
  "school_year",
  "semester",
  "holiday",
  "custom",
];

export const PERIOD_TYPE_LABELS: Record<PeriodType, string> = {
  school_year: "Schuljahr",
  semester: "Halbjahr",
  holiday: "Ferien",
  custom: "Sonstiges",
};

/**
 * Soft warning attached to period create/update responses, e.g. when the
 * saved period overlaps other active periods. Advisory only — the save
 * has already succeeded when warnings arrive.
 */
export interface BackendCalendarPeriodWarning {
  code: "overlapping_active_periods" | (string & {});
  message: string;
  overlapping_period_ids?: number[];
  overlapping_period_names?: string[];
}

export interface CalendarPeriodWarning {
  code: "overlapping_active_periods" | (string & {});
  message: string;
  overlappingPeriodIds: string[];
  overlappingPeriodNames: string[];
}

/** Backend wire shape (snake_case, int64 IDs). */
export interface BackendCalendarPeriod {
  id: number;
  tenant_id: number;
  name: string;
  period_type: PeriodType;
  start_date: string; // YYYY-MM-DD
  end_date: string; // YYYY-MM-DD
  week_cycle_length: number;
  week_cycle_anchor?: string | null; // YYYY-MM-DD
  is_active: boolean;
  created_at: string;
  updated_at: string;
  /** Only present on create/update responses. */
  warnings?: BackendCalendarPeriodWarning[] | null;
  /** Advisory reference counts used for usage and delete-impact copy. */
  enrollment_phase_count?: number;
  activity_group_count?: number;
  schedule_count?: number;
  student_enrollment_count?: number;
  supervisor_count?: number;
  activity_instance_count?: number;
}

/** Frontend shape (camelCase, string IDs). */
export interface CalendarPeriod {
  id: string;
  tenantId: string;
  name: string;
  periodType: PeriodType;
  startDate: string; // YYYY-MM-DD
  endDate: string; // YYYY-MM-DD
  weekCycleLength: number;
  weekCycleAnchor: string | null;
  isActive: boolean;
  createdAt: string;
  updatedAt: string;
  /**
   * How many Anmeldephasen reference this period (advisory). Optional so
   * fixtures and older callers stay valid; mapPeriod always sets it.
   */
  enrollmentPhaseCount?: number;
  /** How many activity templates use this as their template-level period. */
  activityGroupCount?: number;
  /** How many Regeltermine (activities.schedules) reference this period (advisory). */
  scheduleCount?: number;
  /** How many student roster rows reference this period (advisory). */
  studentEnrollmentCount?: number;
  /** How many staff roster rows reference this period (advisory). */
  supervisorCount?: number;
  /** How many materialized appointments reference this period (advisory). */
  activityInstanceCount?: number;
}

/** POST/PUT body — backend expects YYYY-MM-DD strings, not Date objects. */
export interface CalendarPeriodInput {
  name: string;
  period_type: PeriodType;
  start_date: string;
  end_date: string;
  week_cycle_length: number;
  week_cycle_anchor?: string;
  is_active: boolean;
}

export function mapPeriod(raw: BackendCalendarPeriod): CalendarPeriod {
  return {
    id: String(raw.id),
    tenantId: String(raw.tenant_id),
    name: raw.name,
    periodType: raw.period_type,
    startDate: raw.start_date,
    endDate: raw.end_date,
    weekCycleLength: raw.week_cycle_length,
    weekCycleAnchor: raw.week_cycle_anchor ?? null,
    isActive: raw.is_active,
    createdAt: raw.created_at,
    updatedAt: raw.updated_at,
    enrollmentPhaseCount: raw.enrollment_phase_count ?? 0,
    activityGroupCount: raw.activity_group_count ?? 0,
    scheduleCount: raw.schedule_count ?? 0,
    studentEnrollmentCount: raw.student_enrollment_count ?? 0,
    supervisorCount: raw.supervisor_count ?? 0,
    activityInstanceCount: raw.activity_instance_count ?? 0,
  };
}

export function mapPeriods(raw: BackendCalendarPeriod[]): CalendarPeriod[] {
  return raw.map(mapPeriod);
}

export function mapPeriodWarnings(
  raw: BackendCalendarPeriodWarning[] | null | undefined,
): CalendarPeriodWarning[] {
  return (raw ?? []).map((warning) => ({
    code: warning.code,
    message: warning.message,
    overlappingPeriodIds: (warning.overlapping_period_ids ?? []).map(String),
    overlappingPeriodNames: warning.overlapping_period_names ?? [],
  }));
}

/** Result of a period create/update — the saved period plus soft warnings. */
export interface CalendarPeriodSaveResult {
  period: CalendarPeriod;
  warnings: CalendarPeriodWarning[];
}

export function mapPeriodWithWarnings(
  raw: BackendCalendarPeriod,
): CalendarPeriodSaveResult {
  return {
    period: mapPeriod(raw),
    warnings: mapPeriodWarnings(raw.warnings),
  };
}

/**
 * Raw 1-based slot of the week containing isoDate inside the period's week
 * cycle (1=A, 2=B, 3=C, …). Mirrors the backend predicate
 * ShouldMaterializeWeekPattern (calendar_period_service.go): floor the day
 * difference to the cycle anchor by 7, then a 1-based modulo over the cycle
 * length. Returns null when the period has no usable cycle (length <= 1 or no
 * anchor — materialization is fail-open there) or the date is invalid.
 * Slots beyond 2 exist for cycles longer than two weeks and can never be
 * addressed by the biweekly A/B week_pattern — callers must distinguish
 * "no cycle configured" (null) from "week beyond B" (slot > 2).
 */
export function weekCycleSlotForDate(
  period: CalendarPeriod | null | undefined,
  isoDate: string,
): number | null {
  if (!period || period.weekCycleLength <= 1 || !period.weekCycleAnchor) {
    return null;
  }
  const toUTCDays = (iso: string): number => {
    const [year, month, day] = iso.split("-").map(Number);
    if (!year || !month || !day) return Number.NaN;
    const utcMs = Date.UTC(year, month - 1, day);
    // Date.UTC silently rolls invalid components over (2026-02-30 becomes
    // March 2nd); only a lossless round-trip proves the date is real.
    const roundTrip = new Date(utcMs);
    if (
      roundTrip.getUTCFullYear() !== year ||
      roundTrip.getUTCMonth() !== month - 1 ||
      roundTrip.getUTCDate() !== day
    ) {
      return Number.NaN;
    }
    return Math.floor(utcMs / 86_400_000);
  };
  const daysDiff = toUTCDays(isoDate) - toUTCDays(period.weekCycleAnchor);
  if (Number.isNaN(daysDiff)) return null;
  const cycle = period.weekCycleLength;
  return (((Math.floor(daysDiff / 7) % cycle) + cycle) % cycle) + 1;
}

/**
 * Resolves whether the week containing isoDate is Woche A (1) or Woche B (2)
 * inside the period's A/B cycle. Returns null when the period has no usable
 * cycle or when the resolved slot is beyond B (cycle length > 2) — use
 * weekCycleSlotForDate to tell those cases apart.
 */
export function weekPatternForDate(
  period: CalendarPeriod | null | undefined,
  isoDate: string,
): 1 | 2 | null {
  const slot = weekCycleSlotForDate(period, isoDate);
  if (slot === 1) return 1;
  if (slot === 2) return 2;
  return null;
}

/**
 * Mirrors backend ShouldMaterializeWeekPattern for advisory frontend checks.
 * A missing cycle remains fail-open; in longer cycles, A/B schedules skip
 * slots C and beyond exactly like the materializer.
 */
export function shouldMaterializeWeekPattern(
  period: CalendarPeriod | null | undefined,
  isoDate: string,
  weekPattern: number,
): boolean {
  if (weekPattern === 0) return true;
  const slot = weekCycleSlotForDate(period, isoDate);
  return slot === null || slot === weekPattern;
}

/**
 * Letter for a 1-based week-cycle slot (1=A, 2=B, 3=C, …). Falls back to the
 * numeric slot for cycles longer than 26 weeks (the UI caps at 4, the backend
 * only enforces >= 1).
 */
export function weekCycleSlotLetter(slot: number): string {
  return slot >= 1 && slot <= 26
    ? String.fromCharCode(64 + slot)
    : String(slot);
}

/**
 * Display label for a period's week cycle ("A/B-Wochen", "A/B/C-Wochen", …).
 * Returns null when the period repeats weekly (no cycle to label).
 */
export function weekCycleLabel(cycleLength: number): string | null {
  if (cycleLength <= 1) return null;
  // The editor supports up to four slots. Persisted/API values may be much
  // larger, so never allocate or join an array proportional to untrusted data.
  if (cycleLength > 4) return `${cycleLength}-Wochen-Zyklus`;
  const letters = Array.from({ length: cycleLength }, (_, i) =>
    weekCycleSlotLetter(i + 1),
  );
  return `${letters.join("/")}-Wochen`;
}

export function findPeriodForDate(
  periods: CalendarPeriod[],
  isoDate: string,
): CalendarPeriod | null {
  const matches = periods
    .filter((p) => p.isActive)
    .filter((p) => p.startDate <= isoDate && isoDate <= p.endDate)
    .sort((a, b) => Number(a.id) - Number(b.id));

  return matches[0] ?? null;
}

interface DayPeriodAssignment {
  date: string;
  period: CalendarPeriod | null;
}

export function mapPeriodsForDates(
  periods: CalendarPeriod[],
  isoDates: string[],
): DayPeriodAssignment[] {
  return isoDates.map((date) => ({
    date,
    period: findPeriodForDate(periods, date),
  }));
}

export function uniqueAssignedPeriods(
  assignments: DayPeriodAssignment[],
): CalendarPeriod[] {
  const byID = new Map<string, CalendarPeriod>();
  for (const assignment of assignments) {
    if (assignment.period) {
      byID.set(assignment.period.id, assignment.period);
    }
  }
  return [...byID.values()].sort((a, b) => Number(a.id) - Number(b.id));
}

/**
 * Formats "1 Anmeldephase · 3 Regeltermine" from the advisory reference
 * counts. Returns an empty string when nothing references the period —
 * callers render their own "Nicht verwendet" fallback.
 */
export function formatPeriodUsage(
  enrollmentPhaseCount: number,
  scheduleCount: number,
  separator = " · ",
  extra?: {
    activityGroupCount?: number;
    studentEnrollmentCount?: number;
    supervisorCount?: number;
    activityInstanceCount?: number;
  },
): string {
  const parts: string[] = [];
  if (enrollmentPhaseCount > 0) {
    parts.push(
      enrollmentPhaseCount === 1
        ? "1 Anmeldephase"
        : `${enrollmentPhaseCount} Anmeldephasen`,
    );
  }
  const activityGroupCount = extra?.activityGroupCount ?? 0;
  if (activityGroupCount > 0) {
    parts.push(
      activityGroupCount === 1
        ? "1 Aktivitätsvorlage"
        : `${activityGroupCount} Aktivitätsvorlagen`,
    );
  }
  if (scheduleCount > 0) {
    parts.push(
      scheduleCount === 1 ? "1 Regeltermin" : `${scheduleCount} Regeltermine`,
    );
  }
  const studentEnrollmentCount = extra?.studentEnrollmentCount ?? 0;
  if (studentEnrollmentCount > 0) {
    parts.push(
      studentEnrollmentCount === 1
        ? "1 Schülerzuordnung"
        : `${studentEnrollmentCount} Schülerzuordnungen`,
    );
  }
  const supervisorCount = extra?.supervisorCount ?? 0;
  if (supervisorCount > 0) {
    parts.push(
      supervisorCount === 1
        ? "1 Mitarbeitenden-Zuordnung"
        : `${supervisorCount} Mitarbeitenden-Zuordnungen`,
    );
  }
  const activityInstanceCount = extra?.activityInstanceCount ?? 0;
  if (activityInstanceCount > 0) {
    parts.push(
      activityInstanceCount === 1
        ? "1 Termininstanz"
        : `${activityInstanceCount} Termininstanzen`,
    );
  }
  return parts.join(separator);
}

/**
 * Formats "01.09.2025 – 31.07.2026" for the list view. Both dates are
 * already YYYY-MM-DD so we splice without going through Date.
 */
export function formatPeriodRange(p: CalendarPeriod): string {
  return `${germanDate(p.startDate)} – ${germanDate(p.endDate)}`;
}

function germanDate(iso: string): string {
  const [y, m, d] = iso.split("-");
  return `${d}.${m}.${y}`;
}
