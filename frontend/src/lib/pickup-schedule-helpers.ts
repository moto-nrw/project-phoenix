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

// Helper to format date for display (German format)
export function formatExceptionDate(dateStr: string): string {
  const date = new Date(dateStr);
  return date.toLocaleDateString("de-DE", {
    weekday: "short",
    day: "2-digit",
    month: "2-digit",
    year: "numeric",
  });
}

// Helper to check if a date is in the past
export function isDateInPast(dateStr: string): boolean {
  const date = new Date(dateStr);
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
