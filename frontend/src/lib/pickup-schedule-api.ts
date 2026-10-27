import type { RosterImpactWire } from "~/lib/roster-impact";
import { ApiError, apiErrorFromBody, transportFetch } from "./api-error";
// Pickup Schedule API Client
// Calls Next.js API routes which proxy to the Go backend

import type {
  PickupData,
  PickupExceptionFormData,
  PickupException,
  PickupNoteFormData,
  PickupNote,
  WeekdayNoteEntry,
  BackendPickupData,
  BackendPickupException,
  BackendPickupNote,
  BackendDepartureException,
  DepartureException,
  DepartureMode,
} from "./pickup-schedule-helpers";
import {
  mapDepartureExceptionResponse,
  mapPickupDataResponse,
  mapPickupExceptionResponse,
  mapPickupNoteResponse,
  mapPickupExceptionFormToBackend,
  mapPickupNoteFormToBackend,
} from "./pickup-schedule-helpers";
import type { ArrivalScheduleInput } from "./student-arrival-api";

// API Response Types
interface ApiResponse<T> {
  status: string;
  data?: T;
  message?: string;
}

export interface BulkPickupScheduleInput {
  weekday: number;
  pickup_time: string;
}

export interface BulkPickupResult {
  students_affected: number;
}

export interface PickupAdjustmentSelection {
  offering_id: string;
  selected_days: string[];
}

export interface PickupAdjustmentMatch {
  offering_id: string;
  name: string;
  selected_days: string[];
  selections: PickupAdjustmentSelection[];
}

export interface PickupAdjustmentCatalogItem {
  offering_id: string;
  name: string;
  description?: string;
  days_of_week_mode: "fixed" | "parent_choice";
  available_days: string[];
  selection_group?: string;
  selection_rule: string;
  is_required: boolean;
  price_cents?: number;
  includes_lunch: boolean;
  includes_holiday_care: boolean;
  selected: boolean;
  selected_days: string[];
  automatic: boolean;
  is_active: boolean;
  capacity?: number;
  free_slots?: number;
  pickup_times: Record<string, string>;
  counts_as_care: boolean;
}

interface PickupAdjustmentCatalog {
  phase_id: string;
  phase_name: string;
  selection_mode: string;
  earliest_effective_from: string;
  latest_effective_from: string;
  items: PickupAdjustmentCatalogItem[];
}

interface PickupAdjustmentConsequences {
  selections: Array<{
    offering_id: string;
    state: "booked" | "removed";
    days: string[];
  }>;
  manual_planning_conflicts: Array<{
    activity_group_id: string;
    activity_group_name: string;
    days: string[];
    first_date: string;
    occurrence_count: number;
  }>;
  arrival_expectations_follow_bookings: boolean;
}

export interface PickupAdjustmentPreview {
  preview_token: string;
  effective_from: string;
  current_plan: string;
  proposed_plan: string;
  deviates_from_offering: boolean;
  resolution_required: boolean;
  matching_offerings: PickupAdjustmentMatch[];
  offering_catalog?: PickupAdjustmentCatalog;
  offering_consequences?: PickupAdjustmentConsequences;
  removed_manual_notes?: Array<{ weekday: number; note: string }>;
}

export interface PickupAdjustmentPayload {
  schedules: Array<{
    weekday: number;
    pickup_time: string;
    notes?: string;
  }>;
  care_days: number[];
  arrival_schedules?: ArrivalScheduleInput[];
  effective_from: string;
  selections?: PickupAdjustmentSelection[];
  excluded_auto_offering_ids?: string[];
  complete_withdrawal_confirmed?: boolean;
}

export type PickupAdjustmentResolution = "exception" | "offering";

export async function bulkUpsertPickupSchedules(
  studentIds: string[],
  schedules: BulkPickupScheduleInput[],
  confirmedException = false,
): Promise<BulkPickupResult> {
  const numericIds = [...new Set(studentIds)].map((id) =>
    Number.parseInt(id, 10),
  );
  if (
    numericIds.length === 0 ||
    numericIds.length > 500 ||
    numericIds.some((id) => !Number.isSafeInteger(id) || id <= 0)
  ) {
    throw new ApiError("Ungültige Kinderauswahl", 400, {
      code: "general.input",
    });
  }

  const response = await transportFetch("/api/students/pickup-schedules/bulk", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({
      student_ids: numericIds,
      schedules,
      ...(confirmedException ? { confirmed_exception: true } : {}),
    }),
  });
  if (!response.ok) {
    await throwResponseError(response, "Failed to update pickup schedules");
  }
  return parseApiResult<BulkPickupResult>(
    response,
    "Failed to update pickup schedules",
  );
}

export async function previewStudentPickupAdjustment(
  studentId: string,
  payload: PickupAdjustmentPayload,
): Promise<PickupAdjustmentPreview> {
  const response = await transportFetch(
    `/api/students/${studentId}/pickup-schedules/preview`,
    {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(payload),
    },
  );
  if (!response.ok) {
    await throwResponseError(
      response,
      "Gehzeiten konnten nicht geprüft werden",
    );
  }
  return parseApiResult<PickupAdjustmentPreview>(
    response,
    "Gehzeiten konnten nicht geprüft werden",
  );
}

export async function applyStudentPickupAdjustment(
  studentId: string,
  payload: PickupAdjustmentPayload & {
    preview_token: string;
    resolution: PickupAdjustmentResolution;
    reason?: string;
    complete_withdrawal_confirmed?: boolean;
  },
): Promise<PickupAdjustmentApplyResult> {
  const response = await transportFetch(
    `/api/students/${studentId}/pickup-schedules/apply`,
    {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(payload),
    },
  );
  if (!response.ok) {
    await throwResponseError(
      response,
      "Gehzeiten konnten nicht gespeichert werden",
    );
  }
  return parseApiResult<PickupAdjustmentApplyResult>(
    response,
    "Gehzeiten konnten nicht gespeichert werden",
  );
}

/** A switched offering reports its effect on the Regeltermine (#5102). */
export interface PickupAdjustmentApplyResult {
  resolution: PickupAdjustmentResolution;
  roster_impact?: RosterImpactWire;
}

/**
 * The failed response as an ApiError: code, field errors and request ID come
 * from the envelope, so the screen shows the catalog text (#2517). The
 * message stays developer diagnosis.
 */
async function throwResponseError(
  response: Response,
  fallback: string,
): Promise<never> {
  throw apiErrorFromBody(
    fallback,
    response.status,
    await response.json().catch(() => undefined),
  );
}

/**
 * A success status whose body lacks the data is a server fault.
 */
function bodyError(message: string, body: unknown): ApiError {
  return apiErrorFromBody(message, 500, body);
}

/**
 * Parse a JSON API response body; a success without data is a server fault.
 */
async function parseApiResult<T>(
  response: Response,
  fallback: string,
): Promise<T> {
  const result = (await response.json()) as ApiResponse<T>;

  if (!result.data) {
    throw bodyError(fallback, result);
  }

  return result.data;
}

/**
 * Fetch pickup schedules and exceptions for a student
 */
export async function fetchStudentPickupData(
  studentId: string,
  range?: { from: string; to: string },
): Promise<PickupData> {
  const query = new URLSearchParams();
  if (range) {
    query.set("from", range.from);
    query.set("to", range.to);
  }
  const suffix = query.size > 0 ? `?${query.toString()}` : "";
  const response = await transportFetch(
    `/api/students/${studentId}/pickup-schedules${suffix}`,
  );

  if (!response.ok) {
    await throwResponseError(response, "Failed to fetch pickup schedules");
  }

  const result = (await response.json()) as ApiResponse<BackendPickupData>;

  return mapPickupDataResponse(
    result.data ?? { schedules: [], exceptions: [], notes: [] },
  );
}

/**
 * Setzt die Gehzeit eines Wochentags auf die Angebots-Gehzeit zurück (#2290).
 * Der Server lehnt den Reset ab, wenn an diesem Datum keine Angebots-Gehzeit gilt.
 */
export async function resetStudentPickupToOffering(
  studentId: string,
  weekday: number,
  date: string,
): Promise<PickupData> {
  const response = await transportFetch(
    `/api/students/${studentId}/pickup-schedules/reset-offering`,
    {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ weekday, date }),
    },
  );

  if (!response.ok) {
    await throwResponseError(response, "Failed to reset pickup schedule");
  }

  return parseApiResult<BackendPickupData>(
    response,
    "Failed to reset pickup schedule",
  ).then(mapPickupDataResponse);
}

/**
 * Create a pickup exception for a student
 */
export async function createStudentPickupException(
  studentId: string,
  data: PickupExceptionFormData,
): Promise<PickupException> {
  const backendData = mapPickupExceptionFormToBackend(data);

  const response = await transportFetch(
    `/api/students/${studentId}/pickup-exceptions`,
    {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(backendData),
    },
  );

  if (!response.ok) {
    await throwResponseError(response, "Failed to create pickup exception");
  }

  const data_ = await parseApiResult<BackendPickupException>(
    response,
    "Failed to create pickup exception",
  );
  return mapPickupExceptionResponse(data_);
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

  const response = await transportFetch(
    `/api/students/${studentId}/pickup-exceptions/${exceptionId}`,
    {
      method: "PUT",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(backendData),
    },
  );

  if (!response.ok) {
    await throwResponseError(response, "Failed to update pickup exception");
  }

  const data_ = await parseApiResult<BackendPickupException>(
    response,
    "Failed to update pickup exception",
  );
  return mapPickupExceptionResponse(data_);
}

/**
 * Delete a pickup exception
 */
export async function deleteStudentPickupException(
  studentId: string,
  exceptionId: string,
): Promise<void> {
  const response = await transportFetch(
    `/api/students/${studentId}/pickup-exceptions/${exceptionId}`,
    { method: "DELETE" },
  );

  if (!response.ok) {
    await throwResponseError(response, "Failed to delete pickup exception");
  }
}

/**
 * Setzt die Abholart eines einzelnen Tages (#3945). Ein Tag hat höchstens
 * einen Wert; erneutes Speichern ersetzt ihn.
 */
export async function setStudentDepartureException(
  studentId: string,
  date: string,
  data: { departureMode: DepartureMode; reason?: string },
): Promise<DepartureException> {
  const response = await transportFetch(
    `/api/students/${studentId}/departure-exceptions/${date}`,
    {
      method: "PUT",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        departure_mode: data.departureMode,
        reason: data.reason,
      }),
    },
  );

  if (!response.ok) {
    await throwResponseError(response, "Failed to set departure exception");
  }

  const result = await parseApiResult<BackendDepartureException>(
    response,
    "Failed to set departure exception",
  );
  return mapDepartureExceptionResponse(result);
}

/**
 * Gibt den Tag an den Wochenplan zurück: die Abholart des Tages entfällt.
 */
export async function deleteStudentDepartureException(
  studentId: string,
  date: string,
): Promise<void> {
  const response = await transportFetch(
    `/api/students/${studentId}/departure-exceptions/${date}`,
    { method: "DELETE" },
  );

  if (!response.ok) {
    await throwResponseError(response, "Failed to delete departure exception");
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

  const response = await transportFetch(
    `/api/students/${studentId}/pickup-notes`,
    {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(backendData),
    },
  );

  if (!response.ok) {
    await throwResponseError(response, "Failed to create pickup note");
  }

  const data_ = await parseApiResult<BackendPickupNote>(
    response,
    "Failed to create pickup note",
  );
  return mapPickupNoteResponse(data_);
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

  const response = await transportFetch(
    `/api/students/${studentId}/pickup-notes/${noteId}`,
    {
      method: "PUT",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(backendData),
    },
  );

  if (!response.ok) {
    await throwResponseError(response, "Failed to update pickup note");
  }

  const data_ = await parseApiResult<BackendPickupNote>(
    response,
    "Failed to update pickup note",
  );
  return mapPickupNoteResponse(data_);
}

/**
 * Delete a pickup note
 */
export async function deleteStudentPickupNote(
  studentId: string,
  noteId: string,
): Promise<void> {
  const response = await transportFetch(
    `/api/students/${studentId}/pickup-notes/${noteId}`,
    { method: "DELETE" },
  );

  if (!response.ok) {
    await throwResponseError(response, "Failed to delete pickup note");
  }
}

/**
 * Replace all recurring weekday notes in one request. Dated notes remain in
 * place because they belong to the single-day editor.
 */
export async function replaceStudentWeekdayPickupNotes(
  studentId: string,
  notes: readonly WeekdayNoteEntry[],
): Promise<void> {
  const response = await transportFetch(
    `/api/students/${studentId}/pickup-notes`,
    {
      method: "PUT",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ notes }),
    },
  );

  if (!response.ok) {
    await throwResponseError(response, "Failed to update pickup notes");
  }
}

// =============================================================================
// BULK PICKUP TIMES (types for the OGS dashboard)
// =============================================================================

/**
 * Frontend day note in bulk response
 */
interface BulkDayNote {
  id: string;
  content: string;
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
  /** The Abholart set for this day only (#3946); only the supervision
   *  dashboard carries it. */
  dayDepartureMode?: DepartureMode;
  dayNotes: BulkDayNote[];
  notes?: string;
}
