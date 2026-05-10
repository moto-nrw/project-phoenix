export interface PlannedTimetableInstance {
  id: string;
  title: string;
  date: string;
  startTime: string;
  endTime: string;
  roomId: string;
  status: "planned" | "active" | "completed" | "cancelled";
  isOverdue: boolean;
  minutesUntilStart: number;
  expectedStudentsCount: number;
  presentStudentsCount: number;
  assignedStaffIds: string[];
}

export interface TimetableRosterInstance {
  id: string;
  title: string;
  status: "planned" | "active" | "completed" | "cancelled";
  activeGroupId: string | null;
  roomId: string;
  roomName?: string | null;
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
  visitEntryTime: string | null;
}

export interface TimetableRoster {
  instance: TimetableRosterInstance;
  rows: TimetableRosterRow[];
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
  status: PlannedTimetableInstance["status"];
  is_overdue: boolean;
  minutes_until_start: number;
  expected_students_count: number;
  present_students_count: number;
  assigned_staff_ids: number[];
}

interface BackendRosterInstance {
  id: number;
  title: string;
  status: TimetableRosterInstance["status"];
  active_group_id?: number | null;
  room_id: number;
  room_name?: string | null;
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
  visit_entry_time?: string | null;
}

export interface BackendTimetableRoster {
  instance: BackendRosterInstance;
  rows: BackendRosterRow[];
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
    status: raw.status,
    isOverdue: raw.is_overdue,
    minutesUntilStart: raw.minutes_until_start,
    expectedStudentsCount: raw.expected_students_count,
    presentStudentsCount: raw.present_students_count,
    assignedStaffIds: raw.assigned_staff_ids.map(String),
  };
}

export function mapRoster(raw: BackendTimetableRoster): TimetableRoster {
  return {
    instance: {
      id: raw.instance.id.toString(),
      title: raw.instance.title,
      status: raw.instance.status,
      activeGroupId: raw.instance.active_group_id?.toString() ?? null,
      roomId: raw.instance.room_id.toString(),
      roomName: raw.instance.room_name ?? null,
    },
    rows: raw.rows.map((row) => ({
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
      visitEntryTime: row.visit_entry_time ?? null,
    })),
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
