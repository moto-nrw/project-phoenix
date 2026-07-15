// Types + mappers for the student attendance-history API.
//
// The backend returns ISO timestamps in snake_case; this module normalises
// them to camelCase `Date`-friendly shapes used by the UI.

export interface BackendAttendanceHistoryResponse {
  student_id: string;
  days: BackendAttendanceHistoryDay[];
  range: { start: string; end: string };
  clamped: boolean;
  caps: { attendance_days: number; room_detail_days: number };
}

export type AttendanceSlotStatus = "expected" | "present" | "absent";

interface BackendAttendanceHistoryDay {
  date: string;
  attendance: BackendAttendanceRecord | null;
  status_entries?: BackendAttendanceStatusEntry[] | null;
  room_detail_available: boolean;
  visits: BackendAttendanceVisit[] | null;
  slots?: BackendAttendanceSlot[] | null;
}

interface BackendAttendanceRecord {
  check_in_time: string;
  check_out_time?: string | null;
  duration_minutes?: number | null;
  checked_in_by: number;
  checked_out_by?: number | null;
  device_id: number;
  sessions?: BackendAttendanceSession[] | null;
}

interface BackendAttendanceSession {
  check_in_time: string;
  check_out_time?: string | null;
  duration_minutes?: number | null;
}

interface BackendAttendanceSlot {
  instance_id: string;
  title: string;
  start_time: string;
  end_time: string;
  status: AttendanceSlotStatus;
  substatus?: string | null;
  checked_in_at?: string | null;
  checked_out_at?: string | null;
  is_unplanned: boolean;
}

interface BackendAttendanceVisit {
  room_id?: number | null;
  room_name?: string;
  entry_time: string;
  exit_time?: string | null;
  duration_minutes?: number | null;
}

interface BackendAttendanceStatusEntry {
  status: string;
  label: string;
  reported_at: string;
  cleared_at?: string | null;
}

// --- UI-friendly shapes ---

export interface AttendanceHistory {
  studentId: string;
  days: AttendanceHistoryDay[];
  range: { start: Date; end: Date };
  clamped: boolean;
  caps: { attendanceDays: number; roomDetailDays: number };
}

export interface AttendanceHistoryDay {
  date: string; // YYYY-MM-DD
  attendance: AttendanceRecord | null;
  statusEntries: AttendanceStatusEntry[];
  roomDetailAvailable: boolean;
  visits: AttendanceVisit[];
  slots: AttendanceSlot[];
}

interface AttendanceRecord {
  checkInTime: Date;
  checkOutTime: Date | null;
  durationMinutes: number | null;
  checkedInBy: number;
  checkedOutBy: number | null;
  deviceId: number;
  sessions: AttendanceSession[];
}

interface AttendanceSession {
  checkInTime: Date;
  checkOutTime: Date | null;
  durationMinutes: number | null;
}

interface AttendanceSlot {
  instanceId: string;
  title: string;
  startTime: string;
  endTime: string;
  status: AttendanceSlotStatus;
  substatus: string | null;
  checkedInAt: Date | null;
  checkedOutAt: Date | null;
  isUnplanned: boolean;
}

interface AttendanceVisit {
  roomId: number | null;
  roomName: string;
  entryTime: Date;
  exitTime: Date | null;
  durationMinutes: number | null;
}

interface AttendanceStatusEntry {
  status: string;
  label: string;
  reportedAt: Date;
  clearedAt: Date | null;
}

export function mapAttendanceHistoryResponse(
  raw: BackendAttendanceHistoryResponse,
): AttendanceHistory {
  return {
    studentId: raw.student_id,
    days: raw.days.map(mapAttendanceHistoryDay),
    range: {
      start: new Date(raw.range.start),
      end: new Date(raw.range.end),
    },
    clamped: raw.clamped,
    caps: {
      attendanceDays: raw.caps.attendance_days,
      roomDetailDays: raw.caps.room_detail_days,
    },
  };
}

function mapAttendanceHistoryDay(
  day: BackendAttendanceHistoryDay,
): AttendanceHistoryDay {
  return {
    date: day.date,
    attendance: day.attendance ? mapAttendanceRecord(day.attendance) : null,
    statusEntries: (day.status_entries ?? []).map(mapAttendanceStatusEntry),
    roomDetailAvailable: day.room_detail_available,
    visits: (day.visits ?? []).map(mapAttendanceVisit),
    slots: (day.slots ?? []).map(mapAttendanceSlot),
  };
}

function mapAttendanceRecord(rec: BackendAttendanceRecord): AttendanceRecord {
  return {
    checkInTime: new Date(rec.check_in_time),
    checkOutTime: rec.check_out_time ? new Date(rec.check_out_time) : null,
    durationMinutes: rec.duration_minutes ?? null,
    checkedInBy: rec.checked_in_by,
    checkedOutBy: rec.checked_out_by ?? null,
    deviceId: rec.device_id,
    sessions: (rec.sessions ?? [rec]).map((session) => ({
      checkInTime: new Date(session.check_in_time),
      checkOutTime: session.check_out_time
        ? new Date(session.check_out_time)
        : null,
      durationMinutes: session.duration_minutes ?? null,
    })),
  };
}

function mapAttendanceSlot(slot: BackendAttendanceSlot): AttendanceSlot {
  return {
    instanceId: slot.instance_id,
    title: slot.title,
    startTime: slot.start_time,
    endTime: slot.end_time,
    status: slot.status,
    substatus: slot.substatus ?? null,
    checkedInAt: slot.checked_in_at ? new Date(slot.checked_in_at) : null,
    checkedOutAt: slot.checked_out_at ? new Date(slot.checked_out_at) : null,
    isUnplanned: slot.is_unplanned,
  };
}

function mapAttendanceVisit(v: BackendAttendanceVisit): AttendanceVisit {
  return {
    roomId: v.room_id ?? null,
    roomName: v.room_name ?? "",
    entryTime: new Date(v.entry_time),
    exitTime: v.exit_time ? new Date(v.exit_time) : null,
    durationMinutes: v.duration_minutes ?? null,
  };
}

function mapAttendanceStatusEntry(
  entry: BackendAttendanceStatusEntry,
): AttendanceStatusEntry {
  return {
    status: entry.status,
    label: entry.label,
    reportedAt: new Date(entry.reported_at),
    clearedAt: entry.cleared_at ? new Date(entry.cleared_at) : null,
  };
}

// --- Formatting helpers ---

export function formatTime(d: Date): string {
  return d.toLocaleTimeString("de-DE", { hour: "2-digit", minute: "2-digit" });
}

export function formatAttendanceSlotStatus(
  status: AttendanceSlotStatus,
  substatus?: string | null,
): string {
  if (status === "present") return "Anwesend";
  if (substatus === "sick") return "Krank";
  if (substatus === "excused") return "Entschuldigt";
  if (substatus === "field_trip") return "Klassenfahrt";
  if (status === "absent") return "Abwesend";
  return "Erwartet";
}

export function formatDate(dateKey: string): string {
  const d = new Date(`${dateKey}T00:00:00`);
  return d.toLocaleDateString("de-DE", {
    weekday: "long",
    day: "2-digit",
    month: "long",
    year: "numeric",
  });
}

export function formatDuration(minutes: number | null): string {
  if (minutes == null) return "–";
  const h = Math.floor(minutes / 60);
  const m = minutes % 60;
  if (h === 0) return `${m} min`;
  if (m === 0) return `${h} h`;
  return `${h} h ${m} min`;
}
