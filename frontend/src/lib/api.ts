import axios from "axios";
import type { AxiosError, AxiosRequestConfig, AxiosResponse } from "axios";

/**
 * Extended request config with retry tracking properties
 */
interface RetryableRequestConfig extends AxiosRequestConfig {
  _retry?: boolean;
  _retryCount?: number;
}
import { getSession } from "next-auth/react";
import { env } from "~/env";
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
  mapSingleCombinedGroupResponse,
  prepareCombinedGroupForBackend,
  mapGroupsResponse,
  mapCombinedGroupsResponse,
} from "./group-helpers";

// Re-export for external consumers
export { mapGroupResponse, mapCombinedGroupResponse } from "./group-helpers";
import type {
  BackendGroup,
  BackendCombinedGroup,
  CombinedGroup as ImportedCombinedGroup,
  Group as ImportedGroup,
} from "./group-helpers";
import {
  mapSingleRoomResponse,
  prepareRoomForBackend,
  mapRoomsResponse,
} from "./room-helpers";

// Re-export for external consumers
export { mapRoomResponse } from "./room-helpers";
import type { BackendRoom } from "./room-helpers";
import { handleAuthFailure } from "./auth-api";

/**
 * Extended request config with retry tracking properties
 */
interface RetryableRequestConfig extends AxiosRequestConfig {
  _retry?: boolean;
  _retryCount?: number;
}

// Helper function to safely handle errors
function handleApiError(error: unknown, context: string): Error {
  console.error(`${context}:`, error);
  return new Error(
    `${context}: ${error instanceof Error ? error.message : String(error)}`,
  );
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

/**
 * Build query parameters for student API requests
 */
function buildStudentQueryParams(filters?: {
  search?: string;
  inHouse?: boolean;
  groupId?: string;
  page?: number;
  pageSize?: number;
}): URLSearchParams {
  const params = new URLSearchParams();
  if (filters?.search) params.append("search", filters.search);
  if (filters?.inHouse !== undefined)
    params.append("in_house", filters.inHouse.toString());
  if (filters?.groupId) params.append("group_id", filters.groupId);
  if (filters?.page) params.append("page", filters.page.toString());
  if (filters?.pageSize)
    params.append("page_size", filters.pageSize.toString());
  return params;
}

/**
 * Get new token from session (helper for fetchWithRetry)
 */
async function getNewTokenFromSession(): Promise<string | undefined> {
  const session = await getSession();
  return session?.user?.token;
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
}): URLSearchParams {
  const params = new URLSearchParams();
  if (filters?.search) params.append("search", filters.search);
  if (filters?.building) params.append("building", filters.building);
  if (filters?.floor !== undefined)
    params.append("floor", filters.floor.toString());
  if (filters?.category) params.append("category", filters.category);
  if (filters?.occupied !== undefined)
    params.append("occupied", filters.occupied.toString());
  return params;
}

/**
 * Parse rooms response, handling null, non-array, and valid array formats.
 * Returns empty array for invalid formats with warning.
 */
function parseRoomsResponse(responseData: unknown): BackendRoom[] {
  if (!responseData || !Array.isArray(responseData)) {
    console.warn(
      "API returned invalid response format for rooms:",
      responseData,
    );
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

  console.warn("Unexpected room response format:", responseData);
  throw new Error("Unexpected room response format");
}

/**
 * Validate room data before creation.
 * Throws descriptive error if validation fails.
 */
function validateRoomForCreation(room: {
  name?: string;
  capacity?: number;
  category?: string;
}): void {
  if (!room.name) {
    throw new Error("Missing required field: name");
  }
  if (room.capacity === undefined || room.capacity <= 0) {
    throw new Error("Missing required field: capacity must be greater than 0");
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

// Create an Axios instance
const api = axios.create({
  baseURL: env.NEXT_PUBLIC_API_URL, // Client-safe environment variable pointing to the backend server
  headers: {
    "Content-Type": "application/json",
  },
  // Important: Include credentials with every request to ensure cookies are sent
  withCredentials: true,
});

// Add a request interceptor to include the auth token
// Note: This interceptor only runs in client-side code
api.interceptors.request.use(
  async (config) => {
    // Only try to get session if we're in the browser
    if (globalThis.window !== undefined) {
      const session = await getSession();

      // If there's a token, add it to the headers
      if (session?.user?.token) {
        config.headers.Authorization = `Bearer ${session.user.token}`;
      }
    }

    return config;
  },
  (error: Error) => {
    return Promise.reject(error);
  },
);

// Track ongoing refresh attempts to prevent multiple simultaneous refreshes
let isRefreshing = false;
let refreshSubscribers: ((token: string) => void)[] = [];

// Subscribe to token refresh completion
const subscribeTokenRefresh = (callback: (token: string) => void) => {
  refreshSubscribers.push(callback);
};

// Notify all subscribers when refresh is complete
const onTokenRefreshed = (token: string) => {
  refreshSubscribers.forEach((callback) => callback(token));
  refreshSubscribers = [];
};

// Helper: Redirect to login page (browser only)
function redirectToLogin(): void {
  if (globalThis.window !== undefined) {
    globalThis.window.location.href = "/";
  }
}

// Helper: Set authorization header (handles both methods)
function setAuthorizationHeader(
  headers: AxiosRequestConfig["headers"],
  token: string,
): void {
  if (!headers) return;

  const headersObj = headers as Record<string, unknown> & {
    set?: (key: string, value: string) => void;
  };

  if (typeof headersObj.set === "function") {
    headersObj.set("Authorization", `Bearer ${token}`);
  } else {
    headersObj.Authorization = `Bearer ${token}`;
  }
}

// Helper: Queue request for token refresh completion
function queueRequestForRefresh(
  originalRequest: AxiosRequestConfig,
  callerId: string,
): Promise<AxiosResponse> {
  console.log(
    `[${callerId}] Token refresh already in progress, queueing request`,
  );

  return new Promise((resolve) => {
    subscribeTokenRefresh((token: string) => {
      // Ensure headers object exists to prevent promise from hanging
      originalRequest.headers ??= {};
      originalRequest.headers.Authorization = `Bearer ${token}`;
      resolve(api(originalRequest));
    });
  });
}

// Helper: Attempt server-side token refresh
async function attemptServerSideRefresh(
  originalRequest: AxiosRequestConfig,
): Promise<AxiosResponse | null> {
  console.log("Server-side context detected, attempting token refresh");

  try {
    const { refreshSessionTokensOnServer } =
      await import("~/server/auth/token-refresh");
    const refreshed = await refreshSessionTokensOnServer();

    if (!refreshed?.accessToken) {
      console.error(
        "Server-side token refresh failed or returned no access token",
      );
      return null;
    }

    console.log(
      "Server-side token refresh successful, retrying original request",
    );
    originalRequest.headers ??= {};
    setAuthorizationHeader(originalRequest.headers, refreshed.accessToken);
    onTokenRefreshed(refreshed.accessToken);
    return api(originalRequest);
  } catch (serverRefreshError) {
    console.error("Error refreshing token on server", serverRefreshError);
    return null;
  }
}

// Helper: Attempt client-side token refresh
async function attemptClientSideRefresh(
  originalRequest: AxiosRequestConfig,
): Promise<AxiosResponse | null> {
  const refreshSuccessful = await handleAuthFailure();

  if (!refreshSuccessful || !originalRequest.headers) {
    return null;
  }

  const session = await getSession();

  if (!session?.user?.token) {
    return null;
  }

  console.log("Token refresh successful, retrying original request");
  onTokenRefreshed(session.user.token);
  originalRequest.headers.Authorization = `Bearer ${session.user.token}`;
  return api(originalRequest);
}

// Add a response interceptor to handle common errors
api.interceptors.response.use(
  (response: AxiosResponse) => response,
  async (error: AxiosError) => {
    const originalRequest = error.config as RetryableRequestConfig | undefined;

    // Only handle 401 errors that haven't been retried
    if (
      error.response?.status !== 401 ||
      !originalRequest ||
      originalRequest._retry
    ) {
      throw error;
    }

    const callerId = `axios-interceptor-${Date.now()}-${crypto.randomUUID().slice(0, 8)}`;
    console.log(`\n[${callerId}] Axios interceptor: 401 error detected`);
    originalRequest._retry = true;
    originalRequest._retryCount = (originalRequest._retryCount ?? 0) + 1;

    // Limit retry attempts
    if (originalRequest._retryCount > 3) {
      console.error("Max retry attempts reached, giving up");
      redirectToLogin();
      throw error;
    }

    // Queue request if refresh is already in progress
    if (isRefreshing) {
      return queueRequestForRefresh(originalRequest, callerId);
    }

    console.log("Received 401 error, attempting to refresh token");
    isRefreshing = true;

    try {
      // Server-side refresh
      if (globalThis.window === undefined) {
        const result = await attemptServerSideRefresh(originalRequest);
        if (result) return result;
        throw error;
      }

      // Client-side refresh
      const result = await attemptClientSideRefresh(originalRequest);
      if (result) return result;

      console.error("Token refresh failed, redirecting to login");
      redirectToLogin();
    } finally {
      isRefreshing = false;
    }

    throw error;
  },
);

// Re-export types for external usage
export type { Student } from "./student-helpers";
export type Group = ImportedGroup;
export type CombinedGroup = ImportedCombinedGroup;

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
export const studentService = {
  // Get all students
  getStudents: async (filters?: {
    search?: string;
    inHouse?: boolean;
    groupId?: string;
    page?: number;
    pageSize?: number;
  }): Promise<StudentsResult> => {
    const params = buildStudentQueryParams(filters);
    const useProxyApi = globalThis.window !== undefined;
    const baseUrl = useProxyApi
      ? "/api/students"
      : `${env.NEXT_PUBLIC_API_URL}/api/students`;
    const queryString = params.toString();
    const url = queryString ? `${baseUrl}?${queryString}` : baseUrl;

    try {
      if (useProxyApi) {
        // Browser environment: use fetchWithRetry for automatic 401 handling
        const session = await getSession();
        const { data } = await fetchWithRetry<unknown>(
          url,
          session?.user?.token,
          {
            onAuthFailure: handleAuthFailure,
            getNewToken: getNewTokenFromSession,
          },
        );

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

  // Get a specific student by ID
  getStudent: async (id: string): Promise<Student> => {
    // Use the nextjs api route which handles auth token properly
    const useProxyApi = globalThis.window !== undefined;
    const url = useProxyApi
      ? `/api/students/${id}`
      : `${env.NEXT_PUBLIC_API_URL}/api/students/${id}`;

    try {
      if (useProxyApi) {
        // Browser environment: use fetchWithRetry for automatic 401 handling
        // Route handler already maps response, so applyMapping=false
        const session = await getSession();
        const { data } = await fetchWithRetry<unknown>(
          url,
          session?.user?.token,
          {
            onAuthFailure: handleAuthFailure,
            getNewToken: getNewTokenFromSession,
          },
        );

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
    const url = useProxyApi
      ? `/api/students`
      : `${env.NEXT_PUBLIC_API_URL}/api/students`;

    try {
      if (useProxyApi) {
        // Browser environment: use fetch with our Next.js API route
        const session = await getSession();
        const response = await fetch(url, {
          method: "POST",
          credentials: "include",
          headers: session?.user?.token
            ? {
                Authorization: `Bearer ${session.user.token}`,
                "Content-Type": "application/json",
              }
            : undefined,
          body: JSON.stringify(student),
        });

        if (!response.ok) {
          const errorText = await response.text();
          console.error(`API error: ${response.status}`, errorText);
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
    student: Partial<Student>,
  ): Promise<Student> => {
    const useProxyApi = globalThis.window !== undefined;
    const url = useProxyApi
      ? `/api/students/${id}`
      : `${env.NEXT_PUBLIC_API_URL}/api/students/${id}`;

    try {
      if (useProxyApi) {
        // Browser environment: use fetch with our Next.js API route
        // Send frontend format data - the API route will handle transformation
        const session = await getSession();
        const response = await fetch(url, {
          method: "PUT",
          credentials: "include",
          headers: session?.user?.token
            ? {
                Authorization: `Bearer ${session.user.token}`,
                "Content-Type": "application/json",
              }
            : undefined,
          body: JSON.stringify(student),
        });

        if (!response.ok) {
          const errorText = await response.text();
          console.error(`API error: ${response.status}`, errorText);

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
        return mappedResponse;
      } else {
        // Server-side: use axios with the API URL directly
        // For server-side, we need to transform the data since we're calling the backend directly
        const backendUpdates = prepareStudentForBackend(student);
        const response = await api.put(url, backendUpdates);
        const mappedResponse = mapSingleStudentResponse({
          data: response.data as unknown as BackendStudent,
        });
        return mappedResponse;
      }
    } catch (error) {
      throw handleApiError(error, `Error updating student ${id}`);
    }
  },

  // Delete a student
  deleteStudent: async (id: string): Promise<void> => {
    const useProxyApi = globalThis.window !== undefined;
    const url = useProxyApi
      ? `/api/students/${id}`
      : `${env.NEXT_PUBLIC_API_URL}/api/students/${id}`;

    try {
      if (useProxyApi) {
        // Browser environment: use fetch with our Next.js API route
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
          const errorText = await response.text();
          console.error(`API error: ${response.status}`, errorText);
          throw new Error(`API error: ${response.status}`);
        }

        return;
      } else {
        // Server-side: use axios with the API URL directly
        await api.delete(url);
        return;
      }
    } catch (error) {
      throw handleApiError(error, `Error deleting student ${id}`);
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
    const baseUrl = useProxyApi
      ? "/api/groups"
      : `${env.NEXT_PUBLIC_API_URL}/api/groups`;
    const url = queryString ? `${baseUrl}?${queryString}` : baseUrl;

    try {
      if (useProxyApi) {
        // Browser environment: use fetchWithRetry for automatic 401 handling
        const session = await getSession();
        const { response, data } = await fetchWithRetry<unknown>(
          url,
          session?.user?.token,
          {
            onAuthFailure: handleAuthFailure,
            getNewToken: getNewTokenFromSession,
          },
        );

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
      console.error("Error fetching groups:", error);
      throw error;
    }
  },

  // Get a specific group by ID
  getGroup: async (id: string): Promise<Group> => {
    const useProxyApi = globalThis.window !== undefined;
    const url = useProxyApi
      ? `/api/groups/${id}`
      : `${env.NEXT_PUBLIC_API_URL}/api/groups/${id}`;

    try {
      if (useProxyApi) {
        // Browser environment: use fetchWithRetry for automatic 401 handling
        const session = await getSession();
        const { response, data } = await fetchWithRetry<unknown>(
          url,
          session?.user?.token,
          {
            onAuthFailure: handleAuthFailure,
            getNewToken: getNewTokenFromSession,
          },
        );

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
      console.error(`Error fetching group ${id}:`, error);
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
    const url = useProxyApi
      ? `/api/groups`
      : `${env.NEXT_PUBLIC_API_URL}/api/groups`;

    try {
      if (useProxyApi) {
        // Browser environment: use fetch with our Next.js API route
        const session = await getSession();
        const response = await fetch(url, {
          method: "POST",
          credentials: "include",
          headers: session?.user?.token
            ? {
                Authorization: `Bearer ${session.user.token}`,
                "Content-Type": "application/json",
              }
            : undefined,
          body: JSON.stringify(backendGroup),
        });

        if (!response.ok) {
          const errorText = await response.text();
          console.error(`API error: ${response.status}`, errorText);
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
      console.error(`Error creating group:`, error);
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
      : `${env.NEXT_PUBLIC_API_URL}/api/groups/${id}`;

    try {
      if (useProxyApi) {
        // Browser environment: use fetch with our Next.js API route
        const session = await getSession();
        const response = await fetch(url, {
          method: "PUT",
          credentials: "include",
          headers: session?.user?.token
            ? {
                Authorization: `Bearer ${session.user.token}`,
                "Content-Type": "application/json",
              }
            : undefined,
          body: JSON.stringify(backendUpdates),
        });

        if (!response.ok) {
          const errorText = await response.text();
          console.error(`API error: ${response.status}`, errorText);

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
      console.error(`Error updating group ${id}:`, error);
      throw error;
    }
  },

  // Delete a group
  deleteGroup: async (id: string): Promise<void> => {
    const useProxyApi = globalThis.window !== undefined;
    const url = useProxyApi
      ? `/api/groups/${id}`
      : `${env.NEXT_PUBLIC_API_URL}/api/groups/${id}`;

    const knownErrorPatterns = ["cannot delete group with students"];

    try {
      if (useProxyApi) {
        // Browser environment: use fetch with our Next.js API route
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
          const errorText = await response.text();
          console.error(`API error: ${response.status}`, errorText);
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
          throw new Error(detailedError);
        }
        throw axiosError;
      }
    } catch (error) {
      console.error(`Error deleting group ${id}:`, error);
      throw error;
    }
  },

  // Get students in a group
  getGroupStudents: async (id: string): Promise<Student[]> => {
    const useProxyApi = globalThis.window !== undefined;
    const url = useProxyApi
      ? `/api/groups/${id}/students`
      : `${env.NEXT_PUBLIC_API_URL}/api/groups/${id}/students`;

    try {
      if (useProxyApi) {
        // Browser environment: use fetch with our Next.js API route
        const session = await getSession();
        const response = await fetch(url, {
          credentials: "include",
          headers: session?.user?.token
            ? {
                Authorization: `Bearer ${session.user.token}`,
                "Content-Type": "application/json",
              }
            : undefined,
        });

        if (!response.ok) {
          const errorText = await response.text();
          console.error(`API error: ${response.status}`, errorText);
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
      : `${env.NEXT_PUBLIC_API_URL}/api/groups/${groupId}/supervisors`;

    try {
      if (useProxyApi) {
        // Browser environment: use fetch with our Next.js API route
        const session = await getSession();
        const response = await fetch(url, {
          method: "POST",
          credentials: "include",
          headers: session?.user?.token
            ? {
                Authorization: `Bearer ${session.user.token}`,
                "Content-Type": "application/json",
              }
            : undefined,
          body: JSON.stringify({
            supervisor_id: Number.parseInt(supervisorId, 10),
          }),
        });

        if (!response.ok) {
          const errorText = await response.text();
          console.error(`API error: ${response.status}`, errorText);
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
      console.error(
        `Error adding supervisor ${supervisorId} to group ${groupId}:`,
        error,
      );
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
      : `${env.NEXT_PUBLIC_API_URL}/api/groups/${groupId}/supervisors/${supervisorId}`;

    try {
      if (useProxyApi) {
        // Browser environment: use fetch with our Next.js API route
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
          const errorText = await response.text();
          console.error(`API error: ${response.status}`, errorText);
          throw new Error(`API error: ${response.status}`);
        }

        return;
      } else {
        // Server-side: use axios with the API URL directly
        await api.delete(url);
        return;
      }
    } catch (error) {
      console.error(
        `Error removing supervisor ${supervisorId} from group ${groupId}:`,
        error,
      );
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
      : `${env.NEXT_PUBLIC_API_URL}/api/groups/${groupId}/representative`;

    try {
      if (useProxyApi) {
        // Browser environment: use fetch with our Next.js API route
        const session = await getSession();
        const response = await fetch(url, {
          method: "PUT",
          credentials: "include",
          headers: session?.user?.token
            ? {
                Authorization: `Bearer ${session.user.token}`,
                "Content-Type": "application/json",
              }
            : undefined,
          body: JSON.stringify({
            representative_id: Number.parseInt(representativeId, 10),
          }),
        });

        if (!response.ok) {
          const errorText = await response.text();
          console.error(`API error: ${response.status}`, errorText);
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
      console.error(
        `Error setting representative ${representativeId} for group ${groupId}:`,
        error,
      );
      throw error;
    }
  },
};

// Combined Group service for API operations
export const combinedGroupService = {
  // Get all combined groups
  getCombinedGroups: async (): Promise<CombinedGroup[]> => {
    const useProxyApi = globalThis.window !== undefined;
    const url = useProxyApi
      ? "/api/groups/combined"
      : `${env.NEXT_PUBLIC_API_URL}/api/groups/combined`;

    try {
      if (useProxyApi) {
        // Browser environment: use fetch with our Next.js API route
        const session = await getSession();
        const response = await fetch(url, {
          credentials: "include",
          headers: session?.user?.token
            ? {
                Authorization: `Bearer ${session.user.token}`,
                "Content-Type": "application/json",
              }
            : undefined,
        });

        if (!response.ok) {
          const errorText = await response.text();
          console.error(`API error: ${response.status}`, errorText);
          throw new Error(`API error: ${response.status}`);
        }

        const responseData = (await response.json()) as BackendCombinedGroup[];
        return mapCombinedGroupsResponse(responseData);
      } else {
        // Server-side: use axios with the API URL directly
        const response = await api.get(url);
        return mapCombinedGroupsResponse(
          response.data as BackendCombinedGroup[],
        );
      }
    } catch (error) {
      console.error("Error fetching combined groups:", error);
      throw error;
    }
  },

  // Get a specific combined group by ID
  getCombinedGroup: async (id: string): Promise<CombinedGroup> => {
    const useProxyApi = globalThis.window !== undefined;
    const url = useProxyApi
      ? `/api/groups/combined/${id}`
      : `${env.NEXT_PUBLIC_API_URL}/api/groups/combined/${id}`;

    try {
      if (useProxyApi) {
        // Browser environment: use fetch with our Next.js API route
        const session = await getSession();
        const response = await fetch(url, {
          credentials: "include",
          headers: session?.user?.token
            ? {
                Authorization: `Bearer ${session.user.token}`,
                "Content-Type": "application/json",
              }
            : undefined,
        });

        if (!response.ok) {
          const errorText = await response.text();
          console.error(`API error: ${response.status}`, errorText);
          throw new Error(`API error: ${response.status}`);
        }

        const responseData = (await response.json()) as BackendCombinedGroup;
        return mapSingleCombinedGroupResponse({ data: responseData });
      } else {
        // Server-side: use axios with the API URL directly
        const response = await api.get(url);
        return mapSingleCombinedGroupResponse({
          data: response.data as BackendCombinedGroup,
        });
      }
    } catch (error) {
      console.error(`Error fetching combined group ${id}:`, error);
      throw error;
    }
  },

  // Create a new combined group
  createCombinedGroup: async (
    combinedGroup: Omit<CombinedGroup, "id">,
  ): Promise<CombinedGroup> => {
    // Transform from frontend model to backend model
    const backendCombinedGroup = prepareCombinedGroupForBackend(combinedGroup);

    // Basic validation for combined group creation
    if (!backendCombinedGroup.name) {
      throw new Error("Missing required field: name");
    }
    if (!backendCombinedGroup.access_policy) {
      throw new Error("Missing required field: access_policy");
    }

    const useProxyApi = globalThis.window !== undefined;
    const url = useProxyApi
      ? `/api/groups/combined`
      : `${env.NEXT_PUBLIC_API_URL}/api/groups/combined`;

    try {
      if (useProxyApi) {
        // Browser environment: use fetch with our Next.js API route
        const session = await getSession();
        const response = await fetch(url, {
          method: "POST",
          credentials: "include",
          headers: session?.user?.token
            ? {
                Authorization: `Bearer ${session.user.token}`,
                "Content-Type": "application/json",
              }
            : undefined,
          body: JSON.stringify(backendCombinedGroup),
        });

        if (!response.ok) {
          const errorText = await response.text();
          console.error(`API error: ${response.status}`, errorText);
          throw new Error(`API error: ${response.status}`);
        }

        const responseData = (await response.json()) as BackendCombinedGroup;
        return mapSingleCombinedGroupResponse({ data: responseData });
      } else {
        // Server-side: use axios with the API URL directly
        const response = await api.post(url, backendCombinedGroup);
        return mapSingleCombinedGroupResponse({
          data: response.data as BackendCombinedGroup,
        });
      }
    } catch (error) {
      console.error(`Error creating combined group:`, error);
      throw error;
    }
  },

  // Update a combined group
  updateCombinedGroup: async (
    id: string,
    combinedGroup: Partial<CombinedGroup>,
  ): Promise<CombinedGroup> => {
    // Transform from frontend model to backend model updates
    const backendUpdates = prepareCombinedGroupForBackend(combinedGroup);

    const useProxyApi = globalThis.window !== undefined;
    const url = useProxyApi
      ? `/api/groups/combined/${id}`
      : `${env.NEXT_PUBLIC_API_URL}/api/groups/combined/${id}`;

    try {
      if (useProxyApi) {
        // Browser environment: use fetch with our Next.js API route
        const session = await getSession();
        const response = await fetch(url, {
          method: "PUT",
          credentials: "include",
          headers: session?.user?.token
            ? {
                Authorization: `Bearer ${session.user.token}`,
                "Content-Type": "application/json",
              }
            : undefined,
          body: JSON.stringify(backendUpdates),
        });

        if (!response.ok) {
          const errorText = await response.text();
          console.error(`API error: ${response.status}`, errorText);
          throw new Error(`API error: ${response.status}`);
        }

        const responseData = (await response.json()) as BackendCombinedGroup;
        return mapSingleCombinedGroupResponse({ data: responseData });
      } else {
        // Server-side: use axios with the API URL directly
        const response = await api.put(url, backendUpdates);
        return mapSingleCombinedGroupResponse({
          data: response.data as BackendCombinedGroup,
        });
      }
    } catch (error) {
      console.error(`Error updating combined group ${id}:`, error);
      throw error;
    }
  },

  // Delete a combined group
  deleteCombinedGroup: async (id: string): Promise<void> => {
    const useProxyApi = globalThis.window !== undefined;
    const url = useProxyApi
      ? `/api/groups/combined/${id}`
      : `${env.NEXT_PUBLIC_API_URL}/api/groups/combined/${id}`;

    try {
      if (useProxyApi) {
        // Browser environment: use fetch with our Next.js API route
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
          const errorText = await response.text();
          console.error(`API error: ${response.status}`, errorText);
          throw new Error(`API error: ${response.status}`);
        }

        return;
      } else {
        // Server-side: use axios with the API URL directly
        await api.delete(url);
        return;
      }
    } catch (error) {
      console.error(`Error deleting combined group ${id}:`, error);
      throw error;
    }
  },

  // Add a group to a combined group
  addGroupToCombined: async (
    combinedGroupId: string,
    groupId: string,
  ): Promise<void> => {
    const useProxyApi = globalThis.window !== undefined;
    const url = useProxyApi
      ? `/api/groups/combined/${combinedGroupId}/groups`
      : `${env.NEXT_PUBLIC_API_URL}/api/groups/combined/${combinedGroupId}/groups`;

    try {
      if (useProxyApi) {
        // Browser environment: use fetch with our Next.js API route
        const session = await getSession();
        const response = await fetch(url, {
          method: "POST",
          credentials: "include",
          headers: session?.user?.token
            ? {
                Authorization: `Bearer ${session.user.token}`,
                "Content-Type": "application/json",
              }
            : undefined,
          body: JSON.stringify({ group_id: Number.parseInt(groupId, 10) }),
        });

        if (!response.ok) {
          const errorText = await response.text();
          console.error(`API error: ${response.status}`, errorText);
          throw new Error(`API error: ${response.status}`);
        }

        return;
      } else {
        // Server-side: use axios with the API URL directly
        await api.post(url, { group_id: Number.parseInt(groupId, 10) });
        return;
      }
    } catch (error) {
      console.error(
        `Error adding group ${groupId} to combined group ${combinedGroupId}:`,
        error,
      );
      throw error;
    }
  },

  // Remove a group from a combined group
  removeGroupFromCombined: async (
    combinedGroupId: string,
    groupId: string,
  ): Promise<void> => {
    const useProxyApi = globalThis.window !== undefined;
    const url = useProxyApi
      ? `/api/groups/combined/${combinedGroupId}/groups/${groupId}`
      : `${env.NEXT_PUBLIC_API_URL}/api/groups/combined/${combinedGroupId}/groups/${groupId}`;

    try {
      if (useProxyApi) {
        // Browser environment: use fetch with our Next.js API route
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
          const errorText = await response.text();
          console.error(`API error: ${response.status}`, errorText);
          throw new Error(`API error: ${response.status}`);
        }

        return;
      } else {
        // Server-side: use axios with the API URL directly
        await api.delete(url);
        return;
      }
    } catch (error) {
      console.error(
        `Error removing group ${groupId} from combined group ${combinedGroupId}:`,
        error,
      );
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
  }): Promise<Room[]> => {
    const params = buildRoomQueryParams(filters);
    const queryString = params.toString();

    const useProxyApi = globalThis.window !== undefined;
    const baseUrl = useProxyApi
      ? "/api/rooms"
      : `${env.NEXT_PUBLIC_API_URL}/api/rooms`;
    const url = queryString ? `${baseUrl}?${queryString}` : baseUrl;

    try {
      if (useProxyApi) {
        // Browser environment: use fetchWithRetry for automatic 401 handling
        const session = await getSession();
        const { data } = await fetchWithRetry<unknown>(
          url,
          session?.user?.token,
          {
            onAuthFailure: handleAuthFailure,
            getNewToken: getNewTokenFromSession,
          },
        );

        const rooms = parseRoomsResponse(data);
        return mapRoomsResponse(rooms);
      }

      // Server-side: use axios with the API URL directly
      const response = await api.get(url, { params });
      const rooms = parseRoomsResponse(response.data);
      return mapRoomsResponse(rooms);
    } catch (error) {
      console.error("Error fetching rooms:", error);
      throw error;
    }
  },

  // Get a specific room by ID
  getRoom: async (id: string): Promise<Room> => {
    const useProxyApi = globalThis.window !== undefined;
    const url = useProxyApi
      ? `/api/rooms/${id}`
      : `${env.NEXT_PUBLIC_API_URL}/api/rooms/${id}`;

    try {
      if (useProxyApi) {
        // Browser environment: use fetchWithRetry for automatic 401 handling
        const session = await getSession();
        const { response, data } = await fetchWithRetry<unknown>(
          url,
          session?.user?.token,
          {
            onAuthFailure: handleAuthFailure,
            getNewToken: getNewTokenFromSession,
          },
        );

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
      console.error(`Error fetching room ${id}:`, error);
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
    const url = useProxyApi
      ? `/api/rooms`
      : `${env.NEXT_PUBLIC_API_URL}/api/rooms`;

    try {
      if (useProxyApi) {
        const session = await getSession();
        const response = await fetch(url, {
          method: "POST",
          credentials: "include",
          headers: session?.user?.token
            ? {
                Authorization: `Bearer ${session.user.token}`,
                "Content-Type": "application/json",
              }
            : undefined,
          body: JSON.stringify(backendRoom),
        });

        if (!response.ok) {
          const errorText = await response.text();
          console.error(`API error: ${response.status}`, errorText);
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
      console.error(`Error creating room:`, error);
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
      : `${env.NEXT_PUBLIC_API_URL}/api/rooms/${id}`;

    try {
      if (useProxyApi) {
        // Browser environment: use fetch with our Next.js API route
        const session = await getSession();
        const response = await fetch(url, {
          method: "PUT",
          credentials: "include",
          headers: session?.user?.token
            ? {
                Authorization: `Bearer ${session.user.token}`,
                "Content-Type": "application/json",
              }
            : undefined,
          body: JSON.stringify(backendUpdates),
        });

        if (!response.ok) {
          const errorText = await response.text();
          console.error(`API error: ${response.status}`, errorText);

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
      console.error(`Error updating room ${id}:`, error);
      throw error;
    }
  },

  // Delete a room
  deleteRoom: async (id: string): Promise<void> => {
    const useProxyApi = globalThis.window !== undefined;
    const url = useProxyApi
      ? `/api/rooms/${id}`
      : `${env.NEXT_PUBLIC_API_URL}/api/rooms/${id}`;

    try {
      if (useProxyApi) {
        // Browser environment: use fetch with our Next.js API route
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
          const errorText = await response.text();
          console.error(`API error: ${response.status}`, errorText);
          throw new Error(`API error: ${response.status}`);
        }

        return;
      } else {
        // Server-side: use axios with the API URL directly
        await api.delete(url);
        return;
      }
    } catch (error) {
      console.error(`Error deleting room ${id}:`, error);
      throw error;
    }
  },

  // Get rooms grouped by category
  getRoomsByCategory: async (): Promise<Record<string, Room[]>> => {
    const useProxyApi = globalThis.window !== undefined;
    const url = useProxyApi
      ? "/api/rooms/by-category"
      : `${env.NEXT_PUBLIC_API_URL}/api/rooms/by-category`;

    try {
      if (useProxyApi) {
        // Browser environment: use fetch with our Next.js API route
        const session = await getSession();
        const response = await fetch(url, {
          credentials: "include",
          headers: session?.user?.token
            ? {
                Authorization: `Bearer ${session.user.token}`,
                "Content-Type": "application/json",
              }
            : undefined,
        });

        if (!response.ok) {
          const errorText = await response.text();
          console.error(`API error: ${response.status}`, errorText);
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
      console.error("Error fetching rooms by category:", error);
      throw error;
    }
  },
};

export default api;
