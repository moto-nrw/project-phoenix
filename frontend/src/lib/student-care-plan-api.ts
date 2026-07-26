/**
 * Client for the per-child derived care plan (Betreuungsplan) endpoints.
 *
 * Mirrors the Go DTOs in backend/api/timetable/student_day_shapes.go. Backend
 * int64 IDs become frontend strings; snake_case becomes camelCase via the
 * mappers below. The plan is READ-ONLY here — it is the resolved arrival →
 * activities → pickup shape for one student on one day, with per-child
 * attendance. Free-care time and broad day deviations are composed on top in
 * care-plan-helpers.ts and the view, not by this client.
 */

/** Arrival/pickup slot source — mirrors SlotSource* in the backend. */
type CarePlanSlotSource = "schedule" | "exception" | "none";

type CarePlanInstanceStatus = "planned" | "active" | "completed" | "cancelled";

/** Attendance status on a plan instance (schedule.instance_students). */
type CarePlanAttendanceStatus = "expected" | "present" | "absent";

/**
 * One arrival or pickup slot for a day. expectedTime is null when
 * source="none", or when source="exception" and the exception expresses
 * absence (no time). reason is populated only for exceptions that carry one —
 * this is how "kommt heute nicht" / früher abgeholt surfaces.
 */
export interface CarePlanSlot {
  expectedTime: string | null; // HH:MM
  source: CarePlanSlotSource;
  reason?: string;
}

/** Per-child attendance payload on a plan instance. */
interface CarePlanAttendance {
  status: CarePlanAttendanceStatus | string;
  substatus: string | null;
  note: string | null;
  checkedInAt: string | null; // RFC3339, Berlin-local
  checkedOutAt: string | null; // RFC3339, Berlin-local
  /** A persisted visit without a booking (child was there, not enrolled). */
  isUnplanned: boolean;
}

/** One planned (or unplanned) instance as it appears in a child's day. */
export interface CarePlanInstance {
  id: string;
  title: string;
  startTime: string; // HH:MM
  endTime: string; // HH:MM
  roomId: string;
  status: CarePlanInstanceStatus | string;
  activeGroupId: string | null;
  attendance: CarePlanAttendance;
}

/** The resolved care plan for one child on one calendar day. */
export interface CarePlanDay {
  studentId: string;
  date: string; // YYYY-MM-DD
  weekday: number; // 1..7 (Mon..Sun)
  arrival: CarePlanSlot;
  instances: CarePlanInstance[];
  pickup: CarePlanSlot;
}

/** A contiguous, inclusive range of care-plan days. */
export interface CarePlanWeek {
  studentId: string;
  from: string; // YYYY-MM-DD
  to: string; // YYYY-MM-DD
  days: CarePlanDay[];
}

// --- Backend wire shapes (snake_case) --------------------------------------

interface ApiResponse<T> {
  status: string;
  data?: T;
  message?: string;
  error?: string;
}

interface BackendSlot {
  expected_time: string | null;
  source: string;
  reason?: string | null;
}

interface BackendAttendance {
  status: string;
  substatus: string | null;
  note: string | null;
  checked_in_at: string | null;
  checked_out_at: string | null;
  is_unplanned: boolean;
}

interface BackendInstance {
  id: number;
  title: string;
  start_time: string;
  end_time: string;
  room_id: number;
  status: string;
  active_group_id: number | null;
  attendance: BackendAttendance;
}

interface BackendDay {
  student_id: number;
  date: string;
  weekday: number;
  arrival: BackendSlot;
  instances: BackendInstance[] | null;
  pickup: BackendSlot;
}

interface BackendWeek {
  student_id: number;
  from: string;
  to: string;
  days: BackendDay[] | null;
}

// --- Mappers ----------------------------------------------------------------

function mapSlot(s: BackendSlot): CarePlanSlot {
  const slot: CarePlanSlot = {
    expectedTime: s.expected_time,
    source: (s.source as CarePlanSlotSource) ?? "none",
  };
  if (s.reason) slot.reason = s.reason;
  return slot;
}

function mapAttendance(a: BackendAttendance): CarePlanAttendance {
  return {
    status: a.status,
    substatus: a.substatus,
    note: a.note,
    checkedInAt: a.checked_in_at,
    checkedOutAt: a.checked_out_at,
    isUnplanned: a.is_unplanned,
  };
}

function mapInstance(i: BackendInstance): CarePlanInstance {
  return {
    id: i.id.toString(),
    title: i.title,
    startTime: i.start_time,
    endTime: i.end_time,
    roomId: i.room_id.toString(),
    status: i.status,
    activeGroupId:
      i.active_group_id != null ? i.active_group_id.toString() : null,
    attendance: mapAttendance(i.attendance),
  };
}

function mapDay(d: BackendDay): CarePlanDay {
  return {
    studentId: d.student_id.toString(),
    date: d.date,
    weekday: d.weekday,
    arrival: mapSlot(d.arrival),
    instances: (d.instances ?? []).map(mapInstance),
    pickup: mapSlot(d.pickup),
  };
}

function mapWeek(w: BackendWeek): CarePlanWeek {
  return {
    studentId: w.student_id.toString(),
    from: w.from,
    to: w.to,
    days: (w.days ?? []).map(mapDay),
  };
}

async function parseApiResult<T>(
  response: Response,
  fallback: string,
): Promise<T> {
  const result = (await response.json()) as ApiResponse<T>;
  if (result.status === "error" || !result.data) {
    throw new Error(result.error ?? fallback);
  }
  return result.data;
}

// --- Fetchers ---------------------------------------------------------------

const LOAD_ERROR = "Betreuungsplan konnte nicht geladen werden";

/** Fetch one child's plan for a single day. */
export async function fetchStudentCarePlanDay(
  studentId: string,
  date: string,
): Promise<CarePlanDay> {
  const response = await fetch(
    `/api/timetable/student/${studentId}/day?date=${date}`,
  );
  if (!response.ok) throw new Error(LOAD_ERROR);
  const data = await parseApiResult<BackendDay>(response, LOAD_ERROR);
  return mapDay(data);
}

/** Fetch one child's plan across an inclusive [from, to] range (max 14 days). */
export async function fetchStudentCarePlanWeek(
  studentId: string,
  from: string,
  to: string,
): Promise<CarePlanWeek> {
  const response = await fetch(
    `/api/timetable/student/${studentId}/week?from=${from}&to=${to}`,
  );
  if (!response.ok) throw new Error(LOAD_ERROR);
  const data = await parseApiResult<BackendWeek>(response, LOAD_ERROR);
  return mapWeek(data);
}
