// Types and mapping helpers for planned staff shifts (Dienstplan, #1376).
// Backend wire format: snake_case, bigint shift/series IDs as strings, dates as
// "YYYY-MM-DD", times as "HH:MM".

export interface BackendStaffShift {
  id: string;
  staff_id: number;
  date: string;
  start_time: string;
  end_time: string;
  break_minutes: number;
  shift_type_id?: number | null;
  /** Resolved Schichtart, embedded so staff (who cannot read /shift-types) still
   *  see the label and color (#1844). Present only when the shift has a type. */
  shift_type_name?: string | null;
  shift_type_color?: string | null;
  notes?: string;
  series_id?: string | null;
  /** Immutable source slot for a moved series occurrence. */
  series_occurrence_date?: string | null;
  detached?: boolean;
  cancelled?: boolean;
  change_reason?: string | null;
  origin_shift_id?: string | null;
}

export interface StaffShift {
  id: string;
  staffId: string;
  /** Calendar day as "YYYY-MM-DD" */
  date: string;
  /** Wall-clock "HH:MM" */
  startTime: string;
  /** Wall-clock "HH:MM" */
  endTime: string;
  breakMinutes: number;
  /** Id of the linked shift type (Schichtart), or null if untyped */
  shiftTypeId: string | null;
  /** Resolved Schichtart label (#1844), or null when untyped. */
  shiftTypeName: string | null;
  /** Resolved Schichtart color as "#RRGGBB" (#1844), or null when untyped. */
  shiftTypeColor: string | null;
  notes: string;
  /** Id of the shift series this row was materialized from (#1889), or null
   *  for a standalone shift. */
  seriesId: string | null;
  /** Original recurrence slot for a moved series occurrence, or null for a
   * standalone shift. */
  seriesOccurrenceDate?: string | null;
  /** True when the row was edited via "Nur diese Woche" — series re-plans
   *  leave it alone. */
  detached: boolean;
  /** True when the shift does not take place: the staff member is absent or
   *  the gap is deliberately left open (#1841). Excluded from planned minutes. */
  cancelled: boolean;
  /** Optional "why" for a flexible daily change (moved times, cancellation, or
   *  replacement, #1841). */
  changeReason: string | null;
  /** When set, this shift covers another (cancelled) shift as a replacement;
   *  several replacements sharing one origin split a gap across people (#1841). */
  originShiftId: string | null;
}

type CoverageStatus = "covered" | "uncovered" | "not_applicable";

type CoverageReason = "absent" | "dienstplan_not_used" | null;

interface BackendUncoveredInterval {
  start_time: string;
  end_time: string;
}

interface UncoveredInterval {
  /** Wall-clock "HH:MM" */
  startTime: string;
  /** Wall-clock "HH:MM" */
  endTime: string;
}

interface BackendStaffScheduleStaff {
  id: number;
  first_name: string;
  last_name: string;
}

export interface StaffScheduleStaff {
  id: string;
  firstName: string;
  lastName: string;
}

interface BackendStaffScheduleAssignment {
  instance_id: number;
  staff_id: number;
  date: string;
  start_time: string;
  end_time: string;
  activity_title: string;
  room_id: number;
  room_name: string;
  status: string;
  is_absent: boolean;
  is_substitute: boolean;
  absence_reason: string | null;
  coverage_status: CoverageStatus;
  coverage_reason: CoverageReason;
  uncovered_intervals: BackendUncoveredInterval[];
}

export interface StaffScheduleAssignment {
  instanceId: string;
  staffId: string;
  /** Calendar day as "YYYY-MM-DD" */
  date: string;
  /** Wall-clock "HH:MM" */
  startTime: string;
  /** Wall-clock "HH:MM" */
  endTime: string;
  activityTitle: string;
  /** The effective room for this concrete staff assignment. */
  roomId: string;
  /** The effective room for this concrete staff assignment. */
  roomName: string;
  status: string;
  isAbsent: boolean;
  isSubstitute: boolean;
  absenceReason: string | null;
  coverageStatus: CoverageStatus;
  coverageReason: CoverageReason;
  uncoveredIntervals: UncoveredInterval[];
}

interface BackendStaffWeeklySummary {
  staff_id: number;
  week_start: string;
  planned_minutes: number;
  target_minutes: number | null;
  delta_minutes: number | null;
}

export interface StaffWeeklySummary {
  staffId: string;
  /** Monday of the summarized calendar week as "YYYY-MM-DD". */
  weekStart: string;
  /** Net shift minutes (span minus break) planned in that week. */
  plannedMinutes: number;
  /** Contractual weekly minutes (Arbeitszeitmodell); null when none resolves. */
  targetMinutes: number | null;
  /** plannedMinutes - targetMinutes; null when targetMinutes is null. */
  deltaMinutes: number | null;
}

export interface BackendStaffScheduleOverview {
  from: string;
  to: string;
  dienstplan_in_use: boolean;
  dienstplan_used_weeks?: string[];
  staff: BackendStaffScheduleStaff[];
  shifts: BackendStaffShift[];
  assignments: BackendStaffScheduleAssignment[];
  weekly_summaries?: BackendStaffWeeklySummary[];
}

export interface StaffScheduleOverview {
  /** Inclusive calendar-day range start as "YYYY-MM-DD". */
  from: string;
  /** Inclusive calendar-day range end as "YYYY-MM-DD". */
  to: string;
  dienstplanInUse: boolean;
  dienstplanUsedWeeks: string[];
  staff: StaffScheduleStaff[];
  shifts: StaffShift[];
  assignments: StaffScheduleAssignment[];
  weeklySummaries: StaffWeeklySummary[];
}

export function mapStaffShift(data: BackendStaffShift): StaffShift {
  return {
    id: data.id.toString(),
    staffId: data.staff_id.toString(),
    date: data.date.slice(0, 10),
    startTime: data.start_time.slice(0, 5),
    endTime: data.end_time.slice(0, 5),
    breakMinutes: data.break_minutes,
    shiftTypeId:
      data.shift_type_id != null ? data.shift_type_id.toString() : null,
    shiftTypeName: data.shift_type_name ?? null,
    shiftTypeColor: data.shift_type_color ?? null,
    notes: data.notes ?? "",
    seriesId: data.series_id != null ? data.series_id.toString() : null,
    ...(data.series_occurrence_date != null
      ? { seriesOccurrenceDate: data.series_occurrence_date.slice(0, 10) }
      : {}),
    detached: data.detached ?? false,
    cancelled: data.cancelled ?? false,
    changeReason: data.change_reason ?? null,
    originShiftId:
      data.origin_shift_id != null ? data.origin_shift_id.toString() : null,
  };
}

export function mapStaffScheduleOverview(
  data: BackendStaffScheduleOverview,
): StaffScheduleOverview {
  return {
    from: data.from.slice(0, 10),
    to: data.to.slice(0, 10),
    dienstplanInUse: data.dienstplan_in_use,
    dienstplanUsedWeeks: (data.dienstplan_used_weeks ?? []).map((week) =>
      week.slice(0, 10),
    ),
    staff: data.staff.map((member) => ({
      id: member.id.toString(),
      firstName: member.first_name,
      lastName: member.last_name,
    })),
    shifts: data.shifts.map(mapStaffShift),
    assignments: data.assignments.map((assignment) => ({
      instanceId: assignment.instance_id.toString(),
      staffId: assignment.staff_id.toString(),
      date: assignment.date.slice(0, 10),
      startTime: assignment.start_time.slice(0, 5),
      endTime: assignment.end_time.slice(0, 5),
      activityTitle: assignment.activity_title,
      roomId: assignment.room_id.toString(),
      roomName: assignment.room_name,
      status: assignment.status,
      isAbsent: assignment.is_absent,
      isSubstitute: assignment.is_substitute,
      absenceReason: assignment.absence_reason,
      coverageStatus: assignment.coverage_status,
      coverageReason: assignment.coverage_reason,
      uncoveredIntervals: assignment.uncovered_intervals.map((interval) => ({
        startTime: interval.start_time.slice(0, 5),
        endTime: interval.end_time.slice(0, 5),
      })),
    })),
    weeklySummaries: (data.weekly_summaries ?? []).map((summary) => ({
      staffId: summary.staff_id.toString(),
      weekStart: summary.week_start.slice(0, 10),
      plannedMinutes: summary.planned_minutes,
      targetMinutes: summary.target_minutes,
      deltaMinutes: summary.delta_minutes,
    })),
  };
}

// ─── Own Betreuungsplan assignments ("Mein Tag", #1844) ─────────────────────
// The self-scoped GET /api/time-tracking/assignments shape. Distinct from the
// admin StaffScheduleAssignment above (that one carries coverage math against
// the Dienstplan; this one carries the block-level Vertretungsplan state a
// staff member sees for their own day).

export interface BackendOwnAssignment {
  instance_id: number;
  title: string;
  group_name?: string | null;
  room_name?: string;
  date: string;
  start_time: string;
  end_time: string;
  status: string;
  cancelled: boolean;
  is_primary: boolean;
  is_substitute: boolean;
  is_absent: boolean;
  absence_reason?: string | null;
  cancel_reason?: string | null;
  understaffed_ack: boolean;
}

export interface OwnAssignment {
  instanceId: string;
  /** Block title (activity instance title). */
  title: string;
  /** Activity/Betreuungsgruppe name, or null for a spontaneous block. */
  groupName: string | null;
  /** Effective room name for this block, or "" when unresolved. */
  roomName: string;
  /** Calendar day as "YYYY-MM-DD". */
  date: string;
  /** Wall-clock "HH:MM". */
  startTime: string;
  /** Wall-clock "HH:MM". */
  endTime: string;
  status: string;
  /** The block does not take place ("fällt aus"). */
  cancelled: boolean;
  isPrimary: boolean;
  /** This staff member stands in for the block (Vertretung, #1840). */
  isSubstitute: boolean;
  /** This staff member was pulled from the block (Vertretungsplan, #1840). */
  isAbsent: boolean;
  absenceReason: string | null;
  cancelReason: string | null;
  /** Admin acknowledged the block running understaffed (#1840). */
  understaffedAck: boolean;
}

export function mapOwnAssignment(data: BackendOwnAssignment): OwnAssignment {
  return {
    instanceId: data.instance_id.toString(),
    title: data.title,
    groupName: data.group_name ?? null,
    roomName: data.room_name ?? "",
    date: data.date.slice(0, 10),
    startTime: data.start_time.slice(0, 5),
    endTime: data.end_time.slice(0, 5),
    status: data.status,
    cancelled: data.cancelled,
    isPrimary: data.is_primary,
    isSubstitute: data.is_substitute,
    isAbsent: data.is_absent,
    absenceReason: data.absence_reason ?? null,
    cancelReason: data.cancel_reason ?? null,
    understaffedAck: data.understaffed_ack,
  };
}

const HOURS_FORMAT = new Intl.NumberFormat("de-DE", {
  maximumFractionDigits: 2,
});

/** 1215 → "20,25 h", 2400 → "40 h" */
export function formatPlannedHours(minutes: number): string {
  return `${HOURS_FORMAT.format(minutes / 60)} h`;
}

/** -300 → "−5 h", 90 → "+1,5 h", 0 → "±0 h" (U+2212 minus sign) */
export function formatDeltaHours(minutes: number): string {
  if (minutes === 0) return "±0 h";
  const sign = minutes > 0 ? "+" : "−";
  return `${sign}${HOURS_FORMAT.format(Math.abs(minutes) / 60)} h`;
}

/** "08:00–16:00" */
export function formatShiftLabel(shift: StaffShift): string {
  return `${shift.startTime}–${shift.endTime}`;
}

/** "2026-07-13" → "13.07." — the day/month sublabel of a grid column. */
export function formatColumnDate(isoDate: string): string {
  const [, m, d] = isoDate.split("-");
  return `${d}.${m}.`;
}

/**
 * Delta tone of a weekly summary label: under contract → "under" (red), over →
 * "over" (amber), exact or no target → "neutral". Structurally identical to the
 * ui kit's `CoverageTone` — the grids pass this value straight to
 * `CoverageIndicator`'s `tone` — but declared here so lib never imports from
 * components (the dependency direction must stay one-way).
 */
export type SummaryTone = "neutral" | "under" | "over";

/**
 * The minimal delta shape shared by the week-view row-header summary
 * (`StaffWeeklySummary`) and the half-year cell summary. Both satisfy it
 * structurally, so `summaryTone`/`summaryLabel` work for either.
 */
export interface SummaryDelta {
  plannedMinutes: number;
  targetMinutes: number | null;
  deltaMinutes: number | null;
}

// Under contract → red, over → amber, exact/no target → neutral (docs/05
// Abschnitt 2.3). Only the free-text label of the CoverageIndicator is tinted.
export function summaryTone(summary: SummaryDelta): SummaryTone {
  if (summary.targetMinutes === null || summary.deltaMinutes === null) {
    return "neutral";
  }
  if (summary.deltaMinutes < 0) return "under";
  if (summary.deltaMinutes > 0) return "over";
  return "neutral";
}

// "18/20,25 h" with a target, "18 h" without one (docs/05 Abschnitt 2.3).
export function summaryLabel(summary: SummaryDelta): string {
  const planned = formatPlannedHours(summary.plannedMinutes);
  if (summary.targetMinutes === null) return planned;
  const plannedValue = planned.replace(/\s*h$/, "");
  return `${plannedValue}/${formatPlannedHours(summary.targetMinutes)}`;
}

/**
 * Groups shifts for the week grid: staffId -> date ("YYYY-MM-DD") -> shifts
 * (sorted by start time, as delivered by the backend).
 */
export function groupShiftsByStaffAndDate(
  shifts: readonly StaffShift[],
): Map<string, Map<string, StaffShift[]>> {
  const byStaff = new Map<string, Map<string, StaffShift[]>>();
  for (const shift of shifts) {
    let byDate = byStaff.get(shift.staffId);
    if (!byDate) {
      byDate = new Map<string, StaffShift[]>();
      byStaff.set(shift.staffId, byDate);
    }
    const list = byDate.get(shift.date);
    if (list) {
      list.push(shift);
    } else {
      byDate.set(shift.date, [shift]);
    }
  }
  return byStaff;
}
