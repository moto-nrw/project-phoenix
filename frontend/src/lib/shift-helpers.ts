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
