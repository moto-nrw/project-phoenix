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

export interface BackendAttendanceHistoryDay {
  date: string;
  attendance: BackendAttendanceRecord | null;
  room_detail_available: boolean;
  visits: BackendAttendanceVisit[] | null;
}

export interface BackendAttendanceRecord {
  check_in_time: string;
  check_out_time?: string | null;
  duration_minutes?: number | null;
  checked_in_by: number;
  checked_out_by?: number | null;
  // Nullable as of backend migration 1.15.41: web-originated school
  // check-ins have no kiosk and emit `"device_id": null`.
  device_id?: number | null;
}

export interface BackendAttendanceVisit {
  room_id?: number | null;
  room_name?: string;
  entry_time: string;
  exit_time?: string | null;
  duration_minutes?: number | null;
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
  roomDetailAvailable: boolean;
  visits: AttendanceVisit[];
}

export interface AttendanceRecord {
  checkInTime: Date;
  checkOutTime: Date | null;
  durationMinutes: number | null;
  checkedInBy: number;
  checkedOutBy: number | null;
  /** null for web-originated check-ins (no kiosk involved). */
  deviceId: number | null;
}

export interface AttendanceVisit {
  roomId: number | null;
  roomName: string;
  entryTime: Date;
  exitTime: Date | null;
  durationMinutes: number | null;
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
    roomDetailAvailable: day.room_detail_available,
    visits: (day.visits ?? []).map(mapAttendanceVisit),
  };
}

function mapAttendanceRecord(rec: BackendAttendanceRecord): AttendanceRecord {
  return {
    checkInTime: new Date(rec.check_in_time),
    checkOutTime: rec.check_out_time ? new Date(rec.check_out_time) : null,
    durationMinutes: rec.duration_minutes ?? null,
    checkedInBy: rec.checked_in_by,
    checkedOutBy: rec.checked_out_by ?? null,
    deviceId: rec.device_id ?? null,
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

// --- Formatting helpers ---

export function formatTime(d: Date): string {
  return d.toLocaleTimeString("de-DE", { hour: "2-digit", minute: "2-digit" });
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
