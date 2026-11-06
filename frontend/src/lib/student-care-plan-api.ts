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

import { bffJson } from "./bff-fetch";

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
interface CarePlanWeek {
  studentId: string;
  from: string; // YYYY-MM-DD
  to: string; // YYYY-MM-DD
  days: CarePlanDay[];
}

// --- Backend wire shapes (snake_case) --------------------------------------

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
    source: (s.source as CarePlanSlotSource | undefined) ?? "none",
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

// --- Fetchers ---------------------------------------------------------------

/** Fetch one child's plan for a single day. */
export async function fetchStudentCarePlanDay(
  studentId: string,
  date: string,
): Promise<CarePlanDay> {
  return mapDay(
    await bffJson<BackendDay>(
      `/api/timetable/student/${encodeURIComponent(studentId)}/day?date=${date}`,
    ),
  );
}

/** Fetch one child's plan across an inclusive [from, to] range (max 14 days). */
export async function fetchStudentCarePlanWeek(
  studentId: string,
  from: string,
  to: string,
): Promise<CarePlanWeek> {
  return mapWeek(
    await bffJson<BackendWeek>(
      `/api/timetable/student/${encodeURIComponent(studentId)}/week?from=${from}&to=${to}`,
    ),
  );
}

/** One recorded correction of a child's attendance in a block. */
export interface AttendanceCorrection {
  readonly field_name: string;
  readonly old_value?: string | null;
  readonly new_value?: string | null;
  readonly reason: string;
  readonly actor_name?: string | null;
  readonly corrected_at: string;
}

/** The fields a correction changes, with the reason it is made. */
interface AttendanceCorrectionInput {
  reason: string;
  status?: string;
  substatus?: string | null;
  note?: string | null;
}

function correctionPath(instanceId: string, studentId: string): string {
  return `/api/timetable/instances/${encodeURIComponent(instanceId)}/students/${encodeURIComponent(studentId)}`;
}

/** The correction trail of a child's attendance in one block. */
export async function fetchAttendanceCorrections(
  instanceId: string,
  studentId: string,
): Promise<AttendanceCorrection[]> {
  const trail = await bffJson<{ corrections?: AttendanceCorrection[] }>(
    `${correctionPath(instanceId, studentId)}/corrections`,
  );
  return trail.corrections ?? [];
}

export async function correctAttendance(
  instanceId: string,
  studentId: string,
  correction: AttendanceCorrectionInput,
): Promise<void> {
  await bffJson<unknown>(
    `${correctionPath(instanceId, studentId)}/correction`,
    {
      method: "POST",
      body: correction,
    },
  );
}
