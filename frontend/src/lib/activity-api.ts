// lib/activity-api.ts
import { sessionFetch } from "./session-cache";
import api from "./api";
import { handleDomainApiError } from "./api-helpers";
import { ApiError, apiErrorFromResponse } from "./api-error";

// Error handler using shared utility. A structured ApiError (browser path)
// passes through unchanged, so code, field errors and request ID reach the
// shared display path.
function handleActivityApiError(error: unknown, context: string): never {
  if (error instanceof ApiError) throw error;
  handleDomainApiError(error, context);
}
import {
  mapActivityCategoryResponse,
  mapActivityScheduleResponse,
  mapTimeframeResponse,
  mapStudentEnrollmentResponse,
  prepareActivityScheduleForBackend,
  type Activity,
  type ActivityCategory,
  type UpdateActivityRequest,
  type ActivityFilter,
  type BackendActivityCategory,
  type ActivityStudent,
  type BackendStudentEnrollment,
  type ActivitySchedule,
  type BackendActivitySchedule,
  type Timeframe,
  type BackendTimeframe,
} from "./activity-helpers";

// Re-export types for external use
export type { Activity, ActivityCategory } from "./activity-helpers";

// Generic API response interface
interface ApiResponse<T> {
  data: T;
  message?: string;
  status?: string;
}

// Available time slot type
interface AvailableTimeSlot {
  weekday: string;
  timeframe_id?: string;
}

function isBrowserApiContext(): boolean {
  return globalThis.window !== undefined;
}

async function resolveActivityApiUrl(path: string): Promise<string> {
  if (isBrowserApiContext()) {
    return path;
  }

  const { getServerApiUrl } = await import("~/lib/server-api-url");
  return `${getServerApiUrl()}${path}`;
}

// Helper: Build URL with query params for activities
function buildActivitiesUrl(baseUrl: string, filters?: ActivityFilter): string {
  const params = new URLSearchParams();
  if (filters?.search) params.append("search", filters.search);
  if (filters?.category_id) params.append("category_id", filters.category_id);
  if (filters?.is_open_ags !== undefined) {
    params.append("is_open_ags", filters.is_open_ags.toString());
  }
  const queryString = params.toString();
  return queryString ? `${baseUrl}?${queryString}` : baseUrl;
}

// Helper: Check if response has nested data.data structure
function hasNestedDataStructure(
  data: unknown,
): data is { data: { data: Activity[] } } {
  if (!data || typeof data !== "object" || !("data" in data)) return false;
  const outer = data as { data: unknown };
  if (!outer.data || typeof outer.data !== "object" || !("data" in outer.data))
    return false;
  const inner = outer.data as { data: unknown };
  return Array.isArray(inner.data);
}

// Helper: Check if response has direct data array structure
function hasDirectDataStructure(data: unknown): data is { data: Activity[] } {
  return (
    !!data &&
    typeof data === "object" &&
    "data" in data &&
    Array.isArray((data as { data: unknown }).data)
  );
}

// Helper: Parse activity response from browser fetch
function parseActivitiesResponse(responseData: unknown): Activity[] {
  if (hasNestedDataStructure(responseData)) {
    return responseData.data.data;
  }
  if (hasDirectDataStructure(responseData)) {
    return responseData.data;
  }
  if (Array.isArray(responseData)) {
    return responseData as Activity[];
  }
  return [];
}

// Helper: Parse enrolled students response
// Backend returns StudentResponse with {id, first_name, last_name}
// We use mapStudentEnrollmentResponse which handles this structure
function parseEnrolledStudentsResponse(
  responseData: unknown,
): ActivityStudent[] {
  const isActivityStudent = (value: unknown): value is ActivityStudent => {
    if (!value || typeof value !== "object") {
      return false;
    }

    return "student_id" in value && "name" in value;
  };

  const normalizeActivityStudent = (
    student: ActivityStudent,
  ): ActivityStudent => ({
    ...student,
    id: String(student.id),
    student_id: String(student.student_id),
    created_at:
      student.created_at instanceof Date
        ? student.created_at
        : new Date(student.created_at),
    updated_at:
      student.updated_at instanceof Date
        ? student.updated_at
        : new Date(student.updated_at),
  });

  // Check for wrapped response with data property
  if (
    responseData &&
    typeof responseData === "object" &&
    "data" in responseData
  ) {
    const wrapped = responseData as { data: unknown };
    if (!Array.isArray(wrapped.data)) {
      return [];
    }

    return wrapped.data.map((item) =>
      isActivityStudent(item)
        ? normalizeActivityStudent(item)
        : mapStudentEnrollmentResponse(item as BackendStudentEnrollment),
    );
  }
  // Handle direct array response
  return Array.isArray(responseData)
    ? responseData.map((item) =>
        isActivityStudent(item)
          ? normalizeActivityStudent(item)
          : mapStudentEnrollmentResponse(item as BackendStudentEnrollment),
      )
    : [];
}

// Helper: Parse schedules response (wrapped or direct array)
function parseSchedulesResponse(responseData: unknown): ActivitySchedule[] {
  // Check for wrapped response with data property
  if (
    responseData &&
    typeof responseData === "object" &&
    "data" in responseData
  ) {
    const wrapped = responseData as { data: unknown };
    return Array.isArray(wrapped.data)
      ? wrapped.data.map(mapActivityScheduleResponse)
      : [];
  }
  // Handle direct array response
  return Array.isArray(responseData)
    ? responseData.map(mapActivityScheduleResponse)
    : [];
}

// Helper: Check if item is a valid timeframe-like object (has "id" property)
function isValidTimeframeObject(item: unknown): boolean {
  return item !== null && typeof item === "object" && "id" in item;
}

// Helper: Check if timeframe item is already frontend type (has string ID)
function isAlreadyFrontendTimeframe(item: unknown): boolean {
  return (
    isValidTimeframeObject(item) &&
    typeof (item as { id: unknown }).id === "string"
  );
}

// Helper: Parse timeframe array for wrapped response (always maps if not frontend type)
function parseTimeframeArrayWrapped(data: unknown[]): Timeframe[] {
  if (data.length === 0) {
    return [];
  }
  if (isAlreadyFrontendTimeframe(data[0])) {
    return data as Timeframe[];
  }
  return (data as BackendTimeframe[]).map(mapTimeframeResponse);
}

// Helper: Parse timeframe array for direct response (validates first, then maps)
function parseTimeframeArrayDirect(data: unknown[]): Timeframe[] {
  if (data.length === 0) {
    return [];
  }
  // Original behavior: direct arrays require valid timeframe object with "id"
  if (!isValidTimeframeObject(data[0])) {
    return [];
  }
  if (isAlreadyFrontendTimeframe(data[0])) {
    return data as Timeframe[];
  }
  return (data as BackendTimeframe[]).map(mapTimeframeResponse);
}

// Helper: Parse timeframes response (handles wrapped, direct array, and type detection)
function parseTimeframesResponse(responseData: unknown): Timeframe[] {
  // Not an object - can't process
  if (!responseData || typeof responseData !== "object") {
    return [];
  }

  // Direct array response - uses stricter validation (original behavior)
  if (Array.isArray(responseData)) {
    return parseTimeframeArrayDirect(responseData);
  }

  // Wrapped response with data property - maps without extra validation (original behavior)
  if ("data" in responseData && responseData.data) {
    const wrappedData = (responseData as { data: unknown }).data;
    if (Array.isArray(wrappedData)) {
      return parseTimeframeArrayWrapped(wrappedData);
    }
  }

  return [];
}

// Get all activities
export async function fetchActivities(
  filters?: ActivityFilter,
): Promise<Activity[]> {
  const baseUrl = "/api/activities";
  const url = buildActivitiesUrl(baseUrl, filters);

  // Browser environment: use fetch with our Next.js API route
  const response = await sessionFetch(url, {
    method: "GET",
  });

  if (!response.ok) {
    throw await apiErrorFromResponse(response, `API error: ${response.status}`);
  }

  const responseData = (await response.json()) as unknown;
  return parseActivitiesResponse(responseData);
}

// Get enrolled students for an activity
export async function getEnrolledStudents(
  activityId: string,
): Promise<ActivityStudent[]> {
  const url = `/api/activities/${activityId}/students`;

  try {
    const response = await sessionFetch(url, {
      method: "GET",
    });

    if (!response.ok) {
      throw await apiErrorFromResponse(
        response,
        `API error: ${response.status}`,
      );
    }

    const responseData = (await response.json()) as unknown;
    return parseEnrolledStudentsResponse(responseData);
  } catch (error) {
    handleActivityApiError(error, "fetch enrolled students");
  }
}

// Update an activity
export async function updateActivity(
  id: string,
  data: UpdateActivityRequest,
): Promise<Activity> {
  const url = `/api/activities/${id}`;

  const response = await sessionFetch(url, {
    method: "PUT",
    body: JSON.stringify(data),
  });

  if (!response.ok) {
    throw await apiErrorFromResponse(response, `API error: ${response.status}`);
  }

  const responseData = (await response.json()) as
    ApiResponse<Activity> | Activity;

  // Extract the data from the response wrapper if needed
  if (
    responseData &&
    typeof responseData === "object" &&
    "data" in responseData
  ) {
    return responseData.data;
  }
  return responseData;
}

// Delete an activity
export async function deleteActivity(id: string): Promise<void> {
  const url = `/api/activities/${id}`;

  const response = await sessionFetch(url, {
    method: "DELETE",
  });

  if (!response.ok) {
    throw await apiErrorFromResponse(response, `API error: ${response.status}`);
  }
}

// Get all categories
export async function getCategories(): Promise<ActivityCategory[]> {
  const url = "/api/activities/categories";

  try {
    const response = await sessionFetch(url, {
      method: "GET",
    });

    if (!response.ok) {
      throw await apiErrorFromResponse(
        response,
        `API error: ${response.status}`,
      );
    }

    const responseData = (await response.json()) as
      ApiResponse<BackendActivityCategory[]> | BackendActivityCategory[];

    // Extract the array from the response wrapper, then map to the frontend
    // shape. The browser path previously returned the raw backend payload
    // (snake_case shift_type_id, numeric id), which broke consumers relying
    // on the mapped ActivityCategory contract (e.g. the shift-type mapping
    // modal preselecting category.shiftTypeId).
    const rawCategories =
      responseData && typeof responseData === "object" && "data" in responseData
        ? responseData.data
        : responseData;
    return Array.isArray(rawCategories)
      ? rawCategories.map(mapActivityCategoryResponse)
      : [];
  } catch (error) {
    handleActivityApiError(error, "fetch categories");
  }
}

// Get schedules for an activity
export async function getActivitySchedules(
  activityId: string,
): Promise<ActivitySchedule[]> {
  const useProxyApi = isBrowserApiContext();
  const url = await resolveActivityApiUrl(
    `/api/activities/${activityId}/schedules`,
  );

  try {
    if (useProxyApi) {
      const response = await sessionFetch(url, {
        method: "GET",
      });

      if (!response.ok) {
        throw await apiErrorFromResponse(
          response,
          `API error: ${response.status}`,
        );
      }

      const responseData = (await response.json()) as unknown;
      return parseSchedulesResponse(responseData);
    }

    const response = await api.get<ApiResponse<BackendActivitySchedule[]>>(url);
    return parseSchedulesResponse(response.data);
  } catch {
    // Bewusst still: die leere Liste meldet den Fehlschlag, der Aufrufer
    // entscheidet über die Anzeige.
    return [];
  }
}

// Get a single schedule for an activity
export async function getActivitySchedule(
  activityId: string,
  scheduleId: string,
): Promise<ActivitySchedule | null> {
  const useProxyApi = isBrowserApiContext();
  const url = await resolveActivityApiUrl(
    `/api/activities/${activityId}/schedules/${scheduleId}`,
  );

  try {
    if (useProxyApi) {
      const response = await sessionFetch(url, {
        method: "GET",
      });

      if (!response.ok) {
        throw await apiErrorFromResponse(
          response,
          `API error: ${response.status}`,
        );
      }

      const responseData = (await response.json()) as
        ApiResponse<BackendActivitySchedule> | BackendActivitySchedule;

      // Extract the data from the response wrapper if needed
      if (
        responseData &&
        typeof responseData === "object" &&
        "data" in responseData
      ) {
        return mapActivityScheduleResponse(responseData.data);
      }
      return mapActivityScheduleResponse(responseData);
    } else {
      const response = await api.get<ApiResponse<BackendActivitySchedule>>(url);
      return mapActivityScheduleResponse(response.data.data);
    }
  } catch {
    // Bewusst still: null meldet den Fehlschlag, der Aufrufer entscheidet
    // über die Anzeige.
    return null;
  }
}

// Get all available timeframes
export async function getTimeframes(): Promise<Timeframe[]> {
  const useProxyApi = isBrowserApiContext();
  const url = await resolveActivityApiUrl("/api/schedules/timeframes");

  try {
    if (useProxyApi) {
      const response = await sessionFetch(url, {
        method: "GET",
      });

      if (!response.ok) {
        throw await apiErrorFromResponse(
          response,
          `API error: ${response.status}`,
        );
      }

      const responseData = (await response.json()) as unknown;
      return parseTimeframesResponse(responseData);
    }

    const response = await api.get<ApiResponse<BackendTimeframe[]>>(url);
    return parseTimeframesResponse(response.data);
  } catch (error) {
    handleActivityApiError(error, "fetch timeframes");
  }
}

// Get available time slots
export async function getAvailableTimeSlots(
  activityId: string,
  date?: string,
): Promise<AvailableTimeSlot[]> {
  const useProxyApi = isBrowserApiContext();
  let url = await resolveActivityApiUrl(
    `/api/activities/${activityId}/schedules/available`,
  );

  // Add date parameter if provided
  if (date) {
    url += `?date=${encodeURIComponent(date)}`;
  }

  try {
    if (useProxyApi) {
      const response = await sessionFetch(url, {
        method: "GET",
      });

      if (!response.ok) {
        throw await apiErrorFromResponse(
          response,
          `API error: ${response.status}`,
        );
      }

      const responseData = (await response.json()) as
        ApiResponse<AvailableTimeSlot[]> | AvailableTimeSlot[];

      // Extract the array from the response wrapper if needed
      if (
        responseData &&
        typeof responseData === "object" &&
        "data" in responseData
      ) {
        return responseData.data || [];
      }
      return responseData || [];
    } else {
      const response = await api.get<ApiResponse<AvailableTimeSlot[]>>(url);
      return response.data.data || [];
    }
  } catch {
    // Bewusst still: die leere Liste meldet den Fehlschlag, der Aufrufer
    // entscheidet über die Anzeige.
    return [];
  }
}

// Create a new schedule for an activity
export async function createActivitySchedule(
  activityId: string,
  scheduleData: Partial<ActivitySchedule>,
): Promise<ActivitySchedule> {
  const useProxyApi = isBrowserApiContext();
  const url = await resolveActivityApiUrl(
    `/api/activities/${activityId}/schedules`,
  );

  // Prepare backend data
  const backendData = prepareActivityScheduleForBackend(scheduleData);

  try {
    if (useProxyApi) {
      const response = await sessionFetch(url, {
        method: "POST",
        body: JSON.stringify(backendData),
      });

      if (!response.ok) {
        throw await apiErrorFromResponse(
          response,
          `API error: ${response.status}`,
        );
      }

      const responseData = (await response.json()) as
        ApiResponse<BackendActivitySchedule> | BackendActivitySchedule;

      // Extract the data from the response wrapper if needed
      if (
        responseData &&
        typeof responseData === "object" &&
        "data" in responseData
      ) {
        return mapActivityScheduleResponse(responseData.data);
      }
      return mapActivityScheduleResponse(responseData);
    } else {
      const response = await api.post<ApiResponse<BackendActivitySchedule>>(
        url,
        backendData,
      );
      return mapActivityScheduleResponse(response.data.data);
    }
  } catch (error) {
    handleActivityApiError(error, "create activity schedule");
  }
}

// Update a schedule for an activity
export async function updateActivitySchedule(
  activityId: string,
  scheduleId: string,
  scheduleData: Partial<ActivitySchedule>,
): Promise<ActivitySchedule | null> {
  const useProxyApi = isBrowserApiContext();
  const url = await resolveActivityApiUrl(
    `/api/activities/${activityId}/schedules/${scheduleId}`,
  );

  // Prepare backend data
  const backendData = prepareActivityScheduleForBackend(scheduleData);

  try {
    if (useProxyApi) {
      const response = await sessionFetch(url, {
        method: "PUT",
        body: JSON.stringify(backendData),
      });

      if (!response.ok) {
        throw await apiErrorFromResponse(
          response,
          `API error: ${response.status}`,
        );
      }

      const responseData = (await response.json()) as
        ApiResponse<BackendActivitySchedule> | BackendActivitySchedule;

      // Extract the data from the response wrapper if needed
      if (
        responseData &&
        typeof responseData === "object" &&
        "data" in responseData
      ) {
        return mapActivityScheduleResponse(responseData.data);
      }
      return mapActivityScheduleResponse(responseData);
    } else {
      const response = await api.put<ApiResponse<BackendActivitySchedule>>(
        url,
        backendData,
      );
      return mapActivityScheduleResponse(response.data.data);
    }
  } catch {
    // Bewusst still: null meldet den Fehlschlag, der Aufrufer entscheidet
    // über die Anzeige.
    return null;
  }
}

// Delete a schedule for an activity
export async function deleteActivitySchedule(
  activityId: string,
  scheduleId: string,
): Promise<boolean> {
  const useProxyApi = isBrowserApiContext();
  const url = await resolveActivityApiUrl(
    `/api/activities/${activityId}/schedules/${scheduleId}`,
  );

  try {
    if (useProxyApi) {
      const response = await sessionFetch(url, {
        method: "DELETE",
      });

      if (!response.ok) {
        throw await apiErrorFromResponse(
          response,
          `API error: ${response.status}`,
        );
      }

      return true;
    } else {
      await api.delete(url);
      return true;
    }
  } catch {
    // Bewusst still: false meldet den Fehlschlag, der Aufrufer entscheidet
    // über die Anzeige.
    return false;
  }
}
