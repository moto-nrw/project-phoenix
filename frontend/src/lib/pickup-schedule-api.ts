// Pickup Schedule API Client
// Calls Next.js API routes which proxy to the Go backend

import type {
  PickupData,
  BulkPickupScheduleFormData,
  PickupExceptionFormData,
  PickupException,
  BackendPickupData,
  BackendPickupException,
} from "./pickup-schedule-helpers";
import {
  mapPickupDataResponse,
  mapPickupExceptionResponse,
  mapBulkPickupScheduleFormToBackend,
  mapPickupExceptionFormToBackend,
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
    result.data ?? { schedules: [], exceptions: [] },
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
