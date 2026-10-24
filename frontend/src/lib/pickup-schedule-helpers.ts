// Pickup Schedule Type Definitions and Mapping Helpers
import { toISODate as formatDateISO } from "./date-helpers";
import type { StudentStatusKind } from "./student-status-days-api";
import type { DepartureMode } from "./student-helpers";

// Frontend Pickup Schedule Type
export interface PickupSchedule {
  id: string;
  studentId: string;
  weekday: number;
  weekdayName: string;
  pickupTime: string; // HH:MM format
  notes?: string;
  /** "staff" (von Hand gepflegt) oder "care_offering" (projizierte Angebots-Gehzeit) */
  source?: string;
  careOfferingId?: string;
  careOfferingName?: string;
  createdBy: string;
  createdAt: string;
  updatedAt: string;
}

// Backend Pickup Schedule Response
export interface BackendPickupSchedule {
  id: number;
  student_id: number;
  weekday: number;
  weekday_name: string;
  pickup_time: string; // HH:MM format
  notes?: string;
  source?: string; // "staff" | "care_offering" (#2290)
  care_offering_id?: string;
  care_offering_name?: string;
  created_by: number;
  created_at: string;
  updated_at: string;
}

// Frontend Pickup Exception Type
export interface PickupException {
  id: string;
  studentId: string;
  exceptionDate: string; // YYYY-MM-DD format
  pickupTime?: string; // HH:MM format, null = no pickup (absent)
  reason?: string;
  source?: string; // "staff" or "guardian" (parent-set)
  /** Teilabwesenheits-Cutoff (HH:MM), falls die Ausnahme eine traegt. */
  excusedFrom?: string;
  /** true = Abmeldung automatisch aus vorverlegter Abholzeit abgeleitet (#2360). */
  excusedAuto?: boolean;
  createdBy: string;
  createdAt: string;
  updatedAt: string;
}

// Backend Pickup Exception Response
export interface BackendPickupException {
  id: number;
  student_id: number;
  exception_date: string; // YYYY-MM-DD format
  pickup_time?: string; // HH:MM format
  reason?: string;
  source?: string; // "staff" or "guardian"
  excused_from?: string;
  excused_auto?: boolean;
  created_by: number;
  created_at: string;
  updated_at: string;
}

// Frontend Pickup Note Type. A note is either dated (noteDate) or recurs on a
// weekday (weekday 1-5, #3369). The recurring shape needs no pickup time, so a
// day the child is not expected can still carry a note.
export interface PickupNote {
  id: string;
  studentId: string;
  noteDate: string; // YYYY-MM-DD format; "" for a recurring note
  weekday?: number;
  content: string;
  createdBy: string;
  createdAt: string;
  updatedAt: string;
}

// Backend Pickup Note Response
export interface BackendPickupNote {
  id: number;
  student_id: number;
  note_date?: string; // YYYY-MM-DD format; absent for a recurring note
  weekday?: number;
  content: string;
  created_by: number;
  created_at: string;
  updated_at: string;
}

// Combined Pickup Data (schedules + exceptions + notes)
// Abholart eines einzelnen Tages, von der OGS gesetzt oder bestätigt (#3945).
export type { DepartureMode };

export interface DepartureException {
  id: string;
  studentId: string;
  exceptionDate: string; // YYYY-MM-DD
  departureMode: DepartureMode;
  reason?: string;
}

export interface BackendDepartureException {
  id: number;
  student_id: number;
  exception_date: string;
  departure_mode: DepartureMode;
  reason?: string;
}

export interface PickupData {
  schedules: PickupSchedule[];
  effectiveSchedules?: DatedPickupSchedule[];
  exceptions: PickupException[];
  notes: PickupNote[];
  departureExceptions?: DepartureException[];
}

export interface DatedPickupSchedule {
  date: string;
  schedule: PickupSchedule | null;
  offeringSchedule?: PickupSchedule | null;
}

// Backend Combined Pickup Data Response
export interface BackendPickupData {
  schedules: BackendPickupSchedule[];
  effective_schedules?: Array<{
    date: string;
    schedule: BackendPickupSchedule | null;
    offering_schedule?: BackendPickupSchedule | null;
  }>;
  exceptions: BackendPickupException[];
  notes: BackendPickupNote[];
  departure_exceptions?: BackendDepartureException[];
}

// Request types for creating/updating schedules
export interface PickupScheduleFormData {
  weekday: number;
  pickupTime: string; // HH:MM format
  notes?: string;
}

// Backend schedule request
export interface BackendPickupScheduleRequest {
  weekday: number;
  pickup_time: string;
  notes?: string;
}

// Bulk schedule update request
export interface BulkPickupScheduleFormData {
  schedules: PickupScheduleFormData[];
  effectiveDate?: string;
}

// Backend bulk schedule request
export interface BackendBulkPickupScheduleRequest {
  schedules: BackendPickupScheduleRequest[];
  effective_date?: string;
}

// Request types for creating/updating exceptions
export interface PickupExceptionFormData {
  exceptionDate: string; // YYYY-MM-DD format
  pickupTime?: string; // HH:MM format, undefined = no pickup
  clearPickupTime?: boolean;
  reason?: string;
}

// Backend exception request
export interface BackendPickupExceptionRequest {
  exception_date: string;
  pickup_time?: string;
  clear_pickup_time?: boolean;
  reason?: string;
}

// Request types for creating/updating notes
export type PickupNoteFormData =
  | { noteDate: string; content: string } // dated, YYYY-MM-DD
  | { weekday: number; content: string }; // recurring, 1-5 (#3369)

// Backend note request
export type BackendPickupNoteRequest =
  { note_date: string; content: string } | { weekday: number; content: string };

/** The note of one weekday without a pickup time (#3369). */
export interface WeekdayNoteEntry {
  readonly weekday: number;
  readonly content: string;
}

// Mapping Functions

export function mapPickupScheduleResponse(
  data: BackendPickupSchedule,
): PickupSchedule {
  return {
    id: data.id.toString(),
    studentId: data.student_id.toString(),
    weekday: data.weekday,
    weekdayName: data.weekday_name,
    pickupTime: data.pickup_time,
    notes: data.notes,
    source: data.source,
    careOfferingId: data.care_offering_id,
    careOfferingName: data.care_offering_name,
    createdBy: data.created_by.toString(),
    createdAt: data.created_at,
    updatedAt: data.updated_at,
  };
}

export function pickupScheduleSourceLabel(
  schedule: PickupSchedule | undefined,
): string | null {
  if (!schedule) return null;
  if (schedule.source !== "care_offering") return "von Hand";
  return schedule.careOfferingName
    ? `aus Angebot „${schedule.careOfferingName}“`
    : "aus Angebot";
}

export function mapPickupExceptionResponse(
  data: BackendPickupException,
): PickupException {
  return {
    id: data.id.toString(),
    studentId: data.student_id.toString(),
    exceptionDate: data.exception_date,
    pickupTime: data.pickup_time,
    reason: data.reason,
    source: data.source,
    excusedFrom: data.excused_from,
    excusedAuto: data.excused_auto,
    createdBy: data.created_by.toString(),
    createdAt: data.created_at,
    updatedAt: data.updated_at,
  };
}

export function mapPickupNoteResponse(data: BackendPickupNote): PickupNote {
  return {
    id: data.id.toString(),
    studentId: data.student_id.toString(),
    noteDate: data.note_date ?? "",
    ...(data.weekday ? { weekday: data.weekday } : {}),
    content: data.content,
    createdBy: data.created_by.toString(),
    createdAt: data.created_at,
    updatedAt: data.updated_at,
  };
}

export function mapPickupDataResponse(data: BackendPickupData): PickupData {
  return {
    schedules: (data.schedules ?? []).map(mapPickupScheduleResponse),
    effectiveSchedules: (data.effective_schedules ?? []).map((dated) => ({
      date: dated.date,
      schedule: dated.schedule
        ? mapPickupScheduleResponse(dated.schedule)
        : null,
      offeringSchedule: dated.offering_schedule
        ? mapPickupScheduleResponse(dated.offering_schedule)
        : null,
    })),
    exceptions: (data.exceptions ?? []).map(mapPickupExceptionResponse),
    notes: (data.notes ?? []).map(mapPickupNoteResponse),
    departureExceptions: (data.departure_exceptions ?? []).map(
      mapDepartureExceptionResponse,
    ),
  };
}

export function mapDepartureExceptionResponse(
  data: BackendDepartureException,
): DepartureException {
  return {
    id: data.id.toString(),
    studentId: data.student_id.toString(),
    exceptionDate: data.exception_date,
    departureMode: data.departure_mode,
    reason: data.reason,
  };
}

export function mapPickupScheduleFormToBackend(
  data: PickupScheduleFormData,
): BackendPickupScheduleRequest {
  return {
    weekday: data.weekday,
    pickup_time: data.pickupTime,
    notes: data.notes,
  };
}

export function mapBulkPickupScheduleFormToBackend(
  data: BulkPickupScheduleFormData,
): BackendBulkPickupScheduleRequest {
  return {
    schedules: data.schedules.map(mapPickupScheduleFormToBackend),
    ...(data.effectiveDate ? { effective_date: data.effectiveDate } : {}),
  };
}

export function mapPickupExceptionFormToBackend(
  data: PickupExceptionFormData,
): BackendPickupExceptionRequest {
  return {
    exception_date: data.exceptionDate,
    pickup_time: data.pickupTime,
    ...(data.clearPickupTime ? { clear_pickup_time: true } : {}),
    reason: data.reason,
  };
}

export function mapPickupNoteFormToBackend(
  data: PickupNoteFormData,
): BackendPickupNoteRequest {
  return "weekday" in data
    ? { weekday: data.weekday, content: data.content }
    : { note_date: data.noteDate, content: data.content };
}

// Weekday constants matching backend (Monday=1 to Friday=5)
export const WEEKDAYS = [
  { value: 1, label: "Montag", shortLabel: "Mo" },
  { value: 2, label: "Dienstag", shortLabel: "Di" },
  { value: 3, label: "Mittwoch", shortLabel: "Mi" },
  { value: 4, label: "Donnerstag", shortLabel: "Do" },
  { value: 5, label: "Freitag", shortLabel: "Fr" },
] as const;

// Helper to get weekday label
export function getWeekdayLabel(weekday: number): string {
  const found = WEEKDAYS.find((w) => w.value === weekday);
  return found ? found.label : `Tag ${weekday}`;
}

// Helper to format time for display (removes seconds if present)
export function formatPickupTime(time: string): string {
  // If time is in HH:MM:SS format, strip the seconds
  if (time.length > 5) {
    return time.substring(0, 5);
  }
  return time;
}

// Helper to merge existing schedules with empty template
export function mergeSchedulesWithTemplate(
  existingSchedules: PickupSchedule[],
): PickupScheduleFormData[] {
  return WEEKDAYS.map((w) => {
    const existing = existingSchedules.find((s) => s.weekday === w.value);
    return {
      weekday: w.value,
      pickupTime: existing ? formatPickupTime(existing.pickupTime) : "",
      notes: existing?.notes,
    };
  });
}

// ============================================
// Week View Helpers
// ============================================

/**
 * Format week range as "27.01.2025 - 31.01.2025"
 */
export function formatWeekRange(start: Date, end: Date): string {
  const startDay = start.getDate().toString().padStart(2, "0");
  const startMonth = (start.getMonth() + 1).toString().padStart(2, "0");
  const startYear = start.getFullYear();
  const endDay = end.getDate().toString().padStart(2, "0");
  const endMonth = (end.getMonth() + 1).toString().padStart(2, "0");
  const endYear = end.getFullYear();
  return `${startDay}.${startMonth}.${startYear} - ${endDay}.${endMonth}.${endYear}`;
}

/**
 * Check if two dates are the same day
 */
export function isSameDay(a: Date, b: Date): boolean {
  return (
    a.getFullYear() === b.getFullYear() &&
    a.getMonth() === b.getMonth() &&
    a.getDate() === b.getDate()
  );
}

/**
 * Get weekday number (1=Mon, 5=Fri) from Date
 * Returns null for weekend days
 */
export function getWeekdayFromDate(date: Date): number | null {
  const jsDay = date.getDay(); // 0=Sun, 1=Mon, ..., 6=Sat
  if (jsDay === 0 || jsDay === 6) return null; // Weekend
  return jsDay; // 1-5 for Mon-Fri
}

/**
 * Format date as ISO string (YYYY-MM-DD) for comparison with exceptions
 */
export { formatDateISO };

/**
 * Data for a single day in the week view
 */
export interface DayData {
  date: Date;
  weekday: number; // 1-5 (Mon-Fri)
  isToday: boolean;
  showSick: boolean;
  showClassTrip: boolean;
  showExcused: boolean;
  exception: PickupException | undefined;
  baseSchedule: PickupSchedule | undefined;
  offeringSchedule?: PickupSchedule | undefined;
  effectiveTime: string | undefined;
  effectiveNotes: string | undefined;
  isException: boolean;
  notes: PickupNote[]; // Day-specific notes
  /** The recurring note of this weekday, if any (#3369). */
  weekdayNote?: PickupNote;
  /** Die Abholart, die die OGS nur für diesen Tag gesetzt hat (#3945). */
  departureException?: DepartureException;
}

/**
 * Get merged data for a specific day
 */
export function getDayData(
  date: Date,
  schedules: PickupSchedule[],
  exceptions: PickupException[],
  isSickToday: boolean,
  notes: PickupNote[] = [],
  isExcusedToday = false,
  statusForDate: StudentStatusKind | null = null,
  effectiveSchedules: DatedPickupSchedule[] = [],
  weekendFollowsFriday = false,
  departureExceptions: DepartureException[] = [],
): DayData {
  const calendarWeekday = getWeekdayFromDate(date);
  const weekday = calendarWeekday ?? 5;
  const dateStr = formatDateISO(date);
  const today = new Date();

  // Handle weekend dates explicitly - no schedule available
  if (calendarWeekday === null && !weekendFollowsFriday) {
    return {
      date,
      weekday: 0, // Weekend indicator
      isToday: isSameDay(date, today),
      showSick: false,
      showClassTrip: false,
      showExcused: false,
      exception: undefined,
      baseSchedule: undefined,
      offeringSchedule: undefined,
      effectiveTime: undefined,
      effectiveNotes: undefined,
      isException: false,
      notes: [],
    };
  }

  // Check for exception on this specific date
  const exception = exceptions.find((e) => e.exceptionDate === dateStr);

  // Check if this is today and student is sick / excused. The two flags are
  // mutually exclusive at the backend level; if they somehow both arrive as
  // true we let sick win to stay consistent with LocationBadge precedence.
  const isToday = isSameDay(date, today);
  const showSick = statusForDate
    ? statusForDate === "sick"
    : isToday && isSickToday;
  const showClassTrip = statusForDate === "class_trip";
  const showExcused = statusForDate
    ? statusForDate === "excused"
    : isToday && !showSick && !showClassTrip && isExcusedToday;

  // Get base schedule for this weekday
  const datedSchedule = effectiveSchedules.find(
    (dated) => dated.date === dateStr,
  );
  const baseSchedule = datedSchedule
    ? (datedSchedule.schedule ?? undefined)
    : schedules.find((s) => s.weekday === weekday);
  const offeringSchedule = datedSchedule?.offeringSchedule ?? undefined;

  // Determine effective pickup time:
  // - If sick, class trip, or excused today: no pickup time (child is not being picked up)
  // - If exception exists: use exception's pickup time (even if null = absent)
  // - Otherwise: use base schedule's pickup time
  let effectiveTime: string | undefined;
  if (showSick || showClassTrip || showExcused) {
    effectiveTime = undefined;
  } else if (exception) {
    effectiveTime = exception.pickupTime; // null means absent, don't fall back
  } else {
    effectiveTime = baseSchedule?.pickupTime;
  }

  // Filter notes for this specific date. The recurring weekday note (#3369)
  // stays separate: the day editor lists and edits dated notes only.
  const dayNotes = notes.filter((n) => n.noteDate === dateStr);
  const weekdayNote = notes.find((n) => n.weekday === weekday);

  return {
    date,
    weekday,
    isToday,
    showSick,
    showClassTrip,
    showExcused,
    exception,
    baseSchedule,
    offeringSchedule,
    effectiveTime,
    effectiveNotes: showClassTrip
      ? "Klassenfahrt"
      : (exception?.reason ?? baseSchedule?.notes),
    isException: !!exception,
    notes: dayNotes,
    weekdayNote,
    departureException: departureExceptions.find(
      (entry) => entry.exceptionDate === dateStr,
    ),
  };
}
