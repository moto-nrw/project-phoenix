// lib/activity-api.ts
import { getSession } from "next-auth/react";
import { env } from "~/env";
import api from "./api";
import { handleAuthFailure } from "./auth-api";

// Standardized error handling function for activities API
function handleActivityApiError(error: unknown, context: string): never {
  // If we have a structured error message with status code
  if (error instanceof Error) {
    const regex = /API error \((\d+)\):/;
    const match = regex.exec(error.message);
    if (match?.[1]) {
      const status = Number.parseInt(match[1], 10);
      const errorMessage = `Failed to ${context}: ${error.message}`;
      throw new Error(
        JSON.stringify({
          status,
          message: errorMessage,
          code: `ACTIVITY_API_ERROR_${status}`,
        }),
      );
    }
  }

  // Default error response
  throw new Error(
    JSON.stringify({
      status: 500,
      message: `Failed to ${context}: ${error instanceof Error ? error.message : "Unknown error"}`,
      code: "ACTIVITY_API_ERROR_UNKNOWN",
    }),
  );
}
import {
  mapActivityResponse,
  mapActivityCategoryResponse,
  mapSupervisorResponse,
  mapActivityStudentResponse,
  mapActivityScheduleResponse,
  mapTimeframeResponse,
  prepareActivityForBackend,
  prepareActivityScheduleForBackend,
  type Activity,
  type ActivityCategory,
  type CreateActivityRequest,
  type UpdateActivityRequest,
  type ActivityFilter,
  type BackendActivity,
  type BackendActivityCategory,
  type BackendSupervisor,
  type BackendActivitySupervisor,
  type ActivityStudent,
  type BackendActivityStudent,
  type ActivitySchedule,
  type BackendActivitySchedule,
  type Timeframe,
  type BackendTimeframe,
} from "./activity-helpers";

// Re-export types for external use
export type {
  Activity,
  ActivityCategory,
  Supervisor,
  BackendActivity,
  BackendActivityCategory,
  BackendSupervisor,
  BackendActivitySupervisor,
  ActivityStudent,
  BackendActivityStudent,
  ActivitySchedule,
  BackendActivitySchedule,
  Timeframe,
  BackendTimeframe,
} from "./activity-helpers";

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
function parseEnrolledStudentsResponse(
  responseData: unknown,
): ActivityStudent[] {
  // Check for wrapped response with data property
  if (
    responseData &&
    typeof responseData === "object" &&
    "data" in responseData
  ) {
    const wrapped = responseData as { data: unknown };
    return Array.isArray(wrapped.data)
      ? wrapped.data.map(mapActivityStudentResponse)
      : [];
  }
  // Handle direct array response
  return Array.isArray(responseData)
    ? responseData.map(mapActivityStudentResponse)
    : [];
}

// Helper: Type guard for BackendActivity
function isBackendActivity(data: unknown): data is BackendActivity {
  return (
    !!data &&
    typeof data === "object" &&
    "id" in data &&
    "name" in data &&
    "max_participants" in data &&
    "category_id" in data
  );
}

// Helper: Check if data is a non-null object
function isNonNullObject(data: unknown): data is Record<string, unknown> {
  return typeof data === "object" && data !== null;
}

// Helper: Parse double-wrapped response { data: { data: Activity } }
// Returns Activity only if it's a valid BackendActivity, otherwise null
function parseDoubleWrappedData(innerData: unknown): Activity | null {
  if (!isNonNullObject(innerData) || !("data" in innerData)) {
    return null;
  }
  const deepData = innerData.data;
  if (isBackendActivity(deepData)) {
    return mapActivityResponse(deepData);
  }
  // Don't return partial objects - fall through to ID extraction
  return null;
}

// Helper: Parse wrapped response data
// Returns Activity only if it's a valid BackendActivity, otherwise null
function parseWrappedData(innerData: unknown): Activity | null {
  // Try double-wrapped first
  const doubleWrapped = parseDoubleWrappedData(innerData);
  if (doubleWrapped) {
    return doubleWrapped;
  }
  // Try single-wrapped BackendActivity
  if (isBackendActivity(innerData)) {
    return mapActivityResponse(innerData);
  }
  // Don't return partial objects - fall through to ID extraction
  return null;
}

// Helper: Extract ID from nested response data
function extractIdFromResponse(responseData: unknown): string | null {
  if (!isNonNullObject(responseData)) {
    return null;
  }
  // Check wrapped response { data: ... }
  if ("data" in responseData && isNonNullObject(responseData.data)) {
    const innerData = responseData.data;
    // Double-wrapped { data: { data: { id: ... } } }
    if (
      "data" in innerData &&
      isNonNullObject(innerData.data) &&
      "id" in innerData.data
    ) {
      return String(innerData.data.id);
    }
    // Single-wrapped { data: { id: ... } }
    if ("id" in innerData) {
      return String(innerData.id);
    }
  }
  // Direct response { id: ... }
  if ("id" in responseData) {
    return String(responseData.id);
  }
  return null;
}

// Helper: Parse activity creation response
// Returns full Activity if BackendActivity found, otherwise fallback with extracted ID
function parseCreateActivityResponse(
  responseData: unknown,
  fallback: Activity,
): Activity {
  if (!isNonNullObject(responseData)) {
    return fallback;
  }

  // Try to parse as full BackendActivity (handles wrapped and direct)
  if ("data" in responseData && responseData.data) {
    const result = parseWrappedData(responseData.data);
    if (result) {
      return result;
    }
  }

  if (isBackendActivity(responseData)) {
    return mapActivityResponse(responseData);
  }

  // Not a full BackendActivity - extract ID if present and return fallback
  const extractedId = extractIdFromResponse(responseData);
  if (extractedId) {
    return { ...fallback, id: extractedId };
  }

  return fallback;
}

// Helper: Create safe fallback activity from request data
function createSafeActivity(data: CreateActivityRequest): Activity {
  return {
    id: "0",
    name: data.name ?? "",
    max_participant: data.max_participants ?? 0,
    is_open_ags: false,
    supervisor_id: data.supervisor_ids?.[0]
      ? String(data.supervisor_ids[0])
      : "",
    ag_category_id: String(data.category_id ?? ""),
    created_at: new Date(),
    updated_at: new Date(),
    participant_count: 0,
    times: [],
    students: [],
  };
}

// Get all activities
export async function fetchActivities(
  filters?: ActivityFilter,
): Promise<Activity[]> {
  const useProxyApi = globalThis.window !== undefined;
  const baseUrl = useProxyApi
    ? "/api/activities"
    : `${env.NEXT_PUBLIC_API_URL}/api/activities`;
  const url = buildActivitiesUrl(baseUrl, filters);

  if (useProxyApi) {
    // Browser environment: use fetch with our Next.js API route
    const session = await getSession();
    const response = await fetch(url, {
      method: "GET",
      credentials: "include",
      headers: session?.user?.token
        ? {
            Authorization: `Bearer ${session.user.token}`,
            "Content-Type": "application/json",
          }
        : undefined,
    });

    if (!response.ok) {
      throw new Error(`API error: ${response.status}`);
    }

    const responseData = (await response.json()) as unknown;
    return parseActivitiesResponse(responseData);
  }

  // Server-side: use axios with the API URL directly
  const response = await api.get<ApiResponse<BackendActivity[]>>(url);
  if (hasDirectDataStructure(response.data)) {
    return response.data.data.map(mapActivityResponse);
  }
  return [];
}

// Fetch a single activity by ID (wrapper for consistency with other fetch functions)
export async function fetchActivity(id: string): Promise<Activity> {
  return getActivity(id);
}

// Get a single activity by ID
export async function getActivity(id: string): Promise<Activity> {
  const useProxyApi = globalThis.window !== undefined;
  const url = useProxyApi
    ? `/api/activities/${id}`
    : `${env.NEXT_PUBLIC_API_URL}/api/activities/${id}`;

  if (useProxyApi) {
    const session = await getSession();
    const response = await fetch(url, {
      method: "GET",
      credentials: "include",
      headers: session?.user?.token
        ? {
            Authorization: `Bearer ${session.user.token}`,
            "Content-Type": "application/json",
          }
        : undefined,
    });

    if (!response.ok) {
      throw new Error(`API error: ${response.status}`);
    }

    const responseData = (await response.json()) as
      | ApiResponse<Activity>
      | Activity;

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

  const response = await api.get<ApiResponse<BackendActivity>>(url);
  return mapActivityResponse(response.data.data);
}

// Get enrolled students for an activity
export async function getEnrolledStudents(
  activityId: string,
): Promise<ActivityStudent[]> {
  const useProxyApi = globalThis.window !== undefined;
  const url = useProxyApi
    ? `/api/activities/${activityId}/students`
    : `${env.NEXT_PUBLIC_API_URL}/api/activities/${activityId}/students`;

  try {
    if (useProxyApi) {
      const session = await getSession();
      const response = await fetch(url, {
        method: "GET",
        credentials: "include",
        headers: session?.user?.token
          ? {
              Authorization: `Bearer ${session.user.token}`,
              "Content-Type": "application/json",
            }
          : undefined,
      });

      if (!response.ok) {
        throw new Error(`API error: ${response.status}`);
      }

      const responseData = (await response.json()) as unknown;
      return parseEnrolledStudentsResponse(responseData);
    }

    const response = await api.get<ApiResponse<BackendActivityStudent[]>>(url);
    return parseEnrolledStudentsResponse(response.data);
  } catch (error) {
    handleActivityApiError(error, "fetch enrolled students");
  }
}

// Enroll a student in an activity
export async function enrollStudent(
  activityId: string,
  studentData: { studentId: string },
): Promise<{ success: boolean }> {
  const useProxyApi = globalThis.window !== undefined;
  // Update URL to match backend endpoint structure which expects the studentId in the URL path
  const url = useProxyApi
    ? `/api/activities/${activityId}/enroll/${studentData.studentId}`
    : `${env.NEXT_PUBLIC_API_URL}/api/activities/${activityId}/enroll/${studentData.studentId}`;

  // No request body needed since backend extracts IDs from URL path

  if (useProxyApi) {
    const session = await getSession();
    const response = await fetch(url, {
      method: "POST",
      credentials: "include",
      headers: {
        "Content-Type": "application/json",
        ...(session?.user?.token && {
          Authorization: `Bearer ${session.user.token}`,
        }),
      },
      // No body needed
    });

    if (!response.ok) {
      throw new Error(`API error: ${response.status}`);
    }

    return { success: true };
  }

  // Send empty object as body
  await api.post(url, {});
  return { success: true };
}

// Unenroll a student from an activity
export async function unenrollStudent(
  activityId: string,
  studentId: string,
): Promise<void> {
  const useProxyApi = globalThis.window !== undefined;
  const url = useProxyApi
    ? `/api/activities/${activityId}/students/${studentId}`
    : `${env.NEXT_PUBLIC_API_URL}/api/activities/${activityId}/students/${studentId}`;

  if (useProxyApi) {
    const session = await getSession();
    const response = await fetch(url, {
      method: "DELETE",
      credentials: "include",
      headers: session?.user?.token
        ? {
            Authorization: `Bearer ${session.user.token}`,
            "Content-Type": "application/json",
          }
        : undefined,
    });

    if (!response.ok) {
      throw new Error(`API error: ${response.status}`);
    }
    return;
  }

  await api.delete(url);
}

// Create a new activity
export async function createActivity(
  data: CreateActivityRequest,
): Promise<Activity> {
  const useProxyApi = globalThis.window !== undefined;
  const url = useProxyApi
    ? "/api/activities"
    : `${env.NEXT_PUBLIC_API_URL}/api/activities`;

  const safeActivity = createSafeActivity(data);

  if (useProxyApi) {
    const session = await getSession();
    const response = await fetch(url, {
      method: "POST",
      credentials: "include",
      headers: {
        "Content-Type": "application/json",
        ...(session?.user?.token && {
          Authorization: `Bearer ${session.user.token}`,
        }),
      },
      body: JSON.stringify(data),
    });

    if (!response.ok) {
      throw new Error(`API error: ${response.status}`);
    }

    try {
      const responseData = (await response.json()) as unknown;
      return parseCreateActivityResponse(responseData, safeActivity);
    } catch {
      // Even if parsing fails, we know the POST was successful, so return safe activity
      return safeActivity;
    }
  }

  const response = await api.post<ApiResponse<BackendActivity>>(url, data);
  return parseCreateActivityResponse(response, safeActivity);
}

// Update an activity
export async function updateActivity(
  id: string,
  data: UpdateActivityRequest,
): Promise<Activity> {
  const useProxyApi = globalThis.window !== undefined;
  const url = useProxyApi
    ? `/api/activities/${id}`
    : `${env.NEXT_PUBLIC_API_URL}/api/activities/${id}`;

  // Convert UpdateActivityRequest to a format compatible with prepareActivityForBackend
  const activityData: Partial<Activity> = {
    name: data.name,
    max_participant: data.max_participants,
    is_open_ags: data.is_open,
    ag_category_id: String(data.category_id),
    supervisor_id:
      data.supervisor_ids && data.supervisor_ids.length > 0
        ? String(data.supervisor_ids[0])
        : undefined,
  };

  const backendData = prepareActivityForBackend(activityData);

  if (useProxyApi) {
    const session = await getSession();
    const response = await fetch(url, {
      method: "PUT",
      credentials: "include",
      headers: {
        "Content-Type": "application/json",
        ...(session?.user?.token && {
          Authorization: `Bearer ${session.user.token}`,
        }),
      },
      body: JSON.stringify(data),
    });

    if (!response.ok) {
      throw new Error(`API error: ${response.status}`);
    }

    const responseData = (await response.json()) as
      | ApiResponse<Activity>
      | Activity;

    // Extract the data from the response wrapper if needed
    if (
      responseData &&
      typeof responseData === "object" &&
      "data" in responseData
    ) {
      return responseData.data;
    }
    return responseData;
  } else {
    const response = await api.put<ApiResponse<BackendActivity>>(
      url,
      backendData,
    );
    return mapActivityResponse(response.data.data);
  }
}

// Delete an activity
export async function deleteActivity(id: string): Promise<void> {
  const useProxyApi = globalThis.window !== undefined;
  const url = useProxyApi
    ? `/api/activities/${id}`
    : `${env.NEXT_PUBLIC_API_URL}/api/activities/${id}`;

  if (useProxyApi) {
    const session = await getSession();
    const response = await fetch(url, {
      method: "DELETE",
      credentials: "include",
      headers: session?.user?.token
        ? {
            Authorization: `Bearer ${session.user.token}`,
            "Content-Type": "application/json",
          }
        : undefined,
    });

    if (!response.ok) {
      throw new Error(`API error: ${response.status}`);
    }
  } else {
    await api.delete(url);
  }
}

// Get all categories
export async function getCategories(): Promise<ActivityCategory[]> {
  const useProxyApi = globalThis.window !== undefined;
  const url = useProxyApi
    ? "/api/activities/categories"
    : `${env.NEXT_PUBLIC_API_URL}/api/activities/categories`;

  try {
    if (useProxyApi) {
      const session = await getSession();
      const response = await fetch(url, {
        method: "GET",
        credentials: "include",
        headers: session?.user?.token
          ? {
              Authorization: `Bearer ${session.user.token}`,
              "Content-Type": "application/json",
            }
          : undefined,
      });

      if (!response.ok) {
        throw new Error(`API error: ${response.status}`);
      }

      const responseData = (await response.json()) as
        | ApiResponse<ActivityCategory[]>
        | ActivityCategory[];

      // Extract the array from the response wrapper if needed
      if (
        responseData &&
        typeof responseData === "object" &&
        "data" in responseData
      ) {
        return responseData.data;
      }
      return responseData;
    } else {
      const response =
        await api.get<ApiResponse<BackendActivityCategory[]>>(url);
      return Array.isArray(response.data.data)
        ? response.data.data.map(mapActivityCategoryResponse)
        : [];
    }
  } catch (error) {
    handleActivityApiError(error, "fetch categories");
  }
}

// Get all supervisors
export async function getSupervisors(): Promise<
  Array<{ id: string; name: string }>
> {
  const useProxyApi = globalThis.window !== undefined;
  const url = useProxyApi
    ? "/api/activities/supervisors"
    : `${env.NEXT_PUBLIC_API_URL}/api/activities/supervisors`;

  try {
    if (useProxyApi) {
      const session = await getSession();
      const response = await fetch(url, {
        method: "GET",
        credentials: "include",
        headers: session?.user?.token
          ? {
              Authorization: `Bearer ${session.user.token}`,
              "Content-Type": "application/json",
            }
          : undefined,
      });

      if (!response.ok) {
        throw new Error(`API error: ${response.status}`);
      }

      const responseData = (await response.json()) as
        | ApiResponse<Array<{ id: string; name: string }>>
        | Array<{ id: string; name: string }>;

      // Extract the array from the response wrapper if needed
      if (
        responseData &&
        typeof responseData === "object" &&
        "data" in responseData
      ) {
        return responseData.data;
      }
      return responseData;
    } else {
      const response = await api.get<ApiResponse<BackendSupervisor[]>>(url);
      return Array.isArray(response.data.data)
        ? response.data.data.map(mapSupervisorResponse)
        : [];
    }
  } catch {
    return [];
  }
}

// Get schedules for an activity
export async function getActivitySchedules(
  activityId: string,
): Promise<ActivitySchedule[]> {
  const useProxyApi = globalThis.window !== undefined;
  const url = useProxyApi
    ? `/api/activities/${activityId}/schedules`
    : `${env.NEXT_PUBLIC_API_URL}/api/activities/${activityId}/schedules`;

  try {
    if (useProxyApi) {
      const session = await getSession();
      const response = await fetch(url, {
        method: "GET",
        credentials: "include",
        headers: session?.user?.token
          ? {
              Authorization: `Bearer ${session.user.token}`,
              "Content-Type": "application/json",
            }
          : undefined,
      });

      if (!response.ok) {
        throw new Error(`API error: ${response.status}`);
      }

      const responseData = (await response.json()) as
        | ApiResponse<BackendActivitySchedule[]>
        | BackendActivitySchedule[];

      // Extract the array from the response wrapper if needed
      if (
        responseData &&
        typeof responseData === "object" &&
        "data" in responseData
      ) {
        return Array.isArray(responseData.data)
          ? responseData.data.map(mapActivityScheduleResponse)
          : [];
      }
      return Array.isArray(responseData)
        ? responseData.map(mapActivityScheduleResponse)
        : [];
    } else {
      const response =
        await api.get<ApiResponse<BackendActivitySchedule[]>>(url);
      return Array.isArray(response.data.data)
        ? response.data.data.map(mapActivityScheduleResponse)
        : [];
    }
  } catch {
    return [];
  }
}

// Get a single schedule for an activity
export async function getActivitySchedule(
  activityId: string,
  scheduleId: string,
): Promise<ActivitySchedule | null> {
  const useProxyApi = globalThis.window !== undefined;
  const url = useProxyApi
    ? `/api/activities/${activityId}/schedules/${scheduleId}`
    : `${env.NEXT_PUBLIC_API_URL}/api/activities/${activityId}/schedules/${scheduleId}`;

  try {
    if (useProxyApi) {
      const session = await getSession();
      const response = await fetch(url, {
        method: "GET",
        credentials: "include",
        headers: session?.user?.token
          ? {
              Authorization: `Bearer ${session.user.token}`,
              "Content-Type": "application/json",
            }
          : undefined,
      });

      if (!response.ok) {
        throw new Error(`API error: ${response.status}`);
      }

      const responseData = (await response.json()) as
        | ApiResponse<BackendActivitySchedule>
        | BackendActivitySchedule;

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
    return null;
  }
}

// Get all available timeframes
export async function getTimeframes(): Promise<Timeframe[]> {
  const useProxyApi = globalThis.window !== undefined;
  const url = useProxyApi
    ? "/api/schedules/timeframes"
    : `${env.NEXT_PUBLIC_API_URL}/api/schedules/timeframes`;

  try {
    if (useProxyApi) {
      const session = await getSession();
      const response = await fetch(url, {
        method: "GET",
        credentials: "include",
        headers: session?.user?.token
          ? {
              Authorization: `Bearer ${session.user.token}`,
              "Content-Type": "application/json",
            }
          : undefined,
      });

      if (!response.ok) {
        throw new Error(`API error: ${response.status}`);
      }

      const responseData = (await response.json()) as unknown;

      // Handle different response structures
      if (
        responseData &&
        typeof responseData === "object" &&
        responseData !== null
      ) {
        // If it's a wrapped response with data property
        if ("data" in responseData && responseData.data) {
          if (Array.isArray(responseData.data)) {
            // Check if it's already frontend types or needs mapping
            if (
              responseData.data.length > 0 &&
              responseData.data[0] &&
              typeof responseData.data[0] === "object" &&
              responseData.data[0] !== null &&
              "id" in responseData.data[0] &&
              typeof (responseData.data[0] as { id: unknown }).id === "string"
            ) {
              return responseData.data as Timeframe[];
            }
            return (responseData.data as BackendTimeframe[]).map(
              mapTimeframeResponse,
            );
          }
          return [];
        }
        // If it's an array directly
        else if (Array.isArray(responseData)) {
          if (
            responseData.length > 0 &&
            responseData[0] &&
            typeof responseData[0] === "object" &&
            responseData[0] !== null &&
            "id" in responseData[0]
          ) {
            // Check if it's already frontend types or needs mapping
            if (
              "id" in responseData[0] &&
              typeof (responseData[0] as { id: unknown }).id === "string"
            ) {
              return responseData as Timeframe[];
            }
            return (responseData as BackendTimeframe[]).map(
              mapTimeframeResponse,
            );
          }
          return [];
        }
      }
      return [];
    } else {
      const response = await api.get<ApiResponse<BackendTimeframe[]>>(url);
      if (response?.data && Array.isArray(response.data.data)) {
        return response.data.data.map(mapTimeframeResponse);
      }
      return [];
    }
  } catch (error) {
    handleActivityApiError(error, "fetch timeframes");
  }
}

// Get available time slots
export async function getAvailableTimeSlots(
  activityId: string,
  date?: string,
): Promise<AvailableTimeSlot[]> {
  const useProxyApi = globalThis.window !== undefined;
  let url = useProxyApi
    ? `/api/activities/${activityId}/schedules/available`
    : `${env.NEXT_PUBLIC_API_URL}/api/activities/${activityId}/schedules/available`;

  // Add date parameter if provided
  if (date) {
    url += `?date=${encodeURIComponent(date)}`;
  }

  try {
    if (useProxyApi) {
      const session = await getSession();
      const response = await fetch(url, {
        method: "GET",
        credentials: "include",
        headers: session?.user?.token
          ? {
              Authorization: `Bearer ${session.user.token}`,
              "Content-Type": "application/json",
            }
          : undefined,
      });

      if (!response.ok) {
        throw new Error(`API error: ${response.status}`);
      }

      const responseData = (await response.json()) as
        | ApiResponse<AvailableTimeSlot[]>
        | AvailableTimeSlot[];

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
    return [];
  }
}

// Create a new schedule for an activity
export async function createActivitySchedule(
  activityId: string,
  scheduleData: Partial<ActivitySchedule>,
): Promise<ActivitySchedule> {
  const useProxyApi = globalThis.window !== undefined;
  const url = useProxyApi
    ? `/api/activities/${activityId}/schedules`
    : `${env.NEXT_PUBLIC_API_URL}/api/activities/${activityId}/schedules`;

  // Prepare backend data
  const backendData = prepareActivityScheduleForBackend(scheduleData);

  try {
    if (useProxyApi) {
      const session = await getSession();
      const response = await fetch(url, {
        method: "POST",
        credentials: "include",
        headers: {
          "Content-Type": "application/json",
          ...(session?.user?.token && {
            Authorization: `Bearer ${session.user.token}`,
          }),
        },
        body: JSON.stringify(backendData),
      });

      if (!response.ok) {
        throw new Error(`API error: ${response.status}`);
      }

      const responseData = (await response.json()) as
        | ApiResponse<BackendActivitySchedule>
        | BackendActivitySchedule;

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
  const useProxyApi = globalThis.window !== undefined;
  const url = useProxyApi
    ? `/api/activities/${activityId}/schedules/${scheduleId}`
    : `${env.NEXT_PUBLIC_API_URL}/api/activities/${activityId}/schedules/${scheduleId}`;

  // Prepare backend data
  const backendData = prepareActivityScheduleForBackend(scheduleData);

  try {
    if (useProxyApi) {
      const session = await getSession();
      const response = await fetch(url, {
        method: "PUT",
        credentials: "include",
        headers: {
          "Content-Type": "application/json",
          ...(session?.user?.token && {
            Authorization: `Bearer ${session.user.token}`,
          }),
        },
        body: JSON.stringify(backendData),
      });

      if (!response.ok) {
        throw new Error(`API error: ${response.status}`);
      }

      const responseData = (await response.json()) as
        | ApiResponse<BackendActivitySchedule>
        | BackendActivitySchedule;

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
    return null;
  }
}

// Delete a schedule for an activity
export async function deleteActivitySchedule(
  activityId: string,
  scheduleId: string,
): Promise<boolean> {
  const useProxyApi = globalThis.window !== undefined;
  const url = useProxyApi
    ? `/api/activities/${activityId}/schedules/${scheduleId}`
    : `${env.NEXT_PUBLIC_API_URL}/api/activities/${activityId}/schedules/${scheduleId}`;

  try {
    if (useProxyApi) {
      const session = await getSession();
      const response = await fetch(url, {
        method: "DELETE",
        credentials: "include",
        headers: session?.user?.token
          ? {
              Authorization: `Bearer ${session.user.token}`,
              "Content-Type": "application/json",
            }
          : undefined,
      });

      if (!response.ok) {
        throw new Error(`API error: ${response.status}`);
      }

      return true;
    } else {
      await api.delete(url);
      return true;
    }
  } catch {
    return false;
  }
}

// Get all supervisors assigned to an activity
export async function getActivitySupervisors(
  activityId: string,
): Promise<
  Array<{ id: string; staff_id: string; is_primary: boolean; name: string }>
> {
  const useProxyApi = globalThis.window !== undefined;
  const url = useProxyApi
    ? `/api/activities/${activityId}/supervisors`
    : `${env.NEXT_PUBLIC_API_URL}/api/activities/${activityId}/supervisors`;

  try {
    if (useProxyApi) {
      const session = await getSession();
      const response = await fetch(url, {
        method: "GET",
        credentials: "include",
        headers: session?.user?.token
          ? {
              Authorization: `Bearer ${session.user.token}`,
              "Content-Type": "application/json",
            }
          : undefined,
      });

      if (!response.ok) {
        throw new Error(`API error: ${response.status}`);
      }

      const responseData = (await response.json()) as
        | ApiResponse<BackendActivitySupervisor[]>
        | BackendActivitySupervisor[];

      // Extract the array from the response wrapper if needed
      if (
        responseData &&
        typeof responseData === "object" &&
        "data" in responseData
      ) {
        return Array.isArray(responseData.data)
          ? responseData.data.map((s) => ({
              id: String(s.id),
              staff_id: String(s.staff_id),
              is_primary: s.is_primary,
              name:
                s.first_name && s.last_name
                  ? `${s.first_name} ${s.last_name}`
                  : `Supervisor ${s.id}`,
            }))
          : [];
      }
      return Array.isArray(responseData)
        ? responseData.map((s) => ({
            id: String(s.id),
            staff_id: String(s.staff_id),
            is_primary: s.is_primary,
            name:
              s.first_name && s.last_name
                ? `${s.first_name} ${s.last_name}`
                : `Supervisor ${s.id}`,
          }))
        : [];
    } else {
      const response =
        await api.get<ApiResponse<BackendActivitySupervisor[]>>(url);
      return Array.isArray(response.data.data)
        ? response.data.data.map((s) => ({
            id: String(s.id),
            staff_id: String(s.staff_id),
            is_primary: s.is_primary,
            name:
              s.first_name && s.last_name
                ? `${s.first_name} ${s.last_name}`
                : `Supervisor ${s.id}`,
          }))
        : [];
    }
  } catch {
    return [];
  }
}

// Get available supervisors for an activity (not yet assigned)
export async function getAvailableSupervisors(
  activityId: string,
): Promise<Array<{ id: string; name: string }>> {
  const useProxyApi = globalThis.window !== undefined;
  const url = useProxyApi
    ? `/api/activities/${activityId}/supervisors/available`
    : `${env.NEXT_PUBLIC_API_URL}/api/activities/${activityId}/supervisors/available`;

  try {
    if (useProxyApi) {
      const session = await getSession();
      const response = await fetch(url, {
        method: "GET",
        credentials: "include",
        headers: session?.user?.token
          ? {
              Authorization: `Bearer ${session.user.token}`,
              "Content-Type": "application/json",
            }
          : undefined,
      });

      if (!response.ok) {
        throw new Error(`API error: ${response.status}`);
      }

      const responseData = (await response.json()) as
        | ApiResponse<BackendSupervisor[]>
        | BackendSupervisor[];

      // Extract the array from the response wrapper if needed
      if (
        responseData &&
        typeof responseData === "object" &&
        "data" in responseData
      ) {
        return Array.isArray(responseData.data)
          ? responseData.data.map(mapSupervisorResponse)
          : [];
      }
      return Array.isArray(responseData)
        ? responseData.map(mapSupervisorResponse)
        : [];
    } else {
      const response = await api.get<ApiResponse<BackendSupervisor[]>>(url);
      return Array.isArray(response.data.data)
        ? response.data.data.map(mapSupervisorResponse)
        : [];
    }
  } catch {
    return [];
  }
}

// Assign a supervisor to an activity
export async function assignSupervisor(
  activityId: string,
  supervisorData: { staff_id: string; is_primary?: boolean },
): Promise<boolean> {
  const useProxyApi = globalThis.window !== undefined;
  const url = useProxyApi
    ? `/api/activities/${activityId}/supervisors`
    : `${env.NEXT_PUBLIC_API_URL}/api/activities/${activityId}/supervisors`;

  // Convert staff_id to number for backend and set is_primary if defined
  const backendData = {
    staff_id: Number.parseInt(supervisorData.staff_id, 10),
    is_primary: supervisorData.is_primary,
  };

  try {
    if (useProxyApi) {
      const session = await getSession();
      const response = await fetch(url, {
        method: "POST",
        credentials: "include",
        headers: {
          "Content-Type": "application/json",
          ...(session?.user?.token && {
            Authorization: `Bearer ${session.user.token}`,
          }),
        },
        body: JSON.stringify(backendData),
      });

      if (!response.ok) {
        throw new Error(`API error: ${response.status}`);
      }

      return true;
    } else {
      await api.post(url, backendData);
      return true;
    }
  } catch {
    return false;
  }
}

// Update supervisor role (e.g., set/unset primary status)
export async function updateSupervisorRole(
  activityId: string,
  supervisorId: string,
  roleData: { is_primary: boolean },
): Promise<boolean> {
  const useProxyApi = globalThis.window !== undefined;
  const url = useProxyApi
    ? `/api/activities/${activityId}/supervisors/${supervisorId}`
    : `${env.NEXT_PUBLIC_API_URL}/api/activities/${activityId}/supervisors/${supervisorId}`;

  try {
    if (useProxyApi) {
      const session = await getSession();
      const response = await fetch(url, {
        method: "PUT",
        credentials: "include",
        headers: {
          "Content-Type": "application/json",
          ...(session?.user?.token && {
            Authorization: `Bearer ${session.user.token}`,
          }),
        },
        body: JSON.stringify(roleData),
      });

      if (!response.ok) {
        throw new Error(`API error: ${response.status}`);
      }

      return true;
    } else {
      await api.put(url, roleData);
      return true;
    }
  } catch {
    return false;
  }
}

// Remove a supervisor from an activity
export async function removeSupervisor(
  activityId: string,
  supervisorId: string,
): Promise<boolean> {
  const useProxyApi = globalThis.window !== undefined;
  const url = useProxyApi
    ? `/api/activities/${activityId}/supervisors/${supervisorId}`
    : `${env.NEXT_PUBLIC_API_URL}/api/activities/${activityId}/supervisors/${supervisorId}`;

  try {
    if (useProxyApi) {
      const session = await getSession();
      const response = await fetch(url, {
        method: "DELETE",
        credentials: "include",
        headers: session?.user?.token
          ? {
              Authorization: `Bearer ${session.user.token}`,
              "Content-Type": "application/json",
            }
          : undefined,
      });

      if (!response.ok) {
        throw new Error(`API error: ${response.status}`);
      }

      return true;
    } else {
      await api.delete(url);
      return true;
    }
  } catch {
    return false;
  }
}

// Get all students eligible for enrollment (not yet enrolled)
export async function getAvailableStudents(
  activityId: string,
  filters?: { search?: string; group_id?: string },
): Promise<Array<{ id: string; name: string; school_class: string }>> {
  const useProxyApi = globalThis.window !== undefined;
  let url = useProxyApi
    ? `/api/activities/${activityId}/students`
    : `${env.NEXT_PUBLIC_API_URL}/api/activities/${activityId}/students`;

  // Build query parameters - always include available=true
  const params = new URLSearchParams();
  params.append("available", "true");

  // Add additional filters if provided
  if (filters) {
    if (filters.search) params.append("search", filters.search);
    if (filters.group_id) params.append("group_id", filters.group_id);
  }

  const queryString = params.toString();
  if (queryString) {
    url += `?${queryString}`;
  }

  try {
    if (useProxyApi) {
      const session = await getSession();
      const response = await fetch(url, {
        method: "GET",
        credentials: "include",
        headers: session?.user?.token
          ? {
              Authorization: `Bearer ${session.user.token}`,
              "Content-Type": "application/json",
            }
          : undefined,
      });

      if (!response.ok) {
        throw new Error(`API error: ${response.status}`);
      }

      const responseData = (await response.json()) as
        | ApiResponse<Array<{ id: number; name: string; school_class: string }>>
        | Array<{ id: number; name: string; school_class: string }>;

      // Extract the array from the response wrapper if needed
      if (
        responseData &&
        typeof responseData === "object" &&
        "data" in responseData
      ) {
        return Array.isArray(responseData.data)
          ? responseData.data.map((s) => ({
              id: String(s.id),
              name: s.name,
              school_class: s.school_class,
            }))
          : [];
      }
      return Array.isArray(responseData)
        ? responseData.map((s) => ({
            id: String(s.id),
            name: s.name,
            school_class: s.school_class,
          }))
        : [];
    } else {
      const response =
        await api.get<
          ApiResponse<Array<{ id: number; name: string; school_class: string }>>
        >(url);
      return Array.isArray(response.data.data)
        ? response.data.data.map((s) => ({
            id: String(s.id),
            name: s.name,
            school_class: s.school_class,
          }))
        : [];
    }
  } catch {
    return [];
  }
}

// Get activities a student is enrolled in
export async function getStudentEnrollments(
  studentId: string,
): Promise<Activity[]> {
  const useProxyApi = globalThis.window !== undefined;
  const url = useProxyApi
    ? `/api/students/${studentId}/activities`
    : `${env.NEXT_PUBLIC_API_URL}/api/students/${studentId}/activities`;

  try {
    if (useProxyApi) {
      const session = await getSession();
      const response = await fetch(url, {
        method: "GET",
        credentials: "include",
        headers: session?.user?.token
          ? {
              Authorization: `Bearer ${session.user.token}`,
              "Content-Type": "application/json",
            }
          : undefined,
      });

      if (!response.ok) {
        throw new Error(`API error: ${response.status}`);
      }

      const responseData = (await response.json()) as
        | ApiResponse<BackendActivity[]>
        | BackendActivity[];

      // Extract the array from the response wrapper if needed
      if (
        responseData &&
        typeof responseData === "object" &&
        "data" in responseData
      ) {
        return Array.isArray(responseData.data)
          ? responseData.data.map(mapActivityResponse)
          : [];
      }
      return Array.isArray(responseData)
        ? responseData.map(mapActivityResponse)
        : [];
    } else {
      const response = await api.get<ApiResponse<BackendActivity[]>>(url);
      return Array.isArray(response.data.data)
        ? response.data.data.map(mapActivityResponse)
        : [];
    }
  } catch {
    return [];
  }
}

// Alias for getTimeframes to match modal expectations
export async function getAvailableTimeframes(): Promise<Timeframe[]> {
  return getTimeframes();
}

// Batch update student enrollments (add or remove multiple students at once)
export async function updateGroupEnrollments(
  activityId: string,
  data: { student_ids: string[] },
): Promise<boolean> {
  const useProxyApi = globalThis.window !== undefined;
  const url = useProxyApi
    ? `/api/activities/${activityId}/students`
    : `${env.NEXT_PUBLIC_API_URL}/api/activities/${activityId}/students`;

  // The API expects student_ids as an array of strings when using proxy API
  const requestData = useProxyApi
    ? data
    : {
        student_ids: data.student_ids.map((id) => Number.parseInt(id, 10)),
      };

  try {
    if (useProxyApi) {
      let session = await getSession();

      // Check if we have a valid session
      if (!session?.user?.token) {
        throw new Error(
          "No authentication token available. Please log in again.",
        );
      }

      let response = await fetch(url, {
        method: "PUT",
        credentials: "include",
        headers: {
          "Content-Type": "application/json",
          Authorization: `Bearer ${session.user.token}`,
        },
        body: JSON.stringify(requestData),
      });

      // Handle 401 by trying to refresh token
      if (response.status === 401) {
        console.log("Token expired, attempting to refresh...");
        const refreshSuccessful = await handleAuthFailure();

        if (refreshSuccessful) {
          // Get the new session with updated token
          session = await getSession();

          if (session?.user?.token) {
            // Retry the request with new token
            response = await fetch(url, {
              method: "PUT",
              credentials: "include",
              headers: {
                "Content-Type": "application/json",
                Authorization: `Bearer ${session.user.token}`,
              },
              body: JSON.stringify(requestData),
            });
          }
        }
      }

      if (!response.ok) {
        // Provide more specific error messages
        if (response.status === 401) {
          throw new Error("Authentication expired. Please log in again.");
        } else if (response.status === 403) {
          throw new Error("You don't have permission to modify enrollments.");
        } else {
          throw new Error(`API error: ${response.status}`);
        }
      }

      return true;
    } else {
      await api.put(url, requestData);
      return true;
    }
  } catch (error) {
    console.error("Error updating group enrollments:", error);
    throw error; // Re-throw to let caller handle it
  }
}
