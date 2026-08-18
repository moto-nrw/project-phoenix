import type { CareDayStatus } from "./timetable-types";

export interface PlannedTimetableInstance {
  id: string;
  title: string;
  date: string;
  startTime: string;
  endTime: string;
  roomId: string;
  roomName: string | null;
  status: "planned" | "active" | "completed" | "cancelled";
  isOverdue: boolean;
  minutesUntilStart: number;
  expectedStudentsCount: number;
  presentStudentsCount: number;
  /**
   * Assigned children whose care plan does not place them here today (#1747).
   * Excluded from expectedStudentsCount; shown next to it so a lower expected
   * number is explained rather than silently smaller.
   */
  notScheduledStudentsCount: number;
  assignedStaffIds: string[];
  isAssigned: boolean;
  isPrimary: boolean;
  isSubstitute: boolean;
  isAbsent: boolean;
  rosterPreview: TimetableRosterRow[];
  canStart?: boolean;
  startAvailableAt?: string;
  startExpiresAt?: string;
}

interface TimetableRosterInstance {
  id: string;
  title: string;
  status: "planned" | "active" | "completed" | "cancelled";
  isSpontaneous: boolean;
  activeGroupId: string | null;
  roomId: string;
  roomName?: string | null;
  date: string;
  startTime: string;
  endTime: string;
  canComplete: boolean;
  completeAvailableAt: string;
}

export interface TimetableRosterRow {
  studentId: string;
  studentName: string;
  schoolClass: string;
  groupName: string;
  planned: boolean;
  isUnplanned: boolean;
  currentlyPresent: boolean;
  visitId: string | null;
  status: "expected" | "present" | "absent";
  substatus: "late" | "excused" | "sick" | "field_trip" | "other" | null;
  note: string | null;
  checkedInAt: string | null;
  checkedOutAt?: string | null;
  visitEntryTime: string | null;
  warnings: TimetableRosterWarning[];
  /**
   * Care-plan verdict for this child on the instance's date (#1747).
   * "not_scheduled" and "cancelled" are the actionable ones: the row is set
   * apart and left out of the expected count, but stays in the list so a child
   * who turns up anyway can still be checked in with one tap. Older backends
   * omit the field, which reads as "unknown" and changes nothing. Test with
   * isCareDayExpected, never against a single value.
   */
  careDayStatus: CareDayStatus;
  /**
   * Names the other running instance where this child is currently recorded
   * present (#2265). Null when no parallel block holds the child as present;
   * older backends omit the field entirely.
   */
  parallelPresentIn?: TimetableParallelPresence | null;
}

interface TimetableParallelPresence {
  instanceId: string;
  title: string;
  startTime: string;
  endTime: string;
}

interface TimetableRosterWarning {
  kind:
    | "arrival_after_slot_start"
    | "missing_arrival_schedule"
    | "template_class_mismatch"
    | string;
  message: string;
  expectedArrival: string | null;
  slotStart: string | null;
  expectedGroupId: string | null;
  expectedGroupName: string | null;
  currentEducationGroupId: string | null;
}

export interface TimetableRoster {
  instance: TimetableRosterInstance;
  rows: TimetableRosterRow[];
  /**
   * Set only on check-in responses that auto-moved the child out of another
   * running session (#2386). Carries the origin's display name; an empty
   * string means the move happened but the origin has no resolvable name.
   * Null when no move happened; older backends omit the field.
   */
  movedFrom?: string | null;
}

export interface StartOperationResult {
  instanceId: string;
  activeGroupId: string;
  status: string;
  startedAt?: string;
}

export interface AttendancePatchBody {
  status?: "expected" | "present" | "absent";
  substatus?: "late" | "excused" | "sick" | "field_trip" | "other" | null;
  note?: string | null;
}

interface BackendPlannedTimetableInstance {
  id: number;
  title: string;
  date: string;
  start_time: string;
  end_time: string;
  room_id: number;
  room_name?: string | null;
  status: PlannedTimetableInstance["status"];
  is_overdue: boolean;
  minutes_until_start: number;
  expected_students_count: number;
  not_scheduled_students_count?: number;
  present_students_count: number;
  assigned_staff_ids: number[];
  is_assigned?: boolean;
  is_primary?: boolean;
  is_substitute?: boolean;
  is_absent?: boolean;
  roster_preview?: BackendRosterRow[];
  can_start?: boolean;
  start_available_at?: string;
  start_expires_at?: string;
}

interface BackendRosterInstance {
  id: number;
  title: string;
  status: TimetableRosterInstance["status"];
  is_spontaneous?: boolean;
  active_group_id?: number | null;
  room_id: number;
  room_name?: string | null;
  date?: string;
  start_time?: string;
  end_time?: string;
  can_complete?: boolean;
  complete_available_at?: string;
}

interface BackendRosterRow {
  student_id: number;
  student_name: string;
  school_class: string;
  group_name: string;
  planned: boolean;
  is_unplanned: boolean;
  currently_present: boolean;
  visit_id?: number | null;
  status: TimetableRosterRow["status"];
  substatus?: TimetableRosterRow["substatus"];
  note?: string | null;
  checked_in_at?: string | null;
  checked_out_at?: string | null;
  visit_entry_time?: string | null;
  warnings?: BackendRosterWarning[];
  care_day_status?: TimetableRosterRow["careDayStatus"];
  parallel_present_in?: {
    instance_id: number;
    title: string;
    start_time: string;
    end_time: string;
  } | null;
}

interface BackendRosterWarning {
  kind: TimetableRosterWarning["kind"];
  message: string;
  expected_arrival?: string | null;
  slot_start?: string | null;
  expected_group_id?: number | null;
  expected_group_name?: string | null;
  current_education_group_id?: number | null;
}

export interface BackendTimetableRoster {
  instance: BackendRosterInstance;
  rows: BackendRosterRow[];
  moved_from?: string | null;
}

export interface BackendStartOperationResult {
  instance_id: number;
  status: string;
  active_group_id: number;
}

export function mapPlannedInstance(
  raw: BackendPlannedTimetableInstance,
): PlannedTimetableInstance {
  return {
    id: raw.id.toString(),
    title: raw.title,
    date: raw.date,
    startTime: raw.start_time,
    endTime: raw.end_time,
    roomId: raw.room_id.toString(),
    roomName: raw.room_name ?? null,
    status: raw.status,
    isOverdue: raw.is_overdue,
    minutesUntilStart: raw.minutes_until_start,
    expectedStudentsCount: raw.expected_students_count,
    presentStudentsCount: raw.present_students_count,
    notScheduledStudentsCount: raw.not_scheduled_students_count ?? 0,
    assignedStaffIds: raw.assigned_staff_ids.map(String),
    isAssigned: raw.is_assigned ?? false,
    isPrimary: raw.is_primary ?? false,
    isSubstitute: raw.is_substitute ?? false,
    isAbsent: raw.is_absent ?? false,
    rosterPreview: (raw.roster_preview ?? []).map(mapRosterRow),
    canStart: raw.can_start ?? false,
    startAvailableAt: raw.start_available_at ?? "",
    startExpiresAt: raw.start_expires_at ?? "",
  };
}

function mapRosterRow(row: BackendRosterRow): TimetableRosterRow {
  return {
    studentId: row.student_id.toString(),
    studentName: row.student_name,
    schoolClass: row.school_class,
    groupName: row.group_name,
    planned: row.planned,
    isUnplanned: row.is_unplanned,
    currentlyPresent: row.currently_present,
    visitId: row.visit_id?.toString() ?? null,
    status: row.status,
    substatus: row.substatus ?? null,
    note: row.note ?? null,
    checkedInAt: row.checked_in_at ?? null,
    checkedOutAt: row.checked_out_at ?? null,
    visitEntryTime: row.visit_entry_time ?? null,
    careDayStatus: row.care_day_status ?? "unknown",
    parallelPresentIn: row.parallel_present_in
      ? {
          instanceId: row.parallel_present_in.instance_id.toString(),
          title: row.parallel_present_in.title,
          startTime: row.parallel_present_in.start_time,
          endTime: row.parallel_present_in.end_time,
        }
      : null,
    warnings: (row.warnings ?? []).map((warning) => ({
      kind: warning.kind,
      message: warning.message,
      expectedArrival: warning.expected_arrival ?? null,
      slotStart: warning.slot_start ?? null,
      expectedGroupId: warning.expected_group_id?.toString() ?? null,
      expectedGroupName: warning.expected_group_name ?? null,
      currentEducationGroupId:
        warning.current_education_group_id?.toString() ?? null,
    })),
  };
}

export function mapRoster(raw: BackendTimetableRoster): TimetableRoster {
  return {
    instance: {
      id: raw.instance.id.toString(),
      title: raw.instance.title,
      status: raw.instance.status,
      isSpontaneous: raw.instance.is_spontaneous ?? false,
      activeGroupId: raw.instance.active_group_id?.toString() ?? null,
      roomId: raw.instance.room_id.toString(),
      roomName: raw.instance.room_name ?? null,
      date: raw.instance.date ?? "",
      startTime: raw.instance.start_time ?? "",
      endTime: raw.instance.end_time ?? "",
      canComplete: raw.instance.can_complete ?? false,
      completeAvailableAt: raw.instance.complete_available_at ?? "",
    },
    rows: raw.rows.map(mapRosterRow),
    movedFrom: raw.moved_from ?? null,
  };
}

export function mapStartOperation(
  raw: BackendStartOperationResult,
): StartOperationResult {
  return {
    instanceId: raw.instance_id.toString(),
    activeGroupId: raw.active_group_id.toString(),
    status: raw.status,
  };
}
