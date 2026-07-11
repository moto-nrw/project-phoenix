// Types and mapping helpers for planned staff shifts (Dienstplan, #1376).
// Backend wire format: snake_case, int64 ids as numbers, dates as
// "YYYY-MM-DD", times as "HH:MM".

export interface BackendStaffShift {
  id: number;
  staff_id: number;
  date: string;
  start_time: string;
  end_time: string;
  break_minutes: number;
  shift_type_id?: number | null;
  notes?: string;
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
  notes: string;
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

export interface BackendStaffScheduleOverview {
  from: string;
  to: string;
  dienstplan_in_use: boolean;
  staff: BackendStaffScheduleStaff[];
  shifts: BackendStaffShift[];
  assignments: BackendStaffScheduleAssignment[];
}

export interface StaffScheduleOverview {
  /** Inclusive calendar-day range start as "YYYY-MM-DD". */
  from: string;
  /** Inclusive calendar-day range end as "YYYY-MM-DD". */
  to: string;
  dienstplanInUse: boolean;
  staff: StaffScheduleStaff[];
  shifts: StaffShift[];
  assignments: StaffScheduleAssignment[];
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
    notes: data.notes ?? "",
  };
}

export function mapStaffScheduleOverview(
  data: BackendStaffScheduleOverview,
): StaffScheduleOverview {
  return {
    from: data.from.slice(0, 10),
    to: data.to.slice(0, 10),
    dienstplanInUse: data.dienstplan_in_use,
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
  };
}

/** "08:00–16:00" */
export function formatShiftLabel(shift: StaffShift): string {
  return `${shift.startTime}–${shift.endTime}`;
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
