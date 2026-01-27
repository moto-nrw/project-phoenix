// Pickup Schedule Type Definitions and Mapping Helpers

// Frontend Pickup Schedule Type
export interface PickupSchedule {
  id: string;
  studentId: string;
  weekday: number;
  weekdayName: string;
  pickupTime: string; // HH:MM format
  notes?: string;
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
  reason: string;
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
  reason: string;
  created_by: number;
  created_at: string;
  updated_at: string;
}

// Combined Pickup Data (schedules + exceptions)
export interface PickupData {
  schedules: PickupSchedule[];
  exceptions: PickupException[];
}

// Backend Combined Pickup Data Response
export interface BackendPickupData {
  schedules: BackendPickupSchedule[];
  exceptions: BackendPickupException[];
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
}

// Backend bulk schedule request
export interface BackendBulkPickupScheduleRequest {
  schedules: BackendPickupScheduleRequest[];
}

// Request types for creating/updating exceptions
export interface PickupExceptionFormData {
  exceptionDate: string; // YYYY-MM-DD format
  pickupTime?: string; // HH:MM format, undefined = no pickup
  reason: string;
}

// Backend exception request
export interface BackendPickupExceptionRequest {
  exception_date: string;
  pickup_time?: string;
  reason: string;
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
    createdBy: data.created_by.toString(),
    createdAt: data.created_at,
    updatedAt: data.updated_at,
  };
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
    createdBy: data.created_by.toString(),
    createdAt: data.created_at,
    updatedAt: data.updated_at,
  };
}

export function mapPickupDataResponse(data: BackendPickupData): PickupData {
  return {
    schedules: (data.schedules ?? []).map(mapPickupScheduleResponse),
    exceptions: (data.exceptions ?? []).map(mapPickupExceptionResponse),
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
  };
}

export function mapPickupExceptionFormToBackend(
  data: PickupExceptionFormData,
): BackendPickupExceptionRequest {
  return {
    exception_date: data.exceptionDate,
    pickup_time: data.pickupTime,
    reason: data.reason,
  };
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

// Helper to get weekday short label
export function getWeekdayShortLabel(weekday: number): string {
  const found = WEEKDAYS.find((w) => w.value === weekday);
  return found ? found.shortLabel : `T${weekday}`;
}

// Helper to format time for display (removes seconds if present)
export function formatPickupTime(time: string): string {
  // If time is in HH:MM:SS format, strip the seconds
  if (time.length > 5) {
    return time.substring(0, 5);
  }
  return time;
}

// Helper to parse YYYY-MM-DD as local date (not UTC)
// Note: new Date("YYYY-MM-DD") parses as UTC midnight, which can shift the date
// when displayed in local timezone. This function parses as local midnight.
function parseLocalDate(dateStr: string): Date {
  const parts = dateStr.split("-").map(Number);
  const year = parts[0] ?? 1970;
  const month = (parts[1] ?? 1) - 1;
  const day = parts[2] ?? 1;
  return new Date(year, month, day);
}

// Helper to format date for display (German format)
export function formatExceptionDate(dateStr: string): string {
  const date = parseLocalDate(dateStr);
  return date.toLocaleDateString("de-DE", {
    weekday: "short",
    day: "2-digit",
    month: "2-digit",
    year: "numeric",
  });
}

// Helper to check if a date is in the past
export function isDateInPast(dateStr: string): boolean {
  const date = parseLocalDate(dateStr);
  const today = new Date();
  today.setHours(0, 0, 0, 0);
  return date < today;
}

// Helper to get schedule for a specific weekday
export function getScheduleForWeekday(
  schedules: PickupSchedule[],
  weekday: number,
): PickupSchedule | undefined {
  return schedules.find((s) => s.weekday === weekday);
}

// Helper to create empty schedule data for all weekdays
export function createEmptyWeeklySchedule(): PickupScheduleFormData[] {
  return WEEKDAYS.map((w) => ({
    weekday: w.value,
    pickupTime: "",
    notes: undefined,
  }));
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
 * Get the Monday of a week relative to the current week
 * @param weekOffset 0 = current week, 1 = next week, -1 = last week
 */
export function getWeekStart(weekOffset = 0): Date {
  const today = new Date();
  const dayOfWeek = today.getDay(); // 0=Sun, 1=Mon, ..., 6=Sat
  // Calculate days to subtract to get to Monday (if Sunday, go back 6 days)
  const daysToMonday = dayOfWeek === 0 ? 6 : dayOfWeek - 1;
  const monday = new Date(today);
  monday.setDate(today.getDate() - daysToMonday + weekOffset * 7);
  monday.setHours(0, 0, 0, 0);
  return monday;
}

/**
 * Get the Friday of a week relative to the current week
 * @param weekOffset 0 = current week, 1 = next week, -1 = last week
 */
export function getWeekEnd(weekOffset = 0): Date {
  const monday = getWeekStart(weekOffset);
  const friday = new Date(monday);
  friday.setDate(monday.getDate() + 4); // Monday + 4 = Friday
  return friday;
}

/**
 * Get all 5 weekdays (Mon-Fri) for a given week
 * @param weekOffset 0 = current week, 1 = next week, -1 = last week
 */
export function getWeekDays(weekOffset = 0): Date[] {
  const monday = getWeekStart(weekOffset);
  const days: Date[] = [];
  for (let i = 0; i < 5; i++) {
    const day = new Date(monday);
    day.setDate(monday.getDate() + i);
    days.push(day);
  }
  return days;
}

/**
 * Format date as "27.01." (German short format)
 */
export function formatShortDate(date: Date): string {
  const day = date.getDate().toString().padStart(2, "0");
  const month = (date.getMonth() + 1).toString().padStart(2, "0");
  return `${day}.${month}.`;
}

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
 * Get ISO week number (1-53) for a given date
 * Uses ISO 8601 definition: Week 1 is the week containing the first Thursday
 */
export function getCalendarWeek(date: Date): number {
  const d = new Date(
    Date.UTC(date.getFullYear(), date.getMonth(), date.getDate()),
  );
  // Set to nearest Thursday: current date + 4 - current day number (Mon=1, Sun=7)
  const dayNum = d.getUTCDay() || 7;
  d.setUTCDate(d.getUTCDate() + 4 - dayNum);
  // Get first day of year
  const yearStart = new Date(Date.UTC(d.getUTCFullYear(), 0, 1));
  // Calculate full weeks to nearest Thursday
  const weekNo = Math.ceil(
    ((d.getTime() - yearStart.getTime()) / 86400000 + 1) / 7,
  );
  return weekNo;
}

/**
 * Format date as ISO string (YYYY-MM-DD) for comparison with exceptions
 */
export function formatDateISO(date: Date): string {
  const year = date.getFullYear();
  const month = (date.getMonth() + 1).toString().padStart(2, "0");
  const day = date.getDate().toString().padStart(2, "0");
  return `${year}-${month}-${day}`;
}

/**
 * Data for a single day in the week view
 */
export interface DayData {
  date: Date;
  weekday: number; // 1-5 (Mon-Fri)
  isToday: boolean;
  showSick: boolean;
  exception: PickupException | undefined;
  baseSchedule: PickupSchedule | undefined;
  effectiveTime: string | undefined;
  effectiveNotes: string | undefined;
  isException: boolean;
}

/**
 * Get merged data for a specific day
 */
export function getDayData(
  date: Date,
  schedules: PickupSchedule[],
  exceptions: PickupException[],
  isSickToday: boolean,
): DayData {
  const weekday = getWeekdayFromDate(date) ?? 1;
  const dateStr = formatDateISO(date);
  const today = new Date();

  // Check for exception on this specific date
  const exception = exceptions.find((e) => e.exceptionDate === dateStr);

  // Check if this is today and student is sick
  const isToday = isSameDay(date, today);
  const showSick = isToday && isSickToday;

  // Get base schedule for this weekday
  const baseSchedule = schedules.find((s) => s.weekday === weekday);

  // Determine effective pickup time:
  // - If sick today: no pickup time (undefined)
  // - If exception exists: use exception's pickup time (even if null = absent)
  // - Otherwise: use base schedule's pickup time
  const effectiveTime = showSick
    ? undefined
    : exception
      ? exception.pickupTime // Use exception time (null means absent, don't fall back)
      : baseSchedule?.pickupTime;

  return {
    date,
    weekday,
    isToday,
    showSick,
    exception,
    baseSchedule,
    effectiveTime,
    effectiveNotes: exception?.reason ?? baseSchedule?.notes,
    isException: !!exception,
  };
}
