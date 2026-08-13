import type { AxiosError } from "axios";
import { clearSessionCache, getCachedSession } from "./session-cache";
import { createLogger } from "~/lib/logger";
import { env } from "~/env";
import api from "./api-transport";
import { convertToBackendRoom, fetchWithRetry } from "./api-helpers";
import {
  mapSingleStudentResponse,
  mapStudentsResponse,
  mapStudentDetailResponse,
  prepareStudentForBackend,
} from "./student-helpers";
import type {
  BackendStudent,
  BackendStudentDetail,
  Student,
} from "./student-helpers";
import {
  mapSingleGroupResponse,
  mapGroupResponse, // Used internally in getGroup
  prepareGroupForBackend,
  mapGroupsResponse,
} from "./group-helpers";

// Re-export for external consumers
export { mapGroupResponse } from "./group-helpers";
import type { BackendGroup, Group as ImportedGroup } from "./group-helpers";
import {
  mapSingleRoomResponse,
  prepareRoomForBackend,
  mapRoomsResponse,
} from "./room-helpers";

// Re-export for external consumers
export { mapRoomResponse } from "./room-helpers";
import type { BackendRoom } from "./room-helpers";
import { handleAuthFailure } from "./auth-failure";
import {
  ALL_COMPANION_WEEKDAYS,
  notifyStudentCompanionsChanged,
  type CompanionExtensionConfirmation,
  type CompanionWeekday,
} from "./student-companion-api";

// Logger instance for API client
const logger = createLogger({ component: "ApiClient" });

/**
 * Whether the update that just returned should tell mounted "läuft mit" views
 * to refetch (#1694).
 *
 * The announcement is not a harmless refetch: an open Laufgemeinschaft form
 * answers it by discarding its draft, or — when the draft is dirty — by
 * blocking the save until the user reloads. So it has to follow what the write
 * DID, not what the request looked like: the Stammdaten forms resubmit the whole
 * departure plan on every save, so the payload shape says "may have changed
 * links" for a pure name or address edit that changed none, and announcing that
 * costs some other editor their unsaved work.
 *
 * The backend answers the question itself — `companions_changed` on the update
 * response, the same verdict that decides its `student_companions_changed`
 * broadcast — so that answer wins whenever it is present. It is absent only from
 * a response that carries no verdict at all; there we fall back to the
 * conservative payload heuristic, because a missed announcement leaves a stale
 * list on screen.
 */
function companionsChangedFromResponse(payload: unknown): boolean | undefined {
  if (!payload || typeof payload !== "object") return undefined;
  const changed = (payload as { companions_changed?: unknown })
    .companions_changed;
  if (typeof changed === "boolean") return changed;
  // The server-side path hands in the backend's whole { status, data } envelope,
  // the browser path the already-unwrapped student — accept either.
  const inner = (payload as { data?: unknown }).data;
  if (!inner || typeof inner !== "object") return undefined;
  const nested = (inner as { companions_changed?: unknown }).companions_changed;
  return typeof nested === "boolean" ? nested : undefined;
}

/**
 * The fallback heuristic: whether a student PUT payload can change links at all.
 *
 * Three payload shapes CAN, mirroring what the backend broadcasts on: a
 * submitted list (it replaces the stored one), a confirmed plan extension (it
 * widens a linked child's plan), and a departure-plan change (the backend trims
 * links off weekdays the new plan no longer allows). Everything else stays
 * silent — a genuinely remote change still arrives via the post-commit
 * `student_companions_changed` SSE event.
 */
function studentUpdateMayChangeCompanions(
  student: Partial<Student> & {
    companions?: unknown;
    confirmed_companion_extensions?: CompanionExtensionConfirmation[];
  },
): boolean {
  return (
    student.companions !== undefined ||
    (student.confirmed_companion_extensions?.length ?? 0) > 0 ||
    student.allowed_departure_modes !== undefined ||
    student.departure_days !== undefined ||
    student.bus_days !== undefined ||
    student.pickup_days !== undefined
  );
}

// Helper function to safely handle errors
function handleApiError(error: unknown, context: string): Error {
  // Extract error details
  const errorMessage = error instanceof Error ? error.message : String(error);
  const statusMatch = /API error[:\s(]+(\d{3})/.exec(errorMessage);
  const status =
    error &&
    typeof error === "object" &&
    "response" in error &&
    error.response &&
    typeof error.response === "object" &&
    "status" in error.response
      ? (error.response.status as number)
      : statusMatch?.[1]
        ? Number.parseInt(statusMatch[1], 10)
        : undefined;

  const logContext = {
    context,
    error: errorMessage,
    status,
    ...(status === 429 && { rate_limited: true }),
  };
  if (status === 429) {
    logger.warn("api operation rate limited", logContext);
  } else {
    logger.error("api operation failed", logContext);
  }

  return new Error(`${context}: ${errorMessage}`);
}

// Paginated response interface for API responses with pagination metadata
interface PaginatedResponse<T> {
  status: string;
  data: T[];
  pagination: {
    current_page: number;
    page_size: number;
    total_pages: number;
    total_records: number;
  };
  message?: string;
}

// API response wrapper types
interface ApiResponseWrapper<T> {
  success: boolean;
  message?: string;
  data: T;
}

// Pagination info type for student responses
interface StudentPaginationInfo {
  current_page: number;
  page_size: number;
  total_pages: number;
  total_records: number;
}

// Result type for paginated student responses
interface StudentsResult {
  students: Student[];
  pagination?: StudentPaginationInfo;
}

/**
 * Parse various student response formats into a consistent structure.
 * Handles: wrapped ApiResponse, direct paginated, and legacy array formats.
 */
function parseStudentsPaginatedResponse(responseData: unknown): StudentsResult {
  // Format 1: Wrapped ApiResponse { success: true, data: { data: [...], pagination: {...} } }
  if (
    responseData &&
    typeof responseData === "object" &&
    "success" in responseData &&
    "data" in responseData
  ) {
    const wrapper = responseData as ApiResponseWrapper<{
      data?: Student[];
      pagination?: StudentPaginationInfo;
    }>;
    if (
      wrapper.data &&
      typeof wrapper.data === "object" &&
      "data" in wrapper.data
    ) {
      return {
        students: Array.isArray(wrapper.data.data) ? wrapper.data.data : [],
        pagination: wrapper.data.pagination,
      };
    }
  }

  // Format 2: Direct paginated { data: [...], pagination: {...} }
  if (
    responseData &&
    typeof responseData === "object" &&
    "data" in responseData &&
    Array.isArray((responseData as { data: unknown }).data)
  ) {
    const paginatedData = responseData as {
      data: Student[];
      pagination?: StudentPaginationInfo;
    };
    return {
      students: paginatedData.data,
      pagination: paginatedData.pagination,
    };
  }

  // Format 3: Legacy format - just an array
  if (Array.isArray(responseData)) {
    return { students: responseData as Student[] };
  }

  // Fallback - empty result
  return { students: [] };
}

function parseSchoolClassesResponse(responseData: unknown): string[] {
  const value =
    responseData &&
    typeof responseData === "object" &&
    "success" in responseData &&
    "data" in responseData
      ? (responseData as ApiResponseWrapper<unknown>).data
      : responseData;

  if (!Array.isArray(value)) return [];
  return value
    .filter((item): item is string => typeof item === "string")
    .map((item) => item.trim())
    .filter(Boolean);
}

/**
 * Build query parameters for student API requests
 */
function buildStudentQueryParams(filters?: {
  search?: string;
  inHouse?: boolean;
  groupId?: string;
  roomId?: string;
  schoolClass?: string;
  locationState?: "present" | "transit";
  dayStatus?: "comes_today" | "not_coming_today";
  /**
   * Planning day (YYYY-MM-DD) the day-planning status, status days, and
   * planned arrival/pickup times are evaluated for (#1939). Omit for today.
   */
  date?: string;
  bus?: "yes" | "no";
  photoConsent?: "yes" | "no";
  pickupStatus?: "self" | "pickedUp" | "none";
  page?: number;
  pageSize?: number;
  includePickupTimes?: boolean;
  includeArrivalTimes?: boolean;
  includeCompanions?: boolean;
  /**
   * Wire projection (#2097). "slim" drops every field the Kindersuche does not
   * render — address, health info, supervisor notes, guardian contacts, the
   * weekday departure maps, consent and row timestamps — and cuts the response
   * by ~78% for the same rows. Omit for the full projection every other
   * consumer of this endpoint reads.
   */
  view?: "slim";
}): URLSearchParams {
  const params = new URLSearchParams();
  if (filters?.search) params.append("search", filters.search);
  if (filters?.inHouse !== undefined)
    params.append("in_house", filters.inHouse.toString());
  if (filters?.groupId) params.append("group_id", filters.groupId);
  if (filters?.schoolClass) params.append("school_class", filters.schoolClass);
  // room_id narrows the list to students currently checked-in to any
  // active group taking place in this room (#1323). Backend joins via
  // active.visits → active.groups; see api/students/list_helpers.go.
  if (filters?.roomId) params.append("room_id", filters.roomId);
  if (filters?.locationState)
    params.append("location_state", filters.locationState);
  if (filters?.dayStatus) params.append("day_status", filters.dayStatus);
  if (filters?.date) params.append("date", filters.date);
  // Administrative filters (#1492). The backend applies these in-memory in the
  // same pass as day_status, so pagination and counts stay correct server-side.
  if (filters?.bus) params.append("bus", filters.bus);
  if (filters?.photoConsent)
    params.append("photo_consent", filters.photoConsent);
  if (filters?.pickupStatus)
    params.append("pickup_status", filters.pickupStatus);
  if (filters?.page) params.append("page", filters.page.toString());
  if (filters?.pageSize)
    params.append("page_size", filters.pageSize.toString());
  if (filters?.includePickupTimes)
    params.append("include_pickup_times", "true");
  if (filters?.includeArrivalTimes)
    params.append("include_arrival_times", "true");
  // Companion links ("läuft mit") for the shown day, so the Kindersuche can
  // group by Laufgemeinschaft. Opt-in: it costs an extra query per page.
  if (filters?.includeCompanions) params.append("include_companions", "true");
  if (filters?.view) params.append("view", filters.view);
  return params;
}

/**
 * Current session token via the deduplicating session cache.
 */
async function getSessionToken(): Promise<string | undefined> {
  const session = await getCachedSession();
  return session?.user?.token;
}

/**
 * Auth headers for the raw fetch() call sites in the services below.
 * Undefined when unauthenticated, so the request goes out without an
 * Authorization header.
 */
async function getAuthHeaders(): Promise<Record<string, string> | undefined> {
  const token = await getSessionToken();
  return token
    ? {
        Authorization: `Bearer ${token}`,
        "Content-Type": "application/json",
      }
    : undefined;
}

/**
 * Get new token from session (helper for fetchWithRetry).
 * Called after a 401 → token refresh, so the cached session is stale by
 * definition — drop it first, or this (and every concurrent caller for up to
 * 10s) would retry with the dead token.
 */
async function getNewTokenFromSession(): Promise<string | undefined> {
  clearSessionCache();
  return getSessionToken();
}

/**
 * Validate required fields for student creation
 * @throws Error if required fields are missing
 */
function validateStudentForCreation(student: Omit<Student, "id">): void {
  if (!student.first_name) {
    throw new Error("First name is required");
  }
  if (!student.second_name) {
    throw new Error("Last name is required");
  }
  if (!student.school_class) {
    throw new Error("School class is required");
  }
}

/**
 * Parse API error response text to extract detailed error message
 * @returns Error message or null if parsing fails
 */
function parseApiErrorMessage(errorText: string): string | null {
  try {
    const errorJson = JSON.parse(errorText) as { error?: string };
    return errorJson.error ?? null;
  } catch {
    return null;
  }
}

/**
 * Extract error message from API error response with fallback patterns.
 * Tries JSON parsing first, then checks for known error patterns in raw text.
 */
function extractApiError(
  errorText: string,
  fallbackPatterns: string[] = [],
): string | null {
  // Try JSON parsing first
  const jsonError = parseApiErrorMessage(errorText);
  if (jsonError) return jsonError;

  // Check for known error patterns in raw text
  for (const pattern of fallbackPatterns) {
    if (errorText.includes(pattern)) {
      return pattern;
    }
  }

  return null;
}

/**
 * Extract error from Axios error response.
 */
function extractAxiosError(error: unknown): string | null {
  const axiosErr = error as AxiosError;
  if (axiosErr.response?.data) {
    const errorData = axiosErr.response.data as { error?: string };
    return errorData.error ?? null;
  }
  return null;
}

/**
 * Build query parameters for room filters.
 */
function buildRoomQueryParams(filters?: {
  building?: string;
  floor?: number;
  category?: string;
  occupied?: boolean;
  search?: string;
  page?: number;
  pageSize?: number;
}): URLSearchParams {
  const params = new URLSearchParams();
  if (filters?.search) params.append("search", filters.search);
  if (filters?.building) params.append("building", filters.building);
  if (filters?.floor !== undefined)
    params.append("floor", filters.floor.toString());
  if (filters?.category) params.append("category", filters.category);
  if (filters?.occupied !== undefined)
    params.append("occupied", filters.occupied.toString());
  if (filters?.page !== undefined)
    params.append("page", filters.page.toString());
  if (filters?.pageSize !== undefined)
    params.append("page_size", filters.pageSize.toString());
  return params;
}

/**
 * Parse rooms response, handling null, non-array, and valid array formats.
 * Returns empty array for invalid formats with warning.
 */
function parseRoomsResponse(responseData: unknown): BackendRoom[] {
  if (
    responseData &&
    typeof responseData === "object" &&
    "data" in responseData &&
    Array.isArray((responseData as { data?: unknown }).data)
  ) {
    return (responseData as { data: BackendRoom[] }).data;
  }

  if (!responseData || !Array.isArray(responseData)) {
    logger.warn("invalid response format for rooms", {
      response_type: typeof responseData,
      is_array: Array.isArray(responseData),
    });
    return [];
  }
  return responseData as BackendRoom[];
}

/**
 * Extract single BackendRoom from various response formats.
 * Handles: wrapped {data: BackendRoom}, direct BackendRoom with id.
 */
function extractBackendRoom(responseData: unknown): BackendRoom {
  if (!responseData || typeof responseData !== "object") {
    throw new Error("Unexpected room response format");
  }

  const data = responseData as Record<string, unknown>;

  // Format 1: Wrapped { data: BackendRoom }
  if ("data" in data && data.data) {
    return data.data as BackendRoom;
  }

  // Format 2: Direct BackendRoom (has 'id' property)
  if ("id" in data) {
    return convertToBackendRoom(data);
  }

  logger.warn("unexpected room response format", {
    response_type: typeof responseData,
  });
  throw new Error("Unexpected room response format");
}

/**
 * Validate room data before creation.
 * Throws descriptive error if validation fails.
 */
function validateRoomForCreation(room: {
  name?: string;
  capacity?: number | null;
  category?: string;
}): void {
  if (!room.name) {
    throw new Error("Missing required field: name");
  }
  if (
    room.capacity !== undefined &&
    room.capacity !== null &&
    room.capacity <= 0
  ) {
    throw new Error("capacity must be greater than 0");
  }
  if (!room.category) {
    throw new Error("Missing required field: category");
  }
}

/**
 * Parse groups response from API.
 * Handles wrapped {data: BackendGroup[]} and direct BackendGroup[] formats.
 */
function parseGroupsResponse(responseData: unknown): BackendGroup[] {
  // Check if wrapped in ApiResponse format {data: [...]}
  if (
    typeof responseData === "object" &&
    responseData !== null &&
    "data" in responseData
  ) {
    const apiResponse = responseData as { data?: unknown };
    return Array.isArray(apiResponse.data)
      ? (apiResponse.data as BackendGroup[])
      : [];
  }

  // Direct array response
  if (Array.isArray(responseData)) {
    return responseData as BackendGroup[];
  }

  return [];
}

/**
 * Parse single group response, extracting BackendGroup from various wrapper formats.
 * Handles: ApiResponse wrapper, data wrapper, double-wrapped, and direct formats.
 */
function extractBackendGroup(responseData: unknown): BackendGroup {
  if (!responseData || typeof responseData !== "object") {
    throw new Error("Invalid response format from API");
  }

  const data = responseData as Record<string, unknown>;

  // Format 1: ApiResponse { success: true, data: BackendGroup | { data: BackendGroup } }
  if ("success" in data && "data" in data) {
    const innerData = data.data;
    // Check for double-wrapped { data: { data: BackendGroup } }
    if (
      innerData &&
      typeof innerData === "object" &&
      "data" in (innerData as Record<string, unknown>)
    ) {
      return (innerData as { data: BackendGroup }).data;
    }
    return innerData as BackendGroup;
  }

  // Format 2: Simple wrapper { data: BackendGroup }
  if ("data" in data) {
    return data.data as BackendGroup;
  }

  // Format 3: Direct BackendGroup (has 'id' and 'name' properties)
  if ("id" in data && "name" in data) {
    return data as unknown as BackendGroup;
  }

  throw new Error("No group data in response");
}

/**
 * Parse single student response from API.
 * Handles wrapped {data: Student} and direct Student formats.
 * @param responseData - Raw response data
 * @param applyMapping - Whether to apply mapStudentDetailResponse (for backend format)
 */
function parseSingleStudentResponse(
  responseData: unknown,
  applyMapping: boolean,
): Student {
  if (!responseData || typeof responseData !== "object") {
    throw new Error("Invalid student response format");
  }

  // Check if wrapped in {data: ...}
  if ("data" in responseData) {
    const wrapped = responseData as { data: BackendStudentDetail | Student };
    return applyMapping
      ? mapStudentDetailResponse(wrapped.data as BackendStudentDetail)
      : (wrapped.data as Student);
  }

  // Direct response
  return applyMapping
    ? mapStudentDetailResponse(responseData as BackendStudentDetail)
    : (responseData as Student);
}

// Re-export types for external usage
export type { Student } from "./student-helpers";
export type Group = ImportedGroup;

// Room-related interfaces
export interface Room {
  id: string;
  name: string;
  building?: string;
  floor?: number; // Optional (nullable in DB)
  capacity?: number; // Optional (nullable in DB)
  category?: string; // Optional (nullable in DB)
  color?: string; // Optional (nullable in DB)
  deviceId?: string;
  isOccupied: boolean;
  activityName?: string;
  groupName?: string;
  supervisorName?: string;
  studentCount?: number;
  createdAt?: string;
  updatedAt?: string;
}

// API services
/**
 * Raised when a student update was refused because a linked child's own
 * departure plan does not yet allow leaving with another child on the
 * requested days. Nothing was written; re-send with extend_companion_plans
 * after the user confirms.
 */
export class CompanionPlanConflictError extends Error {
  /**
   * The conflicting children and weekdays exactly as the backend reported them.
   * The confirmation has to name them again on the retry, so the backend can
   * tell "the user agreed to this" apart from "this appeared afterwards".
   */
  readonly conflicts: CompanionExtensionConfirmation[];

  /**
   * The untouched response body.
   *
   * Kept because the refusal is not the only thing the body says: the student
   * PUT proxy writes the privacy consent of the same request first, so a
   * refusal can arrive on top of a COMMITTED consent write and carries
   * `details.privacy_consent_saved` to say so. isPrivacyConsentSaved reads
   * exactly this field (the message is only the German sentence), and without
   * it the form reports "nothing was saved" for a request that did change the
   * stored Datenschutz settings.
   */
  readonly body: string;

  constructor(body: string) {
    super(parseConflictMessage(body));
    this.name = "CompanionPlanConflictError";
    this.body = body;
    this.conflicts = parseConflictExtensions(body);
  }
}

/**
 * Reads the 409 body's conflict list into the confirmation shape. Anything
 * unparsable yields an empty list, which makes the retry ask again rather than
 * confirm something we could not read.
 */
export function parseConflictExtensions(
  body: string,
): CompanionExtensionConfirmation[] {
  try {
    const parsed = JSON.parse(body) as {
      conflicts?: { student_id?: number | string; weekdays?: string[] }[];
    };
    return (parsed.conflicts ?? [])
      .filter((conflict) => conflict.student_id !== undefined)
      .map((conflict) => ({
        companion_student_id: String(conflict.student_id),
        weekdays: (conflict.weekdays ?? []).filter(
          (day): day is CompanionWeekday =>
            ALL_COMPANION_WEEKDAYS.includes(day as CompanionWeekday),
        ),
      }));
  } catch {
    return [];
  }
}

/**
 * The backend's stable code for the retriable lock 409 on the student PUT
 * (api/students: CodeCompanionLockBusy). Kept in sync by hand — the wire
 * contract is the string.
 */
export const COMPANION_LOCK_BUSY_CODE = "companion_lock_busy";

/**
 * The backend's stable code for the OTHER expected companion refusal
 * (api/students: CodeCompanionWouldLoseDeparture): removing a link would leave
 * the linked child with an accompanied departure plan and no note and no other
 * link. A 400, not a 409 — nothing to confirm, the user has to fix that child's
 * Heimweg first. Kept in sync by hand — the wire contract is the string.
 */
export const COMPANION_WOULD_LOSE_DEPARTURE_CODE =
  "companion_would_lose_departure";

/**
 * The backend's stable code for the stale-list 409 (api/students:
 * CodeCompanionsChanged): the submitted "läuft mit" list was built on a
 * snapshot someone else has since replaced. Kept in sync by hand — the wire
 * contract is the string.
 */
export const COMPANIONS_CHANGED_CODE = "companions_changed";

/** Shown only when the stale-list refusal arrived without a readable message. */
const COMPANIONS_CHANGED_FALLBACK =
  "Die Laufgemeinschaft dieses Kindes wurde zwischenzeitlich geändert. Bitte neu laden und noch einmal speichern.";

/**
 * Raised when the submitted companion list no longer describes the stored one.
 *
 * Typed rather than folded into the generic error path because the retry is
 * different from every other failure: re-sending the SAME list is exactly what
 * the refusal prevents — it would delete the change it is protecting. The form
 * has to reload first and let the user redo the edit on the current state.
 */
export class CompanionsChangedError extends Error {
  /** The untouched response body — see CompanionPlanConflictError.body. */
  readonly body: string;

  constructor(body: string) {
    super(parseBackendMessage(body, COMPANIONS_CHANGED_FALLBACK));
    this.name = "CompanionsChangedError";
    this.body = body;
  }
}

/** Reports whether a response body carries the stale-list code. */
export function isCompanionsChangedBody(body: string): boolean {
  return bodyHasCode(body, COMPANIONS_CHANGED_CODE);
}

/**
 * Reports whether an error is that refusal, in whichever wrapping it arrived —
 * the typed error from studentService.updateStudent, or the plain Error the
 * generic CRUD service throws with the untouched body attached. Keyed off the
 * CODE, never the status: the student PUT answers 409 for the companion-plan
 * question and the lock collision too.
 */
export function isCompanionsChanged(err: unknown): boolean {
  if (err instanceof CompanionsChangedError) return true;
  if (!(err instanceof Error)) return false;
  const body = (err as Error & { body?: string }).body;
  if (body && isCompanionsChangedBody(body)) return true;
  const match = embeddedJsonObject(err.message);
  return match ? isCompanionsChangedBody(match) : false;
}

/** The German instruction the stale-list refusal carries. */
export function companionsChangedMessage(err: unknown): string {
  if (err instanceof CompanionsChangedError) return err.message;
  if (err instanceof Error) {
    const body = (err as Error & { body?: string }).body;
    if (body) {
      const fromBody = parseBackendMessage(body, "");
      if (fromBody) return fromBody;
    }
    const match = embeddedJsonObject(err.message);
    if (match) {
      const fromMessage = parseBackendMessage(match, "");
      if (fromMessage) return fromMessage;
    }
  }
  return COMPANIONS_CHANGED_FALLBACK;
}

/** Shown only when the refusal arrived without a readable message. */
const COMPANION_DEPARTURE_FALLBACK =
  "Ein verknüpftes Kind hätte danach keine Angabe mehr dazu, mit wem es nach Hause geht. Bitte zuerst den Heimweg dieses Kindes anpassen.";

/**
 * Raised when a student write was refused because it would strand a linked
 * child. Typed like the plan conflict so the forms can keep the backend's
 * German instruction instead of decaying it into "Fehler beim Speichern" — the
 * message names the precondition, and without it the user has no way to tell
 * what has to change before the save can succeed.
 */
export class CompanionDepartureRefusedError extends Error {
  /** The untouched response body — see CompanionPlanConflictError.body. */
  readonly body: string;

  constructor(body: string) {
    super(parseBackendMessage(body, COMPANION_DEPARTURE_FALLBACK));
    this.name = "CompanionDepartureRefusedError";
    this.body = body;
  }
}

/**
 * Reports whether an error is that refusal, in whichever wrapping it arrived:
 * the typed error from studentService.updateStudent, or the plain Error the
 * generic CRUD service throws with the untouched body attached (and, for
 * callers that lost the body, the JSON embedded in the message).
 *
 * Keyed off the CODE, never the status: the student PUT answers 400 for every
 * other rejected field too, and those keep their own generic handling.
 */
export function isCompanionDepartureRefusal(err: unknown): boolean {
  if (err instanceof CompanionDepartureRefusedError) return true;
  if (!(err instanceof Error)) return false;
  const body = (err as Error & { body?: string }).body;
  if (body && isCompanionDepartureBody(body)) return true;
  const match = embeddedJsonObject(err.message);
  return match ? isCompanionDepartureBody(match) : false;
}

/** Reports whether a response body carries the stranded-companion code. */
export function isCompanionDepartureBody(body: string): boolean {
  return bodyHasCode(body, COMPANION_WOULD_LOSE_DEPARTURE_CODE);
}

/** The German instruction the refusal carries, dug out of the same wrappings. */
export function companionDepartureMessage(err: unknown): string {
  if (err instanceof CompanionDepartureRefusedError) return err.message;
  if (err instanceof Error) {
    const body = (err as Error & { body?: string }).body;
    if (body) {
      const fromBody = parseBackendMessage(body, "");
      if (fromBody) return fromBody;
    }
    const match = embeddedJsonObject(err.message);
    if (match) {
      const fromMessage = parseBackendMessage(match, "");
      if (fromMessage) return fromMessage;
    }
  }
  return COMPANION_DEPARTURE_FALLBACK;
}

/**
 * The student PUT proxy writes the privacy consent of the same request first,
 * against a different backend endpoint and therefore a different transaction.
 * When the student write then fails, the request is a partial success: the
 * consent is stored, the rest is not. The proxy marks that error with
 * `details.privacy_consent_saved` (app/api/students/[id]/route.ts) so the form
 * can say which half landed — reporting the usual blanket failure invites the
 * user to abandon the retry and leave a consent change behind unknowingly.
 */
export function isPrivacyConsentSavedBody(body: string): boolean {
  const jsonStart = body.indexOf("{");
  if (jsonStart === -1) return false;
  try {
    const parsed = JSON.parse(body.substring(jsonStart)) as {
      details?: { privacy_consent_saved?: boolean };
    };
    return parsed.details?.privacy_consent_saved === true;
  } catch {
    return false;
  }
}

/** Reports the same in whichever wrapping the error arrived. */
export function isPrivacyConsentSaved(err: unknown): boolean {
  if (!(err instanceof Error)) return false;
  const body = (err as Error & { body?: string }).body;
  if (body && isPrivacyConsentSavedBody(body)) return true;
  const match = embeddedJsonObject(err.message);
  return match ? isPrivacyConsentSavedBody(match) : false;
}

/** Prefixed to the failure text so the saved half is never reported as lost. */
export const PRIVACY_CONSENT_SAVED_NOTICE =
  "Die Datenschutzeinstellungen wurden bereits gespeichert, die übrigen Änderungen nicht.";

/**
 * Returns the message to show for a failed student save, prefixed with the
 * partial-success notice when the consent half of the request committed.
 */
export function withPrivacyConsentSavedNotice(
  err: unknown,
  message: string,
): string {
  return isPrivacyConsentSaved(err)
    ? `${PRIVACY_CONSENT_SAVED_NOTICE} ${message}`
    : message;
}

/**
 * Cuts the JSON object out of an error MESSAGE — first "{" to last "}" — for
 * the callers that no longer hold the response body and have to read the
 * backend envelope back out of the text the CRUD service built from it.
 *
 * Written with indexOf/lastIndexOf rather than the obvious /\{.*\}/s: the
 * greedy match backtracks over the whole message once per candidate closing
 * brace, which is quadratic on a long unterminated message. The result is
 * identical, the scan is linear.
 */
export function embeddedJsonObject(message: string): string | null {
  const start = message.indexOf("{");
  if (start === -1) return null;
  const end = message.lastIndexOf("}");
  if (end < start) return null;
  return message.substring(start, end + 1);
}

/** Reads the backend error envelope's message, falling back when unreadable. */
function parseBackendMessage(body: string, fallback: string): string {
  const jsonStart = body.indexOf("{");
  if (jsonStart === -1) return fallback;
  try {
    const parsed = JSON.parse(body.substring(jsonStart)) as {
      error?: string;
      message?: string;
    };
    for (const candidate of [parsed.error, parsed.message]) {
      if (typeof candidate === "string" && candidate.trim()) {
        return candidate.trim();
      }
    }
    return fallback;
  } catch {
    return fallback;
  }
}

/**
 * Reports whether a complete 409 RESPONSE BODY is the companion-plan question.
 *
 * Strict on purpose: the conflict list has to actually be there. The student PUT
 * answers 409 for several unrelated reasons (companion_lock_busy, the
 * SICK_EXCUSED_CONFLICT code, and whatever a later feature adds), and typing one
 * of those as a CompanionPlanConflictError would replace its real contract with
 * an empty confirmation the user cannot answer. Only the untouched body reaches
 * this function, so "no list in it" means "not this conflict" — the response
 * then follows the generic error path and keeps its own message.
 */
export function isCompanionPlanConflictResponse(body: string): boolean {
  return parseConflictExtensions(body).length > 0;
}

/**
 * Reports whether a 409 body FRAGMENT is the companion-plan question rather than
 * the lock collision.
 *
 * For callers that no longer hold the untouched response — the generic CRUD
 * service reduces it to a message and an attached body, either of which may
 * have lost the list. Use isCompanionPlanConflictResponse whenever the whole
 * body is available.
 *
 * The status alone does not say so: the student PUT also answers 409 for
 * ErrCompanionLockBusy (a linked child is being edited elsewhere — retriable,
 * nothing to confirm). A body carrying that code is never the question; a body
 * carrying conflicts always is. An unreadable body stays the question, which is
 * the established behavior for this form — the retry then simply confirms
 * nothing and the backend asks again.
 */
export function isCompanionPlanConflictBody(body: string): boolean {
  if (parseConflictExtensions(body).length > 0) return true;
  if (bodyHasCode(body, COMPANIONS_CHANGED_CODE)) return false;
  return !bodyHasCode(body, COMPANION_LOCK_BUSY_CODE);
}

function bodyHasCode(body: string, code: string): boolean {
  const jsonStart = body.indexOf("{");
  if (jsonStart === -1) return false;
  try {
    const parsed = JSON.parse(body.substring(jsonStart)) as { code?: string };
    return parsed.code === code;
  } catch {
    return false;
  }
}

function parseConflictMessage(body: string): string {
  try {
    const parsed = JSON.parse(body) as { message?: string; error?: string };
    return (
      parsed.message ??
      parsed.error ??
      "Der Heimweg des verknüpften Kindes erlaubt diese Tage noch nicht."
    );
  } catch {
    return "Der Heimweg des verknüpften Kindes erlaubt diese Tage noch nicht.";
  }
}

export const studentService = {
  // Get all students
  // Pass token to skip redundant getSession() call (saves ~600ms per request)
  getStudents: async (filters?: {
    search?: string;
    inHouse?: boolean;
    groupId?: string;
    roomId?: string;
    schoolClass?: string;
    locationState?: "present" | "transit";
    dayStatus?: "comes_today" | "not_coming_today";
    date?: string;
    bus?: "yes" | "no";
    photoConsent?: "yes" | "no";
    pickupStatus?: "self" | "pickedUp" | "none";
    page?: number;
    pageSize?: number;
    includePickupTimes?: boolean;
    includeArrivalTimes?: boolean;
    includeCompanions?: boolean;
    /** Wire projection (#2097) — see buildStudentQueryParams. */
    view?: "slim";
    token?: string; // Optional: pass token to skip getSession()
  }): Promise<StudentsResult> => {
    const params = buildStudentQueryParams(filters);
    const useProxyApi = globalThis.window !== undefined;
    const baseUrl = useProxyApi
      ? "/api/students"
      : `${env.API_URL}/api/students`;
    const queryString = params.toString();
    const url = queryString ? `${baseUrl}?${queryString}` : baseUrl;

    try {
      if (useProxyApi) {
        // Use provided token or fall back to getSession()
        let authToken = filters?.token;
        if (!authToken) {
          authToken = await getSessionToken();
        }

        const { data } = await fetchWithRetry<unknown>(url, authToken, {
          onAuthFailure: handleAuthFailure,
          getNewToken: getNewTokenFromSession,
        });

        if (data === null) {
          throw new Error("Authentication failed");
        }

        return parseStudentsPaginatedResponse(data);
      }

      // Server-side: use axios with the API URL directly
      const response = await api.get(url, { params });
      const paginatedResponse =
        response.data as PaginatedResponse<BackendStudent>;
      return {
        students: mapStudentsResponse(paginatedResponse.data),
        pagination: paginatedResponse.pagination,
      };
    } catch (error) {
      throw handleApiError(error, "Error fetching students");
    }
  },

  getSchoolClasses: async (filters?: { token?: string }): Promise<string[]> => {
    const useProxyApi = globalThis.window !== undefined;
    const url = useProxyApi
      ? "/api/students/school-classes"
      : `${env.API_URL}/api/students/school-classes`;

    try {
      if (useProxyApi) {
        let authToken = filters?.token;
        if (!authToken) {
          authToken = await getSessionToken();
        }

        const { data } = await fetchWithRetry<unknown>(url, authToken, {
          onAuthFailure: handleAuthFailure,
          getNewToken: getNewTokenFromSession,
        });

        if (data === null) {
          throw new Error("Authentication failed");
        }

        return parseSchoolClassesResponse(data);
      }

      const response = await api.get(url);
      return parseSchoolClassesResponse(response.data);
    } catch (error) {
      throw handleApiError(error, "Error fetching school classes");
    }
  },

  // Get a specific student by ID
  getStudent: async (id: string): Promise<Student> => {
    // Use the nextjs api route which handles auth token properly
    const useProxyApi = globalThis.window !== undefined;
    const url = useProxyApi
      ? `/api/students/${id}`
      : `${env.API_URL}/api/students/${id}`;

    try {
      if (useProxyApi) {
        // Browser environment: use fetchWithRetry for automatic 401 handling
        // Route handler already maps response, so applyMapping=false
        const token = await getSessionToken();
        const { data } = await fetchWithRetry<unknown>(url, token, {
          onAuthFailure: handleAuthFailure,
          getNewToken: getNewTokenFromSession,
        });

        if (data === null) {
          throw new Error("Authentication failed");
        }

        return parseSingleStudentResponse(data, false);
      }

      // Server-side: use axios with the API URL directly (needs mapping)
      const response = await api.get(url);
      return parseSingleStudentResponse(response.data, true);
    } catch (error) {
      throw handleApiError(error, `Error fetching student ${id}`);
    }
  },

  // Create a new student
  createStudent: async (student: Omit<Student, "id">): Promise<Student> => {
    validateStudentForCreation(student);

    const useProxyApi = globalThis.window !== undefined;
    const url = useProxyApi ? `/api/students` : `${env.API_URL}/api/students`;

    try {
      if (useProxyApi) {
        // Browser environment: use fetch with our Next.js API route
        const response = await fetch(url, {
          method: "POST",
          credentials: "include",
          headers: await getAuthHeaders(),
          body: JSON.stringify(student),
        });

        if (!response.ok) {
          const errorText = await response.text();
          logger.error("api error during fetch", {
            status: response.status,
            error_text: errorText.substring(0, 200), // Truncate long errors
          });
          const detailedError = parseApiErrorMessage(errorText);
          throw new Error(
            detailedError
              ? `API error: ${detailedError}`
              : `API error: ${response.status}`,
          );
        }

        const data: unknown = await response.json();
        return mapSingleStudentResponse({ data: data as BackendStudent });
      }

      // Server-side: use axios with the API URL directly
      const backendStudent = prepareStudentForBackend(student);
      const response = await api.post(url, backendStudent);
      return mapSingleStudentResponse({
        data: response.data as unknown as BackendStudent,
      });
    } catch (error) {
      throw handleApiError(error, "Error creating student");
    }
  },

  // Update a student
  updateStudent: async (
    id: string,
    student: Partial<Student> & {
      // Laufgemeinschaft rides along with the departure plan it belongs to.
      // Ids stay strings all the way to the backend, which accepts a quoted
      // decimal for exactly that reason.
      companions?: { companion_student_id: string; weekdays: string[] }[];
      // Fingerprint of the list the caller LOADED — see prepareStudentForBackend.
      companions_fingerprint?: string;
      extend_companion_plans?: boolean;
      confirmed_companion_extensions?: CompanionExtensionConfirmation[];
    },
  ): Promise<Student> => {
    const useProxyApi = globalThis.window !== undefined;
    const url = useProxyApi
      ? `/api/students/${id}`
      : `${env.API_URL}/api/students/${id}`;

    try {
      if (useProxyApi) {
        // Browser environment: use fetch with our Next.js API route
        // Send frontend format data - the API route will handle transformation
        const response = await fetch(url, {
          method: "PUT",
          credentials: "include",
          headers: await getAuthHeaders(),
          body: JSON.stringify(student),
        });

        if (!response.ok) {
          const errorText = await response.text();
          logger.error("api error during fetch", {
            status: response.status,
            error_text: errorText.substring(0, 200), // Truncate long errors
          });

          // A 409 CARRYING A CONFLICT LIST on the student PUT means a linked
          // child's own departure plan does not allow the requested "läuft mit"
          // days. It is a question, not a failure: the caller re-sends with
          // extend_companion_plans after the user confirms. Typed so the modal
          // can tell it apart from a real error. Nothing was written — the
          // handler checks before its first write. A 409 WITHOUT that list is a
          // different conflict (a linked child locked by a concurrent edit, a
          // sick/excused clash, …) and falls through to the generic error path,
          // which shows its own message instead of an unanswerable question.
          if (
            response.status === 409 &&
            isCompanionPlanConflictResponse(errorText)
          ) {
            throw new CompanionPlanConflictError(errorText);
          }

          // The OTHER expected 409: the submitted "läuft mit" list was built on
          // a snapshot someone else has since replaced. Typed because the retry
          // must NOT re-send the same list — that is precisely the write this
          // refusal exists to stop. The form reloads and lets the user redo the
          // edit on the current links.
          if (response.status === 409 && isCompanionsChangedBody(errorText)) {
            throw new CompanionsChangedError(errorText);
          }

          // A 400 CARRYING THE STRANDED-COMPANION CODE is the other expected
          // companion refusal: removing a link would leave the linked child
          // with an accompanied plan and nothing that says who it walks home
          // with. Nothing was written and the message names what has to be
          // fixed first, so it is typed too — the generic branch below folds
          // the body into "API error 400: …", which the forms then replace
          // with an opaque "Fehler beim Speichern" the user cannot act on.
          if (response.status === 400 && isCompanionDepartureBody(errorText)) {
            throw new CompanionDepartureRefusedError(errorText);
          }

          // Try to parse error text as JSON for more detailed error
          try {
            const errorJson = JSON.parse(errorText) as { error?: string };
            if (errorJson.error) {
              throw new Error(
                `API error ${response.status}: ${errorJson.error}`,
              );
            }
          } catch {
            // If parsing fails, use status code + error text
            throw new Error(
              `API error ${response.status}: ${errorText.substring(0, 100)}`,
            );
          }
        }

        // Type assertion to avoid unsafe assignment
        const data: unknown = await response.json();

        // Backend wraps response: {status: "success", data: {...}}
        const wrappedData = data as { status?: string; data?: BackendStudent };
        const actualData = wrappedData.data ?? (data as BackendStudent);

        // Map response to our frontend model
        const mappedResponse = mapSingleStudentResponse({
          data: actualData,
        });
        // The "läuft mit" links are not part of this response, and a departure
        // plan write can change them even when the caller sent no `companions`
        // list (the backend trims links the new plan no longer allows). Tell
        // every mounted companion view to refetch instead of leaving it stale —
        // but only when the write actually changed them.
        if (
          companionsChangedFromResponse(actualData) ??
          studentUpdateMayChangeCompanions(student)
        ) {
          notifyStudentCompanionsChanged();
        }
        return mappedResponse;
      } else {
        // Server-side: use axios with the API URL directly
        // For server-side, we need to transform the data since we're calling the backend directly
        const backendUpdates = prepareStudentForBackend(student);
        const response = await api.put(url, backendUpdates);
        const mappedResponse = mapSingleStudentResponse({
          data: response.data as unknown as BackendStudent,
        });
        if (
          companionsChangedFromResponse(response.data) ??
          studentUpdateMayChangeCompanions(student)
        ) {
          notifyStudentCompanionsChanged();
        }
        return mappedResponse;
      }
    } catch (error) {
      // The companion conflict is a question for the user, not an API failure —
      // keep its type so the caller can offer the confirmation instead of a
      // generic error toast.
      if (error instanceof CompanionPlanConflictError) throw error;
      // Same reasoning for the stranded-companion refusal: it is an instruction
      // for the user, and handleApiError would strip it down to a generic
      // message.
      if (error instanceof CompanionDepartureRefusedError) throw error;
      // Same reasoning for the stale-list refusal: the form has to reload
      // rather than retry, and handleApiError would hide that instruction.
      if (error instanceof CompanionsChangedError) throw error;
      throw handleApiError(error, `Error updating student ${id}`);
    }
  },
};

// Group service for API operations
export const groupService = {
  // Get all groups
  getGroups: async (filters?: { search?: string }): Promise<Group[]> => {
    const params = new URLSearchParams();
    if (filters?.search) params.append("search", filters.search);

    const useProxyApi = globalThis.window !== undefined;
    const queryString = params.toString();
    const baseUrl = useProxyApi ? "/api/groups" : `${env.API_URL}/api/groups`;
    const url = queryString ? `${baseUrl}?${queryString}` : baseUrl;

    try {
      if (useProxyApi) {
        // Browser environment: use fetchWithRetry for automatic 401 handling
        const token = await getSessionToken();
        const { response, data } = await fetchWithRetry<unknown>(url, token, {
          onAuthFailure: handleAuthFailure,
          getNewToken: getNewTokenFromSession,
        });

        // Handle errors: null response means auth failed or permission denied
        // Return empty array for graceful degradation
        if (response === null || data === null) {
          return [];
        }

        return mapGroupsResponse(parseGroupsResponse(data));
      }

      // Server-side: use axios with the API URL directly
      const response = await api.get(url, { params });
      const paginatedResponse =
        response.data as PaginatedResponse<BackendGroup>;
      return mapGroupsResponse(paginatedResponse.data);
    } catch (error) {
      logger.error("failed to fetch groups", {
        error: error instanceof Error ? error.message : String(error),
      });
      throw error;
    }
  },

  // Get a specific group by ID
  getGroup: async (id: string): Promise<Group> => {
    const useProxyApi = globalThis.window !== undefined;
    const url = useProxyApi
      ? `/api/groups/${id}`
      : `${env.API_URL}/api/groups/${id}`;

    try {
      if (useProxyApi) {
        // Browser environment: use fetchWithRetry for automatic 401 handling
        const token = await getSessionToken();
        const { response, data } = await fetchWithRetry<unknown>(url, token, {
          onAuthFailure: handleAuthFailure,
          getNewToken: getNewTokenFromSession,
        });

        if (response === null) {
          throw new Error("Authentication failed");
        }

        const groupData = extractBackendGroup(data);
        return mapGroupResponse(groupData);
      }

      // Server-side: use axios with the API URL directly
      const response = await api.get(url);
      return mapGroupResponse(response.data as BackendGroup);
    } catch (error) {
      logger.error("failed to fetch group", {
        group_id: id,
        error: error instanceof Error ? error.message : String(error),
      });
      throw error;
    }
  },

  // Create a new group
  createGroup: async (group: Omit<Group, "id">): Promise<Group> => {
    // Transform from frontend model to backend model
    const backendGroup = prepareGroupForBackend(group);

    // Basic validation for group creation
    if (!backendGroup.name) {
      throw new Error("Missing required field: name");
    }

    const useProxyApi = globalThis.window !== undefined;
    const url = useProxyApi ? `/api/groups` : `${env.API_URL}/api/groups`;

    try {
      if (useProxyApi) {
        // Browser environment: use fetch with our Next.js API route
        const response = await fetch(url, {
          method: "POST",
          credentials: "include",
          headers: await getAuthHeaders(),
          body: JSON.stringify(backendGroup),
        });

        if (!response.ok) {
          const errorText = await response.text();
          logger.error("api error during fetch", {
            status: response.status,
            error_text: errorText.substring(0, 200), // Truncate long errors
          });
          // Try to parse error for more detailed message
          try {
            const errorJson = JSON.parse(errorText) as { error?: string };
            if (errorJson.error) {
              throw new Error(`API error: ${errorJson.error}`);
            }
          } catch {
            // If parsing fails, use status code
          }
          throw new Error(`API error: ${response.status}`);
        }

        const data = (await response.json()) as BackendGroup;
        return mapSingleGroupResponse({ data });
      } else {
        // Server-side: use axios with the API URL directly
        const response = await api.post(url, backendGroup);
        return mapSingleGroupResponse({ data: response.data as BackendGroup });
      }
    } catch (error) {
      logger.error("failed to create group", {
        error: error instanceof Error ? error.message : String(error),
      });
      throw error;
    }
  },

  // Update a group
  updateGroup: async (id: string, group: Partial<Group>): Promise<Group> => {
    // Transform from frontend model to backend model updates
    const backendUpdates = prepareGroupForBackend(group);

    const useProxyApi = globalThis.window !== undefined;
    const url = useProxyApi
      ? `/api/groups/${id}`
      : `${env.API_URL}/api/groups/${id}`;

    try {
      if (useProxyApi) {
        // Browser environment: use fetch with our Next.js API route
        const response = await fetch(url, {
          method: "PUT",
          credentials: "include",
          headers: await getAuthHeaders(),
          body: JSON.stringify(backendUpdates),
        });

        if (!response.ok) {
          const errorText = await response.text();
          logger.error("api error during fetch", {
            status: response.status,
            error_text: errorText.substring(0, 200), // Truncate long errors
          });

          // Try to parse error text as JSON for more detailed error
          try {
            const errorJson = JSON.parse(errorText) as { error?: string };
            if (errorJson.error) {
              throw new Error(
                `API error ${response.status}: ${errorJson.error}`,
              );
            }
          } catch {
            // If parsing fails, use status code + error text
            throw new Error(
              `API error ${response.status}: ${errorText.substring(0, 100)}`,
            );
          }
        }

        const data = (await response.json()) as BackendGroup;
        return mapSingleGroupResponse({ data });
      } else {
        // Server-side: use axios with the API URL directly
        const response = await api.put(url, backendUpdates);
        return mapSingleGroupResponse({ data: response.data as BackendGroup });
      }
    } catch (error) {
      logger.error("failed to update group", {
        group_id: id,
        error: error instanceof Error ? error.message : String(error),
      });
      throw error;
    }
  },

  // Delete a group
  deleteGroup: async (id: string): Promise<void> => {
    const useProxyApi = globalThis.window !== undefined;
    const url = useProxyApi
      ? `/api/groups/${id}`
      : `${env.API_URL}/api/groups/${id}`;

    const knownErrorPatterns = ["cannot delete group with students"];

    try {
      if (useProxyApi) {
        // Browser environment: use fetch with our Next.js API route
        const response = await fetch(url, {
          method: "DELETE",
          credentials: "include",
          headers: await getAuthHeaders(),
        });

        if (!response.ok) {
          const errorText = await response.text();
          logger.error("api error during fetch", {
            status: response.status,
            error_text: errorText.substring(0, 200), // Truncate long errors
          });
          const detailedError = extractApiError(errorText, knownErrorPatterns);
          throw new Error(detailedError ?? `API error: ${response.status}`);
        }
        return;
      }

      // Server-side: use axios with the API URL directly
      try {
        await api.delete(url);
      } catch (axiosError) {
        const detailedError = extractAxiosError(axiosError);
        if (detailedError) {
          throw new Error(detailedError, { cause: axiosError });
        }
        throw axiosError;
      }
    } catch (error) {
      logger.error("failed to delete group", {
        group_id: id,
        error: error instanceof Error ? error.message : String(error),
      });
      throw error;
    }
  },

  // Get students in a group
  getGroupStudents: async (id: string): Promise<Student[]> => {
    const useProxyApi = globalThis.window !== undefined;
    const url = useProxyApi
      ? `/api/groups/${id}/students`
      : `${env.API_URL}/api/groups/${id}/students`;

    try {
      if (useProxyApi) {
        // Browser environment: use fetch with our Next.js API route
        const response = await fetch(url, {
          credentials: "include",
          headers: await getAuthHeaders(),
        });

        if (!response.ok) {
          const errorText = await response.text();
          logger.error("api error during fetch", {
            status: response.status,
            error_text: errorText.substring(0, 200), // Truncate long errors
          });
          throw new Error(`API error: ${response.status}`);
        }

        // Type assertion to avoid unsafe assignment
        const responseData = (await response.json()) as {
          data?: Student[];
          [key: string]: unknown;
        };

        // The Next.js API route uses route wrapper which may wrap the response
        if (
          responseData &&
          typeof responseData === "object" &&
          "data" in responseData &&
          responseData.data
        ) {
          // If wrapped, extract the data
          return responseData.data;
        }

        // Otherwise, treat as direct array
        return responseData as unknown as Student[];
      } else {
        // Server-side: use axios with the API URL directly
        const response = await api.get(url);
        return mapStudentsResponse(
          (response as { data: unknown }).data as BackendStudent[],
        );
      }
    } catch (error) {
      throw handleApiError(error, `Error fetching students for group ${id}`);
    }
  },

  // Add a supervisor to a group
  addSupervisor: async (
    groupId: string,
    supervisorId: string,
  ): Promise<void> => {
    const useProxyApi = globalThis.window !== undefined;
    const url = useProxyApi
      ? `/api/groups/${groupId}/supervisors`
      : `${env.API_URL}/api/groups/${groupId}/supervisors`;

    try {
      if (useProxyApi) {
        // Browser environment: use fetch with our Next.js API route
        const response = await fetch(url, {
          method: "POST",
          credentials: "include",
          headers: await getAuthHeaders(),
          body: JSON.stringify({
            supervisor_id: Number.parseInt(supervisorId, 10),
          }),
        });

        if (!response.ok) {
          const errorText = await response.text();
          logger.error("api error during fetch", {
            status: response.status,
            error_text: errorText.substring(0, 200), // Truncate long errors
          });
          throw new Error(`API error: ${response.status}`);
        }

        return;
      } else {
        // Server-side: use axios with the API URL directly
        await api.post(url, {
          supervisor_id: Number.parseInt(supervisorId, 10),
        });
        return;
      }
    } catch (error) {
      logger.error("failed to add supervisor to group", {
        supervisor_id: supervisorId,
        group_id: groupId,
        error: error instanceof Error ? error.message : String(error),
      });
      throw error;
    }
  },

  // Remove a supervisor from a group
  removeSupervisor: async (
    groupId: string,
    supervisorId: string,
  ): Promise<void> => {
    const useProxyApi = globalThis.window !== undefined;
    const url = useProxyApi
      ? `/api/groups/${groupId}/supervisors/${supervisorId}`
      : `${env.API_URL}/api/groups/${groupId}/supervisors/${supervisorId}`;

    try {
      if (useProxyApi) {
        // Browser environment: use fetch with our Next.js API route
        const response = await fetch(url, {
          method: "DELETE",
          credentials: "include",
          headers: await getAuthHeaders(),
        });

        if (!response.ok) {
          const errorText = await response.text();
          logger.error("api error during fetch", {
            status: response.status,
            error_text: errorText.substring(0, 200), // Truncate long errors
          });
          throw new Error(`API error: ${response.status}`);
        }

        return;
      } else {
        // Server-side: use axios with the API URL directly
        await api.delete(url);
        return;
      }
    } catch (error) {
      logger.error("failed to remove supervisor from group", {
        supervisor_id: supervisorId,
        group_id: groupId,
        error: error instanceof Error ? error.message : String(error),
      });
      throw error;
    }
  },

  // Set the representative for a group
  setRepresentative: async (
    groupId: string,
    representativeId: string,
  ): Promise<void> => {
    const useProxyApi = globalThis.window !== undefined;
    const url = useProxyApi
      ? `/api/groups/${groupId}/representative`
      : `${env.API_URL}/api/groups/${groupId}/representative`;

    try {
      if (useProxyApi) {
        // Browser environment: use fetch with our Next.js API route
        const response = await fetch(url, {
          method: "PUT",
          credentials: "include",
          headers: await getAuthHeaders(),
          body: JSON.stringify({
            representative_id: Number.parseInt(representativeId, 10),
          }),
        });

        if (!response.ok) {
          const errorText = await response.text();
          logger.error("api error during fetch", {
            status: response.status,
            error_text: errorText.substring(0, 200), // Truncate long errors
          });
          throw new Error(`API error: ${response.status}`);
        }

        return;
      } else {
        // Server-side: use axios with the API URL directly
        await api.put(url, {
          representative_id: Number.parseInt(representativeId, 10),
        });
        return;
      }
    } catch (error) {
      logger.error("failed to set representative for group", {
        representative_id: representativeId,
        group_id: groupId,
        error: error instanceof Error ? error.message : String(error),
      });
      throw error;
    }
  },
};

// Room service for API operations
export const roomService = {
  // Get all rooms
  getRooms: async (filters?: {
    building?: string;
    floor?: number;
    category?: string;
    occupied?: boolean;
    search?: string;
    page?: number;
    pageSize?: number;
  }): Promise<Room[]> => {
    const params = buildRoomQueryParams(filters);
    const queryString = params.toString();

    const useProxyApi = globalThis.window !== undefined;
    const baseUrl = useProxyApi ? "/api/rooms" : `${env.API_URL}/api/rooms`;
    const url = queryString ? `${baseUrl}?${queryString}` : baseUrl;

    try {
      if (useProxyApi) {
        // Browser environment: use fetchWithRetry for automatic 401 handling
        const token = await getSessionToken();
        const { data } = await fetchWithRetry<unknown>(url, token, {
          onAuthFailure: handleAuthFailure,
          getNewToken: getNewTokenFromSession,
        });

        const rooms = parseRoomsResponse(data);
        return mapRoomsResponse(rooms);
      }

      // Server-side: use axios with the API URL directly
      const response = await api.get(url, { params });
      const rooms = parseRoomsResponse(response.data);
      return mapRoomsResponse(rooms);
    } catch (error) {
      logger.error("failed to fetch rooms", {
        error: error instanceof Error ? error.message : String(error),
      });
      throw error;
    }
  },

  // Get a specific room by ID
  getRoom: async (id: string): Promise<Room> => {
    const useProxyApi = globalThis.window !== undefined;
    const url = useProxyApi
      ? `/api/rooms/${id}`
      : `${env.API_URL}/api/rooms/${id}`;

    try {
      if (useProxyApi) {
        // Browser environment: use fetchWithRetry for automatic 401 handling
        const token = await getSessionToken();
        const { response, data } = await fetchWithRetry<unknown>(url, token, {
          onAuthFailure: handleAuthFailure,
          getNewToken: getNewTokenFromSession,
        });

        if (response === null) {
          throw new Error("Authentication failed");
        }

        const roomData = extractBackendRoom(data);
        return mapSingleRoomResponse({ data: roomData });
      }

      // Server-side: use axios with the API URL directly
      const response = await api.get(url);
      const roomData = extractBackendRoom(response.data);
      return mapSingleRoomResponse({ data: roomData });
    } catch (error) {
      logger.error("failed to fetch room", {
        room_id: id,
        error: error instanceof Error ? error.message : String(error),
      });
      throw error;
    }
  },

  // Create a new room
  createRoom: async (room: Omit<Room, "id" | "isOccupied">): Promise<Room> => {
    // Validate room data before transformation
    validateRoomForCreation(room);

    // Transform from frontend model to backend model
    const backendRoom = prepareRoomForBackend(room);

    const useProxyApi = globalThis.window !== undefined;
    const url = useProxyApi ? `/api/rooms` : `${env.API_URL}/api/rooms`;

    try {
      if (useProxyApi) {
        const response = await fetch(url, {
          method: "POST",
          credentials: "include",
          headers: await getAuthHeaders(),
          body: JSON.stringify(backendRoom),
        });

        if (!response.ok) {
          const errorText = await response.text();
          logger.error("api error during fetch", {
            status: response.status,
            error_text: errorText.substring(0, 200), // Truncate long errors
          });
          const errorMessage = parseApiErrorMessage(errorText);
          throw new Error(
            errorMessage
              ? `API error: ${errorMessage}`
              : `API error: ${response.status}`,
          );
        }

        const data = (await response.json()) as BackendRoom;
        return mapSingleRoomResponse({ data });
      }

      // Server-side: use axios with the API URL directly
      const response = await api.post(url, backendRoom);
      return mapSingleRoomResponse({ data: response.data as BackendRoom });
    } catch (error) {
      logger.error("failed to create room", {
        error: error instanceof Error ? error.message : String(error),
      });
      throw error;
    }
  },

  // Update a room
  updateRoom: async (id: string, room: Partial<Room>): Promise<Room> => {
    // Transform from frontend model to backend model updates
    const backendUpdates = prepareRoomForBackend(room);

    const useProxyApi = globalThis.window !== undefined;
    const url = useProxyApi
      ? `/api/rooms/${id}`
      : `${env.API_URL}/api/rooms/${id}`;

    try {
      if (useProxyApi) {
        // Browser environment: use fetch with our Next.js API route
        const response = await fetch(url, {
          method: "PUT",
          credentials: "include",
          headers: await getAuthHeaders(),
          body: JSON.stringify(backendUpdates),
        });

        if (!response.ok) {
          const errorText = await response.text();
          logger.error("api error during fetch", {
            status: response.status,
            error_text: errorText.substring(0, 200), // Truncate long errors
          });

          // Try to parse error text as JSON for more detailed error
          try {
            const errorJson = JSON.parse(errorText) as { error?: string };
            if (errorJson.error) {
              throw new Error(
                `API error ${response.status}: ${errorJson.error}`,
              );
            }
          } catch {
            // If parsing fails, use status code + error text
            throw new Error(
              `API error ${response.status}: ${errorText.substring(0, 100)}`,
            );
          }
        }

        const data = (await response.json()) as BackendRoom;
        return mapSingleRoomResponse({ data });
      } else {
        // Server-side: use axios with the API URL directly
        const response = await api.put(url, backendUpdates);
        return mapSingleRoomResponse({ data: response.data as BackendRoom });
      }
    } catch (error) {
      logger.error("failed to update room", {
        room_id: id,
        error: error instanceof Error ? error.message : String(error),
      });
      throw error;
    }
  },

  // Delete a room
  deleteRoom: async (id: string): Promise<void> => {
    const useProxyApi = globalThis.window !== undefined;
    const url = useProxyApi
      ? `/api/rooms/${id}`
      : `${env.API_URL}/api/rooms/${id}`;

    try {
      if (useProxyApi) {
        // Browser environment: use fetch with our Next.js API route
        const response = await fetch(url, {
          method: "DELETE",
          credentials: "include",
          headers: await getAuthHeaders(),
        });

        if (!response.ok) {
          const errorText = await response.text();
          logger.error("api error during fetch", {
            status: response.status,
            error_text: errorText.substring(0, 200), // Truncate long errors
          });
          throw new Error(`API error: ${response.status}`);
        }

        return;
      } else {
        // Server-side: use axios with the API URL directly
        await api.delete(url);
        return;
      }
    } catch (error) {
      logger.error("failed to delete room", {
        room_id: id,
        error: error instanceof Error ? error.message : String(error),
      });
      throw error;
    }
  },

  // Get rooms grouped by category
  getRoomsByCategory: async (): Promise<Record<string, Room[]>> => {
    const useProxyApi = globalThis.window !== undefined;
    const url = useProxyApi
      ? "/api/rooms/by-category"
      : `${env.API_URL}/api/rooms/by-category`;

    try {
      if (useProxyApi) {
        // Browser environment: use fetch with our Next.js API route
        const response = await fetch(url, {
          credentials: "include",
          headers: await getAuthHeaders(),
        });

        if (!response.ok) {
          const errorText = await response.text();
          logger.error("api error during fetch", {
            status: response.status,
            error_text: errorText.substring(0, 200), // Truncate long errors
          });
          throw new Error(`API error: ${response.status}`);
        }

        const data = (await response.json()) as Record<string, BackendRoom[]>;

        // Transform each category's room array
        const result: Record<string, Room[]> = {};
        for (const [category, rooms] of Object.entries(data)) {
          result[category] = mapRoomsResponse(rooms);
        }

        return result;
      } else {
        // Server-side: use axios with the API URL directly
        const response = await api.get(url);
        const data = response.data as Record<string, BackendRoom[]>;

        // Transform each category's room array
        const result: Record<string, Room[]> = {};
        for (const [category, rooms] of Object.entries(data)) {
          result[category] = mapRoomsResponse(rooms);
        }

        return result;
      }
    } catch (error) {
      logger.error("failed to fetch rooms by category", {
        error: error instanceof Error ? error.message : String(error),
      });
      throw error;
    }
  },
};

export default api;
