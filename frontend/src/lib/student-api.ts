// lib/student-api.ts
import { getCachedSession, sessionFetch } from "./session-cache";
import { createLogger } from "~/lib/logger";
import { isAxiosError } from "axios";

const logger = createLogger({ component: "StudentAPI" });
import { env } from "~/env";
import api from "./api";
import {
  handleDomainApiError,
  isBrowserContext,
  authFetch,
} from "./api-helpers";
import {
  mapStudentResponse,
  mapStudentsResponse,
  mapStudentDetailResponse,
  mapPrivacyConsentResponse,
  type Student,
  type BackendStudent,
  type BackendStudentDetail,
  type BackendPrivacyConsent,
  type BackendUpdateRequest,
  type PrivacyConsent,
} from "./student-helpers";
import {
  mapGroupResponse,
  type Group,
  type BackendGroup,
} from "./group-helpers";

// Filter interface for searching students
export interface StudentFilters {
  search?: string;
  school_class?: string;
  group_id?: string;
  location?: string;
  location_state?: "present" | "transit";
  guardian_name?: string;
  first_name?: string;
  last_name?: string;
  page?: number;
  page_size?: number;
}

// Generic API response interface
interface ApiResponse<T> {
  data: T;
  message?: string;
  status?: string;
}

// Paginated response interface
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

interface WrappedPaginatedResponse<T> {
  status?: string;
  data:
    | PaginatedResponse<T>
    | {
        data: T[];
        pagination: PaginatedResponse<T>["pagination"];
      };
  message?: string;
}

interface StudentEnrollmentExtraFieldOption {
  label: string;
  value: string;
}

interface StudentEnrollmentExtraField {
  key: string;
  label: string;
  type: string;
  target?: string;
  options?: StudentEnrollmentExtraFieldOption[];
  value: unknown;
}

export interface StudentEnrollmentExtraFieldGroup {
  request_id: string;
  phase_name: string;
  submitted_at: string;
  fields: StudentEnrollmentExtraField[];
}

// Error handler using shared utility
function handleStudentApiError(error: unknown, context: string): never {
  handleDomainApiError(error, context, "STUDENT");
}

// Helper: Build URL with optional query params
// Note: Browser uses proxy path (/api/students), server uses direct backend path (/students)
function buildStudentUrl(
  proxyPath: string,
  backendPath: string,
  filters?: StudentFilters,
): { url: string; useProxy: boolean } {
  const useProxy = isBrowserContext();
  const baseUrl = useProxy ? proxyPath : `${env.API_URL}${backendPath}`;

  if (!filters) return { url: baseUrl, useProxy };

  const params = new URLSearchParams();
  if (filters.search) params.append("search", filters.search);
  if (filters.school_class) params.append("school_class", filters.school_class);
  if (filters.group_id) params.append("group_id", filters.group_id);
  if (filters.location) params.append("location", filters.location);
  if (filters.location_state)
    params.append("location_state", filters.location_state);
  if (filters.guardian_name)
    params.append("guardian_name", filters.guardian_name);
  if (filters.first_name) params.append("first_name", filters.first_name);
  if (filters.last_name) params.append("last_name", filters.last_name);
  if (filters.page) params.append("page", filters.page.toString());
  if (filters.page_size)
    params.append("page_size", filters.page_size.toString());

  const queryString = params.toString();
  return { url: queryString ? `${baseUrl}?${queryString}` : baseUrl, useProxy };
}

// Fetch students with filters and pagination
export async function fetchStudents(filters?: StudentFilters): Promise<{
  students: Student[];
  pagination?: {
    current_page: number;
    page_size: number;
    total_pages: number;
    total_records: number;
  };
}> {
  const { url, useProxy } = buildStudentUrl(
    "/api/students",
    "/students",
    filters,
  );

  try {
    if (useProxy) {
      const session = await getCachedSession();
      const responseData = await authFetch<
        | Student[]
        | PaginatedResponse<Student>
        | WrappedPaginatedResponse<Student>
      >(url, { token: session?.user?.token });

      // Next.js API routes wrap GET responses once more:
      // { status, data: { data: students, pagination } }.
      if (
        responseData &&
        typeof responseData === "object" &&
        "data" in responseData &&
        responseData.data &&
        typeof responseData.data === "object" &&
        "data" in responseData.data &&
        "pagination" in responseData.data &&
        Array.isArray(responseData.data.data)
      ) {
        return {
          students: responseData.data.data,
          pagination: responseData.data.pagination,
        };
      }

      // Backend-style paginated response.
      if (
        responseData &&
        typeof responseData === "object" &&
        "data" in responseData &&
        "pagination" in responseData
      ) {
        return {
          students: responseData.data,
          pagination: responseData.pagination,
        };
      }

      // Fallback for non-paginated response
      return { students: Array.isArray(responseData) ? responseData : [] };
    }

    // Server-side: use axios with the API URL directly
    const response = await api.get<PaginatedResponse<BackendStudent>>(url);
    if (response.data?.data) {
      return {
        students: mapStudentsResponse(response.data.data),
        pagination: response.data.pagination,
      };
    }
    return { students: [] };
  } catch (error) {
    handleStudentApiError(error, "fetch students");
  }
}

// Type guard for wrapped API response
function isWrappedResponse<T>(
  responseData: unknown,
): responseData is ApiResponse<T> {
  return (
    responseData !== null &&
    typeof responseData === "object" &&
    "data" in responseData
  );
}

// Helper: Extract data from wrapped API response
function extractApiData<T>(responseData: ApiResponse<T> | T): T {
  if (isWrappedResponse<T>(responseData)) {
    return responseData.data;
  }
  return responseData;
}

// Fetch a single student by ID
export async function fetchStudent(id: string): Promise<Student> {
  const useProxy = isBrowserContext();
  const url = useProxy
    ? `/api/students/${id}`
    : `${env.API_URL}/students/${id}`;

  try {
    if (useProxy) {
      const session = await getCachedSession();
      const responseData = await authFetch<ApiResponse<Student> | Student>(
        url,
        {
          token: session?.user?.token,
        },
      );
      return extractApiData(responseData);
    }

    const response = await api.get<ApiResponse<BackendStudentDetail>>(url);
    return mapStudentDetailResponse(response.data.data);
  } catch (error) {
    handleStudentApiError(error, "fetch student");
  }
}

export async function fetchStudentEnrollmentExtraFields(
  id: string,
): Promise<StudentEnrollmentExtraFieldGroup[]> {
  const useProxy = isBrowserContext();
  const url = useProxy
    ? `/api/students/${id}/enrollment-extra-fields`
    : `${env.API_URL}/api/students/${id}/enrollment-extra-fields`;

  try {
    if (useProxy) {
      const session = await getCachedSession();
      const responseData = await authFetch<
        | ApiResponse<StudentEnrollmentExtraFieldGroup[]>
        | StudentEnrollmentExtraFieldGroup[]
      >(url, {
        token: session?.user?.token,
      });
      const data = extractApiData(responseData);
      return Array.isArray(data) ? data : [];
    }

    const response =
      await api.get<ApiResponse<StudentEnrollmentExtraFieldGroup[]>>(url);
    return response.data?.data ?? [];
  } catch (error) {
    handleStudentApiError(error, "fetch student enrollment extra fields");
  }
}

// Create a new student
export async function createStudent(studentData: {
  first_name: string;
  last_name: string;
  school_class: string;
  guardian_name: string;
  guardian_contact: string;
  group_id?: number;
  tag_id?: string;
  guardian_email?: string;
  guardian_phone?: string;
}): Promise<Student> {
  const useProxy = isBrowserContext();
  const url = useProxy ? "/api/students" : `${env.API_URL}/students`;

  try {
    if (useProxy) {
      const session = await getCachedSession();
      const responseData = await authFetch<
        ApiResponse<BackendStudent> | BackendStudent
      >(url, {
        method: "POST",
        body: studentData,
        token: session?.user?.token,
      });
      const data = extractApiData<BackendStudent>(responseData);
      return mapStudentResponse(data);
    }

    const response = await api.post<ApiResponse<BackendStudent>>(
      url,
      studentData,
    );
    return mapStudentResponse(response.data.data);
  } catch (error) {
    handleStudentApiError(error, "create student");
  }
}

// Update a student
export async function updateStudent(
  id: string,
  studentData: BackendUpdateRequest,
): Promise<Student> {
  const useProxy = isBrowserContext();
  const url = useProxy
    ? `/api/students/${id}`
    : `${env.API_URL}/students/${id}`;

  try {
    if (useProxy) {
      const session = await getCachedSession();
      const responseData = await authFetch<
        ApiResponse<BackendStudent> | BackendStudent
      >(url, { method: "PUT", body: studentData, token: session?.user?.token });
      const data = extractApiData<BackendStudent>(responseData);
      return mapStudentResponse(data);
    }

    const response = await api.put<ApiResponse<BackendStudent>>(
      url,
      studentData,
    );
    return mapStudentResponse(response.data.data);
  } catch (error) {
    handleStudentApiError(error, "update student");
  }
}

// ─── School check-in/out (web UI) ───────────────────────────────────────────

/** Action accepted by POST /api/students/{id}/school-checkin. */
export type SchoolCheckinAction = "in" | "out";

/**
 * Response shape mirrors `AttendanceStatus` on the backend plus the resolved
 * presence label so the UI can update a PresenceBadge optimistically without
 * a follow-up fetch.
 */
export interface SchoolCheckinResponse {
  /**
   * Backend int64 IDs are converted to string at the API boundary
   * (CLAUDE.md §4) so consumers don't lose precision in JSON/JS number land.
   */
  studentId: string;
  status: "checked_in" | "on_yard" | "checked_out" | "not_checked_in";
  checkInTime?: string;
  checkOutTime?: string;
  yardSince?: string;
  location: "Anwesend" | "Schulhof" | "Abwesend";
  /** false when the call was a no-op because the student was already in the target state. */
  changed: boolean;
}

interface BackendSchoolCheckinResponse {
  student_id: number;
  status: "checked_in" | "on_yard" | "checked_out" | "not_checked_in";
  check_in_time?: string;
  check_out_time?: string;
  yard_since?: string;
  location: "Anwesend" | "Schulhof" | "Abwesend";
  changed: boolean;
}

/**
 * Check a student in or out of school via the web UI.
 * Mode-agnostic: always writes active.attendance; room visits stay the
 * exclusive responsibility of the IoT kiosk flow.
 *
 * Throws on 4xx/5xx — callers typically handle 403 (no permission / not a
 * group supervisor) separately from other errors for tailored UI messaging.
 */
export async function schoolCheckinStudent(
  studentId: string,
  action: SchoolCheckinAction,
): Promise<SchoolCheckinResponse> {
  const url = `/api/students/${studentId}/school-checkin`;
  try {
    const session = await getCachedSession();
    const response = await authFetch<
      ApiResponse<BackendSchoolCheckinResponse> | BackendSchoolCheckinResponse
    >(url, {
      method: "POST",
      body: { action },
      token: session?.user?.token,
    });

    const data = extractApiData<BackendSchoolCheckinResponse>(response);
    return {
      // Convert int64 -> string at the boundary per CLAUDE.md §4.
      studentId: data.student_id.toString(),
      status: data.status,
      checkInTime: data.check_in_time,
      checkOutTime: data.check_out_time,
      yardSince: data.yard_since,
      location: data.location,
      changed: data.changed,
    };
  } catch (error) {
    logger.error("school_checkin_failed", {
      student_id: studentId,
      action,
      error: error instanceof Error ? error.message : String(error),
    });
    throw error;
  }
}

// Delete a student
export async function deleteStudent(id: string): Promise<void> {
  const useProxy = isBrowserContext();
  const url = useProxy
    ? `/api/students/${id}`
    : `${env.API_URL}/students/${id}`;

  try {
    if (useProxy) {
      const session = await getCachedSession();
      await authFetch<void>(url, {
        method: "DELETE",
        token: session?.user?.token,
      });
    } else {
      await api.delete(url);
    }
  } catch (error) {
    handleStudentApiError(error, "delete student");
  }
}

// ─── Student photo (Datenverwaltung) ────────────────────────────────────────

/**
 * Upload a photo for a student. Caller is expected to have already passed
 * the file through `compressAvatar` (lib/image-utils) — sending raw 5+ MB
 * camera JPEGs from a phone would hit the backend's `maxStudentPhotoBody`
 * cap and fail with 400.
 *
 * Pre-conditions checked server-side:
 *   1. Tenant has `operations.student_photos_enabled = true`
 *   2. Either the row already has photo_consent_given_at set, OR the
 *      caller passes `consentAcknowledged=true` so the backend can stamp
 *      consent atomically alongside saving the photo. The form-level
 *      checkbox state drives this — the user can tick consent + upload
 *      without an intermediate "Speichern" round-trip.
 *
 * The backend stores filesystem paths internally and returns the stable
 * `/uploads/student-photos/...` URL; consumers should treat the returned
 * URL as opaque and re-render the avatar without local path arithmetic.
 */
export async function uploadStudentPhoto(
  studentId: string,
  file: Blob,
  options: { consentAcknowledged?: boolean } = {},
): Promise<{ photoUrl: string }> {
  const url = `/api/students/${studentId}/photo`;
  const formData = new FormData();
  const photoFile =
    file instanceof File
      ? file
      : new File([file], "student-photo.jpg", {
          type: file.type || "image/jpeg",
        });
  formData.append("photo", photoFile);
  if (options.consentAcknowledged) {
    formData.append("consent_acknowledged", "true");
  }

  const session = await getCachedSession();
  const token = session?.user?.token;
  if (!token) {
    throw new Error("Authentifizierung erforderlich");
  }

  // Use raw fetch — authFetch wraps JSON bodies, but this endpoint expects
  // multipart/form-data. We forward the cookie-derived JWT manually.
  const response = await fetch(url, {
    method: "POST",
    body: formData,
    headers: { Authorization: `Bearer ${token}` },
  });

  if (!response.ok) {
    const text = await response.text();
    logger.error("student_photo_upload_failed", {
      student_id: studentId,
      status: response.status,
      error: text,
    });
    throw new Error(text || `Upload fehlgeschlagen (HTTP ${response.status})`);
  }

  const body = (await response.json()) as
    { data: { photo_url: string } } | { photo_url: string };
  const data = "data" in body ? body.data : body;
  return { photoUrl: data.photo_url };
}

/** Remove a student's photo (idempotent — succeeds even with no photo set). */
export async function deleteStudentPhoto(studentId: string): Promise<void> {
  const url = `/api/students/${studentId}/photo`;
  const session = await getCachedSession();
  await authFetch<void>(url, {
    method: "DELETE",
    token: session?.user?.token,
  });
}

// Fetch all groups (for filter dropdown)
export async function fetchGroups(): Promise<Group[]> {
  const useProxy = isBrowserContext();
  const url = useProxy ? "/api/groups" : `${env.API_URL}/groups`;

  try {
    if (useProxy) {
      const session = await getCachedSession();
      const responseData = await authFetch<BackendGroup[]>(url, {
        token: session?.user?.token,
      });
      const groups = Array.isArray(responseData) ? responseData : [];
      return groups.map(mapGroupResponse);
    }

    const response = await api.get<{ data: BackendGroup[] }>(url);
    const groups = response.data.data || [];
    return groups.map(mapGroupResponse);
  } catch (error) {
    logger.error("failed to fetch groups", { error: String(error) });
    return [];
  }
}

// Fetch student's privacy consent
export async function fetchStudentPrivacyConsent(
  studentId: string,
): Promise<PrivacyConsent | null> {
  const useProxy = isBrowserContext();
  const url = useProxy
    ? `/api/students/${studentId}/privacy-consent`
    : `${env.API_URL}/students/${studentId}/privacy-consent`;

  try {
    if (useProxy) {
      const response = await sessionFetch(url, {
        method: "GET",
        credentials: "include",
      });

      if (!response.ok) {
        if (response.status === 404) return null;
        throw new Error(
          `API error (${response.status}): ${response.statusText}`,
        );
      }

      const responseData =
        (await response.json()) as ApiResponse<BackendPrivacyConsent>;
      return mapPrivacyConsentResponse(responseData.data);
    }

    const response = await api.get<ApiResponse<BackendPrivacyConsent>>(url);
    return mapPrivacyConsentResponse(response.data.data);
  } catch (error) {
    if (isAxiosError(error) && error.response?.status === 404) {
      return null;
    }
    logger.error("failed to fetch privacy consent", {
      student_id: studentId,
      error: String(error),
    });
    throw error;
  }
}

// Update or create student's privacy consent
export async function updateStudentPrivacyConsent(
  studentId: string,
  consentData: {
    policy_version: string;
    accepted: boolean;
    duration_days?: number;
    data_retention_days: number;
    details?: Record<string, unknown>;
  },
): Promise<PrivacyConsent> {
  const useProxy = isBrowserContext();
  const url = useProxy
    ? `/api/students/${studentId}/privacy-consent`
    : `${env.API_URL}/students/${studentId}/privacy-consent`;

  try {
    if (useProxy) {
      const session = await getCachedSession();
      const responseData = await authFetch<ApiResponse<BackendPrivacyConsent>>(
        url,
        { method: "PUT", body: consentData, token: session?.user?.token },
      );
      return mapPrivacyConsentResponse(responseData.data);
    }

    const session = await getCachedSession();
    const response = await api.put<ApiResponse<BackendPrivacyConsent>>(
      url,
      consentData,
      { headers: { Authorization: `Bearer ${session?.user?.token}` } },
    );
    return mapPrivacyConsentResponse(response.data.data);
  } catch (error) {
    handleStudentApiError(error, "update privacy consent");
  }
}
