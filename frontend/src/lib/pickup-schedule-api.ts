// Pickup Schedule API Client
// Calls Next.js API routes which proxy to the Go backend

import type {
  PickupData,
  BulkPickupScheduleFormData,
  PickupExceptionFormData,
  PickupException,
  PickupNoteFormData,
  PickupNote,
  BackendPickupData,
  BackendPickupException,
  BackendPickupNote,
} from "./pickup-schedule-helpers";
import {
  mapPickupDataResponse,
  mapPickupExceptionResponse,
  mapPickupNoteResponse,
  mapBulkPickupScheduleFormToBackend,
  mapPickupExceptionFormToBackend,
  mapPickupNoteFormToBackend,
} from "./pickup-schedule-helpers";

// API Response Types
interface ApiResponse<T> {
  status: string;
  data?: T;
  message?: string;
  error?: string;
}

// Error response from failed JSON parsing
interface ErrorResponse {
  error: string;
}

// Type guard for error responses
function isErrorResponse(value: unknown): value is ErrorResponse {
  return (
    typeof value === "object" &&
    value !== null &&
    "error" in value &&
    typeof (value as ErrorResponse).error === "string"
  );
}

// Error message translations (English backend -> German frontend)
const errorTranslations: Record<string, string> = {
  "invalid weekday": "Ungültiger Wochentag",
  "pickup_time is required": "Abholzeit ist erforderlich",
  "invalid pickup_time format": "Ungültiges Zeitformat (erwartet HH:MM)",
  "exception_date is required": "Datum ist erforderlich",
  "invalid exception_date format":
    "Ungültiges Datumsformat (erwartet JJJJ-MM-TT)",
  "content is required": "Inhalt ist erforderlich",
  "content too long": "Notiz darf maximal 500 Zeichen lang sein",
  "reason is required": "Grund ist erforderlich",
  "exception already exists":
    "Für dieses Datum existiert bereits eine Ausnahme",
  "student not found": "Schüler/in nicht gefunden",
  unauthorized: "Keine Berechtigung",
  forbidden: "Zugriff verweigert",
  "full access required": "Vollzugriff erforderlich",
};

/**
 * Translate backend error messages to user-friendly German messages
 */
function translateApiError(errorMessage: string): string {
  const lowerError = errorMessage.toLowerCase();

  for (const [pattern, translation] of Object.entries(errorTranslations)) {
    if (lowerError.includes(pattern)) {
      return translation;
    }
  }

  return "Ein Fehler ist aufgetreten. Bitte versuchen Sie es erneut.";
}

/**
 * Fetch pickup schedules and exceptions for a student
 */
export async function fetchStudentPickupData(
  studentId: string,
): Promise<PickupData> {
  const response = await fetch(`/api/students/${studentId}/pickup-schedules`);

  if (!response.ok) {
    const error: unknown = await response
      .json()
      .catch(() => ({ error: "Failed to fetch pickup schedules" }));
    const errorMessage = isErrorResponse(error)
      ? translateApiError(error.error)
      : translateApiError(
          `Failed to fetch pickup schedules: ${response.statusText}`,
        );
    throw new Error(errorMessage);
  }

  const result = (await response.json()) as ApiResponse<BackendPickupData>;

  if (result.status === "error") {
    throw new Error(
      translateApiError(result.error ?? "Failed to fetch pickup schedules"),
    );
  }

  return mapPickupDataResponse(
    result.data ?? { schedules: [], exceptions: [], notes: [] },
  );
}

/**
 * Update weekly pickup schedules for a student (bulk upsert)
 */
export async function updateStudentPickupSchedules(
  studentId: string,
  data: BulkPickupScheduleFormData,
): Promise<PickupData> {
  const backendData = mapBulkPickupScheduleFormToBackend(data);

  const response = await fetch(`/api/students/${studentId}/pickup-schedules`, {
    method: "PUT",
    headers: {
      "Content-Type": "application/json",
    },
    body: JSON.stringify(backendData),
  });

  if (!response.ok) {
    const error: unknown = await response
      .json()
      .catch(() => ({ error: "Failed to update pickup schedules" }));
    const errorMessage = isErrorResponse(error)
      ? translateApiError(error.error)
      : translateApiError(
          `Failed to update pickup schedules: ${response.statusText}`,
        );
    throw new Error(errorMessage);
  }

  const result = (await response.json()) as ApiResponse<BackendPickupData>;

  if (result.status === "error" || !result.data) {
    throw new Error(
      translateApiError(result.error ?? "Failed to update pickup schedules"),
    );
  }

  return mapPickupDataResponse(result.data);
}

/**
 * Create a pickup exception for a student
 */
export async function createStudentPickupException(
  studentId: string,
  data: PickupExceptionFormData,
): Promise<PickupException> {
  const backendData = mapPickupExceptionFormToBackend(data);

  const response = await fetch(`/api/students/${studentId}/pickup-exceptions`, {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
    },
    body: JSON.stringify(backendData),
  });

  if (!response.ok) {
    const error: unknown = await response
      .json()
      .catch(() => ({ error: "Failed to create pickup exception" }));
    const errorMessage = isErrorResponse(error)
      ? translateApiError(error.error)
      : translateApiError(
          `Failed to create pickup exception: ${response.statusText}`,
        );
    throw new Error(errorMessage);
  }

  const result = (await response.json()) as ApiResponse<BackendPickupException>;

  if (result.status === "error" || !result.data) {
    throw new Error(
      translateApiError(result.error ?? "Failed to create pickup exception"),
    );
  }

  return mapPickupExceptionResponse(result.data);
}

/**
 * Update a pickup exception
 */
export async function updateStudentPickupException(
  studentId: string,
  exceptionId: string,
  data: PickupExceptionFormData,
): Promise<PickupException> {
  const backendData = mapPickupExceptionFormToBackend(data);

  const response = await fetch(
    `/api/students/${studentId}/pickup-exceptions/${exceptionId}`,
    {
      method: "PUT",
      headers: {
        "Content-Type": "application/json",
      },
      body: JSON.stringify(backendData),
    },
  );

  if (!response.ok) {
    const error: unknown = await response
      .json()
      .catch(() => ({ error: "Failed to update pickup exception" }));
    const errorMessage = isErrorResponse(error)
      ? translateApiError(error.error)
      : translateApiError(
          `Failed to update pickup exception: ${response.statusText}`,
        );
    throw new Error(errorMessage);
  }

  const result = (await response.json()) as ApiResponse<BackendPickupException>;

  if (result.status === "error" || !result.data) {
    throw new Error(
      translateApiError(result.error ?? "Failed to update pickup exception"),
    );
  }

  return mapPickupExceptionResponse(result.data);
}

/**
 * Delete a pickup exception
 */
export async function deleteStudentPickupException(
  studentId: string,
  exceptionId: string,
): Promise<void> {
  const response = await fetch(
    `/api/students/${studentId}/pickup-exceptions/${exceptionId}`,
    {
      method: "DELETE",
    },
  );

  if (!response.ok) {
    const error: unknown = await response
      .json()
      .catch(() => ({ error: "Failed to delete pickup exception" }));
    const errorMessage = isErrorResponse(error)
      ? translateApiError(error.error)
      : translateApiError(
          `Failed to delete pickup exception: ${response.statusText}`,
        );
    throw new Error(errorMessage);
  }

  // 204 No Content means successful deletion
  if (response.status === 204) {
    return;
  }

  // If there's a response body, parse it
  const result = (await response.json()) as ApiResponse<null>;

  if (result.status === "error") {
    throw new Error(
      translateApiError(result.error ?? "Failed to delete pickup exception"),
    );
  }
}

// =============================================================================
// PICKUP NOTES API
// =============================================================================

/**
 * Create a pickup note for a student
 */
export async function createStudentPickupNote(
  studentId: string,
  data: PickupNoteFormData,
): Promise<PickupNote> {
  const backendData = mapPickupNoteFormToBackend(data);

  const response = await fetch(`/api/students/${studentId}/pickup-notes`, {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
    },
    body: JSON.stringify(backendData),
  });

  if (!response.ok) {
    const error: unknown = await response
      .json()
      .catch(() => ({ error: "Failed to create pickup note" }));
    const errorMessage = isErrorResponse(error)
      ? translateApiError(error.error)
      : translateApiError(
          `Failed to create pickup note: ${response.statusText}`,
        );
    throw new Error(errorMessage);
  }

  const result = (await response.json()) as ApiResponse<BackendPickupNote>;

  if (result.status === "error" || !result.data) {
    throw new Error(
      translateApiError(result.error ?? "Failed to create pickup note"),
    );
  }

  return mapPickupNoteResponse(result.data);
}

/**
 * Update a pickup note
 */
export async function updateStudentPickupNote(
  studentId: string,
  noteId: string,
  data: PickupNoteFormData,
): Promise<PickupNote> {
  const backendData = mapPickupNoteFormToBackend(data);

  const response = await fetch(
    `/api/students/${studentId}/pickup-notes/${noteId}`,
    {
      method: "PUT",
      headers: {
        "Content-Type": "application/json",
      },
      body: JSON.stringify(backendData),
    },
  );

  if (!response.ok) {
    const error: unknown = await response
      .json()
      .catch(() => ({ error: "Failed to update pickup note" }));
    const errorMessage = isErrorResponse(error)
      ? translateApiError(error.error)
      : translateApiError(
          `Failed to update pickup note: ${response.statusText}`,
        );
    throw new Error(errorMessage);
  }

  const result = (await response.json()) as ApiResponse<BackendPickupNote>;

  if (result.status === "error" || !result.data) {
    throw new Error(
      translateApiError(result.error ?? "Failed to update pickup note"),
    );
  }

  return mapPickupNoteResponse(result.data);
}

/**
 * Delete a pickup note
 */
export async function deleteStudentPickupNote(
  studentId: string,
  noteId: string,
): Promise<void> {
  const response = await fetch(
    `/api/students/${studentId}/pickup-notes/${noteId}`,
    {
      method: "DELETE",
    },
  );

  if (!response.ok) {
    const error: unknown = await response
      .json()
      .catch(() => ({ error: "Failed to delete pickup note" }));
    const errorMessage = isErrorResponse(error)
      ? translateApiError(error.error)
      : translateApiError(
          `Failed to delete pickup note: ${response.statusText}`,
        );
    throw new Error(errorMessage);
  }

  // 204 No Content means successful deletion
  if (response.status === 204) {
    return;
  }

  // If there's a response body, parse it
  const result = (await response.json()) as ApiResponse<null>;

  if (result.status === "error") {
    throw new Error(
      translateApiError(result.error ?? "Failed to delete pickup note"),
    );
  }
}

// =============================================================================
// BULK PICKUP TIMES API (for OGS dashboard)
// =============================================================================

/**
 * Backend day note in bulk response
 */
interface BulkDayNoteResponse {
  id: number;
  content: string;
}

/**
 * Frontend day note in bulk response
 */
export interface BulkDayNote {
  id: string;
  content: string;
}

/**
 * Bulk pickup time response from backend
 */
export interface BulkPickupTimeResponse {
  student_id: number;
  date: string;
  weekday_name: string;
  pickup_time?: string;
  is_exception: boolean;
  day_notes?: BulkDayNoteResponse[];
  notes?: string;
}

/**
 * Frontend-friendly bulk pickup time
 */
export interface BulkPickupTime {
  studentId: string;
  date: string;
  weekdayName: string;
  pickupTime?: string;
  isException: boolean;
  dayNotes: BulkDayNote[];
  notes?: string;
}

/**
 * Map backend bulk pickup time to frontend format
 */
function mapBulkPickupTimeResponse(
  data: BulkPickupTimeResponse,
): BulkPickupTime {
  return {
    studentId: data.student_id.toString(),
    date: data.date,
    weekdayName: data.weekday_name,
    pickupTime: data.pickup_time,
    isException: data.is_exception,
    dayNotes: (data.day_notes ?? []).map((n) => ({
      id: n.id.toString(),
      content: n.content,
    })),
    notes: data.notes,
  };
}

/**
 * Fetch effective pickup times for multiple students on a given date.
 * Uses bulk backend endpoint (O(2) queries instead of O(N)).
 *
 * @param studentIds - Array of student IDs
 * @param date - Optional date string (YYYY-MM-DD), defaults to today
 * @returns Map of studentId -> pickup time data
 */
export async function fetchBulkPickupTimes(
  studentIds: string[],
  date?: string,
): Promise<Map<string, BulkPickupTime>> {
  if (studentIds.length === 0) {
    return new Map();
  }

  const response = await fetch("/api/students/pickup-times/bulk", {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
    },
    body: JSON.stringify({
      student_ids: studentIds.map((id) => Number.parseInt(id, 10)),
      date: date,
    }),
  });

  if (!response.ok) {
    const error: unknown = await response
      .json()
      .catch(() => ({ error: "Failed to fetch bulk pickup times" }));
    const errorMessage = isErrorResponse(error)
      ? translateApiError(error.error)
      : translateApiError(
          `Failed to fetch bulk pickup times: ${response.statusText}`,
        );
    throw new Error(errorMessage);
  }

  const result = (await response.json()) as ApiResponse<
    BulkPickupTimeResponse[]
  >;

  if (result.status === "error" || !result.data) {
    throw new Error(
      translateApiError(result.error ?? "Failed to fetch bulk pickup times"),
    );
  }

  // Convert array to Map for O(1) lookup
  const pickupTimesMap = new Map<string, BulkPickupTime>();
  for (const item of result.data) {
    const mapped = mapBulkPickupTimeResponse(item);
    pickupTimesMap.set(mapped.studentId, mapped);
  }

  return pickupTimesMap;
}
